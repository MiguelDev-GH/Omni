package store

import (
	"context"
	"testing"
	"time"

	"github.com/MiguelDev-GH/omni/internal/poller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTarget(t *testing.T, s *Store) int64 {
	t.Helper()
	created, err := s.CreateTarget(context.Background(), sample())
	require.NoError(t, err)
	return created.ID
}

func result(id int64, at time.Time) poller.Result {
	return poller.Result{TargetID: id, Port: 3000, Kind: "http", Up: true, StatusCode: 200, LatencyMS: 12.5, At: at}
}

func TestInsertAndReadCheckResult(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	id := newTarget(t, s)

	at := time.Date(2026, 1, 2, 3, 4, 5, 678_901_234, time.FixedZone("x", -3*3600))
	r := poller.Result{TargetID: id, Port: 3000, Kind: "http", Up: false, StatusCode: 503, LatencyMS: 12.5, Err: "falhou", At: at}
	require.NoError(t, s.InsertCheckResult(ctx, r))

	got, err := s.CheckHistory(ctx, id, at.Add(-time.Hour), 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	r.At = at.Truncate(time.Millisecond).UTC()
	assert.Equal(t, r, got[0])
	assert.Equal(t, time.UTC, got[0].At.Location())
}

func TestCheckHistoryFiltersBySince(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	id := newTarget(t, s)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		require.NoError(t, s.InsertCheckResult(ctx, result(id, t0.Add(time.Duration(i)*time.Minute))))
	}
	got, err := s.CheckHistory(ctx, id, t0.Add(time.Minute), 10)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, t0.Add(time.Minute), got[0].At)
	assert.Equal(t, t0.Add(2*time.Minute), got[1].At)
}

func TestCheckHistoryLimitKeepsNewest(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	id := newTarget(t, s)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		require.NoError(t, s.InsertCheckResult(ctx, result(id, t0.Add(time.Duration(i)*time.Minute))))
	}
	got, err := s.CheckHistory(ctx, id, t0, 2)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, t0.Add(3*time.Minute), got[0].At)
	assert.Equal(t, t0.Add(4*time.Minute), got[1].At)
}

func TestCheckHistoryEmpty(t *testing.T) {
	s := openTest(t)
	id := newTarget(t, s)
	got, err := s.CheckHistory(context.Background(), id, time.Unix(0, 0), 10)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Len(t, got, 0)
}

func TestDeleteTargetCascadesHistory(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	id := newTarget(t, s)
	require.NoError(t, s.InsertCheckResult(ctx, result(id, time.Now())))
	require.NoError(t, s.DeleteTarget(ctx, id))
	newTarget(t, s)

	got, err := s.CheckHistory(ctx, id, time.Unix(0, 0), 10)
	require.NoError(t, err)
	assert.Len(t, got, 0)
}

func TestInsertCheckResultUnknownTarget(t *testing.T) {
	s := openTest(t)
	assert.Error(t, s.InsertCheckResult(context.Background(), result(999, time.Now())))
}
