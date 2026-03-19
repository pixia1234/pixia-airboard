package app

import (
	"context"
	"html/template"
	"net/http"

	"pixia-airboard/internal/cache"
	"pixia-airboard/internal/config"
	"pixia-airboard/internal/httpapi"
	"pixia-airboard/internal/service"
	"pixia-airboard/internal/store"
	"pixia-airboard/web"
)

func New(cfg config.Config) (http.Handler, func(), error) {
	ctx := context.Background()

	cacheClient, closeCache, err := cache.New(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}

	dataStore, err := store.New(ctx, cfg, cacheClient)
	if err != nil {
		closeCache()
		return nil, nil, err
	}

	tpls, err := template.ParseFS(web.FS, "templates/*.html")
	if err != nil {
		return nil, nil, err
	}

	authService := service.NewAuthService(cfg.JWTSecret, dataStore)
	handler, err := httpapi.New(cfg, dataStore, cacheClient, authService, tpls, web.FS)
	if err != nil {
		_ = dataStore.Close()
		closeCache()
		return nil, nil, err
	}

	return handler, func() {
		_ = dataStore.Close()
		closeCache()
	}, nil
}
