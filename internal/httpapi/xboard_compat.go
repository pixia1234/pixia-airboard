package httpapi

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"pixia-airboard/internal/model"
)

const frontendVersion = "1.0.10-airboard"

func (s *Server) renderPage(w http.ResponseWriter, r *http.Request) {
	settings, _ := s.store.AppSettings(r.Context())
	adminPath := s.matchAdminPath(r.URL.Path, settings)
	if adminPath != "" {
		s.renderAdminPage(w, r, settings, adminPath)
		return
	}
	if target := s.legacyAdminRedirectTarget(r, settings); target != "" {
		http.Redirect(w, r, target, http.StatusFound)
		return
	}
	s.renderUserPage(w, r, settings)
}

func (s *Server) renderUserPage(w http.ResponseWriter, r *http.Request, settings map[string]string) {
	s.renderTemplate(w, "index.html", s.spaTemplateData(r, settings, "user", s.configuredAdminPath(settings)))
}

func (s *Server) renderAdminPage(w http.ResponseWriter, r *http.Request, settings map[string]string, adminPath string) {
	s.renderTemplate(w, "admin.html", s.spaTemplateData(r, settings, "admin", normalizeAdminPath(adminPath)))
}

func (s *Server) matchAdminPath(path string, settings map[string]string) string {
	segment := firstPathSegment(path)
	if segment == "" {
		return ""
	}
	adminPath := s.configuredAdminPath(settings)
	if segment == adminPath {
		return adminPath
	}
	return ""
}

func (s *Server) legacyAdminRedirectTarget(r *http.Request, settings map[string]string) string {
	segment := firstPathSegment(r.URL.Path)
	if segment == "" {
		return ""
	}
	current := s.configuredAdminPath(settings)
	for _, alias := range []string{normalizeAdminPath(s.cfg.AdminPath), "admin"} {
		if alias == "" || alias == current || segment != alias {
			continue
		}
		tail := strings.TrimPrefix(r.URL.Path, "/"+segment)
		target := "/" + current
		if tail != "" {
			target += tail
		}
		if rawQuery := strings.TrimSpace(r.URL.RawQuery); rawQuery != "" {
			target += "?" + rawQuery
		}
		return target
	}
	return ""
}

func firstPathSegment(path string) string {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	return parts[0]
}

func normalizeAdminPath(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "/")
	if value == "" {
		return "admin"
	}
	return value
}

func (s *Server) configuredAdminPath(settings map[string]string) string {
	candidate := normalizeAdminPath(firstNonEmpty(settings["secure_path"], s.cfg.AdminPath))
	if !isValidAdminPath(candidate) {
		return "admin"
	}
	return candidate
}

func isValidAdminPath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if isReservedAdminPath(value) {
		return false
	}
	for _, ch := range value {
		if ch >= 'a' && ch <= 'z' {
			continue
		}
		if ch >= 'A' && ch <= 'Z' {
			continue
		}
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch == '-' || ch == '_' {
			continue
		}
		return false
	}
	return true
}

func isReservedAdminPath(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "api", "assets", "dashboard", "static", "sub", "theme":
		return true
	default:
		return false
	}
}

func (s *Server) spaTemplateData(r *http.Request, settings map[string]string, page, adminPath string) map[string]any {
	title := pick(settings["app_name"], s.cfg.AppName)
	description := pick(settings["app_description"], "Go + SQLite subscription management panel")
	appURL := firstNonEmpty(settings["app_url"], s.cfg.AppURL, origin(r))

	data := map[string]any{
		"Title":       title,
		"Description": description,
		"Version":     frontendVersion,
		"Bootstrap": toTemplateJS(map[string]any{
			"page":        page,
			"title":       title,
			"description": description,
			"adminPath":   adminPath,
			"apiBase":     "/api/v1",
			"appUrl":      appURL,
		}),
	}
	if page == "user" {
		data["CustomHTML"] = template.HTML(settings["custom_html"])
	}
	return data
}

func (s *Server) registerUserLegacyRoutes(r chi.Router) {
	r.Get("/knowledge/fetch", s.emptySuccessList)
	r.Get("/order/fetch", s.emptyRawList)
	r.Get("/order/detail", s.emptySuccessMap)
	r.Post("/order/save", s.successTrue)
	r.Post("/order/checkout", s.successTrue)
	r.Post("/order/cancel", s.successTrue)
	r.Post("/coupon/check", s.emptySuccessMap)
	r.Get("/ticket/fetch", s.emptyRawList)
	r.Post("/ticket/save", s.successTrue)
	r.Post("/ticket/close", s.successTrue)
	r.Post("/ticket/reply", s.successTrue)
	r.Post("/ticket/withdraw", s.successTrue)
	r.Post("/transfer", s.successTrue)
}

func (s *Server) registerAdminLegacyRoutes(r chi.Router) {
	r.Get("/stat/getStats", s.adminStatGetStats)
	r.Get("/stat/getOrder", s.adminStatGetOrder)
	r.Get("/stat/getTrafficRank", s.adminStatGetTrafficRank)
	r.Post("/stat/getStatUser", s.adminStatGetStatUser)

	r.Get("/theme/getThemes", s.adminThemeGetThemes)
	r.Post("/theme/getThemeConfig", s.adminThemeGetThemeConfig)
	r.Post("/theme/saveThemeConfig", s.adminThemeSaveThemeConfig)
	r.Post("/theme/upload", s.successTrue)
	r.Post("/theme/delete", s.successTrue)

	r.Post("/server/manage/save", s.adminServerManageSave)
	r.Post("/server/manage/update", s.adminServerManageSave)
	r.Post("/server/manage/sort", s.adminServerSort)

	r.Post("/user/fetch", s.adminUserFetch)
	r.Post("/user/destroy", s.adminUserDestroy)
	r.Post("/user/sendMail", s.successTrue)
	r.Post("/user/dumpCSV", s.emptySuccessList)

	r.Get("/config/getEmailTemplate", s.emptySuccessMap)
	r.Post("/config/testSendMail", s.successTrue)
	r.Post("/config/setTelegramWebhook", s.successTrue)

	r.Get("/system/getQueueStats", s.emptySuccessMap)
	r.Get("/system/getQueueWorkload", s.emptySuccessList)
	r.Get("/system/getQueueMasters", s.emptySuccessList)
	r.Get("/system/getHorizonFailedJobs", s.emptyRawList)

	r.Get("/server/group/fetch", s.emptySuccessList)
	r.Post("/server/group/save", s.successTrue)
	r.Post("/server/group/drop", s.successTrue)

	r.Get("/server/route/fetch", s.emptySuccessList)
	r.Post("/server/route/save", s.successTrue)
	r.Post("/server/route/drop", s.successTrue)

	r.Get("/payment/fetch", s.emptySuccessList)
	r.Get("/payment/getPaymentMethods", s.emptySuccessList)
	r.Post("/payment/getPaymentForm", s.emptySuccessMap)
	r.Post("/payment/save", s.successTrue)
	r.Post("/payment/drop", s.successTrue)
	r.Post("/payment/show", s.successTrue)
	r.Post("/payment/sort", s.successTrue)

	r.Get("/knowledge/fetch", s.emptySuccessList)
	r.Get("/knowledge/fetch/{id}", s.emptySuccessMap)
	r.Post("/knowledge/save", s.successTrue)
	r.Post("/knowledge/drop", s.successTrue)
	r.Post("/knowledge/show", s.successTrue)
	r.Post("/knowledge/sort", s.successTrue)

	r.Post("/order/fetch", s.emptyRawList)
	r.Post("/order/detail", s.emptySuccessMap)
	r.Post("/order/paid", s.successTrue)
	r.Post("/order/cancel", s.successTrue)
	r.Post("/order/update", s.successTrue)
	r.Post("/order/assign", s.successTrue)

	r.Post("/ticket/fetch", s.emptyRawList)
	r.Get("/ticket/fetch", s.emptyRawList)
	r.Post("/ticket/reply", s.successTrue)
	r.Post("/ticket/close", s.successTrue)

	r.Post("/coupon/fetch", s.emptyRawList)
	r.Post("/coupon/generate", s.successTrue)
	r.Post("/coupon/drop", s.successTrue)
	r.Post("/coupon/update", s.successTrue)

	r.Get("/group/fetch", s.emptySuccessList)
	r.Post("/group/save", s.successTrue)
	r.Post("/group/drop", s.successTrue)

	r.Get("/plugin/types", s.emptySuccessList)
	r.Get("/plugin/getPlugins", s.emptySuccessList)
	r.Post("/plugin/upload", s.successTrue)
	r.Post("/plugin/delete", s.successTrue)
	r.Post("/plugin/install", s.successTrue)
	r.Post("/plugin/uninstall", s.successTrue)
	r.Post("/plugin/config", s.emptySuccessMap)
	r.Post("/plugin/saveConfig", s.successTrue)

	r.Get("/traffic-reset/logs", s.emptyRawList)
	r.Get("/traffic-reset/stats", s.emptySuccessMap)
	r.Post("/traffic-reset/reset-user", s.successTrue)
	r.Get("/traffic-reset/user/{id}/history", s.emptyRawList)
}

func (s *Server) adminThemeGetThemes(w http.ResponseWriter, r *http.Request) {
	settings, _ := s.store.AppSettings(r.Context())
	success(w, map[string]any{
		"active": "Xboard",
		"themes": map[string]any{
			"Xboard": map[string]any{
				"name":           "Xboard",
				"description":    pick(settings["app_description"], "Xboard compatible theme"),
				"version":        "1.0.0",
				"background_url": settings["background_url"],
				"images":         []string{},
				"can_delete":     false,
				"configs": []map[string]any{
					{
						"label":          "主题色",
						"placeholder":    "请选择主题颜色",
						"field_name":     "theme_color",
						"field_type":     "select",
						"select_options": map[string]string{"default": "默认(绿色)", "blue": "蓝色", "black": "黑色", "darkblue": "暗蓝色"},
						"default_value":  "default",
					},
					{
						"label":       "背景",
						"placeholder": "请输入背景图片 URL",
						"field_name":  "background_url",
						"field_type":  "input",
					},
					{
						"label":       "自定义页脚 HTML",
						"placeholder": "可插入客服脚本等 HTML",
						"field_name":  "custom_html",
						"field_type":  "textarea",
					},
				},
			},
		},
	})
}

func (s *Server) adminThemeGetThemeConfig(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.AppSettings(r.Context())
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取主题配置失败", nil)
		return
	}
	success(w, map[string]any{
		"theme_color":    pick(settings["theme_color"], "default"),
		"background_url": settings["background_url"],
		"custom_html":    settings["custom_html"],
	})
}

func (s *Server) adminThemeSaveThemeConfig(w http.ResponseWriter, r *http.Request) {
	payload, err := readPayload(r)
	if err != nil {
		fail(w, http.StatusBadRequest, "参数错误", nil)
		return
	}
	configValues := map[string]string{}
	if raw, ok := payload["config"].(map[string]any); ok {
		for _, key := range []string{"theme_color", "background_url", "custom_html"} {
			if value, exists := raw[key]; exists {
				configValues[key] = strings.TrimSpace(fmt.Sprint(value))
			}
		}
	}
	if len(configValues) == 0 {
		for _, key := range []string{"theme_color", "background_url", "custom_html"} {
			if value := payload.String(key); value != "" || key == "custom_html" {
				configValues[key] = value
			}
		}
	}
	if len(configValues) == 0 {
		success(w, true)
		return
	}
	if err := s.store.UpdateSettings(r.Context(), configValues); err != nil {
		fail(w, http.StatusInternalServerError, "保存主题配置失败", nil)
		return
	}
	success(w, true)
}

func (s *Server) adminStatGetStats(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListAllUsers(r.Context())
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取统计失败", nil)
		return
	}

	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()
	lastMonthStart := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location()).Unix()
	activeSince := now.Add(-30 * 24 * time.Hour).Unix()

	var currentMonthNewUsers int64
	var lastMonthNewUsers int64
	var activeUsers int64
	var totalUpload int64
	var totalDownload int64
	for _, user := range users {
		if user.CreatedAt >= monthStart {
			currentMonthNewUsers++
		} else if user.CreatedAt >= lastMonthStart && user.CreatedAt < monthStart {
			lastMonthNewUsers++
		}
		if !user.Banned && user.LastLoginAt >= activeSince {
			activeUsers++
		}
		totalUpload += user.U
		totalDownload += user.D
	}

	success(w, map[string]any{
		"todayIncome":            0,
		"currentMonthIncome":     0,
		"dayIncomeGrowth":        0,
		"monthIncomeGrowth":      0,
		"ticketPendingTotal":     0,
		"commissionPendingTotal": 0,
		"currentMonthNewUsers":   currentMonthNewUsers,
		"userGrowth":             growthRate(currentMonthNewUsers, lastMonthNewUsers),
		"totalUsers":             len(users),
		"activeUsers":            activeUsers,
		"monthTraffic": map[string]int64{
			"upload":   totalUpload,
			"download": totalDownload,
		},
		"todayTraffic": map[string]int64{
			"upload":   0,
			"download": 0,
		},
	})
}

func (s *Server) adminStatGetOrder(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	list := make([]map[string]any, 0, 7)
	for offset := 6; offset >= 0; offset-- {
		current := now.AddDate(0, 0, -offset)
		list = append(list, map[string]any{
			"date":             current.Format("2006-01-02"),
			"paid_total":       0,
			"commission_total": 0,
			"paid_count":       0,
			"commission_count": 0,
		})
	}
	success(w, map[string]any{
		"summary": map[string]any{
			"paid_total":       0,
			"paid_count":       0,
			"avg_paid_amount":  0,
			"commission_total": 0,
			"commission_count": 0,
			"commission_rate":  0,
		},
		"list": list,
	})
}

func (s *Server) adminStatGetTrafficRank(w http.ResponseWriter, r *http.Request) {
	requestType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if requestType == "node" {
		servers, err := s.store.ListServers(r.Context(), false)
		if err != nil {
			fail(w, http.StatusInternalServerError, "读取节点失败", nil)
			return
		}
		users, err := s.store.ListAllUsers(r.Context())
		if err != nil {
			fail(w, http.StatusInternalServerError, "读取用户失败", nil)
			return
		}
		rank := make([]map[string]any, 0, len(servers))
		for _, server := range servers {
			var total int64
			for _, user := range users {
				if len(server.PlanIDs) > 0 && !containsInt64(server.PlanIDs, user.PlanID) {
					continue
				}
				total += user.U + user.D
			}
			rank = append(rank, map[string]any{
				"name":   server.Name,
				"value":  total,
				"change": 0,
			})
		}
		sort.SliceStable(rank, func(i, j int) bool {
			return payload(rank[i]).Int64("value") > payload(rank[j]).Int64("value")
		})
		if len(rank) > 10 {
			rank = rank[:10]
		}
		success(w, rank)
		return
	}

	users, err := s.store.ListAllUsers(r.Context())
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取用户失败", nil)
		return
	}
	sort.SliceStable(users, func(i, j int) bool {
		return users[i].U+users[i].D > users[j].U+users[j].D
	})
	if len(users) > 10 {
		users = users[:10]
	}
	rank := make([]map[string]any, 0, len(users))
	for _, user := range users {
		rank = append(rank, map[string]any{
			"name":   user.Email,
			"value":  user.U + user.D,
			"change": 0,
		})
	}
	success(w, rank)
}

func (s *Server) adminStatGetStatUser(w http.ResponseWriter, r *http.Request) {
	payload, _ := readPayload(r)
	user, err := s.store.UserByID(r.Context(), payload.Int64("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "用户不存在", nil)
		return
	}
	success(w, map[string]any{
		"u":               user.U,
		"d":               user.D,
		"total_used":      user.U + user.D,
		"transfer_enable": user.TransferEnable,
		"expired_at":      user.ExpiredAt,
	})
}

func (s *Server) adminServerManageSave(w http.ResponseWriter, r *http.Request) {
	payload, err := readPayload(r)
	if err != nil {
		fail(w, http.StatusBadRequest, "参数错误", nil)
		return
	}

	var current model.Server
	if payload.Int64("id") > 0 {
		current, err = s.store.ServerByID(r.Context(), payload.Int64("id"))
		if err != nil {
			fail(w, http.StatusNotFound, "节点不存在", nil)
			return
		}
	}
	server, err := s.buildAdminServer(r, payload, current)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if server.ID > 0 {
		if err := s.store.UpdateServer(r.Context(), server); err != nil {
			fail(w, http.StatusInternalServerError, "保存节点失败", nil)
			return
		}
	} else {
		if err := s.store.CreateServer(r.Context(), server); err != nil {
			fail(w, http.StatusInternalServerError, "创建节点失败", nil)
			return
		}
	}
	success(w, true)
}

func (s *Server) buildAdminServer(r *http.Request, values payload, current model.Server) (model.Server, error) {
	item := current
	if item.ID == 0 {
		item.Version = 1
		item.Port = 443
		item.Rate = 1
		item.Show = true
		item.IsOnline = true
		item.Type = "vmess"
		item.Network = "ws"
	}

	item.ID = values.Int64Default("id", item.ID)
	item.Type = firstNonEmpty(values.String("type"), values.String("server_type"), item.Type, "vmess")
	item.Name = firstNonEmpty(values.String("name"), values.String("remarks"), item.Name, strings.ToUpper(item.Type)+" Node")
	item.Host = firstNonEmpty(values.String("host"), values.String("address"), item.Host, "example.com")
	item.Port = values.IntDefault("port", item.Port)
	item.Version = values.IntDefault("version", item.Version)
	item.Network = firstNonEmpty(values.String("network"), values.String("transport"), item.Network, "ws")
	item.Path = firstNonEmpty(values.String("path"), values.String("ws_path"), item.Path)
	item.HostHeader = firstNonEmpty(values.String("host_header"), item.HostHeader)
	item.ServerName = firstNonEmpty(values.String("server_name"), values.String("sni"), item.ServerName)
	item.Cipher = firstNonEmpty(values.String("cipher"), item.Cipher, "aes-256-gcm")
	item.Password = firstNonEmpty(values.String("password"), item.Password)
	item.Rate = values.Float64Default("rate", item.Rate)
	item.AllowInsecure = values.BoolDefault("allow_insecure", item.AllowInsecure)
	item.IsOnline = values.BoolDefault("is_online", item.IsOnline)
	item.Show = values.BoolDefault("show", item.Show)
	item.Sort = values.Int64Default("sort", item.Sort)
	item.Tags = values.StringSlice("tags")
	if len(item.Tags) == 0 && len(current.Tags) > 0 {
		item.Tags = current.Tags
	}

	switch security := strings.ToLower(values.String("security")); security {
	case "tls", "xtls", "reality":
		item.TLS = true
	case "none":
		item.TLS = false
	default:
		item.TLS = values.BoolDefault("tls", item.TLS)
	}

	planIDs := values.Int64Slice("plan_ids")
	if len(planIDs) == 0 {
		planIDs = values.Int64Slice("group_ids")
	}
	if len(planIDs) == 0 {
		planIDs = current.PlanIDs
	}
	if len(planIDs) == 0 {
		plans, _ := s.store.ListPlans(r.Context(), false)
		for _, plan := range plans {
			planIDs = append(planIDs, plan.ID)
		}
	}
	item.PlanIDs = planIDs

	if item.Name == "" || item.Host == "" || item.Port <= 0 {
		return model.Server{}, fmt.Errorf("节点参数不完整")
	}
	return item, nil
}

func (s *Server) adminServerSort(w http.ResponseWriter, r *http.Request) {
	payload, _ := readPayload(r)
	serverIDs := payload.Int64Slice("ids")
	if len(serverIDs) == 0 {
		serverIDs = payload.Int64Slice("server_ids")
	}
	if len(serverIDs) == 0 {
		success(w, true)
		return
	}
	if err := s.store.SortServers(r.Context(), serverIDs); err != nil {
		fail(w, http.StatusInternalServerError, "排序失败", nil)
		return
	}
	success(w, true)
}

func (s *Server) adminUserDestroy(w http.ResponseWriter, r *http.Request) {
	payload, _ := readPayload(r)
	userID := payload.Int64("id")
	if userID == 0 {
		fail(w, http.StatusBadRequest, "缺少用户ID", nil)
		return
	}
	if err := s.store.DeleteUser(r.Context(), userID); err != nil {
		fail(w, http.StatusInternalServerError, "删除用户失败", nil)
		return
	}
	_ = s.auth.RemoveAllSessions(r.Context(), userID)
	success(w, true)
}

func growthRate(current, previous int64) float64 {
	if previous <= 0 {
		if current > 0 {
			return 100
		}
		return 0
	}
	return float64(current-previous) / float64(previous) * 100
}

func toTemplateJS(value any) template.JS {
	raw, _ := json.Marshal(value)
	return template.JS(raw)
}
