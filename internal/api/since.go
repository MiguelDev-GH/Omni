package api

import (
	"net/http"
	"time"
)

// parseSince lê ?desde=<RFC3339>; sem ele, devolve agora menos def. Quando ok
// é false, a resposta de erro já foi escrita.
func parseSince(w http.ResponseWriter, r *http.Request, def time.Duration) (time.Time, bool) {
	v := r.URL.Query().Get("desde")
	if v == "" {
		return time.Now().Add(-def), true
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		writeError(w, http.StatusBadRequest, "desde inválido")
		return time.Time{}, false
	}
	return t, true
}
