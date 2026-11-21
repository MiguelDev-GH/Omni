package api

import (
	"context"
	"net/http"
	"time"

	"github.com/MiguelDev-GH/omni/internal/sniffer/l7"
)

const (
	// l7Limit cobre 24 h de janelas de 10 s.
	l7Limit      = 8640
	l7Window     = 15 * time.Minute
	rankingLimit = 100
	rankingSince = 24 * time.Hour
)

type HTTPStore interface {
	HTTPSince(ctx context.Context, since time.Time, limit int) ([]l7.HTTPWindow, error)
	HTTPRoutesSince(ctx context.Context, since time.Time, limit int) ([]l7.RouteStats, error)
}

// RequestsHandler responde GET /api/requisicoes?desde=<RFC3339> com as janelas
// HTTP de 10 s; sem desde, devolve os últimos 15 minutos.
func RequestsHandler(hs HTTPStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, ok := parseSince(w, r, l7Window)
		if !ok {
			return
		}
		list, err := hs.HTTPSince(r.Context(), since, l7Limit)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}

// RoutesHandler responde GET /api/requisicoes/rotas?desde=<RFC3339> com as
// 100 rotas de mais volume; sem desde, das últimas 24 horas.
func RoutesHandler(hs HTTPStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, ok := parseSince(w, r, rankingSince)
		if !ok {
			return
		}
		list, err := hs.HTTPRoutesSince(r.Context(), since, rankingLimit)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}
