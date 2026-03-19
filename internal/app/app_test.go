package app

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	_ "github.com/mattn/go-sqlite3"

	"pixia-airboard/internal/config"
)

type testServer struct {
	cfg config.Config
	db  *sql.DB
	ts  *httptest.Server
}

func newTestServer(t *testing.T, redisAddr string) *testServer {
	t.Helper()

	tempDir := t.TempDir()
	cfg := config.Config{
		Addr:         ":0",
		DBPath:       filepath.Join(tempDir, "airboard.db"),
		JWTSecret:    "test-secret",
		RedisAddr:    redisAddr,
		RedisPrefix:  sanitizeRedisPrefix(t.Name()),
		AppName:      "Pixia Airboard",
		AdminPath:    "admin",
		DefaultEmail: "admin@example.com",
		DefaultPass:  "admin123456",
	}

	handler, cleanup, err := New(cfg)
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	t.Cleanup(cleanup)

	db, err := sql.Open("sqlite3", cfg.DBPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	return &testServer{cfg: cfg, db: db, ts: ts}
}

func sanitizeRedisPrefix(value string) string {
	replacer := strings.NewReplacer("/", "_", " ", "_")
	return "test:" + replacer.Replace(strings.ToLower(value))
}

func TestSecurePathAndSPABootstrap(t *testing.T) {
	srv := newTestServer(t, "")

	userPage := srv.get(t, "/", "")
	mustStatus(t, userPage, http.StatusOK)
	body := readBody(t, userPage)
	mustContain(t, body, `"page":"user"`)
	mustContain(t, body, `"adminPath":"admin"`)

	adminPage := srv.get(t, "/admin", "")
	mustStatus(t, adminPage, http.StatusOK)
	mustContain(t, readBody(t, adminPage), `"page":"admin"`)

	auth := srv.loginAdmin(t)

	rootlessAdmin := srv.get(t, "/api/v1/config/fetch", auth)
	mustStatus(t, rootlessAdmin, http.StatusNotFound)

	scopedAdmin := srv.get(t, "/api/v1/admin/config/fetch", auth)
	mustStatus(t, scopedAdmin, http.StatusOK)

	saveResp := srv.postJSON(t, "/api/v1/admin/config/save", auth, map[string]any{
		"secure_path": "secret",
	})
	mustStatus(t, saveResp, http.StatusOK)

	redirectingOldAdmin := srv.getNoRedirect(t, "/admin", "")
	mustStatus(t, redirectingOldAdmin, http.StatusFound)
	if got := redirectingOldAdmin.Header.Get("Location"); got != "/secret" {
		t.Fatalf("expected redirect to /secret, got %q", got)
	}

	secretPage := srv.get(t, "/secret", "")
	mustStatus(t, secretPage, http.StatusOK)
	secretBody := readBody(t, secretPage)
	mustContain(t, secretBody, `"page":"admin"`)
	mustContain(t, secretBody, `"adminPath":"secret"`)

	oldAPI := srv.get(t, "/api/v1/admin/config/fetch", auth)
	mustStatus(t, oldAPI, http.StatusNotFound)

	newAPI := srv.get(t, "/api/v1/secret/config/fetch", auth)
	mustStatus(t, newAPI, http.StatusOK)

	invalidPath := srv.postJSON(t, "/api/v1/secret/config/save", auth, map[string]any{
		"secure_path": "dashboard",
	})
	mustStatus(t, invalidPath, http.StatusBadRequest)
}

func TestRedisFlushDoesNotInvalidateSessionOrQuickLogin(t *testing.T) {
	redisServer, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer redisServer.Close()

	srv := newTestServer(t, redisServer.Addr())
	auth := srv.loginAdmin(t)

	userInfo := srv.get(t, "/api/v1/user/info", auth)
	mustStatus(t, userInfo, http.StatusOK)

	if got := srv.countRows(t, "sessions"); got != 1 {
		t.Fatalf("expected 1 session row, got %d", got)
	}

	quickLoginResp := srv.postJSON(t, "/api/v1/user/getQuickLoginUrl", auth, map[string]any{
		"redirect": "dashboard",
	})
	mustStatus(t, quickLoginResp, http.StatusOK)

	quickLoginURL := srv.readEnvelopeString(t, quickLoginResp)
	code := mustQuickLoginCode(t, quickLoginURL)

	if got := srv.countRows(t, "quick_logins"); got != 1 {
		t.Fatalf("expected 1 quick_login row, got %d", got)
	}

	redisServer.FlushAll()

	userInfoAfterFlush := srv.get(t, "/api/v1/user/info", auth)
	mustStatus(t, userInfoAfterFlush, http.StatusOK)

	tokenLogin := srv.get(t, "/api/v1/passport/auth/token2Login?verify="+url.QueryEscape(code), "")
	mustStatus(t, tokenLogin, http.StatusOK)

	if got := srv.countRows(t, "quick_logins"); got != 0 {
		t.Fatalf("expected quick_logins table to be empty, got %d", got)
	}
}

func TestXrayRTrafficAcceptsFieldAliases(t *testing.T) {
	srv := newTestServer(t, "")
	adminAuth := srv.loginAdmin(t)

	configResp := srv.get(t, "/api/v1/admin/config/fetch", adminAuth)
	mustStatus(t, configResp, http.StatusOK)

	var configPayload struct {
		Data map[string]any `json:"data"`
	}
	decodeJSON(t, configResp, &configPayload)

	serverToken, _ := configPayload.Data["server_token"].(string)
	if serverToken == "" {
		t.Fatalf("expected server_token in config response")
	}

	var userID int64
	var userUUID string
	var beforeU, beforeD int64
	if err := srv.db.QueryRow(`SELECT id, uuid, u, d FROM users WHERE email = ?`, "demo@example.com").Scan(&userID, &userUUID, &beforeU, &beforeD); err != nil {
		t.Fatalf("query demo user: %v", err)
	}

	trafficResp := srv.postJSON(
		t,
		"/api/v1/agent/xrayr/traffic?token="+url.QueryEscape(serverToken)+"&node_id=1",
		"",
		map[string]any{
			"users": []map[string]any{
				{"uid": userID, "up": 111, "down": 222},
				{"uuid": userUUID, "upload": 333, "download": 444},
			},
		},
	)
	mustStatus(t, trafficResp, http.StatusOK)

	var afterU, afterD int64
	if err := srv.db.QueryRow(`SELECT u, d FROM users WHERE id = ?`, userID).Scan(&afterU, &afterD); err != nil {
		t.Fatalf("query updated traffic: %v", err)
	}
	if afterU != beforeU+444 || afterD != beforeD+666 {
		t.Fatalf("expected traffic to be updated via aliases, got u=%d d=%d from u=%d d=%d", afterU, afterD, beforeU, beforeD)
	}

	demoAuth := srv.login(t, "demo@example.com", "demo123456")
	subscribeResp := srv.get(t, "/api/v1/user/getSubscribe", demoAuth)
	mustStatus(t, subscribeResp, http.StatusOK)

	var subscribePayload struct {
		Data struct {
			U int64 `json:"u"`
			D int64 `json:"d"`
		} `json:"data"`
	}
	decodeJSON(t, subscribeResp, &subscribePayload)
	if subscribePayload.Data.U != afterU || subscribePayload.Data.D != afterD {
		t.Fatalf("expected getSubscribe to expose updated traffic, got u=%d d=%d want u=%d d=%d", subscribePayload.Data.U, subscribePayload.Data.D, afterU, afterD)
	}
}

func (s *testServer) login(t *testing.T, email, password string) string {
	t.Helper()

	resp := s.postJSON(t, "/api/v1/passport/auth/login", "", map[string]any{
		"email":    email,
		"password": password,
	})
	mustStatus(t, resp, http.StatusOK)

	var payload struct {
		Data struct {
			AuthData string `json:"auth_data"`
		} `json:"data"`
	}
	decodeJSON(t, resp, &payload)
	if payload.Data.AuthData == "" {
		t.Fatalf("expected auth_data in login response")
	}
	return payload.Data.AuthData
}

func (s *testServer) loginAdmin(t *testing.T) string {
	t.Helper()
	return s.login(t, s.cfg.DefaultEmail, s.cfg.DefaultPass)
}

func (s *testServer) countRows(t *testing.T, table string) int {
	t.Helper()

	var count int
	query := "SELECT COUNT(1) FROM " + table
	if err := s.db.QueryRow(query).Scan(&count); err != nil {
		t.Fatalf("count rows in %s: %v", table, err)
	}
	return count
}

func (s *testServer) get(t *testing.T, path, auth string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, s.ts.URL+path, nil)
	if err != nil {
		t.Fatalf("new get request: %v", err)
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := s.ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do get request: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func (s *testServer) getNoRedirect(t *testing.T, path, auth string) *http.Response {
	t.Helper()
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(http.MethodGet, s.ts.URL+path, nil)
	if err != nil {
		t.Fatalf("new get request: %v", err)
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do get request: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func (s *testServer) postJSON(t *testing.T, path, auth string, payload any) *http.Response {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, s.ts.URL+path, strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("new post request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := s.ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do post request: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func mustQuickLoginCode(t *testing.T, value string) string {
	t.Helper()
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatalf("parse quick login url: %v", err)
	}
	code := parsed.Query().Get("verify")
	if code == "" {
		t.Fatalf("expected verify code in quick login url %q", value)
	}
	return code
}

func decodeJSON(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	raw := readBody(t, resp)
	if err := json.Unmarshal([]byte(raw), dst); err != nil {
		t.Fatalf("decode json %q: %v", raw, err)
	}
}

func (s *testServer) readEnvelopeString(t *testing.T, resp *http.Response) string {
	t.Helper()
	var payload struct {
		Data string `json:"data"`
	}
	decodeJSON(t, resp, &payload)
	if payload.Data == "" {
		t.Fatalf("expected string data in response")
	}
	return payload.Data
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	resp.Body = io.NopCloser(strings.NewReader(string(raw)))
	return string(raw)
}

func mustStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("expected status %d, got %d, body=%s", want, resp.StatusCode, readBody(t, resp))
	}
}

func mustContain(t *testing.T, body, snippet string) {
	t.Helper()
	if !strings.Contains(body, snippet) {
		t.Fatalf("expected body to contain %q, got %s", snippet, body)
	}
}
