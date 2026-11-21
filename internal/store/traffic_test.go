package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/sniffer"
)

func window(at time.Time, bytes uint64) sniffer.Window {
	return sniffer.Window{
		Start: at, Bytes: bytes, Packets: 2, Dropped: 1,
		ByProtocol: map[string]uint64{"HTTP": bytes},
		TopSources: []sniffer.SourceCount{{IP: "10.0.0.1", Bytes: bytes, Packets: 2}},
	}
}

func TestInsertAndReadTrafficWindow(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	w := window(at, 1500)
	require.NoError(t, s.InsertTrafficWindow(ctx, w))

	got, err := s.TrafficSince(ctx, at.Add(-time.Minute), 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, w, got[0])
}

func TestTrafficSinceOrdersFiltersAndLimits(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		require.NoError(t, s.InsertTrafficWindow(ctx, window(t0.Add(time.Duration(i)*time.Second), uint64(i+1))))
	}
	got, err := s.TrafficSince(ctx, t0.Add(time.Second), 3)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, []uint64{3, 4, 5}, []uint64{got[0].Bytes, got[1].Bytes, got[2].Bytes})

	empty, err := s.TrafficSince(ctx, t0.Add(time.Hour), 3)
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}

func TestPruneTraffic(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, s.InsertTrafficWindow(ctx, window(t0, 1)))
	require.NoError(t, s.InsertTrafficWindow(ctx, window(t0.Add(time.Hour), 2)))
	require.NoError(t, s.PruneTraffic(ctx, t0.Add(time.Minute)))

	got, err := s.TrafficSince(ctx, time.Time{}, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, uint64(2), got[0].Bytes)
}
