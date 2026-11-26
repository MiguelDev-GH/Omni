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

func TestCheckHistoryLimitIsPerPort(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	id := newTarget(t, s)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// A porta 3000 é verificada bem mais vezes; a 5432 não pode sumir do histórico.
	for i := 0; i < 6; i++ {
		require.NoError(t, s.InsertCheckResult(ctx, result(id, t0.Add(time.Duration(i)*time.Minute))))
	}
	db := poller.Result{TargetID: id, Port: 5432, Kind: "tcp", Up: true, At: t0}
	require.NoError(t, s.InsertCheckResult(ctx, db))

	got, err := s.CheckHistory(ctx, id, t0, 2)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, 5432, got[0].Port)
	assert.Equal(t, t0.Add(4*time.Minute), got[1].At)
	assert.Equal(t, t0.Add(5*time.Minute), got[2].At)
}

func TestPruneChecksDeletesOlder(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	id := newTarget(t, s)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		require.NoError(t, s.InsertCheckResult(ctx, result(id, t0.Add(time.Duration(i)*time.Hour))))
	}
	require.NoError(t, s.PruneChecks(ctx, t0.Add(time.Hour)))
	got, err := s.CheckHistory(ctx, id, time.Unix(0, 0), 10)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, t0.Add(time.Hour), got[0].At)
}

func TestCheckHistoryKeepsProbeDetails(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	id := newTarget(t, s)
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	noTLS := false
	dns := poller.Result{TargetID: id, Port: 1053, Kind: "dns", Up: true, Degraded: true, Detail: "NXDOMAIN", LatencyMS: 1, At: at}
	smtp := poller.Result{TargetID: id, Port: 1025, Kind: "smtp", Up: true, StartTLS: &noTLS, Detail: "aceita destinatário postmaster", LatencyMS: 2, At: at.Add(time.Second)}
	web := result(id, at.Add(2*time.Second))
	for _, r := range []poller.Result{dns, smtp, web} {
		require.NoError(t, s.InsertCheckResult(ctx, r))
	}

	for name, get := range map[string]func() ([]poller.Result, error){
		"CheckHistory": func() ([]poller.Result, error) { return s.CheckHistory(ctx, id, at.Add(-time.Hour), 10) },
		"ChecksBetween": func() ([]poller.Result, error) {
			return s.ChecksBetween(ctx, id, at.Add(-time.Hour), at.Add(time.Hour))
		},
	} {
		got, err := get()
		require.NoError(t, err, name)
		require.Len(t, got, 3, name)
		assert.True(t, got[0].Degraded, name)
		assert.Equal(t, "NXDOMAIN", got[0].Detail, name)
		assert.Nil(t, got[0].StartTLS, name)
		require.NotNil(t, got[1].StartTLS, name)
		assert.False(t, *got[1].StartTLS, name)
		assert.Equal(t, "aceita destinatário postmaster", got[1].Detail, name)
		assert.Nil(t, got[2].StartTLS, name)
		assert.False(t, got[2].Degraded, name)
	}
}
