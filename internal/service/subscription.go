package service

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"pixia-airboard/internal/model"
)

type SubscriptionFormat string

const (
	FormatV2           SubscriptionFormat = "v2"
	FormatRaw          SubscriptionFormat = "raw"
	FormatClash        SubscriptionFormat = "clash"
	FormatShadowrocket SubscriptionFormat = "shadowrocket"
)

func BuildSubscriptionURL(baseURL, suffix string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return baseURL + "/sub/" + url.PathEscape(strings.Trim(suffix, "/"))
}

func BuildSubscriptionVariants(baseURL, suffix string) map[string]string {
	base := BuildSubscriptionURL(baseURL, suffix)
	return map[string]string{
		"default":      base,
		"v2":           base + "?target=v2",
		"clash":        base + "?target=clash",
		"shadowrocket": base + "?target=shadowrocket",
	}
}

func DetectSubscriptionFormat(target, flag, userAgent string) SubscriptionFormat {
	value := strings.ToLower(strings.TrimSpace(firstNonEmpty(target, flag, userAgent)))
	switch {
	case strings.Contains(value, "clash"), strings.Contains(value, "meta"), strings.Contains(value, "stash"):
		return FormatClash
	case strings.Contains(value, "shadowrocket"), strings.Contains(value, "surge"), strings.Contains(value, "rocket"):
		return FormatShadowrocket
	case strings.Contains(value, "raw"), strings.Contains(value, "plain"):
		return FormatRaw
	case strings.Contains(value, "v2"), strings.Contains(value, "v2ray"), strings.Contains(value, "sing-box"), strings.Contains(value, "nekobox"), strings.Contains(value, "nekoray"):
		return FormatV2
	default:
		return FormatV2
	}
}

func VisibleServersForUser(user model.User, servers []model.Server) []model.Server {
	var visible []model.Server
	for _, server := range servers {
		if !server.Show {
			continue
		}
		if len(server.PlanIDs) == 0 || containsInt64(server.PlanIDs, user.PlanID) {
			server.CacheKey = fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%d:%d:%s:%s", server.ID, server.UpdatedAt, server.Type, server.Name))))
			visible = append(visible, server)
		}
	}
	sort.SliceStable(visible, func(i, j int) bool {
		if visible[i].Sort == visible[j].Sort {
			return visible[i].ID < visible[j].ID
		}
		return visible[i].Sort < visible[j].Sort
	})
	return visible
}

func RenderSubscription(user model.User, servers []model.Server, format SubscriptionFormat) (string, string) {
	switch format {
	case FormatClash:
		return ClashSubscription(user, servers), "text/yaml; charset=utf-8"
	case FormatShadowrocket:
		return RawSubscription(user, servers), "text/plain; charset=utf-8"
	case FormatRaw:
		return RawSubscription(user, servers), "text/plain; charset=utf-8"
	default:
		return base64.StdEncoding.EncodeToString([]byte(RawSubscription(user, servers))), "text/plain; charset=utf-8"
	}
}

func RawSubscription(user model.User, servers []model.Server) string {
	var lines []string
	for _, server := range servers {
		switch server.Type {
		case "vmess":
			lines = append(lines, buildVmess(user.UUID, server))
		case "vless":
			lines = append(lines, buildVless(user.UUID, server))
		case "trojan":
			password := server.Password
			if password == "" {
				password = user.UUID
			}
			lines = append(lines, buildTrojan(password, server))
		case "shadowsocks":
			lines = append(lines, buildShadowsocks(pick(server.Password, user.UUID), server))
		case "hysteria", "hysteria2":
			password := server.Password
			if password == "" {
				password = user.UUID
			}
			lines = append(lines, buildHysteria(password, server))
		}
	}
	return strings.Join(lines, "\n")
}

func ClashSubscription(user model.User, servers []model.Server) string {
	var builder strings.Builder
	builder.WriteString("mixed-port: 7890\n")
	builder.WriteString("allow-lan: false\n")
	builder.WriteString("mode: rule\n")
	builder.WriteString("log-level: info\n")
	builder.WriteString("external-controller: 127.0.0.1:9090\n")
	builder.WriteString("proxies:\n")

	var names []string
	for _, server := range servers {
		if rendered, ok := buildClashProxy(user, server); ok {
			builder.WriteString(rendered)
			names = append(names, server.Name)
		}
	}

	builder.WriteString("proxy-groups:\n")
	builder.WriteString("  - name: \"Proxy\"\n")
	builder.WriteString("    type: select\n")
	builder.WriteString("    proxies:\n")
	builder.WriteString("      - \"DIRECT\"\n")
	for _, name := range names {
		builder.WriteString("      - " + yamlString(name) + "\n")
	}

	builder.WriteString("rules:\n")
	builder.WriteString("  - MATCH,\"Proxy\"\n")
	return builder.String()
}

func SubscriptionUserInfoHeader(user model.User) string {
	total := user.TransferEnable
	expire := user.ExpiredAt
	return fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", user.U, user.D, total, expire)
}

func TrafficString(used, total int64) string {
	return humanBytes(total-used) + " / " + humanBytes(total)
}

func ResetDay() int {
	now := time.Now()
	nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	return int(nextMonth.Sub(now).Hours() / 24)
}

func humanBytes(value int64) string {
	if value < 0 {
		value = 0
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	floatValue := float64(value)
	index := 0
	for floatValue >= 1024 && index < len(units)-1 {
		floatValue /= 1024
		index++
	}
	return fmt.Sprintf("%.2f %s", floatValue, units[index])
}

func buildVmess(uuid string, server model.Server) string {
	payload := map[string]string{
		"v":    "2",
		"ps":   server.Name,
		"add":  server.Host,
		"port": fmt.Sprintf("%d", server.Port),
		"id":   uuid,
		"aid":  "0",
		"net":  pick(server.Network, "ws"),
		"type": "none",
		"host": server.HostHeader,
		"path": server.Path,
		"tls":  tlsString(server.TLS),
	}
	if server.ServerName != "" {
		payload["sni"] = server.ServerName
	}
	raw, _ := json.Marshal(payload)
	return "vmess://" + base64.StdEncoding.EncodeToString(raw)
}

func buildVless(uuid string, server model.Server) string {
	query := url.Values{}
	query.Set("encryption", "none")
	query.Set("type", pick(server.Network, "ws"))
	if server.Path != "" {
		query.Set("path", server.Path)
	}
	if server.HostHeader != "" {
		query.Set("host", server.HostHeader)
	}
	if server.TLS {
		query.Set("security", "tls")
	}
	if server.ServerName != "" {
		query.Set("sni", server.ServerName)
	}
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", uuid, server.Host, server.Port, query.Encode(), url.QueryEscape(server.Name))
}

func buildTrojan(password string, server model.Server) string {
	query := url.Values{}
	query.Set("allowInsecure", boolFlag(server.AllowInsecure))
	if server.ServerName != "" {
		query.Set("sni", server.ServerName)
		query.Set("peer", server.ServerName)
	}
	return fmt.Sprintf("trojan://%s@%s:%d?%s#%s", password, server.Host, server.Port, query.Encode(), url.QueryEscape(server.Name))
}

func buildShadowsocks(password string, server model.Server) string {
	raw := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", pick(server.Cipher, "aes-256-gcm"), password)))
	return fmt.Sprintf("ss://%s@%s:%d#%s", raw, server.Host, server.Port, url.QueryEscape(server.Name))
}

func buildHysteria(password string, server model.Server) string {
	query := url.Values{}
	query.Set("insecure", boolFlag(server.AllowInsecure))
	if server.ServerName != "" {
		query.Set("sni", server.ServerName)
	}
	return fmt.Sprintf("hysteria2://%s@%s:%d?%s#%s", password, server.Host, server.Port, query.Encode(), url.QueryEscape(server.Name))
}

func buildClashProxy(user model.User, server model.Server) (string, bool) {
	var builder strings.Builder
	builder.WriteString("  - name: " + yamlString(server.Name) + "\n")
	builder.WriteString("    server: " + yamlString(server.Host) + "\n")
	builder.WriteString(fmt.Sprintf("    port: %d\n", server.Port))
	builder.WriteString("    udp: true\n")

	switch server.Type {
	case "vmess":
		builder.WriteString("    type: vmess\n")
		builder.WriteString("    uuid: " + yamlString(user.UUID) + "\n")
		builder.WriteString("    alterId: 0\n")
		builder.WriteString("    cipher: auto\n")
		writeTLS(&builder, server)
		writeNetwork(&builder, server)
	case "vless":
		builder.WriteString("    type: vless\n")
		builder.WriteString("    uuid: " + yamlString(user.UUID) + "\n")
		builder.WriteString("    cipher: auto\n")
		writeTLS(&builder, server)
		writeNetwork(&builder, server)
	case "trojan":
		builder.WriteString("    type: trojan\n")
		builder.WriteString("    password: " + yamlString(pick(server.Password, user.UUID)) + "\n")
		writeTLS(&builder, server)
	case "shadowsocks":
		builder.WriteString("    type: ss\n")
		builder.WriteString("    cipher: " + yamlString(pick(server.Cipher, "aes-256-gcm")) + "\n")
		builder.WriteString("    password: " + yamlString(pick(server.Password, user.UUID)) + "\n")
	case "hysteria", "hysteria2":
		builder.WriteString("    type: hysteria2\n")
		builder.WriteString("    password: " + yamlString(pick(server.Password, user.UUID)) + "\n")
		if server.ServerName != "" {
			builder.WriteString("    sni: " + yamlString(server.ServerName) + "\n")
		}
		if server.AllowInsecure {
			builder.WriteString("    skip-cert-verify: true\n")
		}
	default:
		return "", false
	}
	return builder.String(), true
}

func writeTLS(builder *strings.Builder, server model.Server) {
	if !server.TLS {
		return
	}
	builder.WriteString("    tls: true\n")
	if server.ServerName != "" {
		builder.WriteString("    servername: " + yamlString(server.ServerName) + "\n")
		builder.WriteString("    sni: " + yamlString(server.ServerName) + "\n")
	}
	if server.AllowInsecure {
		builder.WriteString("    skip-cert-verify: true\n")
	}
}

func writeNetwork(builder *strings.Builder, server model.Server) {
	network := pick(server.Network, "ws")
	builder.WriteString("    network: " + yamlString(network) + "\n")
	switch network {
	case "ws":
		builder.WriteString("    ws-opts:\n")
		builder.WriteString("      path: " + yamlString(pick(server.Path, "/")) + "\n")
		if server.HostHeader != "" {
			builder.WriteString("      headers:\n")
			builder.WriteString("        Host: " + yamlString(server.HostHeader) + "\n")
		}
	case "grpc":
		builder.WriteString("    grpc-opts:\n")
		builder.WriteString("      grpc-service-name: " + yamlString(pick(server.Path, "grpc")) + "\n")
	}
}

func yamlString(value string) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func containsInt64(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func pick(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func tlsString(enabled bool) string {
	if enabled {
		return "tls"
	}
	return ""
}

func boolFlag(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
