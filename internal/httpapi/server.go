package httpapi

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"math"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"pixia-airboard/internal/cache"
	"pixia-airboard/internal/config"
	"pixia-airboard/internal/model"
	"pixia-airboard/internal/service"
	"pixia-airboard/internal/store"
)

type Server struct {
	cfg       config.Config
	store     *store.Store
	cache     *cache.Cache
	auth      *service.AuthService
	templates *template.Template
	staticFS  fs.FS
}

type contextKey string

const userContextKey contextKey = "auth-user"

const (
	subscriptionBodyCacheTTL = 60 * time.Second
	nodeUsersCacheTTL        = 30 * time.Second
)

func New(cfg config.Config, dataStore *store.Store, cacheClient *cache.Cache, auth *service.AuthService, templates *template.Template, assets fs.FS) (http.Handler, error) {
	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		return nil, err
	}

	s := &Server{
		cfg:       cfg,
		store:     dataStore,
		cache:     cacheClient,
		auth:      auth,
		templates: templates,
		staticFS:  staticFS,
	}
	return s.routes(), nil
}

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(s.staticFS))))
	r.Handle("/theme/*", http.StripPrefix("/", http.FileServer(http.FS(s.staticFS))))
	r.Handle("/assets/*", http.StripPrefix("/", http.FileServer(http.FS(s.staticFS))))
	r.Get("/sub/{suffix}", s.publicSubscribeBySuffix)
	s.registerAPIRoutes(r)

	r.Get("/", s.renderPage)
	r.Get("/*", s.renderPage)

	return r
}

func (s *Server) registerAPIRoutes(r chi.Router) {
	r.Route("/api/v1", s.registerAPIV1Routes)
	r.Route("/api/v2", s.registerAPIV2Routes)
}

func (s *Server) registerAPIV1Routes(api chi.Router) {
	s.registerPublicRoutes(api)
	s.registerUserRoutes(api)
	s.registerClientRoutes(api)
	s.registerAgentRoutes(api)
	s.registerAdminVersionedRoutes(api)
}

func (s *Server) registerAPIV2Routes(api chi.Router) {
	s.registerPublicRoutes(api)
	s.registerUserCompatRoutes(api)
	s.registerAdminVersionedRoutes(api)
}

func (s *Server) registerUserRoutes(api chi.Router) {
	api.Group(func(r chi.Router) {
		r.Use(s.userMiddleware)
		r.Route("/user", func(r chi.Router) {
			r.Get("/resetSecurity", s.userResetSecurity)
			r.Get("/info", s.userInfo)
			r.Post("/changePassword", s.userChangePassword)
			r.Post("/update", s.userUpdate)
			r.Get("/getSubscribe", s.userGetSubscribe)
			r.Get("/getStat", s.userGetStat)
			r.Get("/checkLogin", s.userCheckLogin)
			r.Post("/getQuickLoginUrl", s.userGetQuickLoginURL)
			r.Get("/getActiveSession", s.userGetActiveSession)
			r.Post("/removeActiveSession", s.userRemoveActiveSession)

			r.Get("/plan/fetch", s.userPlanFetch)

			r.Get("/invite/save", s.userInviteSave)
			r.Get("/invite/fetch", s.userInviteFetch)
			r.Get("/invite/details", s.emptyRawList)

			r.Get("/notice/fetch", s.userNoticeFetch)

			r.Get("/server/fetch", s.userServerFetch)

			r.Get("/comm/config", s.userCommConfig)
			r.Get("/stat/getTrafficLog", s.emptyRawList)
			s.registerUserLegacyRoutes(r)
		})
	})
}

func (s *Server) registerUserCompatRoutes(api chi.Router) {
	api.Group(func(r chi.Router) {
		r.Use(s.userMiddleware)
		r.Route("/user", func(r chi.Router) {
			r.Get("/resetSecurity", s.userResetSecurity)
			r.Get("/info", s.userInfo)
			r.Post("/changePassword", s.userChangePassword)
			r.Get("/getSubscribe", s.userGetSubscribe)
			r.Get("/getStat", s.userGetStat)
			r.Get("/checkLogin", s.userCheckLogin)
			r.Post("/getQuickLoginUrl", s.userGetQuickLoginURL)
			r.Get("/getActiveSession", s.userGetActiveSession)
			r.Post("/removeActiveSession", s.userRemoveActiveSession)

			r.Get("/plan/fetch", s.userPlanFetch)
			r.Get("/invite/save", s.userInviteSave)
			r.Get("/invite/fetch", s.userInviteFetch)
			r.Get("/invite/details", s.emptyRawList)
			r.Get("/notice/fetch", s.userNoticeFetch)
			r.Get("/server/fetch", s.userServerFetch)
			r.Get("/comm/config", s.userCommConfig)
			r.Get("/stat/getTrafficLog", s.emptyRawList)
			s.registerUserLegacyRoutes(r)
		})
	})
}

func (s *Server) registerClientRoutes(api chi.Router) {
	api.Route("/client", func(r chi.Router) {
		r.Get("/subscribe", s.clientSubscribe)
		r.Get("/app/getConfig", s.clientAppConfig)
		r.Get("/app/getVersion", s.clientAppVersion)
	})
}

func (s *Server) registerAgentRoutes(api chi.Router) {
	api.Route("/agent/xrayr", func(r chi.Router) {
		r.Get("/config", s.agentXrayRConfig)
		r.Get("/users", s.agentXrayRUsers)
		r.Post("/traffic", s.agentXrayRTraffic)
		r.Post("/alive", s.agentXrayRAlive)
	})

	api.Route("/server", func(r chi.Router) {
		r.Get("/UniProxy/config", s.agentXrayRConfig)
		r.Get("/UniProxy/user", s.agentXrayRUsers)
		r.Post("/UniProxy/push", s.agentXrayRTraffic)
		r.Post("/UniProxy/alive", s.agentXrayRAlive)
	})
}

func (s *Server) registerAdminVersionedRoutes(api chi.Router) {
	api.Route("/{adminPath}", func(r chi.Router) {
		r.Use(s.adminPathMiddleware)
		r.Use(s.adminMiddleware)
		s.registerAdminRoutes(r)
	})
}

func (s *Server) registerPublicRoutes(r chi.Router) {
	r.Route("/guest", func(r chi.Router) {
		r.Get("/comm/config", s.guestCommConfig)
		r.Get("/plan/fetch", s.guestPlanFetch)
	})

	r.Route("/passport", func(r chi.Router) {
		r.Post("/auth/register", s.passportRegister)
		r.Post("/auth/login", s.passportLogin)
		r.Get("/auth/token2Login", s.passportToken2Login)
		r.Post("/auth/forget", s.successTrue)
		r.Post("/auth/getQuickLoginUrl", s.successTrue)
		r.Post("/auth/loginWithMailLink", s.successTrue)
		r.Post("/comm/sendEmailVerify", s.successTrue)
		r.Post("/comm/pv", s.successTrue)
	})
}

func (s *Server) registerAdminRoutes(r chi.Router) {
	r.Get("/config/fetch", s.adminConfigFetch)
	r.Post("/config/save", s.adminConfigSave)

	r.Get("/plan/fetch", s.adminPlanFetch)
	r.Post("/plan/save", s.adminPlanSave)
	r.Post("/plan/drop", s.adminPlanDrop)
	r.Post("/plan/update", s.adminPlanSave)
	r.Post("/plan/sort", s.adminPlanSort)

	r.Get("/server/manage/getNodes", s.adminServerFetch)
	for _, serverType := range []string{"vmess", "vless", "trojan", "shadowsocks", "hysteria"} {
		r.Post("/server/"+serverType+"/save", s.adminServerSave(serverType))
		r.Post("/server/"+serverType+"/drop", s.adminServerDrop)
		r.Post("/server/"+serverType+"/update", s.adminServerSave(serverType))
		r.Post("/server/"+serverType+"/copy", s.adminServerCopy)
	}

	r.Get("/user/fetch", s.adminUserFetch)
	r.Post("/user/update", s.adminUserUpdate)
	r.Get("/user/getUserInfoById", s.adminUserInfo)
	r.Post("/user/generate", s.adminUserGenerate)
	r.Post("/user/ban", s.adminUserBan)
	r.Post("/user/resetSecret", s.adminUserResetSecret)
	r.Get("/user/subscription/fetch", s.adminSubscriptionFetch)
	r.Post("/user/subscription/save", s.adminSubscriptionSave)
	r.Post("/user/subscription/drop", s.adminSubscriptionDrop)
	r.Post("/user/subscription/reset", s.adminSubscriptionReset)

	r.Get("/stat/getStat", s.adminStatGetStat)

	r.Get("/notice/fetch", s.adminNoticeFetch)
	r.Post("/notice/save", s.adminNoticeSave)
	r.Post("/notice/update", s.adminNoticeSave)
	r.Post("/notice/drop", s.adminNoticeDrop)

	r.Get("/system/getSystemStatus", s.adminSystemStatus)
	s.registerAdminLegacyRoutes(r)
}

func (s *Server) renderIndex(w http.ResponseWriter, r *http.Request) {
	settings, _ := s.store.AppSettings(r.Context())
	s.renderUserPage(w, r, settings)
}

func (s *Server) renderAdmin(w http.ResponseWriter, r *http.Request) {
	settings, _ := s.store.AppSettings(r.Context())
	s.renderAdminPage(w, r, settings, s.configuredAdminPath(settings))
}

func (s *Server) renderTemplate(w http.ResponseWriter, name string, data map[string]any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) guestCommConfig(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.AppSettings(r.Context())
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取配置失败", nil)
		return
	}
	success(w, map[string]any{
		"tos_url":                settings["tos_url"],
		"is_email_verify":        0,
		"is_invite_force":        0,
		"email_whitelist_suffix": 0,
		"is_recaptcha":           0,
		"recaptcha_site_key":     "",
		"app_description":        settings["app_description"],
		"app_url":                pick(settings["app_url"], origin(r)),
		"logo":                   settings["logo"],
	})
}

func (s *Server) guestPlanFetch(w http.ResponseWriter, r *http.Request) {
	plans, err := s.store.ListPlans(r.Context(), true)
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取套餐失败", nil)
		return
	}
	success(w, plans)
}

func (s *Server) passportRegister(w http.ResponseWriter, r *http.Request) {
	payload, err := readPayload(r)
	if err != nil {
		fail(w, http.StatusBadRequest, "参数错误", nil)
		return
	}

	email := strings.TrimSpace(strings.ToLower(payload.String("email")))
	password := payload.String("password")
	if _, err := mail.ParseAddress(email); err != nil || password == "" {
		fail(w, http.StatusBadRequest, "邮箱或密码格式错误", nil)
		return
	}
	if _, err := s.store.UserByEmail(r.Context(), email); err == nil {
		fail(w, http.StatusBadRequest, "Email already exists", nil)
		return
	}

	plans, err := s.store.ListPlans(r.Context(), true)
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取套餐失败", nil)
		return
	}
	var plan model.Plan
	if len(plans) > 0 {
		plan = plans[0]
	}

	user, err := s.store.CreateUser(r.Context(), model.User{
		Email:          email,
		RemindExpire:   true,
		RemindTraffic:  true,
		TransferEnable: plan.TransferEnable,
		ExpiredAt:      time.Now().Add(30 * 24 * time.Hour).Unix(),
		PlanID:         plan.ID,
		GroupID:        plan.GroupID,
	}, password)
	if err != nil {
		fail(w, http.StatusInternalServerError, "注册失败", nil)
		return
	}

	user.LastLoginAt = time.Now().Unix()
	_ = s.store.UpdateUser(r.Context(), user)
	authData, err := s.auth.Issue(r.Context(), user, r.RemoteAddr, r.UserAgent())
	if err != nil {
		fail(w, http.StatusInternalServerError, "登录态创建失败", nil)
		return
	}

	success(w, map[string]any{
		"token":     user.Token,
		"is_admin":  user.IsAdmin,
		"auth_data": authData,
	})
}

func (s *Server) passportLogin(w http.ResponseWriter, r *http.Request) {
	payload, err := readPayload(r)
	if err != nil {
		fail(w, http.StatusBadRequest, "参数错误", nil)
		return
	}

	user, err := s.store.UserByEmail(r.Context(), payload.String("email"))
	if err != nil || s.store.CheckPassword(user, payload.String("password")) != nil {
		fail(w, http.StatusBadRequest, "Incorrect email or password", nil)
		return
	}
	if user.Banned {
		fail(w, http.StatusForbidden, "账号已被禁用", nil)
		return
	}

	user.LastLoginAt = time.Now().Unix()
	_ = s.store.UpdateUser(r.Context(), user)
	authData, err := s.auth.Issue(r.Context(), user, r.RemoteAddr, r.UserAgent())
	if err != nil {
		fail(w, http.StatusInternalServerError, "登录失败", nil)
		return
	}

	success(w, map[string]any{
		"token":     user.Token,
		"is_admin":  user.IsAdmin,
		"auth_data": authData,
	})
}

func (s *Server) passportToken2Login(w http.ResponseWriter, r *http.Request) {
	code := firstNonEmpty(r.URL.Query().Get("verify"), r.URL.Query().Get("token"))
	if code == "" {
		fail(w, http.StatusBadRequest, "缺少验证参数", nil)
		return
	}
	user, redirect, err := s.auth.ConsumeQuickLogin(r.Context(), code)
	if err != nil {
		fail(w, http.StatusUnauthorized, "验证链接已失效", nil)
		return
	}
	authData, err := s.auth.Issue(r.Context(), user, r.RemoteAddr, r.UserAgent())
	if err != nil {
		fail(w, http.StatusInternalServerError, "登录失败", nil)
		return
	}
	success(w, map[string]any{
		"token":     user.Token,
		"is_admin":  user.IsAdmin,
		"auth_data": authData,
		"redirect":  redirect,
	})
}

func (s *Server) userInfo(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	success(w, map[string]any{
		"email":              user.Email,
		"transfer_enable":    user.TransferEnable,
		"last_login_at":      user.LastLoginAt,
		"created_at":         user.CreatedAt,
		"banned":             user.Banned,
		"remind_expire":      user.RemindExpire,
		"remind_traffic":     user.RemindTraffic,
		"expired_at":         user.ExpiredAt,
		"balance":            user.Balance,
		"commission_balance": user.CommissionBalance,
		"plan_id":            user.PlanID,
		"discount":           0,
		"commission_rate":    0,
		"telegram_id":        0,
		"uuid":               user.UUID,
		"avatar_url":         gravatarURL(user.Email),
	})
}

func (s *Server) userCheckLogin(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	payload := map[string]any{
		"is_login": true,
	}
	if user.IsAdmin {
		payload["is_admin"] = true
	}
	success(w, payload)
}

func (s *Server) userGetSubscribe(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	settings, _ := s.store.AppSettings(r.Context())
	links, err := s.store.ListSubscriptionLinksByUserID(r.Context(), user.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取订阅链接失败", nil)
		return
	}
	var plan any
	if user.PlanID > 0 {
		if current, err := s.store.PlanByID(r.Context(), user.PlanID); err == nil {
			plan = current
		}
	}
	baseURL := s.subscribeBaseURL(r, settings)
	linkPayload := make([]map[string]any, 0, len(links))
	primaryURL := ""
	for _, link := range links {
		variants := service.BuildSubscriptionVariants(baseURL, link.Suffix)
		if link.IsPrimary && primaryURL == "" {
			primaryURL = variants["default"]
		}
		linkPayload = append(linkPayload, map[string]any{
			"id":           link.ID,
			"name":         link.Name,
			"suffix":       link.Suffix,
			"is_primary":   link.IsPrimary,
			"enabled":      link.Enabled,
			"last_used_at": link.LastUsedAt,
			"urls":         variants,
		})
	}
	if primaryURL == "" && len(linkPayload) > 0 {
		if urls, ok := linkPayload[0]["urls"].(map[string]string); ok {
			primaryURL = urls["default"]
		}
	}
	success(w, map[string]any{
		"plan_id":         user.PlanID,
		"token":           user.Token,
		"expired_at":      user.ExpiredAt,
		"u":               user.U,
		"d":               user.D,
		"transfer_enable": user.TransferEnable,
		"email":           user.Email,
		"uuid":            user.UUID,
		"plan":            plan,
		"subscribe_url":   primaryURL,
		"subscribe_urls":  linkPayload,
		"reset_day":       service.ResetDay(),
	})
}

func (s *Server) userGetStat(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	invites, err := s.store.CountUsersByInviter(r.Context(), user.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取统计失败", nil)
		return
	}
	success(w, []int{0, 0, invites})
}

func (s *Server) userServerFetch(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	servers, err := s.store.ListServers(r.Context(), true)
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取节点失败", nil)
		return
	}
	visible := service.VisibleServersForUser(user, servers)
	hash := md5.Sum([]byte(fmt.Sprintf("%v", visible)))
	eTag := `"` + hex.EncodeToString(hash[:]) + `"`
	if r.Header.Get("If-None-Match") == eTag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	var payload []map[string]any
	for _, server := range visible {
		payload = append(payload, map[string]any{
			"id":            server.ID,
			"type":          server.Type,
			"version":       server.Version,
			"name":          server.Name,
			"rate":          server.Rate,
			"tags":          server.Tags,
			"is_online":     server.IsOnline,
			"cache_key":     server.CacheKey,
			"last_check_at": server.LastCheckAt,
		})
	}
	w.Header().Set("ETag", eTag)
	writeJSON(w, http.StatusOK, map[string]any{"data": payload})
}

func (s *Server) userPlanFetch(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if id > 0 {
		plan, err := s.store.PlanByID(r.Context(), id)
		if err != nil {
			fail(w, http.StatusNotFound, "Subscription plan does not exist", nil)
			return
		}
		success(w, plan)
		return
	}
	plans, err := s.store.ListPlans(r.Context(), true)
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取套餐失败", nil)
		return
	}
	success(w, plans)
}

func (s *Server) userNoticeFetch(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("current"))
	notices, total, err := s.store.ListNotices(r.Context(), true, page, 5)
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取公告失败", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":  notices,
		"total": total,
	})
}

func (s *Server) userCommConfig(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"currency":        "CNY",
		"currency_symbol": "¥",
	})
}

func (s *Server) userResetSecurity(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	user.UUID = newUUID()
	user.Token = newToken()
	if err := s.store.UpdateUser(r.Context(), user); err != nil {
		fail(w, http.StatusInternalServerError, "重置失败", nil)
		return
	}
	links, err := s.store.ResetUserSubscriptionLinks(r.Context(), user.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "重置订阅链接失败", nil)
		return
	}
	settings, _ := s.store.AppSettings(r.Context())
	baseURL := s.subscribeBaseURL(r, settings)
	items := make([]map[string]any, 0, len(links))
	for _, link := range links {
		items = append(items, map[string]any{
			"id":         link.ID,
			"name":       link.Name,
			"suffix":     link.Suffix,
			"is_primary": link.IsPrimary,
			"enabled":    link.Enabled,
			"urls":       service.BuildSubscriptionVariants(baseURL, link.Suffix),
		})
	}
	success(w, items)
}

func (s *Server) userChangePassword(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	payload, err := readPayload(r)
	if err != nil {
		fail(w, http.StatusBadRequest, "参数错误", nil)
		return
	}
	if s.store.CheckPassword(user, payload.String("old_password")) != nil {
		fail(w, http.StatusBadRequest, "The old password is wrong", nil)
		return
	}
	if err := s.store.UpdateUserPassword(r.Context(), user.ID, payload.String("new_password")); err != nil {
		fail(w, http.StatusInternalServerError, "Save failed", nil)
		return
	}
	success(w, true)
}

func (s *Server) userUpdate(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	payload, err := readPayload(r)
	if err != nil {
		fail(w, http.StatusBadRequest, "参数错误", nil)
		return
	}
	user.RemindExpire = payload.BoolDefault("remind_expire", user.RemindExpire)
	user.RemindTraffic = payload.BoolDefault("remind_traffic", user.RemindTraffic)
	if err := s.store.UpdateUser(r.Context(), user); err != nil {
		fail(w, http.StatusInternalServerError, "Save failed", nil)
		return
	}
	success(w, true)
}

func (s *Server) userGetQuickLoginURL(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	payload, _ := readPayload(r)
	code, err := s.auth.CreateQuickLogin(r.Context(), user.ID, payload.String("redirect"))
	if err != nil {
		fail(w, http.StatusInternalServerError, "创建快捷登录失败", nil)
		return
	}
	success(w, fmt.Sprintf("%s/?verify=%s", s.subscribeBaseURL(r, nil), code))
}

func (s *Server) userGetActiveSession(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	sessions, err := s.auth.Sessions(r.Context(), user.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取会话失败", nil)
		return
	}
	payload := make(map[string]map[string]any, len(sessions))
	for _, session := range sessions {
		payload[session.ID] = map[string]any{
			"ip":       session.IP,
			"login_at": session.LoginAt,
			"ua":       session.UA,
		}
	}
	success(w, payload)
}

func (s *Server) userRemoveActiveSession(w http.ResponseWriter, r *http.Request) {
	payload, _ := readPayload(r)
	if err := s.auth.RemoveSession(r.Context(), payload.String("session_id")); err != nil {
		fail(w, http.StatusInternalServerError, "删除会话失败", nil)
		return
	}
	success(w, true)
}

func (s *Server) userInviteSave(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	success(w, map[string]any{
		"code": strings.ToUpper(user.Token[:8]),
	})
}

func (s *Server) userInviteFetch(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	invites, _ := s.store.CountUsersByInviter(r.Context(), user.ID)
	success(w, map[string]any{
		"stat": map[string]any{
			"invite_count": invites,
		},
		"codes": []string{strings.ToUpper(user.Token[:8])},
	})
}

func (s *Server) clientSubscribe(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		http.Error(w, "token is null", http.StatusForbidden)
		return
	}
	user, err := s.store.UserByToken(r.Context(), token)
	if err != nil || user.Banned {
		http.Error(w, "token is error", http.StatusForbidden)
		return
	}
	if user.ExpiredAt > 0 && user.ExpiredAt < time.Now().Unix() {
		http.Error(w, "user expired", http.StatusForbidden)
		return
	}
	servers, err := s.store.ListServers(r.Context(), true)
	if err != nil {
		http.Error(w, "read servers failed", http.StatusInternalServerError)
		return
	}
	s.writeSubscription(w, r, user, servers)
}

func (s *Server) publicSubscribeBySuffix(w http.ResponseWriter, r *http.Request) {
	suffix := chi.URLParam(r, "suffix")
	if strings.TrimSpace(suffix) == "" {
		http.NotFound(w, r)
		return
	}
	link, err := s.store.SubscriptionLinkBySuffix(r.Context(), suffix)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	user, err := s.store.UserByID(r.Context(), link.UserID)
	if err != nil || user.Banned {
		http.Error(w, "user not found", http.StatusForbidden)
		return
	}
	if user.ExpiredAt > 0 && user.ExpiredAt < time.Now().Unix() {
		http.Error(w, "user expired", http.StatusForbidden)
		return
	}
	servers, err := s.store.ListServers(r.Context(), true)
	if err != nil {
		http.Error(w, "read servers failed", http.StatusInternalServerError)
		return
	}
	_ = s.store.TouchSubscriptionLink(r.Context(), link.ID)
	s.writeSubscription(w, r, user, servers)
}

func (s *Server) writeSubscription(w http.ResponseWriter, r *http.Request, user model.User, servers []model.Server) {
	format := service.DetectSubscriptionFormat(r.URL.Query().Get("target"), firstNonEmpty(r.URL.Query().Get("flag"), r.URL.Query().Get("format")), r.UserAgent())
	cacheKey := fmt.Sprintf("subscription:user:%d:format:%s:uuid:%s:plan:%d", user.ID, format, user.UUID, user.PlanID)

	var body string
	var contentType string
	if s.cache != nil && s.cache.Enabled() {
		if cached, err := s.cache.GetString(r.Context(), cacheKey); err == nil {
			body = cached
		}
	}
	if body == "" {
		visible := service.VisibleServersForUser(user, servers)
		renderedBody, renderedContentType := service.RenderSubscription(user, visible, format)
		body = renderedBody
		contentType = renderedContentType
		if s.cache != nil && s.cache.Enabled() {
			_ = s.cache.SetString(r.Context(), cacheKey, body, subscriptionBodyCacheTTL)
		}
	} else {
		switch format {
		case service.FormatClash:
			contentType = "text/yaml; charset=utf-8"
		default:
			contentType = "text/plain; charset=utf-8"
		}
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("subscription-userinfo", service.SubscriptionUserInfoHeader(user))
	w.Header().Set("profile-web-page-url", s.subscribeBaseURL(r, nil))
	w.Header().Set("content-disposition", `attachment; filename="subscription"`)
	w.Header().Set("profile-update-interval", "24")
	_, _ = io.WriteString(w, body)
}

func (s *Server) serverUniProxyConfig(w http.ResponseWriter, r *http.Request) {
	node, err := s.authorizeNodeRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	response := s.buildXrayRNodeConfig(node)
	response["node"] = node
	response["base_config"] = map[string]any{
		"push_interval": 60,
		"pull_interval": 60,
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) buildXrayRNodeConfig(node model.Server) map[string]any {
	network := firstNonEmpty(node.Network, "tcp")
	if node.Type == "vmess" || node.Type == "vless" {
		network = firstNonEmpty(node.Network, "ws")
	}
	hostHeader := firstNonEmpty(node.HostHeader, node.ServerName, node.Host)
	networkSettings := map[string]any{
		"path":        node.Path,
		"host":        hostHeader,
		"serviceName": strings.TrimPrefix(node.Path, "/"),
	}
	switch network {
	case "ws", "httpupgrade", "splithttp":
		networkSettings["headers"] = map[string]any{
			"Host": hostHeader,
		}
	case "tcp":
		networkSettings["header"] = map[string]any{
			"type": "none",
		}
	}

	tlsValue := 0
	if node.TLS {
		tlsValue = 1
	}

	response := map[string]any{
		"server_port": node.Port,
		"cipher":      firstNonEmpty(node.Cipher, "aes-256-gcm"),
		"network":     network,
		"networkSettings": map[string]any{
			"path":        networkSettings["path"],
			"host":        networkSettings["host"],
			"headers":     networkSettings["headers"],
			"serviceName": networkSettings["serviceName"],
			"header":      networkSettings["header"],
		},
		"network_settings": map[string]any{
			"path":        networkSettings["path"],
			"host":        networkSettings["host"],
			"headers":     networkSettings["headers"],
			"serviceName": networkSettings["serviceName"],
			"header":      networkSettings["header"],
		},
		"tls": tlsValue,
		"tls_settings": map[string]any{
			"server_port": strconv.Itoa(node.Port),
			"dest":        firstNonEmpty(node.Host, node.ServerName),
			"server_name": firstNonEmpty(node.ServerName, node.Host),
			"private_key": "",
			"short_id":    "",
		},
		"host":        firstNonEmpty(node.Host, node.ServerName),
		"server_name": firstNonEmpty(node.ServerName, node.Host),
		"obfs":        "",
		"obfs_settings": map[string]any{
			"path": node.Path,
			"host": hostHeader,
		},
		"server_key": "",
		"routes":     []any{},
	}
	if password := strings.TrimSpace(node.Password); password != "" {
		response["server_key"] = password
	}
	return response
}

func (s *Server) serverUniProxyUsers(w http.ResponseWriter, r *http.Request) {
	node, err := s.authorizeNodeRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	users, err := s.availableUsersForNode(r.Context(), node.ID)
	if err != nil {
		http.Error(w, "read users failed", http.StatusInternalServerError)
		return
	}
	hash := md5.Sum([]byte(fmt.Sprintf("%v", users)))
	eTag := `"` + hex.EncodeToString(hash[:]) + `"`
	if r.Header.Get("If-None-Match") == eTag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("ETag", eTag)
	writeJSON(w, http.StatusOK, map[string]any{
		"users": users,
	})
}

func (s *Server) serverUniProxyPush(w http.ResponseWriter, r *http.Request) {
	if _, err := s.authorizeNodeRequest(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	}
	items := decodeTrafficItems(raw)
	for _, item := range items {
		userID := payloadFirstInt64(item, "user_id", "uid", "id")
		if userID == 0 {
			if uuid := item.String("uuid"); uuid != "" {
				users, _ := s.store.ListAllUsers(r.Context())
				for _, candidate := range users {
					if candidate.UUID == uuid {
						userID = candidate.ID
						break
					}
				}
			}
		}
		if userID == 0 {
			continue
		}
		upload := payloadFirstInt64(item, "u", "up", "upload")
		download := payloadFirstInt64(item, "d", "down", "download")
		_ = s.store.AddTrafficByUserID(r.Context(), userID, upload, download)
	}
	s.invalidateNodeUsersCache(r.Context())
	success(w, true)
}

func (s *Server) serverUniProxyAlive(w http.ResponseWriter, r *http.Request) {
	node, err := s.authorizeNodeRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	node.IsOnline = true
	node.LastCheckAt = time.Now().Unix()
	if err := s.store.UpdateServer(r.Context(), node); err != nil {
		http.Error(w, "save node state failed", http.StatusInternalServerError)
		return
	}
	success(w, true)
}

func (s *Server) agentXrayRConfig(w http.ResponseWriter, r *http.Request) {
	s.serverUniProxyConfig(w, r)
}

func (s *Server) agentXrayRUsers(w http.ResponseWriter, r *http.Request) {
	s.serverUniProxyUsers(w, r)
}

func (s *Server) agentXrayRTraffic(w http.ResponseWriter, r *http.Request) {
	s.serverUniProxyPush(w, r)
}

func (s *Server) agentXrayRAlive(w http.ResponseWriter, r *http.Request) {
	s.serverUniProxyAlive(w, r)
}

func (s *Server) clientAppConfig(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"name":    s.cfg.AppName,
		"version": "0.1.0",
	})
}

func (s *Server) clientAppVersion(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"version": "0.1.0",
	})
}

func (s *Server) adminConfigFetch(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.AppSettings(r.Context())
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取配置失败", nil)
		return
	}
	success(w, buildAdminConfigResponse(r.URL.Query().Get("key"), settings))
}

func (s *Server) adminConfigSave(w http.ResponseWriter, r *http.Request) {
	payload, err := readPayload(r)
	if err != nil {
		fail(w, http.StatusBadRequest, "参数错误", nil)
		return
	}
	settings := map[string]string{}
	for key, raw := range payload {
		if key == "site_url" {
			key = "app_url"
		}
		if !isSupportedAdminSettingKey(key) {
			continue
		}
		switch value := raw.(type) {
		case nil:
			if key == "secure_path" {
				settings[key] = normalizeAdminPath("")
				continue
			}
			settings[key] = ""
		case string:
			if key == "secure_path" {
				value = normalizeAdminPath(value)
				if !isValidAdminPath(value) {
					fail(w, http.StatusBadRequest, "secure_path 不合法", nil)
					return
				}
				settings[key] = value
				continue
			}
			settings[key] = strings.TrimSpace(value)
		case []string:
			encoded, _ := json.Marshal(value)
			settings[key] = string(encoded)
		case []any:
			items := make([]string, 0, len(value))
			for _, item := range value {
				item = strings.TrimSpace(fmt.Sprint(item))
				if item == "" {
					continue
				}
				items = append(items, fmt.Sprint(item))
			}
			encoded, _ := json.Marshal(items)
			settings[key] = string(encoded)
		case json.Number, float64, int, int64, bool:
			settings[key] = strings.TrimSpace(fmt.Sprint(value))
		}
	}
	if len(settings) == 0 {
		success(w, true)
		return
	}
	if err := s.store.UpdateSettings(r.Context(), settings); err != nil {
		fail(w, http.StatusInternalServerError, "保存失败", nil)
		return
	}
	success(w, true)
}

func (s *Server) adminPlanFetch(w http.ResponseWriter, r *http.Request) {
	plans, err := s.store.ListPlans(r.Context(), false)
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取套餐失败", nil)
		return
	}
	counts, _ := s.store.PlanUserCounts(r.Context())
	for index := range plans {
		plans[index].TransferEnable = bytesToGiBValue(plans[index].TransferEnable)
		plans[index].Count = counts[plans[index].ID]
	}
	success(w, plans)
}

func (s *Server) adminPlanSave(w http.ResponseWriter, r *http.Request) {
	payload, err := readPayload(r)
	if err != nil {
		fail(w, http.StatusBadRequest, "参数错误", nil)
		return
	}
	plan := model.Plan{
		ID:             payload.Int64("id"),
		Name:           payload.StringDefault("name", "New Plan"),
		Price:          payload.Float64("price"),
		TransferEnable: normalizeAdminPlanTransferLimit(payload.Int64Default("transfer_enable", 128)),
		SpeedLimit:     payload.Int64Default("speed_limit", 100),
		Show:           payload.BoolDefault("show", true),
		Sort:           payload.Int64Default("sort", 99),
		GroupID:        payload.Int64Default("group_id", 1),
		Content:        payload.String("content"),
	}
	if plan.ID > 0 {
		if err := s.store.UpdatePlan(r.Context(), plan); err != nil {
			fail(w, http.StatusInternalServerError, "保存失败", nil)
			return
		}
	} else {
		if err := s.store.CreatePlan(r.Context(), plan); err != nil {
			fail(w, http.StatusInternalServerError, "创建失败", nil)
			return
		}
	}
	success(w, true)
}

func (s *Server) adminPlanDrop(w http.ResponseWriter, r *http.Request) {
	payload, _ := readPayload(r)
	if err := s.store.DeletePlan(r.Context(), payload.Int64("id")); err != nil {
		fail(w, http.StatusInternalServerError, "删除失败", nil)
		return
	}
	success(w, true)
}

func (s *Server) adminPlanSort(w http.ResponseWriter, r *http.Request) {
	payload, _ := readPayload(r)
	planIDs := payload.Int64Slice("plan_ids")
	if len(planIDs) == 0 {
		planIDs = payload.Int64Slice("ids")
	}
	if err := s.store.SortPlans(r.Context(), planIDs); err != nil {
		fail(w, http.StatusInternalServerError, "排序失败", nil)
		return
	}
	success(w, true)
}

func (s *Server) adminUserFetch(w http.ResponseWriter, r *http.Request) {
	payload, _ := readPayload(r)
	page := payload.IntDefault("current", 1)
	pageSize := payload.IntDefault("pageSize", 10)
	sortField := firstNonEmpty(payload.String("sort"), payload.String("sort_field"))
	desc := strings.EqualFold(firstNonEmpty(payload.String("sort_type"), payload.String("order")), "DESC")

	users, total, err := s.store.ListUsers(r.Context(), page, pageSize, sortField, desc)
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取用户失败", nil)
		return
	}
	plans, _ := s.store.ListPlans(r.Context(), false)
	planNames := make(map[int64]string, len(plans))
	for _, plan := range plans {
		planNames[plan.ID] = plan.Name
	}

	settings, _ := s.store.AppSettings(r.Context())
	baseURL := s.subscribeBaseURL(r, settings)
	rows := make([]map[string]any, 0, len(users))
	for _, user := range users {
		subscribeURL := ""
		subscribeSuffix := ""
		speedLimit := int64(0)
		if primaryLink, err := s.store.PrimarySubscriptionLinkByUserID(r.Context(), user.ID); err == nil {
			subscribeSuffix = primaryLink.Suffix
			subscribeURL = service.BuildSubscriptionURL(baseURL, primaryLink.Suffix)
		}
		if plan, err := s.store.PlanByID(r.Context(), user.PlanID); err == nil {
			speedLimit = plan.SpeedLimit
		}
		rows = append(rows, map[string]any{
			"id":                 user.ID,
			"email":              user.Email,
			"uuid":               user.UUID,
			"token":              user.Token,
			"is_admin":           user.IsAdmin,
			"is_staff":           user.IsStaff,
			"banned":             user.Banned,
			"transfer_enable":    user.TransferEnable,
			"u":                  user.U,
			"d":                  user.D,
			"total_used":         user.U + user.D,
			"expired_at":         user.ExpiredAt,
			"plan_id":            user.PlanID,
			"plan_name":          planNames[user.PlanID],
			"subscribe_url":      subscribeURL,
			"subscribe_suffix":   subscribeSuffix,
			"device_limit":       0,
			"speed_limit":        speedLimit,
			"last_reset_at":      0,
			"next_reset_at":      0,
			"reset_count":        0,
			"created_at":         user.CreatedAt,
			"last_login_at":      user.LastLoginAt,
			"balance":            user.Balance,
			"commission_balance": user.CommissionBalance,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":  rows,
		"total": total,
	})
}

func (s *Server) adminUserInfo(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	user, err := s.store.UserByID(r.Context(), id)
	if err != nil {
		fail(w, http.StatusNotFound, "用户不存在", nil)
		return
	}
	success(w, user)
}

func (s *Server) adminUserUpdate(w http.ResponseWriter, r *http.Request) {
	payload, err := readPayload(r)
	if err != nil {
		fail(w, http.StatusBadRequest, "参数错误", nil)
		return
	}

	user, err := s.store.UserByID(r.Context(), payload.Int64("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "用户不存在", nil)
		return
	}

	if value := payload.String("email"); value != "" {
		user.Email = value
	}
	user.Banned = payload.BoolDefault("banned", user.Banned)
	user.IsAdmin = payload.BoolDefault("is_admin", user.IsAdmin)
	user.IsStaff = payload.BoolDefault("is_staff", user.IsStaff)
	user.TransferEnable = payload.Int64Default("transfer_enable", user.TransferEnable)
	user.ExpiredAt = payload.Int64Default("expired_at", user.ExpiredAt)
	if planID := payload.Int64("plan_id"); planID > 0 {
		user.PlanID = planID
		if plan, err := s.store.PlanByID(r.Context(), planID); err == nil {
			user.GroupID = plan.GroupID
		}
	}
	if err := s.store.UpdateUser(r.Context(), user); err != nil {
		fail(w, http.StatusInternalServerError, "保存失败", nil)
		return
	}
	if password := payload.String("password"); password != "" {
		if err := s.store.UpdateUserPassword(r.Context(), user.ID, password); err != nil {
			fail(w, http.StatusInternalServerError, "保存失败", nil)
			return
		}
	}
	if user.Banned {
		_ = s.auth.RemoveAllSessions(r.Context(), user.ID)
	}
	success(w, true)
}

func (s *Server) adminUserGenerate(w http.ResponseWriter, r *http.Request) {
	payload, err := readPayload(r)
	if err != nil {
		fail(w, http.StatusBadRequest, "参数错误", nil)
		return
	}

	count := payload.IntDefault("generate_count", 1)
	email := payload.String("email")
	prefix := payload.String("email_prefix")
	suffix := payload.StringDefault("email_suffix", "example.com")
	password := payload.StringDefault("password", "ChangeMe123!")
	planID := payload.Int64("plan_id")
	plan, _ := s.store.PlanByID(r.Context(), planID)

	for i := 0; i < count; i++ {
		currentEmail := email
		if currentEmail == "" {
			currentEmail = fmt.Sprintf("%s%s@%s", prefix, newToken()[:6], suffix)
		}
		if _, err := s.store.CreateUser(r.Context(), model.User{
			Email:          currentEmail,
			RemindExpire:   true,
			RemindTraffic:  true,
			TransferEnable: plan.TransferEnable,
			ExpiredAt:      payload.Int64Default("expired_at", time.Now().Add(30*24*time.Hour).Unix()),
			PlanID:         plan.ID,
			GroupID:        plan.GroupID,
		}, password); err != nil {
			fail(w, http.StatusInternalServerError, "生成失败", nil)
			return
		}
		email = ""
	}
	success(w, true)
}

func (s *Server) adminSubscriptionFetch(w http.ResponseWriter, r *http.Request) {
	userID, _ := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
	if userID == 0 {
		fail(w, http.StatusBadRequest, "缺少用户ID", nil)
		return
	}
	links, err := s.store.ListSubscriptionLinksByUserID(r.Context(), userID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取订阅链接失败", nil)
		return
	}
	settings, _ := s.store.AppSettings(r.Context())
	baseURL := s.subscribeBaseURL(r, settings)
	rows := make([]map[string]any, 0, len(links))
	for _, link := range links {
		rows = append(rows, map[string]any{
			"id":           link.ID,
			"user_id":      link.UserID,
			"name":         link.Name,
			"suffix":       link.Suffix,
			"is_primary":   link.IsPrimary,
			"enabled":      link.Enabled,
			"last_used_at": link.LastUsedAt,
			"urls":         service.BuildSubscriptionVariants(baseURL, link.Suffix),
		})
	}
	success(w, rows)
}

func (s *Server) adminSubscriptionSave(w http.ResponseWriter, r *http.Request) {
	payload, err := readPayload(r)
	if err != nil {
		fail(w, http.StatusBadRequest, "参数错误", nil)
		return
	}
	link := model.SubscriptionLink{
		ID:        payload.Int64("id"),
		UserID:    payload.Int64("user_id"),
		Name:      payload.StringDefault("name", "Subscription"),
		Suffix:    payload.String("suffix"),
		IsPrimary: payload.BoolDefault("is_primary", false),
		Enabled:   payload.BoolDefault("enabled", true),
	}
	if link.UserID == 0 {
		fail(w, http.StatusBadRequest, "缺少用户ID", nil)
		return
	}
	if link.ID > 0 {
		current, err := s.store.SubscriptionLinkByID(r.Context(), link.ID)
		if err != nil {
			fail(w, http.StatusNotFound, "订阅链接不存在", nil)
			return
		}
		current.Name = link.Name
		if link.Suffix != "" {
			current.Suffix = link.Suffix
		}
		current.IsPrimary = link.IsPrimary
		current.Enabled = link.Enabled
		if err := s.store.UpdateSubscriptionLink(r.Context(), current); err != nil {
			fail(w, http.StatusInternalServerError, "保存失败", nil)
			return
		}
		success(w, true)
		return
	}
	if _, err := s.store.CreateSubscriptionLink(r.Context(), link); err != nil {
		fail(w, http.StatusInternalServerError, "创建失败，后缀可能重复", nil)
		return
	}
	success(w, true)
}

func (s *Server) adminSubscriptionDrop(w http.ResponseWriter, r *http.Request) {
	payload, _ := readPayload(r)
	if err := s.store.DeleteSubscriptionLink(r.Context(), payload.Int64("id")); err != nil {
		fail(w, http.StatusInternalServerError, "删除失败", nil)
		return
	}
	success(w, true)
}

func (s *Server) adminSubscriptionReset(w http.ResponseWriter, r *http.Request) {
	payload, _ := readPayload(r)
	userID := payload.Int64("user_id")
	if userID == 0 {
		fail(w, http.StatusBadRequest, "缺少用户ID", nil)
		return
	}
	links, err := s.store.ResetUserSubscriptionLinks(r.Context(), userID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "重置失败", nil)
		return
	}
	success(w, map[string]any{
		"count": len(links),
	})
}

func (s *Server) adminUserBan(w http.ResponseWriter, r *http.Request) {
	payload, _ := readPayload(r)
	user, err := s.store.UserByID(r.Context(), payload.Int64("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "用户不存在", nil)
		return
	}
	user.Banned = !user.Banned
	if err := s.store.UpdateUser(r.Context(), user); err != nil {
		fail(w, http.StatusInternalServerError, "保存失败", nil)
		return
	}
	if user.Banned {
		_ = s.auth.RemoveAllSessions(r.Context(), user.ID)
	}
	success(w, true)
}

func (s *Server) adminUserResetSecret(w http.ResponseWriter, r *http.Request) {
	payload, _ := readPayload(r)
	user, err := s.store.UserByID(r.Context(), payload.Int64("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "用户不存在", nil)
		return
	}
	user.UUID = newUUID()
	user.Token = newToken()
	if err := s.store.UpdateUser(r.Context(), user); err != nil {
		fail(w, http.StatusInternalServerError, "保存失败", nil)
		return
	}
	if _, err := s.store.ResetUserSubscriptionLinks(r.Context(), user.ID); err != nil {
		fail(w, http.StatusInternalServerError, "重置订阅失败", nil)
		return
	}
	success(w, true)
}

func (s *Server) adminNoticeFetch(w http.ResponseWriter, r *http.Request) {
	notices, total, err := s.store.ListNotices(r.Context(), false, 1, 100)
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取公告失败", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":  notices,
		"total": total,
	})
}

func (s *Server) adminNoticeSave(w http.ResponseWriter, r *http.Request) {
	payload, err := readPayload(r)
	if err != nil {
		fail(w, http.StatusBadRequest, "参数错误", nil)
		return
	}
	notice := model.Notice{
		ID:      payload.Int64("id"),
		Title:   payload.StringDefault("title", "新公告"),
		Content: payload.String("content"),
		Show:    payload.BoolDefault("show", true),
	}
	if notice.ID > 0 {
		if err := s.store.UpdateNotice(r.Context(), notice); err != nil {
			fail(w, http.StatusInternalServerError, "保存失败", nil)
			return
		}
	} else {
		if err := s.store.CreateNotice(r.Context(), notice); err != nil {
			fail(w, http.StatusInternalServerError, "创建失败", nil)
			return
		}
	}
	success(w, true)
}

func (s *Server) adminNoticeDrop(w http.ResponseWriter, r *http.Request) {
	payload, _ := readPayload(r)
	if err := s.store.DeleteNotice(r.Context(), payload.Int64("id")); err != nil {
		fail(w, http.StatusInternalServerError, "删除失败", nil)
		return
	}
	success(w, true)
}

func (s *Server) adminServerFetch(w http.ResponseWriter, r *http.Request) {
	servers, err := s.store.ListServers(r.Context(), false)
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取节点失败", nil)
		return
	}
	success(w, servers)
}

func (s *Server) adminServerSave(serverType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload, err := readPayload(r)
		if err != nil {
			fail(w, http.StatusBadRequest, "参数错误", nil)
			return
		}
		item := model.Server{
			ID:            payload.Int64("id"),
			Name:          payload.StringDefault("name", strings.ToUpper(serverType)+" Node"),
			Type:          serverType,
			Version:       payload.IntDefault("version", 1),
			Host:          payload.StringDefault("host", "example.com"),
			Port:          payload.IntDefault("port", 443),
			Network:       payload.StringDefault("network", "ws"),
			Path:          payload.String("path"),
			HostHeader:    payload.String("host_header"),
			TLS:           payload.BoolDefault("tls", true),
			ServerName:    payload.String("server_name"),
			AllowInsecure: payload.BoolDefault("allow_insecure", false),
			Cipher:        payload.StringDefault("cipher", "aes-256-gcm"),
			Password:      payload.String("password"),
			Rate:          payload.Float64Default("rate", 1),
			Tags:          payload.StringSlice("tags"),
			PlanIDs:       payload.Int64Slice("plan_ids"),
			IsOnline:      payload.BoolDefault("is_online", true),
			Show:          payload.BoolDefault("show", true),
			Sort:          payload.Int64Default("sort", 99),
		}
		if item.ID > 0 {
			if err := s.store.UpdateServer(r.Context(), item); err != nil {
				fail(w, http.StatusInternalServerError, "保存失败", nil)
				return
			}
		} else {
			if err := s.store.CreateServer(r.Context(), item); err != nil {
				fail(w, http.StatusInternalServerError, "创建失败", nil)
				return
			}
		}
		success(w, true)
	}
}

func (s *Server) adminServerDrop(w http.ResponseWriter, r *http.Request) {
	payload, _ := readPayload(r)
	if err := s.store.DeleteServer(r.Context(), payload.Int64("id")); err != nil {
		fail(w, http.StatusInternalServerError, "删除失败", nil)
		return
	}
	success(w, true)
}

func (s *Server) adminServerCopy(w http.ResponseWriter, r *http.Request) {
	payload, _ := readPayload(r)
	item, err := s.store.ServerByID(r.Context(), payload.Int64("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "节点不存在", nil)
		return
	}
	item.ID = 0
	item.Name = item.Name + " Copy"
	if err := s.store.CreateServer(r.Context(), item); err != nil {
		fail(w, http.StatusInternalServerError, "复制失败", nil)
		return
	}
	success(w, true)
}

func (s *Server) adminStatGetStat(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.DashboardStats(r.Context())
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取统计失败", nil)
		return
	}
	success(w, stats)
}

func (s *Server) adminSystemStatus(w http.ResponseWriter, r *http.Request) {
	stats, _ := s.store.DashboardStats(r.Context())
	success(w, map[string]any{
		"uptime":  "running",
		"users":   stats["users"],
		"plans":   stats["plans"],
		"servers": stats["servers"],
		"db":      "sqlite",
	})
}

func (s *Server) userMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := s.auth.ParseUser(r.Context(), extractAuthData(r))
		if err != nil {
			fail(w, http.StatusForbidden, "未登录或登陆已过期", nil)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	})
}

func (s *Server) adminPathMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		settings, err := s.store.AppSettings(r.Context())
		if err != nil {
			fail(w, http.StatusInternalServerError, "读取配置失败", nil)
			return
		}
		if chi.URLParam(r, "adminPath") != s.configuredAdminPath(settings) {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) adminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := s.auth.ParseUser(r.Context(), extractAuthData(r))
		if err != nil || !user.IsAdmin {
			fail(w, http.StatusForbidden, "未登录或登陆已过期", nil)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	})
}

func mustUser(r *http.Request) model.User {
	user, _ := r.Context().Value(userContextKey).(model.User)
	return user
}

func extractAuthData(r *http.Request) string {
	if raw := strings.TrimSpace(r.Header.Get("Authorization")); raw != "" {
		return raw
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("auth_data")); raw != "" {
		return raw
	}
	_ = r.ParseForm()
	return strings.TrimSpace(r.Form.Get("auth_data"))
}

func supportedAdminSettings(settings map[string]string) map[string]string {
	filtered := make(map[string]string)
	for key, value := range settings {
		if isSupportedAdminSettingKey(key) {
			filtered[key] = value
		}
	}
	return filtered
}

func isSupportedAdminSettingKey(key string) bool {
	switch key {
	case "app_name",
		"app_description",
		"app_url",
		"logo",
		"subscribe_url",
		"tos_url",
		"secure_path",
		"server_token",
		"server_pull_interval",
		"server_push_interval",
		"device_limit_mode",
		"server_ws_enable",
		"server_ws_url",
		"email_verify",
		"safe_mode_enable",
		"email_whitelist_enable",
		"email_whitelist_suffix",
		"email_gmail_limit_enable",
		"captcha_enable",
		"captcha_type",
		"recaptcha_key",
		"recaptcha_site_key",
		"recaptcha_v3_secret_key",
		"recaptcha_v3_site_key",
		"recaptcha_v3_score_threshold",
		"turnstile_secret_key",
		"turnstile_site_key",
		"register_limit_by_ip_enable",
		"register_limit_count",
		"register_limit_expire",
		"password_limit_enable",
		"password_limit_count",
		"password_limit_expire",
		"force_https",
		"stop_register",
		"ticket_must_wait_reply",
		"try_out_plan_id",
		"try_out_hour",
		"currency",
		"currency_symbol",
		"windows_version",
		"windows_download_url",
		"macos_version",
		"macos_download_url",
		"android_version",
		"android_download_url",
		"theme_color",
		"background_url",
		"custom_html":
		return true
	default:
		return false
	}
}

func buildAdminConfigResponse(key string, settings map[string]string) map[string]any {
	switch strings.TrimSpace(strings.ToLower(key)) {
	case "site":
		siteURL := settingString(settings, "app_url", "")
		return map[string]any{
			"site": map[string]any{
				"logo":                   settingString(settings, "logo", ""),
				"force_https":            settingInt(settings, "force_https", 0),
				"stop_register":          settingInt(settings, "stop_register", 0),
				"ticket_must_wait_reply": settingInt(settings, "ticket_must_wait_reply", 0),
				"app_name":               settingString(settings, "app_name", ""),
				"app_description":        settingString(settings, "app_description", ""),
				"app_url":                siteURL,
				"site_url":               siteURL,
				"subscribe_url":          settingString(settings, "subscribe_url", ""),
				"try_out_plan_id":        settingInt(settings, "try_out_plan_id", 0),
				"try_out_hour":           settingInt(settings, "try_out_hour", 0),
				"tos_url":                settingString(settings, "tos_url", ""),
				"currency":               settingString(settings, "currency", "CNY"),
				"currency_symbol":        settingString(settings, "currency_symbol", "¥"),
			},
		}
	case "safe":
		return map[string]any{
			"safe": map[string]any{
				"email_verify":                 settingBool(settings, "email_verify", false),
				"safe_mode_enable":             settingBool(settings, "safe_mode_enable", false),
				"secure_path":                  settingString(settings, "secure_path", normalizeAdminPath("")),
				"email_whitelist_enable":       settingBool(settings, "email_whitelist_enable", false),
				"email_whitelist_suffix":       settingStringSlice(settings, "email_whitelist_suffix"),
				"email_gmail_limit_enable":     settingBool(settings, "email_gmail_limit_enable", false),
				"captcha_enable":               settingBool(settings, "captcha_enable", false),
				"captcha_type":                 settingString(settings, "captcha_type", "recaptcha"),
				"recaptcha_key":                settingString(settings, "recaptcha_key", ""),
				"recaptcha_site_key":           settingString(settings, "recaptcha_site_key", ""),
				"recaptcha_v3_secret_key":      settingString(settings, "recaptcha_v3_secret_key", ""),
				"recaptcha_v3_site_key":        settingString(settings, "recaptcha_v3_site_key", ""),
				"recaptcha_v3_score_threshold": settingString(settings, "recaptcha_v3_score_threshold", "0.5"),
				"turnstile_secret_key":         settingString(settings, "turnstile_secret_key", ""),
				"turnstile_site_key":           settingString(settings, "turnstile_site_key", ""),
				"register_limit_by_ip_enable":  settingBool(settings, "register_limit_by_ip_enable", false),
				"register_limit_count":         settingString(settings, "register_limit_count", ""),
				"register_limit_expire":        settingString(settings, "register_limit_expire", ""),
				"password_limit_enable":        settingBool(settings, "password_limit_enable", false),
				"password_limit_count":         settingString(settings, "password_limit_count", ""),
				"password_limit_expire":        settingString(settings, "password_limit_expire", ""),
			},
		}
	case "server":
		return map[string]any{
			"server": map[string]any{
				"server_pull_interval": settingInt(settings, "server_pull_interval", 0),
				"server_push_interval": settingInt(settings, "server_push_interval", 0),
				"server_token":         settingString(settings, "server_token", ""),
				"device_limit_mode":    settingInt(settings, "device_limit_mode", 0),
				"server_ws_enable":     settingBool(settings, "server_ws_enable", false),
				"server_ws_url":        settingString(settings, "server_ws_url", ""),
			},
		}
	case "app":
		return map[string]any{
			"app": map[string]any{
				"windows_version":      settingString(settings, "windows_version", ""),
				"windows_download_url": settingString(settings, "windows_download_url", ""),
				"macos_version":        settingString(settings, "macos_version", ""),
				"macos_download_url":   settingString(settings, "macos_download_url", ""),
				"android_version":      settingString(settings, "android_version", ""),
				"android_download_url": settingString(settings, "android_download_url", ""),
			},
		}
	default:
		flat := make(map[string]any)
		for key, value := range supportedAdminSettings(settings) {
			flat[key] = value
		}
		return flat
	}
}

func settingString(settings map[string]string, key, fallback string) string {
	if settings == nil {
		return fallback
	}
	if value, ok := settings[key]; ok {
		return strings.TrimSpace(value)
	}
	return fallback
}

func settingInt(settings map[string]string, key string, fallback int) int {
	value := settingString(settings, key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func settingBool(settings map[string]string, key string, fallback bool) bool {
	value := strings.ToLower(settingString(settings, key, ""))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off", "":
		return fallback && value == ""
	default:
		return fallback
	}
}

func settingStringSlice(settings map[string]string, key string) []string {
	value := settingString(settings, key, "")
	if value == "" {
		return []string{}
	}
	var items []string
	if err := json.Unmarshal([]byte(value), &items); err == nil {
		return compactStrings(items)
	}
	return compactStrings(strings.Split(value, ","))
}

func compactStrings(items []string) []string {
	compacted := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		compacted = append(compacted, item)
	}
	return compacted
}

func (s *Server) subscribeBaseURL(r *http.Request, settings map[string]string) string {
	if settings != nil {
		if value := strings.TrimSpace(settings["subscribe_url"]); value != "" {
			return strings.TrimRight(value, "/")
		}
		if value := strings.TrimSpace(settings["app_url"]); value != "" {
			return strings.TrimRight(value, "/")
		}
	}
	if strings.TrimSpace(s.cfg.AppURL) != "" {
		return strings.TrimRight(s.cfg.AppURL, "/")
	}
	return origin(r)
}

func origin(r *http.Request) string {
	scheme := "http"
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		scheme = forwarded
	} else if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func (s *Server) authorizeNodeRequest(r *http.Request) (model.Server, error) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	nodeID, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("node_id")), 10, 64)
	if token == "" || nodeID == 0 {
		payload, err := readPayload(r)
		if err != nil {
			return model.Server{}, err
		}
		if token == "" {
			token = payload.String("token")
		}
		if nodeID == 0 {
			nodeID = payload.Int64("node_id")
		}
	}
	settings, err := s.store.AppSettings(r.Context())
	if err != nil {
		return model.Server{}, err
	}
	if token == "" || token != settings["server_token"] {
		return model.Server{}, fmt.Errorf("invalid server token")
	}
	if nodeID == 0 {
		return model.Server{}, fmt.Errorf("missing node_id")
	}
	node, err := s.store.ServerByID(r.Context(), nodeID)
	if err != nil {
		return model.Server{}, err
	}
	return node, nil
}

func (s *Server) invalidateNodeUsersCache(ctx context.Context) {
	if s.cache == nil || !s.cache.Enabled() {
		return
	}
	servers, err := s.store.ListServers(ctx, false)
	if err != nil {
		return
	}
	keys := make([]string, 0, len(servers))
	for _, server := range servers {
		keys = append(keys, fmt.Sprintf("node_users:%d", server.ID))
	}
	_ = s.cache.Delete(ctx, keys...)
}

func (s *Server) availableUsersForNode(ctx context.Context, nodeID int64) ([]map[string]any, error) {
	cacheKey := fmt.Sprintf("node_users:%d", nodeID)
	if s.cache != nil && s.cache.Enabled() {
		var cached []map[string]any
		if err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil {
			return cached, nil
		}
	}

	node, err := s.store.ServerByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	users, err := s.store.ListAllUsers(ctx)
	if err != nil {
		return nil, err
	}
	plans, _ := s.store.ListPlans(ctx, false)
	planSpeed := make(map[int64]int64, len(plans))
	for _, plan := range plans {
		planSpeed[plan.ID] = plan.SpeedLimit
	}
	var result []map[string]any
	now := time.Now().Unix()
	for _, user := range users {
		if user.Banned {
			continue
		}
		if user.ExpiredAt > 0 && user.ExpiredAt < now {
			continue
		}
		if len(node.PlanIDs) > 0 && !containsInt64(node.PlanIDs, user.PlanID) {
			continue
		}
		result = append(result, map[string]any{
			"id":              user.ID,
			"uuid":            user.UUID,
			"email":           user.Email,
			"token":           user.Token,
			"speed_limit":     planSpeed[user.PlanID],
			"u":               user.U,
			"d":               user.D,
			"transfer_enable": user.TransferEnable,
			"expired_at":      user.ExpiredAt,
		})
	}
	if s.cache != nil && s.cache.Enabled() {
		_ = s.cache.SetJSON(ctx, cacheKey, result, nodeUsersCacheTTL)
	}
	return result, nil
}

func success(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "操作成功",
		"data":    data,
		"error":   nil,
	})
}

func fail(w http.ResponseWriter, status int, message string, details any) {
	writeJSON(w, status, map[string]any{
		"status":  "fail",
		"message": message,
		"data":    nil,
		"error":   details,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(normalizeJSONPayload(status, data))
}

func normalizeJSONPayload(status int, data any) any {
	mapped, ok := data.(map[string]any)
	if !ok {
		return data
	}
	result := make(map[string]any, len(mapped)+2)
	for key, value := range mapped {
		result[key] = value
	}
	if _, exists := result["code"]; !exists {
		if status >= 200 && status < 300 {
			result["code"] = http.StatusOK
		} else {
			result["code"] = status
		}
	}
	if _, exists := result["message"]; !exists {
		if status >= 200 && status < 300 {
			result["message"] = "操作成功"
		} else {
			result["message"] = http.StatusText(status)
		}
	}
	return result
}

func gravatarURL(email string) string {
	sum := md5.Sum([]byte(strings.TrimSpace(strings.ToLower(email))))
	return "https://www.gravatar.com/avatar/" + hex.EncodeToString(sum[:]) + "?s=64&d=identicon"
}

func (s *Server) successTrue(w http.ResponseWriter, r *http.Request) {
	success(w, true)
}

func (s *Server) emptySuccessList(w http.ResponseWriter, r *http.Request) {
	success(w, []any{})
}

func (s *Server) emptySuccessMap(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{})
}

func (s *Server) emptyRawList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"data":  []any{},
		"total": 0,
	})
}

type payload map[string]any

func readPayload(r *http.Request) (payload, error) {
	result := payload{}
	for key, values := range r.URL.Query() {
		if len(values) == 1 {
			result[key] = values[0]
		} else if len(values) > 1 {
			clone := make([]string, len(values))
			copy(clone, values)
			result[key] = clone
		}
	}

	if r.Method == http.MethodGet {
		return result, nil
	}
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		defer r.Body.Close()
		if r.Body == nil {
			return result, nil
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		if len(strings.TrimSpace(string(raw))) == 0 {
			return result, nil
		}
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			return nil, err
		}
		for key, value := range body {
			result[key] = value
		}
		return result, nil
	}

	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	for key, values := range r.Form {
		if len(values) == 1 {
			result[key] = values[0]
		} else if len(values) > 1 {
			clone := make([]string, len(values))
			copy(clone, values)
			result[key] = clone
		}
	}
	return result, nil
}

func (p payload) String(key string) string {
	value, ok := p[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		if math.Mod(typed, 1) == 0 {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(typed)
	}
}

func (p payload) StringDefault(key, fallback string) string {
	if value := p.String(key); value != "" {
		return value
	}
	return fallback
}

func (p payload) Int(key string) int {
	return int(p.Int64(key))
}

func (p payload) IntDefault(key string, fallback int) int {
	if _, ok := p[key]; !ok {
		return fallback
	}
	return int(p.Int64Default(key, int64(fallback)))
}

func (p payload) Int64(key string) int64 {
	return p.Int64Default(key, 0)
}

func (p payload) Int64Default(key string, fallback int64) int64 {
	value, ok := p[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return parsed
		}
	case string:
		if parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil {
			return parsed
		}
	case bool:
		if typed {
			return 1
		}
		return 0
	}
	return fallback
}

func (p payload) Float64(key string) float64 {
	return p.Float64Default(key, 0)
}

func (p payload) Float64Default(key string, fallback float64) float64 {
	value, ok := p[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		if parsed, err := typed.Float64(); err == nil {
			return parsed
		}
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func (p payload) BoolDefault(key string, fallback bool) bool {
	value, ok := p[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		typed = strings.TrimSpace(strings.ToLower(typed))
		return typed == "1" || typed == "true" || typed == "yes" || typed == "on"
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return fallback
	}
}

func (p payload) StringSlice(key string) []string {
	value, ok := p[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, strings.TrimSpace(fmt.Sprint(item)))
		}
		return out
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		parts := strings.FieldsFunc(typed, func(r rune) bool {
			return r == ',' || r == '|' || r == '，'
		})
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
		return out
	default:
		return nil
	}
}

func (p payload) Int64Slice(key string) []int64 {
	value, ok := p[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []any:
		var out []int64
		for _, item := range typed {
			out = append(out, payload{"_": item}.Int64("_"))
		}
		return out
	case []string:
		var out []int64
		for _, item := range typed {
			out = append(out, payload{"_": item}.Int64("_"))
		}
		return out
	case string:
		parts := p.StringSlice(key)
		var out []int64
		for _, part := range parts {
			out = append(out, payload{"_": part}.Int64("_"))
		}
		return out
	default:
		return nil
	}
}

func (p payload) ObjectSlice(key string) []payload {
	value, ok := p[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []payload:
		return typed
	case []map[string]any:
		out := make([]payload, 0, len(typed))
		for _, item := range typed {
			out = append(out, payload(item))
		}
		return out
	case []any:
		out := make([]payload, 0, len(typed))
		for _, item := range typed {
			if mapped, ok := item.(map[string]any); ok {
				out = append(out, payload(mapped))
			}
		}
		return out
	default:
		return nil
	}
}

func pick(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func containsInt64(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func bytesToGiBValue(value int64) int64 {
	const gib = int64(1024 * 1024 * 1024)
	if value <= 0 {
		return 0
	}
	if value < gib {
		return 1
	}
	return value / gib
}

func normalizeAdminPlanTransferLimit(value int64) int64 {
	const gib = int64(1024 * 1024 * 1024)
	if value <= 0 {
		return 0
	}
	if value >= gib {
		return value
	}
	return value * gib
}

func decodeTrafficItems(raw []byte) []payload {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}
	switch typed := decoded.(type) {
	case []any:
		var items []payload
		for _, entry := range typed {
			switch item := entry.(type) {
			case map[string]any:
				items = append(items, payload(item))
			case []any:
				if len(item) >= 3 {
					items = append(items, payload{
						"user_id": item[0],
						"u":       item[1],
						"d":       item[2],
					})
				}
			}
		}
		return items
	case map[string]any:
		payloadMap := payload(typed)
		items := payloadMap.ObjectSlice("users")
		if len(items) == 0 {
			items = payloadMap.ObjectSlice("data")
		}
		if len(items) == 0 {
			for key, value := range typed {
				switch entry := value.(type) {
				case []any:
					if len(entry) >= 2 {
						items = append(items, payload{
							"user_id": key,
							"u":       entry[0],
							"d":       entry[1],
						})
					}
				case map[string]any:
					current := payload(entry)
					if current.Int64("user_id") == 0 {
						current["user_id"] = key
					}
					items = append(items, current)
				}
			}
		}
		return items
	default:
		return nil
	}
}

func payloadFirstInt64(p payload, keys ...string) int64 {
	for _, key := range keys {
		if _, ok := p[key]; ok {
			return p.Int64(key)
		}
	}
	return 0
}
