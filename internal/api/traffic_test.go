package api

import (
	"context"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/MiguelDev-GH/omni/internal/sniffer"
	"github.com/MiguelDev-GH/omni/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTrafficServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omni.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	srv := NewServer(st, tok)
	srv.Handle("GET /api/trafego", TrafficHandler(st))
	return srv, st
}

func TestTrafficDefaultsToLastFifteenMinutes(t *testing.T) {
	srv, st := newTrafficServer(t)
	now := time.Now().UTC().Truncate(time.Second)
	for _, at := range []time.Time{now.Add(-time.Hour), now.Add(-time.Minute)} {
		require.NoError(t, st.InsertTrafficWindow(context.Background(), sniffer.Window{
			Start: at, Bytes: 100, Packets: 1, ByProtocol: map[string]uint64{"HTTP": 100},
			TopSources: []sniffer.SourceCount{{IP: "10.0.0.1", Bytes: 100, Packets: 1}},
		}))
	}
	rec := do(t, srv, "GET", "/api/trafego", "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decode[[]map[string]any](t, rec)
	require.Len(t, got, 1)
	for _, k := range []string{"inicio", "bytes", "pacotes", "descartados", "protocolos", "top_origens"} {
		assert.Contains(t, got[0], k)
	}

	rec = do(t, srv, "GET", "/api/trafego?desde="+url.QueryEscape(now.Add(-2*time.Hour).Format(time.RFC3339)), "", true)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, decode[[]map[string]any](t, rec), 2)
}

func TestTrafficErrors(t *testing.T) {
	srv, _ := newTrafficServer(t)
	rec := do(t, srv, "GET", "/api/trafego?desde=ontem", "", true)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"erro":"desde inválido"}`, rec.Body.String())

	rec = do(t, srv, "GET", "/api/trafego", "", false)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = do(t, srv, "GET", "/api/trafego", "", true)
	assert.JSONEq(t, `[]`, rec.Body.String())
}
