package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/sysmetrics"
)

func sysSample(at time.Time, v float64) sysmetrics.Snapshot {
	return sysmetrics.Snapshot{CPUPercent: v, MemPercent: v + 1, DiskPercent: v + 2, At: at}
}

func TestAddAndReadSystemSample(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	snap := sysSample(at, 12.5)
	require.NoError(t, s.AddSystemSample(ctx, snap))

	got, err := s.SystemSince(ctx, at.Add(-time.Minute), 0, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, snap, got[0])
}

func TestSystemSinceOrdersFiltersAndLimits(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		require.NoError(t, s.AddSystemSample(ctx, sysSample(t0.Add(time.Duration(i)*5*time.Second), float64(i))))
	}
	got, err := s.SystemSince(ctx, t0.Add(5*time.Second), 0, 3)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, []float64{2, 3, 4}, []float64{got[0].CPUPercent, got[1].CPUPercent, got[2].CPUPercent})

	empty, err := s.SystemSince(ctx, t0.Add(time.Hour), 0, 3)
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}

func TestSystemSinceAveragesPerStep(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// 12 leituras por minuto, durante 3 minutos.
	for i := 0; i < 36; i++ {
		require.NoError(t, s.AddSystemSample(ctx, sysSample(t0.Add(time.Duration(i)*5*time.Second), float64(i/12*10))))
	}
	got, err := s.SystemSince(ctx, t0, time.Minute, 100)
	require.NoError(t, err)
	require.Len(t, got, 3)
	for i, snap := range got {
		assert.Equal(t, t0.Add(time.Duration(i)*time.Minute), snap.At)
		assert.InDelta(t, float64(i*10), snap.CPUPercent, 1e-9)
		assert.InDelta(t, float64(i*10+2), snap.DiskPercent, 1e-9)
	}
}

func TestPruneSystem(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, s.AddSystemSample(ctx, sysSample(t0, 1)))
	require.NoError(t, s.AddSystemSample(ctx, sysSample(t0.Add(time.Hour), 2)))
	require.NoError(t, s.PruneSystem(ctx, t0.Add(time.Minute)))

	got, err := s.SystemSince(ctx, t0.Add(-time.Hour), 0, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 2.0, got[0].CPUPercent)
}

func TestSystemSinceRoundTripsIO(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	io := sysmetrics.Snapshot{CPUPercent: 1, MemPercent: 2, DiskPercent: 3, IO: true,
		ReadBps: 1000, WriteBps: 2000, IOPS: 30, DiskLatencyMS: 4, DiskBusy: 50, At: at}
	noIO := sysSample(at.Add(5*time.Second), 1)
	require.NoError(t, s.AddSystemSample(ctx, io))
	require.NoError(t, s.AddSystemSample(ctx, noIO))

	got, err := s.SystemSince(ctx, at, 0, 10)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, io, got[0])
	assert.Equal(t, noIO, got[1])

	// Na média por minuto, io vale se alguma leitura do minuto tinha I/O, e
	// as taxas são a média só dessas leituras.
	avg, err := s.SystemSince(ctx, at, time.Minute, 10)
	require.NoError(t, err)
	require.Len(t, avg, 1)
	assert.True(t, avg[0].IO)
	assert.InDelta(t, 30.0, avg[0].IOPS, 1e-9)

	between, err := s.SystemBetween(ctx, at, at.Add(time.Minute), time.Minute)
	require.NoError(t, err)
	require.Len(t, between, 1)
	assert.InDelta(t, 4.0, between[0].DiskLatencyMS, 1e-9)
}
