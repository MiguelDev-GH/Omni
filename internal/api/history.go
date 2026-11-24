package api

import (
	"context"
	"net/http"
	"time"

	"github.com/MiguelDev-GH/omni/internal/poller"
	"github.com/MiguelDev-GH/omni/internal/target"
)

const (
	// historyPerPort cobre 24 h com o intervalo padrão de 30 s em cada porta.
	historyPerPort = 2880
	historyWindow  = 24 * time.Hour
)

type HistoryStore interface {
	GetTarget(ctx context.Context, id int64) (target.Target, error)
	CheckHistory(ctx context.Context, targetID int64, since time.Time, perPort int) ([]poller.Result, error)
}

// HistoryHandler responde GET /api/alvos/{id}/historico?desde=<RFC3339>.
func HistoryHandler(hs HistoryStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		since := time.Now().Add(-historyWindow)
		if v := r.URL.Query().Get("desde"); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				writeError(w, http.StatusBadRequest, "desde inválido")
				return
			}
			since = t
		}
		if _, err := hs.GetTarget(r.Context(), id); err != nil {
			storeError(w, err)
			return
		}
		list, err := hs.CheckHistory(r.Context(), id, since, historyPerPort)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}
