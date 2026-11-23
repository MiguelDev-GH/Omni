package api

import (
	"context"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/MiguelDev-GH/omni/internal/store"
	"github.com/MiguelDev-GH/omni/internal/sysmetrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSystemServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omni.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	srv := NewServer(st, tok)
	srv.Handle("GET /api/sistema", SystemHandler(st))
	return srv, st
}

// fillSystem grava uma leitura a cada 5 s, de now-span até now.
func fillSystem(t *testing.T, st *store.Store, now time.Time, span time.Duration) {
	t.Helper()
	for at := now.Add(-span); !at.After(now); at = at.Add(5 * time.Second) {
		require.NoError(t, st.AddSystemSample(context.Background(), sysmetrics.Snapshot{
			CPUPercent: 10, MemPercent: 20, DiskPercent: 30, At: at,
		}))
	}
}

func sinceAgo(d time.Duration) string {
	return url.QueryEscape(time.Now().Add(-d).UTC().Format(time.RFC3339))
}

func TestSystemDefaultsToLastFifteenMinutes(t *testing.T) {
	srv, st := newSystemServer(t)
	now := time.Now().UTC().Truncate(time.Second)
	for _, at := range []time.Time{now.Add(-time.Hour), now.Add(-time.Minute)} {
		require.NoError(t, st.AddSystemSample(context.Background(), sysmetrics.Snapshot{
			CPUPercent: 10, MemPercent: 20, DiskPercent: 30, At: at,
		}))
	}
	rec := do(t, srv, "GET", "/api/sistema", "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decode[[]map[string]any](t, rec)
	require.Len(t, got, 1)
	for _, k := range []string{"cpu", "memoria", "disco", "hora"} {
		assert.Contains(t, got[0], k)
	}
}

func TestSystemLimitsSinceToOneDay(t *testing.T) {
	srv, st := newSystemServer(t)
	now := time.Now().UTC()
	for _, at := range []time.Time{now.Add(-30 * time.Hour), now.Add(-time.Minute)} {
		require.NoError(t, st.AddSystemSample(context.Background(), sysmetrics.Snapshot{At: at}))
	}
	rec := do(t, srv, "GET", "/api/sistema?desde="+sinceAgo(48*time.Hour), "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Len(t, decode[[]map[string]any](t, rec), 1)
}

func TestSystemSamplesPerMinuteAboveOneHour(t *testing.T) {
	srv, st := newSystemServer(t)
	fillSystem(t, st, time.Now().UTC(), 2*time.Hour)

	rec := do(t, srv, "GET", "/api/sistema?desde="+sinceAgo(2*time.Hour), "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decode[[]map[string]any](t, rec)
	assert.LessOrEqual(t, len(got), 120)
	assert.GreaterOrEqual(t, len(got), 119)
	assert.InDelta(t, 10.0, got[0]["cpu"], 1e-9)
}

func TestSystemKeepsEverySampleUpToOneHour(t *testing.T) {
	srv, st := newSystemServer(t)
	fillSystem(t, st, time.Now().UTC(), 10*time.Minute)

	rec := do(t, srv, "GET", "/api/sistema?desde="+sinceAgo(15*time.Minute), "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Len(t, decode[[]map[string]any](t, rec), 121)
}

func TestSystemRejectsBadSince(t *testing.T) {
	srv, _ := newSystemServer(t)
	rec := do(t, srv, "GET", "/api/sistema?desde=ontem", "", true)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
