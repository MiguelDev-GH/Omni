package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/MiguelDev-GH/omni/internal/discovery"
)

// DiscoveryHandler responde GET /api/descoberta com os serviços desta
// máquina que podem virar alvos.
func DiscoveryHandler(list func(context.Context) ([]discovery.Listener, error), ownPort int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ls, err := list(r.Context())
		if err != nil {
			slog.Error("listar serviços da máquina", "erro", err)
			writeError(w, http.StatusInternalServerError, "não foi possível listar os serviços")
			return
		}
		writeJSON(w, http.StatusOK, discovery.Suggest(ls, ownPort))
	})
}
