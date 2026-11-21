package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/sniffer/l7"
)

func TestRequestsAndRoutes(t *testing.T) {
	srv, st := newTrafficServer(t)
	srv.Handle("GET /api/requisicoes", RequestsHandler(st))
	srv.Handle("GET /api/requisicoes/rotas", RoutesHandler(st))

	stats := l7.NewHTTPStats()
	now := time.Now().UTC().Truncate(time.Second)
	stats.Flush(now.Add(-time.Minute))
	stats.Add(l7.HTTPExchange{Method: "GET", Route: "/users/:id", Status: 200, Latency: 3 * time.Millisecond})
	stats.Add(l7.HTTPExchange{Method: "POST", Route: "/login", Status: 500, Latency: 80 * time.Millisecond})
	require.NoError(t, st.InsertHTTPWindow(context.Background(), stats.Flush(now)))

	rec := do(t, srv, "GET", "/api/requisicoes", "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	windows := decode[[]map[string]any](t, rec)
	require.Len(t, windows, 1)
	for _, k := range []string{"inicio", "requisicoes", "status", "p50_ms", "p95_ms"} {
		assert.Contains(t, windows[0], k)
	}

	rec = do(t, srv, "GET", "/api/requisicoes/rotas", "", true)
	require.Equal(t, http.StatusOK, rec.Code)
	routes := decode[[]map[string]any](t, rec)
	require.Len(t, routes, 2)
	assert.ElementsMatch(t, []string{"metodo", "rota", "requisicoes", "erros", "p50_ms", "p95_ms"}, keys(routes[0]))

	rec = do(t, srv, "GET", "/api/requisicoes/rotas?desde=x", "", true)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	rec = do(t, srv, "GET", "/api/requisicoes", "", false)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
