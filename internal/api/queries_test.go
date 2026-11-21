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

func TestQueriesAndSlowQueries(t *testing.T) {
	srv, st := newTrafficServer(t)
	srv.Handle("GET /api/consultas", QueriesHandler(st))
	srv.Handle("GET /api/consultas/lentas", SlowQueriesHandler(st))

	stats := l7.NewDBStats()
	now := time.Now().UTC().Truncate(time.Second)
	stats.Flush(now.Add(-time.Minute))
	stats.Add(l7.DBQuery{Query: "SELECT ?", Kind: "SELECT", Latency: time.Millisecond})
	stats.Add(l7.DBQuery{Query: "UPDATE t SET a = ?", Kind: "UPDATE", Latency: 300 * time.Millisecond})
	require.NoError(t, st.InsertDBWindow(context.Background(), stats.Flush(now)))

	rec := do(t, srv, "GET", "/api/consultas", "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	windows := decode[[]map[string]any](t, rec)
	require.Len(t, windows, 1)
	assert.EqualValues(t, 2, windows[0]["consultas"])

	rec = do(t, srv, "GET", "/api/consultas/lentas?limite=1", "", true)
	require.Equal(t, http.StatusOK, rec.Code)
	slow := decode[[]map[string]any](t, rec)
	require.Len(t, slow, 1)
	assert.Equal(t, "UPDATE t SET a = ?", slow[0]["consulta"])
	assert.ElementsMatch(t, []string{"consulta", "tipo", "execucoes", "erros", "p50_ms", "p95_ms"}, keys(slow[0]))

	for _, q := range []string{"limite=0", "limite=abc", "limite=1000", "desde=x"} {
		rec = do(t, srv, "GET", "/api/consultas/lentas?"+q, "", true)
		assert.Equal(t, http.StatusBadRequest, rec.Code, q)
	}
}
