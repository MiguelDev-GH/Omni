package api

import (
	"context"
	"net/http"
	"time"

	"github.com/MiguelDev-GH/omni/internal/alerts"
)

const (
	// ReportMaxWindow é o maior intervalo de um relatório, igual à retenção
	// padrão das verificações.
	ReportMaxWindow   = 168 * time.Hour
	alertHistoryLimit = 5000
)

type AlertHistoryStore interface {
	AlertsBetween(ctx context.Context, from, to time.Time, limit int) ([]alerts.Alert, error)
}

// parseRange lê ?desde= e ?ate= em RFC3339; sem ate, vale agora, e sem desde,
// ate menos def. Recusa intervalos invertidos ou maiores que max. Quando ok é
// false, a resposta de erro já foi escrita.
func parseRange(w http.ResponseWriter, r *http.Request, def, max time.Duration) (from, to time.Time, ok bool) {
	to = time.Now()
	if v := r.URL.Query().Get("ate"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "ate inválido")
			return from, to, false
		}
		to = t
	}
	from = to.Add(-def)
	if v := r.URL.Query().Get("desde"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "desde inválido")
			return from, to, false
		}
		from = t
	}
	if !from.Before(to) {
		writeError(w, http.StatusBadRequest, "desde deve ser anterior a ate")
		return from, to, false
	}
	if to.Sub(from) > max {
		writeError(w, http.StatusBadRequest, "intervalo maior que "+max.String())
		return from, to, false
	}
	return from, to, true
}

// AlertHistoryHandler responde GET /api/alertas/historico?desde=&ate= com as
// mudanças de nível gravadas no intervalo, da mais nova para a mais antiga.
// Sem parâmetros, cobre as últimas 24 h; o intervalo máximo é de 168 h.
func AlertHistoryHandler(as AlertHistoryStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		from, to, ok := parseRange(w, r, 24*time.Hour, ReportMaxWindow)
		if !ok {
			return
		}
		list, err := as.AlertsBetween(r.Context(), from, to, alertHistoryLimit)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}
