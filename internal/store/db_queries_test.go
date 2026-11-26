package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/sniffer/l7"
)

func TestDBWindowsAndSlowQueries(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	stats := l7.NewDBStats()
	stats.Flush(at)
	for range 50 {
		stats.Add(l7.DBQuery{Query: "SELECT ?", Kind: "SELECT", Latency: time.Millisecond})
	}
	stats.Add(l7.DBQuery{Query: "UPDATE t SET a = ?", Kind: "UPDATE", Latency: 400 * time.Millisecond, Error: true})
	require.NoError(t, s.InsertDBWindow(ctx, stats.Flush(at.Add(10*time.Second))))
	stats.Add(l7.DBQuery{Query: "SELECT ?", Kind: "SELECT", Latency: time.Millisecond})
	require.NoError(t, s.InsertDBWindow(ctx, stats.Flush(at.Add(20*time.Second))))

	windows, err := s.DBSince(ctx, at, 10)
	require.NoError(t, err)
	require.Len(t, windows, 2)
	assert.Equal(t, uint64(51), windows[0].Queries)
	assert.Equal(t, uint64(1), windows[0].Errors)
	assert.Equal(t, map[string]uint64{"SELECT": 50, "UPDATE": 1}, windows[0].ByKind)

	slow, err := s.SlowQueriesSince(ctx, at, 20)
	require.NoError(t, err)
	require.Len(t, slow, 2)
	assert.Equal(t, "UPDATE t SET a = ?", slow[0].Query)
	assert.InDelta(t, 400, slow[0].P95MS, 100)
	assert.Equal(t, uint64(51), slow[1].Count)

	slow, err = s.SlowQueriesSince(ctx, at, 1)
	require.NoError(t, err)
	assert.Len(t, slow, 1)

	require.NoError(t, s.PruneTraffic(ctx, at.Add(5*time.Second)))
	windows, err = s.DBSince(ctx, at, 10)
	require.NoError(t, err)
	assert.Len(t, windows, 1)
}

func TestDBWindowRoundTripsTxFailed(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	stats := l7.NewDBStats()
	stats.Flush(at)
	stats.Add(l7.DBQuery{Query: "ROLLBACK", Kind: "ROLLBACK", Latency: time.Millisecond})
	stats.AddTxFailure()
	require.NoError(t, s.InsertDBWindow(ctx, stats.Flush(at.Add(10*time.Second))))

	for name, get := range map[string]func() ([]l7.DBWindow, error){
		"DBSince":   func() ([]l7.DBWindow, error) { return s.DBSince(ctx, at, 10) },
		"DBBetween": func() ([]l7.DBWindow, error) { return s.DBBetween(ctx, at, at.Add(time.Hour)) },
	} {
		got, err := get()
		require.NoError(t, err, name)
		require.Len(t, got, 1, name)
		assert.Equal(t, uint64(1), got[0].TxFailed, name)
		assert.Equal(t, uint64(1), got[0].ByKind["ROLLBACK"], name)
	}
}
