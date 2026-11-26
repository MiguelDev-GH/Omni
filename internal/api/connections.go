package api

import (
	"context"
	"net/http"
	"time"

	"github.com/MiguelDev-GH/omni/internal/connstats"
)

const (
	connWindow    = 15 * time.Minute
	connMaxWindow = 24 * time.Hour
)

type ConnectionsStore interface {
	ConnectionsSince(ctx context.Context, since time.Time, limit int) ([]connstats.Snapshot, error)
}

// ConnectionsHandler responde GET /api/conexoes?desde=<RFC3339> com as coletas
// de conexões por porta; sem desde, devolve os últimos 15 minutos. desde mais
// antigo que 24 h vale como 24 h.
func ConnectionsHandler(cs ConnectionsStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, ok := parseSince(w, r, connWindow)
		if !ok {
			return
		}
		now := time.Now()
		if oldest := now.Add(-connMaxWindow); since.Before(oldest) {
			since = oldest
		}
		limit := int(now.Sub(since)/connstats.DefaultInterval) + 1
		list, err := cs.ConnectionsSince(r.Context(), since, limit)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}
