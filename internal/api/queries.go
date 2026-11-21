package api

import (
	"context"
	"net/http"
	"time"

	"github.com/MiguelDev-GH/omni/internal/sniffer/l7"
)

const (
	slowDefault = 20
	slowMax     = 200
)

type DBStore interface {
	DBSince(ctx context.Context, since time.Time, limit int) ([]l7.DBWindow, error)
	SlowQueriesSince(ctx context.Context, since time.Time, limit int) ([]l7.QueryStats, error)
}

// QueriesHandler responde GET /api/consultas?desde=<RFC3339> com as janelas
// de consultas de 10 s; sem desde, devolve os últimos 15 minutos.
func QueriesHandler(ds DBStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, ok := parseSince(w, r, l7Window)
		if !ok {
			return
		}
		list, err := ds.DBSince(r.Context(), since, l7Limit)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}

// SlowQueriesHandler responde GET /api/consultas/lentas?desde=&limite= com as
// consultas de maior p95; por padrão, as 20 piores das últimas 24 horas.
func SlowQueriesHandler(ds DBStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, ok := parseSince(w, r, rankingSince)
		if !ok {
			return
		}
		limit, ok := parseLimit(w, r, slowDefault, slowMax)
		if !ok {
			return
		}
		list, err := ds.SlowQueriesSince(r.Context(), since, limit)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}
