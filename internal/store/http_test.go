package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/sniffer/l7"
)

func httpWindow(t *testing.T, at time.Time, latencies map[string][]time.Duration) l7.HTTPWindow {
	t.Helper()
	s := l7.NewHTTPStats()
	s.Flush(at)
	for route, ls := range latencies {
		for _, l := range ls {
			s.Add(l7.HTTPExchange{Method: "GET", Route: route, Status: 200, Latency: l})
		}
	}
	return s.Flush(at.Add(10 * time.Second))
}

func TestHTTPWindowsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	w1 := httpWindow(t, at, map[string][]time.Duration{"/a": {time.Millisecond, 2 * time.Millisecond}, "/b": {100 * time.Millisecond}})
	w2 := httpWindow(t, at.Add(10*time.Second), map[string][]time.Duration{"/a": {time.Millisecond}})
	require.NoError(t, s.InsertHTTPWindow(ctx, w1))
	require.NoError(t, s.InsertHTTPWindow(ctx, w2))
	require.NoError(t, s.InsertHTTPWindow(ctx, w2)) // regravar substitui

	got, err := s.HTTPSince(ctx, at, 10)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, at, got[0].Start)
	assert.Equal(t, uint64(3), got[0].Requests)
	assert.Equal(t, map[string]uint64{"2xx": 3}, got[0].ByStatus)
	assert.Equal(t, w1.P95MS, got[0].P95MS)
	assert.Empty(t, got[0].Routes)

	routes, err := s.HTTPRoutesSince(ctx, at, 10)
	require.NoError(t, err)
	require.Len(t, routes, 2)
	assert.Equal(t, "/a", routes[0].Route)
	assert.Equal(t, uint64(3), routes[0].Requests)
	assert.InDelta(t, 100, routes[1].P95MS, 25)

	require.NoError(t, s.PruneTraffic(ctx, at.Add(5*time.Second)))
	got, err = s.HTTPSince(ctx, at, 10)
	require.NoError(t, err)
	assert.Len(t, got, 1)
	routes, err = s.HTTPRoutesSince(ctx, at, 10)
	require.NoError(t, err)
	require.Len(t, routes, 1)
	assert.Equal(t, uint64(1), routes[0].Requests)
}
