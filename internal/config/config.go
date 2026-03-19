package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Addr          string
	DBPath        string
	JWTSecret     string
	RedisAddr     string
	RedisPassword string
	RedisPrefix   string
	RedisDB       int
	AppURL        string
	AppName       string
	AdminPath     string
	DefaultEmail  string
	DefaultPass   string
}

func Load() Config {
	root := env("AIRBOARD_DB_PATH", filepath.Join("data", "airboard.db"))
	return Config{
		Addr:          env("AIRBOARD_ADDR", ":8080"),
		DBPath:        root,
		JWTSecret:     env("AIRBOARD_JWT_SECRET", "pixia-airboard-dev-secret"),
		RedisAddr:     env("AIRBOARD_REDIS_ADDR", ""),
		RedisPassword: env("AIRBOARD_REDIS_PASSWORD", ""),
		RedisPrefix:   env("AIRBOARD_REDIS_PREFIX", "airboard"),
		RedisDB:       envInt("AIRBOARD_REDIS_DB", 0),
		AppURL:        strings.TrimRight(env("AIRBOARD_APP_URL", ""), "/"),
		AppName:       env("AIRBOARD_APP_NAME", "Pixia Airboard"),
		AdminPath:     normalizePath(env("AIRBOARD_ADMIN_PATH", "admin")),
		DefaultEmail:  env("AIRBOARD_ADMIN_EMAIL", "admin@example.com"),
		DefaultPass:   env("AIRBOARD_ADMIN_PASSWORD", "admin123456"),
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func normalizePath(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "/")
	if value == "" {
		return "admin"
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
