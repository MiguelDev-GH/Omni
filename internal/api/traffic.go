package api

import (
	"context"
	"net/http"
	"time"

	"github.com/MiguelDev-GH/omni/internal/sniffer"
)

const (
	trafficLimit  = 3600
	trafficWindow = 15 * time.Minute
)

type TrafficStore interface {
	TrafficSince(ctx context.Context, since time.Time, limit int) ([]sniffer.Window, error)
}

// TrafficHandler responde GET /api/trafego?desde=<RFC3339> com as janelas do
// sniffer; sem desde, devolve os últimos 15 minutos.
func TrafficHandler(ts TrafficStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, ok := parseSince(w, r, trafficWindow)
		if !ok {
			return
		}
		list, err := ts.TrafficSince(r.Context(), since, trafficLimit)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}
