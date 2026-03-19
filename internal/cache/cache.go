package cache

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"pixia-airboard/internal/config"
)

var ErrCacheMiss = errors.New("cache miss")

type Cache struct {
	client *redis.Client
	prefix string
}

func New(ctx context.Context, cfg config.Config) (*Cache, func(), error) {
	if strings.TrimSpace(cfg.RedisAddr) == "" {
		return nil, func() {}, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, nil, err
	}

	cache := &Cache{
		client: client,
		prefix: normalizePrefix(cfg.RedisPrefix),
	}
	return cache, func() {
		_ = client.Close()
	}, nil
}

func (c *Cache) Enabled() bool {
	return c != nil && c.client != nil
}

func (c *Cache) GetString(ctx context.Context, key string) (string, error) {
	if !c.Enabled() {
		return "", ErrCacheMiss
	}
	value, err := c.client.Get(ctx, c.withPrefix(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrCacheMiss
		}
		return "", err
	}
	return value, nil
}

func (c *Cache) SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	if !c.Enabled() {
		return nil
	}
	return c.client.Set(ctx, c.withPrefix(key), value, ttl).Err()
}

func (c *Cache) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	if !c.Enabled() {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, c.withPrefix(key), raw, ttl).Err()
}

func (c *Cache) GetJSON(ctx context.Context, key string, dst any) error {
	if !c.Enabled() {
		return ErrCacheMiss
	}
	raw, err := c.client.Get(ctx, c.withPrefix(key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrCacheMiss
		}
		return err
	}
	return json.Unmarshal(raw, dst)
}

func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	if !c.Enabled() || len(keys) == 0 {
		return nil
	}
	prefixed := make([]string, 0, len(keys))
	for _, key := range keys {
		prefixed = append(prefixed, c.withPrefix(key))
	}
	return c.client.Del(ctx, prefixed...).Err()
}

func (c *Cache) withPrefix(key string) string {
	if c.prefix == "" {
		return key
	}
	return c.prefix + ":" + key
}

func normalizePrefix(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, ":")
	if value == "" {
		return "airboard"
	}
	return value
}
