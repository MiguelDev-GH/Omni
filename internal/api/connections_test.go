package api

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/MiguelDev-GH/omni/internal/connstats"
	"github.com/MiguelDev-GH/omni/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionsHandlerDefaultsTo15Minutes(t *testing.T) {
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omni.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	srv := NewServer(st, tok)
	srv.Handle("GET /api/conexoes", ConnectionsHandler(st))

	now := time.Now().UTC().Truncate(time.Second)
	for _, at := range []time.Time{now.Add(-time.Hour), now.Add(-time.Minute)} {
		require.NoError(t, st.AddConnSnapshot(context.Background(), connstats.Snapshot{At: at, Ports: []connstats.PortConns{
			{Port: 3000, Kind: connstats.KindApp, Established: 4, States: map[string]int{"ESTABLISHED": 4}, Clients: []connstats.ClientCount{}},
		}}))
	}
	rec := do(t, srv, "GET", "/api/conexoes", "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decode[[]map[string]any](t, rec)
	require.Len(t, got, 1)
	assert.Contains(t, got[0], "hora")
	ports := got[0]["portas"].([]any)
	assert.Equal(t, 4.0, ports[0].(map[string]any)["estabelecidas"])

	rec = do(t, srv, "GET", "/api/conexoes?desde="+sinceAgo(2*time.Hour), "", true)
	assert.Len(t, decode[[]map[string]any](t, rec), 2)

	rec = do(t, srv, "GET", "/api/conexoes?desde=ontem", "", true)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
