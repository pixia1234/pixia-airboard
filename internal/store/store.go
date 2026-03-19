package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"

	"pixia-airboard/internal/cache"
	"pixia-airboard/internal/config"
	"pixia-airboard/internal/model"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	db    *sql.DB
	cache *cache.Cache
}

func New(ctx context.Context, cfg config.Config, cacheClient *cache.Cache) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", cfg.DBPath+"?_busy_timeout=10000&_journal_mode=WAL")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}

	store := &Store{db: db, cache: cacheClient}
	if err := store.bootstrap(ctx, cfg); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) bootstrap(ctx context.Context, cfg config.Config) error {
	if _, err := s.db.ExecContext(ctx, schemaSQL); err != nil {
		return err
	}
	return s.seed(ctx, cfg)
}

func (s *Store) seed(ctx context.Context, cfg config.Config) error {
	now := time.Now().Unix()
	settings := map[string]string{
		"app_name":                       cfg.AppName,
		"app_description":                "Go + SQLite subscription management panel",
		"app_url":                        cfg.AppURL,
		"logo":                           "PA",
		"subscribe_url":                  cfg.AppURL,
		"secure_path":                    cfg.AdminPath,
		"server_token":                   randomToken(18),
		"tos_url":                        "",
		"email_verify":                   "0",
		"invite_force":                   "0",
		"recaptcha_enable":               "0",
		"currency":                       "CNY",
		"currency_symbol":                "¥",
		"commission_distribution_enable": "0",
	}

	for key, value := range settings {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO settings(key, value, updated_at)
			VALUES(?, ?, ?)
			ON CONFLICT(key) DO NOTHING
		`, key, value, now); err != nil {
			return err
		}
	}

	var planCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM plans`).Scan(&planCount); err != nil {
		return err
	}
	if planCount == 0 {
		plans := []model.Plan{
			{Name: "Starter", Price: 19.9, TransferEnable: 128 * gigaByte, SpeedLimit: 100, Show: true, Sort: 1, GroupID: 1, Content: "适合轻量使用，含基础节点。"},
			{Name: "Pro", Price: 59.9, TransferEnable: 512 * gigaByte, SpeedLimit: 500, Show: true, Sort: 2, GroupID: 1, Content: "适合长期稳定使用，含全区域节点。"},
		}
		for _, plan := range plans {
			if err := s.CreatePlan(ctx, plan); err != nil {
				return err
			}
		}
	}

	var noticeCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM notices`).Scan(&noticeCount); err != nil {
		return err
	}
	if noticeCount == 0 {
		notices := []model.Notice{
			{Title: "欢迎使用 Pixia Airboard", Content: "这是一个使用 Go + SQLite 构建的订阅管理面板骨架，接口风格兼容 Xboard 常用用户侧 API。", Show: true},
			{Title: "默认管理员账号", Content: fmt.Sprintf("管理员账号：%s，密码：%s。建议首次登录后立即修改。", cfg.DefaultEmail, cfg.DefaultPass), Show: true},
		}
		for _, notice := range notices {
			if err := s.CreateNotice(ctx, notice); err != nil {
				return err
			}
		}
	}

	var serverCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM servers`).Scan(&serverCount); err != nil {
		return err
	}
	if serverCount == 0 {
		servers := []model.Server{
			{Name: "Tokyo Premium", Type: "vmess", Version: 1, Host: "tokyo.example.com", Port: 443, Network: "ws", Path: "/ray", TLS: true, ServerName: "tokyo.example.com", Rate: 1, Tags: []string{"Japan", "Premium"}, PlanIDs: []int64{1, 2}, IsOnline: true, Show: true, Sort: 1},
			{Name: "Singapore Edge", Type: "vless", Version: 1, Host: "sg.example.com", Port: 443, Network: "ws", Path: "/edge", TLS: true, ServerName: "sg.example.com", Rate: 1, Tags: []string{"Singapore", "Edge"}, PlanIDs: []int64{1, 2}, IsOnline: true, Show: true, Sort: 2},
			{Name: "Los Angeles Turbo", Type: "trojan", Version: 1, Host: "la.example.com", Port: 443, TLS: true, ServerName: "la.example.com", Password: "trojan-password", Rate: 1.2, Tags: []string{"USA", "Streaming"}, PlanIDs: []int64{2}, IsOnline: true, Show: true, Sort: 3},
			{Name: "Hong Kong SS", Type: "shadowsocks", Version: 1, Host: "hk.example.com", Port: 8388, Cipher: "aes-256-gcm", Password: "ss-password", Rate: 0.8, Tags: []string{"HongKong"}, PlanIDs: []int64{1, 2}, IsOnline: true, Show: true, Sort: 4},
		}
		for _, server := range servers {
			if err := s.CreateServer(ctx, server); err != nil {
				return err
			}
		}
	}

	var userCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM users`).Scan(&userCount); err != nil {
		return err
	}
	if userCount == 0 {
		admin, err := s.CreateUser(ctx, model.User{
			Email:          cfg.DefaultEmail,
			IsAdmin:        true,
			IsStaff:        true,
			RemindExpire:   true,
			RemindTraffic:  true,
			TransferEnable: 512 * gigaByte,
			ExpiredAt:      now + 365*24*3600,
			PlanID:         2,
			GroupID:        1,
		}, cfg.DefaultPass)
		if err != nil {
			return err
		}
		_, _ = s.CreateUser(ctx, model.User{
			Email:          "demo@example.com",
			IsAdmin:        false,
			IsStaff:        false,
			RemindExpire:   true,
			RemindTraffic:  true,
			TransferEnable: 128 * gigaByte,
			ExpiredAt:      now + 30*24*3600,
			PlanID:         1,
			GroupID:        1,
			InviteUserID:   admin.ID,
		}, "demo123456")
	}

	if err := s.EnsureDefaultSubscriptionLinks(ctx); err != nil {
		return err
	}

	return nil
}

func (s *Store) AppSettings(ctx context.Context) (map[string]string, error) {
	if s.cache != nil {
		var cached map[string]string
		if err := s.cache.GetJSON(ctx, settingsCacheKey, &cached); err == nil && len(cached) > 0 {
			return cached, nil
		}
	}

	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		settings[key] = value
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	_ = s.cacheSettings(ctx, settings)
	return settings, nil
}

func (s *Store) UpdateSettings(ctx context.Context, values map[string]string) error {
	now := time.Now().Unix()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for key, value := range values {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO settings(key, value, updated_at)
			VALUES(?, ?, ?)
			ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at
		`, key, value, now); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	if s.cache != nil {
		_ = s.cache.Delete(ctx, settingsCacheKey)
	}
	return nil
}

func (s *Store) CreateUser(ctx context.Context, user model.User, rawPassword string) (model.User, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, err
	}

	now := time.Now().Unix()
	if user.UUID == "" {
		user.UUID = newUUID()
	}
	if user.Token == "" {
		user.Token = randomToken(32)
	}
	if user.CreatedAt == 0 {
		user.CreatedAt = now
	}
	user.UpdatedAt = now

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO users(
			email, password_hash, uuid, token, is_admin, is_staff, banned,
			remind_expire, remind_traffic, transfer_enable, u, d, expired_at,
			balance, commission_balance, plan_id, group_id, invite_user_id,
			created_at, updated_at, last_login_at
		)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user.Email, string(passwordHash), user.UUID, user.Token, boolToInt(user.IsAdmin), boolToInt(user.IsStaff), boolToInt(user.Banned),
		boolToInt(user.RemindExpire), boolToInt(user.RemindTraffic), user.TransferEnable, user.U, user.D, user.ExpiredAt,
		user.Balance, user.CommissionBalance, user.PlanID, user.GroupID, user.InviteUserID,
		user.CreatedAt, user.UpdatedAt, user.LastLoginAt,
	)
	if err != nil {
		return model.User{}, err
	}
	id, _ := res.LastInsertId()
	createdUser, err := s.UserByID(ctx, id)
	if err != nil {
		return model.User{}, err
	}
	if _, err := s.CreateSubscriptionLink(ctx, model.SubscriptionLink{
		UserID:    createdUser.ID,
		Name:      "Default",
		Suffix:    s.randomSubscriptionSuffix(),
		IsPrimary: true,
		Enabled:   true,
	}); err != nil {
		return model.User{}, err
	}
	return createdUser, nil
}

func (s *Store) UserByID(ctx context.Context, id int64) (model.User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, uuid, token, is_admin, is_staff, banned,
			remind_expire, remind_traffic, transfer_enable, u, d, expired_at,
			balance, commission_balance, plan_id, group_id, invite_user_id,
			created_at, updated_at, last_login_at
		FROM users
		WHERE id = ?
	`, id)
	return scanUser(row)
}

func (s *Store) UserByEmail(ctx context.Context, email string) (model.User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, uuid, token, is_admin, is_staff, banned,
			remind_expire, remind_traffic, transfer_enable, u, d, expired_at,
			balance, commission_balance, plan_id, group_id, invite_user_id,
			created_at, updated_at, last_login_at
		FROM users
		WHERE lower(email) = lower(?)
	`, email)
	return scanUser(row)
}

func (s *Store) UserByToken(ctx context.Context, token string) (model.User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, uuid, token, is_admin, is_staff, banned,
			remind_expire, remind_traffic, transfer_enable, u, d, expired_at,
			balance, commission_balance, plan_id, group_id, invite_user_id,
			created_at, updated_at, last_login_at
		FROM users
		WHERE token = ?
	`, token)
	return scanUser(row)
}

func (s *Store) UpdateUser(ctx context.Context, user model.User) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users SET
			email = ?, uuid = ?, token = ?, is_admin = ?, is_staff = ?, banned = ?,
			remind_expire = ?, remind_traffic = ?, transfer_enable = ?, u = ?, d = ?,
			expired_at = ?, balance = ?, commission_balance = ?, plan_id = ?, group_id = ?,
			invite_user_id = ?, updated_at = ?, last_login_at = ?
		WHERE id = ?
	`, user.Email, user.UUID, user.Token, boolToInt(user.IsAdmin), boolToInt(user.IsStaff), boolToInt(user.Banned),
		boolToInt(user.RemindExpire), boolToInt(user.RemindTraffic), user.TransferEnable, user.U, user.D, user.ExpiredAt,
		user.Balance, user.CommissionBalance, user.PlanID, user.GroupID, user.InviteUserID, time.Now().Unix(), user.LastLoginAt, user.ID,
	)
	return err
}

func (s *Store) UpdateUserPassword(ctx context.Context, userID int64, rawPassword string) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`, string(passwordHash), time.Now().Unix(), userID)
	return err
}

func (s *Store) CheckPassword(user model.User, rawPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(rawPassword))
}

func (s *Store) ListUsers(ctx context.Context, page, pageSize int, sort string, desc bool) ([]model.User, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	sort = safeSort(sort, map[string]bool{
		"id": true, "created_at": true, "last_login_at": true, "email": true,
	})
	order := "ASC"
	if desc {
		order = "DESC"
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM users`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, email, password_hash, uuid, token, is_admin, is_staff, banned,
			remind_expire, remind_traffic, transfer_enable, u, d, expired_at,
			balance, commission_balance, plan_id, group_id, invite_user_id,
			created_at, updated_at, last_login_at
		FROM users
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, sort, order), pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		user, err := scanUserFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, user)
	}
	return users, total, rows.Err()
}

func (s *Store) ListAllUsers(ctx context.Context) ([]model.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, email, password_hash, uuid, token, is_admin, is_staff, banned,
			remind_expire, remind_traffic, transfer_enable, u, d, expired_at,
			balance, commission_balance, plan_id, group_id, invite_user_id,
			created_at, updated_at, last_login_at
		FROM users
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		user, err := scanUserFromRows(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Store) CreatePlan(ctx context.Context, plan model.Plan) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO plans(name, price, transfer_enable, speed_limit, show, sort, group_id, content, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, plan.Name, plan.Price, plan.TransferEnable, plan.SpeedLimit, boolToInt(plan.Show), plan.Sort, plan.GroupID, plan.Content, now, now)
	return err
}

func (s *Store) UpdatePlan(ctx context.Context, plan model.Plan) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE plans
		SET name = ?, price = ?, transfer_enable = ?, speed_limit = ?, show = ?, sort = ?, group_id = ?, content = ?, updated_at = ?
		WHERE id = ?
	`, plan.Name, plan.Price, plan.TransferEnable, plan.SpeedLimit, boolToInt(plan.Show), plan.Sort, plan.GroupID, plan.Content, time.Now().Unix(), plan.ID)
	return err
}

func (s *Store) DeletePlan(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM plans WHERE id = ?`, id)
	return err
}

func (s *Store) PlanByID(ctx context.Context, id int64) (model.Plan, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, price, transfer_enable, speed_limit, show, sort, group_id, content, created_at, updated_at
		FROM plans
		WHERE id = ?
	`, id)
	return scanPlan(row)
}

func (s *Store) ListPlans(ctx context.Context, visibleOnly bool) ([]model.Plan, error) {
	query := `
		SELECT id, name, price, transfer_enable, speed_limit, show, sort, group_id, content, created_at, updated_at
		FROM plans
	`
	if visibleOnly {
		query += ` WHERE show = 1`
	}
	query += ` ORDER BY sort ASC, id ASC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plans []model.Plan
	for rows.Next() {
		plan, err := scanPlanFromRows(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, rows.Err()
}

func (s *Store) PlanUserCounts(ctx context.Context) (map[int64]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT plan_id, COUNT(1) FROM users WHERE plan_id > 0 GROUP BY plan_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]int64)
	for rows.Next() {
		var planID, count int64
		if err := rows.Scan(&planID, &count); err != nil {
			return nil, err
		}
		result[planID] = count
	}
	return result, rows.Err()
}

func (s *Store) SortPlans(ctx context.Context, planIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for index, planID := range planIDs {
		if _, err := tx.ExecContext(ctx, `UPDATE plans SET sort = ?, updated_at = ? WHERE id = ?`, index+1, time.Now().Unix(), planID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) CreateServer(ctx context.Context, server model.Server) error {
	now := time.Now().Unix()
	tags, _ := json.Marshal(server.Tags)
	planIDs, _ := json.Marshal(server.PlanIDs)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO servers(
			name, type, version, host, port, network, path, host_header, tls,
			server_name, allow_insecure, cipher, password, rate, tags, plan_ids,
			is_online, show, sort, last_check_at, created_at, updated_at
		)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, server.Name, server.Type, server.Version, server.Host, server.Port, server.Network, server.Path, server.HostHeader,
		boolToInt(server.TLS), server.ServerName, boolToInt(server.AllowInsecure), server.Cipher, server.Password, server.Rate,
		string(tags), string(planIDs), boolToInt(server.IsOnline), boolToInt(server.Show), server.Sort, now, now, now)
	return err
}

func (s *Store) UpdateServer(ctx context.Context, server model.Server) error {
	tags, _ := json.Marshal(server.Tags)
	planIDs, _ := json.Marshal(server.PlanIDs)
	_, err := s.db.ExecContext(ctx, `
		UPDATE servers SET
			name = ?, type = ?, version = ?, host = ?, port = ?, network = ?, path = ?, host_header = ?,
			tls = ?, server_name = ?, allow_insecure = ?, cipher = ?, password = ?, rate = ?, tags = ?, plan_ids = ?,
			is_online = ?, show = ?, sort = ?, last_check_at = ?, updated_at = ?
		WHERE id = ?
	`, server.Name, server.Type, server.Version, server.Host, server.Port, server.Network, server.Path, server.HostHeader,
		boolToInt(server.TLS), server.ServerName, boolToInt(server.AllowInsecure), server.Cipher, server.Password, server.Rate,
		string(tags), string(planIDs), boolToInt(server.IsOnline), boolToInt(server.Show), server.Sort, time.Now().Unix(), time.Now().Unix(), server.ID)
	return err
}

func (s *Store) DeleteServer(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM servers WHERE id = ?`, id)
	return err
}

func (s *Store) ServerByID(ctx context.Context, id int64) (model.Server, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, type, version, host, port, network, path, host_header, tls,
			server_name, allow_insecure, cipher, password, rate, tags, plan_ids,
			is_online, show, sort, last_check_at, created_at, updated_at
		FROM servers
		WHERE id = ?
	`, id)
	return scanServer(row)
}

func (s *Store) ListServers(ctx context.Context, visibleOnly bool) ([]model.Server, error) {
	query := `
		SELECT id, name, type, version, host, port, network, path, host_header, tls,
			server_name, allow_insecure, cipher, password, rate, tags, plan_ids,
			is_online, show, sort, last_check_at, created_at, updated_at
		FROM servers
	`
	if visibleOnly {
		query += ` WHERE show = 1`
	}
	query += ` ORDER BY sort ASC, id ASC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []model.Server
	for rows.Next() {
		server, err := scanServerFromRows(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, rows.Err()
}

func (s *Store) CreateNotice(ctx context.Context, notice model.Notice) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO notices(title, content, show, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?)
	`, notice.Title, notice.Content, boolToInt(notice.Show), now, now)
	return err
}

func (s *Store) UpdateNotice(ctx context.Context, notice model.Notice) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE notices SET title = ?, content = ?, show = ?, updated_at = ? WHERE id = ?
	`, notice.Title, notice.Content, boolToInt(notice.Show), time.Now().Unix(), notice.ID)
	return err
}

func (s *Store) DeleteNotice(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM notices WHERE id = ?`, id)
	return err
}

func (s *Store) ListNotices(ctx context.Context, visibleOnly bool, page, pageSize int) ([]model.Notice, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 5
	}
	where := ""
	if visibleOnly {
		where = " WHERE show = 1"
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM notices`+where).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, content, show, created_at, updated_at
		FROM notices`+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var notices []model.Notice
	for rows.Next() {
		notice, err := scanNoticeFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		notices = append(notices, notice)
	}
	return notices, total, rows.Err()
}

func (s *Store) CreateSession(ctx context.Context, session model.Session) error {
	if s.cache != nil && s.cache.Enabled() {
		key := sessionCachePrefix + session.ID
		if err := s.cache.SetJSON(ctx, key, session, sessionCacheTTL); err != nil {
			return err
		}
		ids, _ := s.cachedSessionIDs(ctx, session.UserID)
		ids = appendUniqueString(ids, session.ID)
		return s.cache.SetJSON(ctx, userSessionsCachePrefix+strconv.FormatInt(session.UserID, 10), ids, sessionCacheTTL)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions(id, user_id, ip, ua, login_at)
		VALUES(?, ?, ?, ?, ?)
	`, session.ID, session.UserID, session.IP, session.UA, session.LoginAt)
	return err
}

func (s *Store) SessionByID(ctx context.Context, id string) (model.Session, error) {
	if s.cache != nil && s.cache.Enabled() {
		var session model.Session
		if err := s.cache.GetJSON(ctx, sessionCachePrefix+id, &session); err == nil {
			return session, nil
		}
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, user_id, ip, ua, login_at FROM sessions WHERE id = ?`, id)
	var session model.Session
	if err := row.Scan(&session.ID, &session.UserID, &session.IP, &session.UA, &session.LoginAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Session{}, ErrNotFound
		}
		return model.Session{}, err
	}
	if s.cache != nil && s.cache.Enabled() {
		_ = s.cache.SetJSON(ctx, sessionCachePrefix+id, session, sessionCacheTTL)
	}
	return session, nil
}

func (s *Store) ListSessionsByUserID(ctx context.Context, userID int64) ([]model.Session, error) {
	if s.cache != nil && s.cache.Enabled() {
		ids, err := s.cachedSessionIDs(ctx, userID)
		if err == nil && len(ids) > 0 {
			sessions := make([]model.Session, 0, len(ids))
			for _, id := range ids {
				session, err := s.SessionByID(ctx, id)
				if err == nil {
					sessions = append(sessions, session)
				}
			}
			return sessions, nil
		}
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, ip, ua, login_at
		FROM sessions
		WHERE user_id = ?
		ORDER BY login_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []model.Session
	for rows.Next() {
		var session model.Session
		if err := rows.Scan(&session.ID, &session.UserID, &session.IP, &session.UA, &session.LoginAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	if s.cache != nil && s.cache.Enabled() {
		session, err := s.SessionByID(ctx, id)
		if err == nil {
			ids, _ := s.cachedSessionIDs(ctx, session.UserID)
			ids = removeString(ids, id)
			if err := s.cache.SetJSON(ctx, userSessionsCachePrefix+strconv.FormatInt(session.UserID, 10), ids, sessionCacheTTL); err != nil {
				return err
			}
		}
		return s.cache.Delete(ctx, sessionCachePrefix+id)
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	return err
}

func (s *Store) DeleteSessionsByUserID(ctx context.Context, userID int64) error {
	if s.cache != nil && s.cache.Enabled() {
		ids, _ := s.cachedSessionIDs(ctx, userID)
		keys := make([]string, 0, len(ids)+1)
		keys = append(keys, userSessionsCachePrefix+strconv.FormatInt(userID, 10))
		for _, id := range ids {
			keys = append(keys, sessionCachePrefix+id)
		}
		return s.cache.Delete(ctx, keys...)
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

func (s *Store) SaveQuickLogin(ctx context.Context, token model.QuickLogin) error {
	if s.cache != nil && s.cache.Enabled() {
		ttl := time.Until(time.Unix(token.ExpiresAt, 0))
		if ttl < time.Second {
			ttl = time.Second
		}
		return s.cache.SetJSON(ctx, quickLoginCachePrefix+token.Code, token, ttl)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO quick_logins(code, user_id, redirect, expires_at)
		VALUES(?, ?, ?, ?)
		ON CONFLICT(code) DO UPDATE SET user_id = excluded.user_id, redirect = excluded.redirect, expires_at = excluded.expires_at
	`, token.Code, token.UserID, token.Redirect, token.ExpiresAt)
	return err
}

func (s *Store) ConsumeQuickLogin(ctx context.Context, code string) (model.QuickLogin, error) {
	if s.cache != nil && s.cache.Enabled() {
		var token model.QuickLogin
		if err := s.cache.GetJSON(ctx, quickLoginCachePrefix+code, &token); err != nil {
			if errors.Is(err, cache.ErrCacheMiss) {
				return model.QuickLogin{}, ErrNotFound
			}
			return model.QuickLogin{}, err
		}
		if err := s.cache.Delete(ctx, quickLoginCachePrefix+code); err != nil {
			return model.QuickLogin{}, err
		}
		return token, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.QuickLogin{}, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `SELECT code, user_id, redirect, expires_at FROM quick_logins WHERE code = ?`, code)
	var token model.QuickLogin
	if err := row.Scan(&token.Code, &token.UserID, &token.Redirect, &token.ExpiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.QuickLogin{}, ErrNotFound
		}
		return model.QuickLogin{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM quick_logins WHERE code = ?`, code); err != nil {
		return model.QuickLogin{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.QuickLogin{}, err
	}
	return token, nil
}

func (s *Store) EnsureDefaultSubscriptionLinks(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM users`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var userIDs []int64
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return err
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, userID := range userIDs {
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM subscription_links WHERE user_id = ?`, userID).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			if _, err := s.CreateSubscriptionLink(ctx, model.SubscriptionLink{
				UserID:    userID,
				Name:      "Default",
				Suffix:    s.randomSubscriptionSuffix(),
				IsPrimary: true,
				Enabled:   true,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) CreateSubscriptionLink(ctx context.Context, link model.SubscriptionLink) (model.SubscriptionLink, error) {
	if strings.TrimSpace(link.Name) == "" {
		link.Name = "Subscription"
	}
	if strings.TrimSpace(link.Suffix) == "" {
		link.Suffix = s.randomSubscriptionSuffix()
	}
	link.Suffix = normalizeSuffix(link.Suffix)
	now := time.Now().Unix()
	if link.CreatedAt == 0 {
		link.CreatedAt = now
	}
	link.UpdatedAt = now
	if !link.Enabled {
		link.Enabled = true
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.SubscriptionLink{}, err
	}
	defer tx.Rollback()

	if link.IsPrimary {
		if _, err := tx.ExecContext(ctx, `UPDATE subscription_links SET is_primary = 0, updated_at = ? WHERE user_id = ?`, now, link.UserID); err != nil {
			return model.SubscriptionLink{}, err
		}
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO subscription_links(user_id, name, suffix, is_primary, enabled, last_used_at, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
	`, link.UserID, link.Name, link.Suffix, boolToInt(link.IsPrimary), boolToInt(link.Enabled), link.LastUsedAt, link.CreatedAt, link.UpdatedAt)
	if err != nil {
		return model.SubscriptionLink{}, err
	}
	id, _ := res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return model.SubscriptionLink{}, err
	}
	return s.SubscriptionLinkByID(ctx, id)
}

func (s *Store) SubscriptionLinkByID(ctx context.Context, id int64) (model.SubscriptionLink, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, suffix, is_primary, enabled, last_used_at, created_at, updated_at
		FROM subscription_links
		WHERE id = ?
	`, id)
	return scanSubscriptionLink(row)
}

func (s *Store) SubscriptionLinkBySuffix(ctx context.Context, suffix string) (model.SubscriptionLink, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, suffix, is_primary, enabled, last_used_at, created_at, updated_at
		FROM subscription_links
		WHERE suffix = ? AND enabled = 1
	`, normalizeSuffix(suffix))
	return scanSubscriptionLink(row)
}

func (s *Store) ListSubscriptionLinksByUserID(ctx context.Context, userID int64) ([]model.SubscriptionLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, name, suffix, is_primary, enabled, last_used_at, created_at, updated_at
		FROM subscription_links
		WHERE user_id = ?
		ORDER BY is_primary DESC, created_at ASC, id ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []model.SubscriptionLink
	for rows.Next() {
		link, err := scanSubscriptionLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func (s *Store) UpdateSubscriptionLink(ctx context.Context, link model.SubscriptionLink) error {
	link.Suffix = normalizeSuffix(link.Suffix)
	now := time.Now().Unix()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if link.IsPrimary {
		if _, err := tx.ExecContext(ctx, `UPDATE subscription_links SET is_primary = 0, updated_at = ? WHERE user_id = ?`, now, link.UserID); err != nil {
			return err
		}
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE subscription_links
		SET name = ?, suffix = ?, is_primary = ?, enabled = ?, updated_at = ?
		WHERE id = ?
	`, link.Name, link.Suffix, boolToInt(link.IsPrimary), boolToInt(link.Enabled), now, link.ID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteSubscriptionLink(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var userID int64
	var isPrimary int
	if err := tx.QueryRowContext(ctx, `SELECT user_id, is_primary FROM subscription_links WHERE id = ?`, id).Scan(&userID, &isPrimary); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM subscription_links WHERE id = ?`, id); err != nil {
		return err
	}
	if isPrimary == 1 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE subscription_links
			SET is_primary = 1, updated_at = ?
			WHERE id = (
				SELECT id FROM subscription_links WHERE user_id = ? ORDER BY created_at ASC, id ASC LIMIT 1
			)
		`, time.Now().Unix(), userID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) TouchSubscriptionLink(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE subscription_links SET last_used_at = ?, updated_at = ? WHERE id = ?`, time.Now().Unix(), time.Now().Unix(), id)
	return err
}

func (s *Store) ResetUserSubscriptionLinks(ctx context.Context, userID int64) ([]model.SubscriptionLink, error) {
	links, err := s.ListSubscriptionLinksByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		link.Suffix = s.randomSubscriptionSuffix()
		if err := s.UpdateSubscriptionLink(ctx, link); err != nil {
			return nil, err
		}
	}
	return s.ListSubscriptionLinksByUserID(ctx, userID)
}

func (s *Store) PrimarySubscriptionLinkByUserID(ctx context.Context, userID int64) (model.SubscriptionLink, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, suffix, is_primary, enabled, last_used_at, created_at, updated_at
		FROM subscription_links
		WHERE user_id = ?
		ORDER BY is_primary DESC, created_at ASC, id ASC
		LIMIT 1
	`, userID)
	return scanSubscriptionLink(row)
}

func (s *Store) randomSubscriptionSuffix() string {
	return strings.ToLower(randomToken(6))
}

func (s *Store) CountUsersByInviter(ctx context.Context, inviterID int64) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM users WHERE invite_user_id = ?`, inviterID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) DashboardStats(ctx context.Context) (map[string]int64, error) {
	queries := map[string]string{
		"users":   `SELECT COUNT(1) FROM users`,
		"plans":   `SELECT COUNT(1) FROM plans`,
		"servers": `SELECT COUNT(1) FROM servers`,
		"notices": `SELECT COUNT(1) FROM notices`,
	}
	result := make(map[string]int64, len(queries))
	for key, query := range queries {
		var count int64
		if err := s.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			return nil, err
		}
		result[key] = count
	}
	return result, nil
}

func (s *Store) AddTrafficByUserID(ctx context.Context, userID int64, upload, download int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET u = u + ?, d = d + ?, updated_at = ?
		WHERE id = ?
	`, upload, download, time.Now().Unix(), userID)
	return err
}

func (s *Store) cacheSettings(ctx context.Context, values map[string]string) error {
	if s.cache == nil || !s.cache.Enabled() {
		return nil
	}
	return s.cache.SetJSON(ctx, settingsCacheKey, values, settingsCacheTTL)
}

func (s *Store) cachedSessionIDs(ctx context.Context, userID int64) ([]string, error) {
	if s.cache == nil || !s.cache.Enabled() {
		return nil, cache.ErrCacheMiss
	}
	var ids []string
	if err := s.cache.GetJSON(ctx, userSessionsCachePrefix+strconv.FormatInt(userID, 10), &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func scanUser(row rowScanner) (model.User, error) {
	var user model.User
	var isAdmin, isStaff, banned, remindExpire, remindTraffic int
	if err := row.Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.UUID, &user.Token, &isAdmin, &isStaff, &banned,
		&remindExpire, &remindTraffic, &user.TransferEnable, &user.U, &user.D, &user.ExpiredAt,
		&user.Balance, &user.CommissionBalance, &user.PlanID, &user.GroupID, &user.InviteUserID,
		&user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, err
	}
	user.IsAdmin = isAdmin == 1
	user.IsStaff = isStaff == 1
	user.Banned = banned == 1
	user.RemindExpire = remindExpire == 1
	user.RemindTraffic = remindTraffic == 1
	return user, nil
}

func scanUserFromRows(rows *sql.Rows) (model.User, error) {
	return scanUser(rows)
}

func scanPlan(row rowScanner) (model.Plan, error) {
	var plan model.Plan
	var show int
	if err := row.Scan(&plan.ID, &plan.Name, &plan.Price, &plan.TransferEnable, &plan.SpeedLimit, &show, &plan.Sort, &plan.GroupID, &plan.Content, &plan.CreatedAt, &plan.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Plan{}, ErrNotFound
		}
		return model.Plan{}, err
	}
	plan.Show = show == 1
	return plan, nil
}

func scanPlanFromRows(rows *sql.Rows) (model.Plan, error) {
	return scanPlan(rows)
}

func scanServer(row rowScanner) (model.Server, error) {
	var server model.Server
	var tls, allowInsecure, isOnline, show int
	var tags, planIDs string
	if err := row.Scan(
		&server.ID, &server.Name, &server.Type, &server.Version, &server.Host, &server.Port, &server.Network, &server.Path, &server.HostHeader, &tls,
		&server.ServerName, &allowInsecure, &server.Cipher, &server.Password, &server.Rate, &tags, &planIDs,
		&isOnline, &show, &server.Sort, &server.LastCheckAt, &server.CreatedAt, &server.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Server{}, ErrNotFound
		}
		return model.Server{}, err
	}
	server.TLS = tls == 1
	server.AllowInsecure = allowInsecure == 1
	server.IsOnline = isOnline == 1
	server.Show = show == 1
	server.Tags = decodeStringArray(tags)
	server.PlanIDs = decodeInt64Array(planIDs)
	server.CacheKey = fmt.Sprintf("%d-%d-%s-%s-%d", server.ID, server.UpdatedAt, server.Name, server.Type, server.Port)
	return server, nil
}

func scanServerFromRows(rows *sql.Rows) (model.Server, error) {
	return scanServer(rows)
}

func scanNoticeFromRows(rows *sql.Rows) (model.Notice, error) {
	var notice model.Notice
	var show int
	if err := rows.Scan(&notice.ID, &notice.Title, &notice.Content, &show, &notice.CreatedAt, &notice.UpdatedAt); err != nil {
		return model.Notice{}, err
	}
	notice.Show = show == 1
	return notice, nil
}

func scanSubscriptionLink(row rowScanner) (model.SubscriptionLink, error) {
	var link model.SubscriptionLink
	var isPrimary, enabled int
	if err := row.Scan(&link.ID, &link.UserID, &link.Name, &link.Suffix, &isPrimary, &enabled, &link.LastUsedAt, &link.CreatedAt, &link.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.SubscriptionLink{}, ErrNotFound
		}
		return model.SubscriptionLink{}, err
	}
	link.IsPrimary = isPrimary == 1
	link.Enabled = enabled == 1
	return link, nil
}

func decodeStringArray(raw string) []string {
	if raw == "" {
		return nil
	}
	var result []string
	_ = json.Unmarshal([]byte(raw), &result)
	return result
}

func decodeInt64Array(raw string) []int64 {
	if raw == "" {
		return nil
	}
	var result []int64
	_ = json.Unmarshal([]byte(raw), &result)
	return result
}

func appendUniqueString(values []string, target string) []string {
	for _, value := range values {
		if value == target {
			return values
		}
	}
	return append(values, target)
}

func removeString(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func safeSort(value string, allow map[string]bool) string {
	value = strings.TrimSpace(value)
	if allow[value] {
		return value
	}
	return "created_at"
}

func normalizeSuffix(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.Trim(value, "/")
	value = strings.ReplaceAll(value, " ", "-")
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			builder.WriteRune(r)
		}
	}
	result := builder.String()
	if result == "" {
		return strings.ToLower(randomToken(6))
	}
	return result
}

type rowScanner interface {
	Scan(dest ...any) error
}

const gigaByte int64 = 1024 * 1024 * 1024

const (
	settingsCacheKey        = "settings:all"
	settingsCacheTTL        = 5 * time.Minute
	sessionCacheTTL         = 30 * 24 * time.Hour
	quickLoginCachePrefix   = "quick_login:"
	sessionCachePrefix      = "session:"
	userSessionsCachePrefix = "user_sessions:"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	uuid TEXT NOT NULL UNIQUE,
	token TEXT NOT NULL UNIQUE,
	is_admin INTEGER NOT NULL DEFAULT 0,
	is_staff INTEGER NOT NULL DEFAULT 0,
	banned INTEGER NOT NULL DEFAULT 0,
	remind_expire INTEGER NOT NULL DEFAULT 1,
	remind_traffic INTEGER NOT NULL DEFAULT 1,
	transfer_enable INTEGER NOT NULL DEFAULT 0,
	u INTEGER NOT NULL DEFAULT 0,
	d INTEGER NOT NULL DEFAULT 0,
	expired_at INTEGER NOT NULL DEFAULT 0,
	balance REAL NOT NULL DEFAULT 0,
	commission_balance REAL NOT NULL DEFAULT 0,
	plan_id INTEGER NOT NULL DEFAULT 0,
	group_id INTEGER NOT NULL DEFAULT 1,
	invite_user_id INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	last_login_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS plans (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	price REAL NOT NULL DEFAULT 0,
	transfer_enable INTEGER NOT NULL DEFAULT 0,
	speed_limit INTEGER NOT NULL DEFAULT 0,
	show INTEGER NOT NULL DEFAULT 1,
	sort INTEGER NOT NULL DEFAULT 0,
	group_id INTEGER NOT NULL DEFAULT 1,
	content TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS servers (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	version INTEGER NOT NULL DEFAULT 1,
	host TEXT NOT NULL,
	port INTEGER NOT NULL,
	network TEXT NOT NULL DEFAULT '',
	path TEXT NOT NULL DEFAULT '',
	host_header TEXT NOT NULL DEFAULT '',
	tls INTEGER NOT NULL DEFAULT 0,
	server_name TEXT NOT NULL DEFAULT '',
	allow_insecure INTEGER NOT NULL DEFAULT 0,
	cipher TEXT NOT NULL DEFAULT '',
	password TEXT NOT NULL DEFAULT '',
	rate REAL NOT NULL DEFAULT 1,
	tags TEXT NOT NULL DEFAULT '[]',
	plan_ids TEXT NOT NULL DEFAULT '[]',
	is_online INTEGER NOT NULL DEFAULT 1,
	show INTEGER NOT NULL DEFAULT 1,
	sort INTEGER NOT NULL DEFAULT 0,
	last_check_at INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS notices (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	title TEXT NOT NULL,
	content TEXT NOT NULL,
	show INTEGER NOT NULL DEFAULT 1,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	user_id INTEGER NOT NULL,
	ip TEXT NOT NULL,
	ua TEXT NOT NULL,
	login_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS quick_logins (
	code TEXT PRIMARY KEY,
	user_id INTEGER NOT NULL,
	redirect TEXT NOT NULL,
	expires_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS subscription_links (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	suffix TEXT NOT NULL UNIQUE,
	is_primary INTEGER NOT NULL DEFAULT 0,
	enabled INTEGER NOT NULL DEFAULT 1,
	last_used_at INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);
`
