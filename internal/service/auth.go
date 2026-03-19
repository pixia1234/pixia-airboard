package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"pixia-airboard/internal/model"
	"pixia-airboard/internal/store"
)

var ErrUnauthorized = errors.New("unauthorized")

type claims struct {
	UserID    int64  `json:"id"`
	SessionID string `json:"session"`
	jwt.RegisteredClaims
}

type AuthService struct {
	secret []byte
	store  *store.Store
}

func NewAuthService(secret string, dataStore *store.Store) *AuthService {
	return &AuthService{
		secret: []byte(secret),
		store:  dataStore,
	}
}

func (s *AuthService) Issue(ctx context.Context, user model.User, ip, ua string) (string, error) {
	sessionID := opaqueToken(24)
	if err := s.store.CreateSession(ctx, model.Session{
		ID:      sessionID,
		UserID:  user.ID,
		IP:      ip,
		UA:      ua,
		LoginAt: time.Now().Unix(),
	}); err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		UserID:    user.ID,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
	return token.SignedString(s.secret)
}

func (s *AuthService) ParseUser(ctx context.Context, raw string) (model.User, error) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "Bearer "))
	if raw == "" {
		return model.User{}, ErrUnauthorized
	}

	token, err := jwt.ParseWithClaims(raw, &claims{}, func(token *jwt.Token) (any, error) {
		return s.secret, nil
	})
	if err != nil || !token.Valid {
		return model.User{}, ErrUnauthorized
	}

	parsed, ok := token.Claims.(*claims)
	if !ok {
		return model.User{}, ErrUnauthorized
	}
	if _, err := s.store.SessionByID(ctx, parsed.SessionID); err != nil {
		return model.User{}, ErrUnauthorized
	}

	user, err := s.store.UserByID(ctx, parsed.UserID)
	if err != nil || user.Banned {
		return model.User{}, ErrUnauthorized
	}
	return user, nil
}

func (s *AuthService) Sessions(ctx context.Context, userID int64) ([]model.Session, error) {
	return s.store.ListSessionsByUserID(ctx, userID)
}

func (s *AuthService) RemoveSession(ctx context.Context, sessionID string) error {
	return s.store.DeleteSession(ctx, sessionID)
}

func (s *AuthService) RemoveAllSessions(ctx context.Context, userID int64) error {
	return s.store.DeleteSessionsByUserID(ctx, userID)
}

func (s *AuthService) CreateQuickLogin(ctx context.Context, userID int64, redirect string) (string, error) {
	code := opaqueToken(16)
	if redirect == "" {
		redirect = "dashboard"
	}
	err := s.store.SaveQuickLogin(ctx, model.QuickLogin{
		Code:      code,
		UserID:    userID,
		Redirect:  redirect,
		ExpiresAt: time.Now().Add(5 * time.Minute).Unix(),
	})
	return code, err
}

func (s *AuthService) ConsumeQuickLogin(ctx context.Context, code string) (model.User, string, error) {
	token, err := s.store.ConsumeQuickLogin(ctx, code)
	if err != nil {
		return model.User{}, "", err
	}
	if token.ExpiresAt < time.Now().Unix() {
		return model.User{}, "", ErrUnauthorized
	}
	user, err := s.store.UserByID(ctx, token.UserID)
	if err != nil {
		return model.User{}, "", err
	}
	return user, token.Redirect, nil
}

func opaqueToken(size int) string {
	if size < 1 {
		size = 16
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}
