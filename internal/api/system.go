package api

import (
	"context"
	"net/http"
	"time"

	"github.com/MiguelDev-GH/omni/internal/sysmetrics"
)

const (
	systemWindow    = 15 * time.Minute
	systemMaxWindow = 24 * time.Hour
	// Acima de systemRawWindow, a resposta traz uma média por systemStep.
	systemRawWindow = time.Hour
	systemStep      = time.Minute
	systemRawLimit  = 3600
)

type SystemStore interface {
	SystemSince(ctx context.Context, since time.Time, step time.Duration, limit int) ([]sysmetrics.Snapshot, error)
}

// SystemHandler responde GET /api/sistema?desde=<RFC3339> com as leituras de
// CPU, memória e disco; sem desde, devolve os últimos 15 minutos. desde mais
// antigo que 24 h vale como 24 h, e janelas acima de 1 h trazem um ponto por
// minuto.
func SystemHandler(ss SystemStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, ok := parseSince(w, r, systemWindow)
		if !ok {
			return
		}
		now := time.Now()
		if oldest := now.Add(-systemMaxWindow); since.Before(oldest) {
			since = oldest
		}
		var step time.Duration
		limit := systemRawLimit
		if window := now.Sub(since); window > systemRawWindow {
			step = systemStep
			limit = int(window / systemStep)
		}
		list, err := ss.SystemSince(r.Context(), since, step, limit)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}
