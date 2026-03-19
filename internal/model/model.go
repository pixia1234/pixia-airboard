package model

type User struct {
	ID                int64   `json:"id"`
	Email             string  `json:"email"`
	PasswordHash      string  `json:"-"`
	UUID              string  `json:"uuid"`
	Token             string  `json:"token"`
	IsAdmin           bool    `json:"is_admin"`
	IsStaff           bool    `json:"is_staff"`
	Banned            bool    `json:"banned"`
	RemindExpire      bool    `json:"remind_expire"`
	RemindTraffic     bool    `json:"remind_traffic"`
	TransferEnable    int64   `json:"transfer_enable"`
	U                 int64   `json:"u"`
	D                 int64   `json:"d"`
	ExpiredAt         int64   `json:"expired_at"`
	Balance           float64 `json:"balance"`
	CommissionBalance float64 `json:"commission_balance"`
	PlanID            int64   `json:"plan_id"`
	GroupID           int64   `json:"group_id"`
	InviteUserID      int64   `json:"invite_user_id"`
	CreatedAt         int64   `json:"created_at"`
	UpdatedAt         int64   `json:"updated_at"`
	LastLoginAt       int64   `json:"last_login_at"`
}

type Plan struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Price          float64 `json:"price"`
	TransferEnable int64   `json:"transfer_enable"`
	SpeedLimit     int64   `json:"speed_limit"`
	Show           bool    `json:"show"`
	Sort           int64   `json:"sort"`
	GroupID        int64   `json:"group_id"`
	Content        string  `json:"content"`
	CreatedAt      int64   `json:"created_at"`
	UpdatedAt      int64   `json:"updated_at"`
	Count          int64   `json:"count,omitempty"`
}

type Server struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	Version       int      `json:"version"`
	Host          string   `json:"host"`
	Port          int      `json:"port"`
	Network       string   `json:"network"`
	Path          string   `json:"path"`
	HostHeader    string   `json:"host_header"`
	TLS           bool     `json:"tls"`
	ServerName    string   `json:"server_name"`
	AllowInsecure bool     `json:"allow_insecure"`
	Cipher        string   `json:"cipher"`
	Password      string   `json:"password"`
	Rate          float64  `json:"rate"`
	Tags          []string `json:"tags"`
	PlanIDs       []int64  `json:"plan_ids"`
	IsOnline      bool     `json:"is_online"`
	Show          bool     `json:"show"`
	Sort          int64    `json:"sort"`
	LastCheckAt   int64    `json:"last_check_at"`
	CreatedAt     int64    `json:"created_at"`
	UpdatedAt     int64    `json:"updated_at"`
	CacheKey      string   `json:"cache_key"`
}

type Notice struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Show      bool   `json:"show"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type Session struct {
	ID      string `json:"id"`
	UserID  int64  `json:"user_id"`
	IP      string `json:"ip"`
	UA      string `json:"ua"`
	LoginAt int64  `json:"login_at"`
}

type QuickLogin struct {
	Code      string
	UserID    int64
	Redirect  string
	ExpiresAt int64
}

type SubscriptionLink struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	Name       string `json:"name"`
	Suffix     string `json:"suffix"`
	IsPrimary  bool   `json:"is_primary"`
	Enabled    bool   `json:"enabled"`
	LastUsedAt int64  `json:"last_used_at"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
}
