package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/connstats"
)

func connSnapshot(at time.Time, established int) connstats.Snapshot {
	return connstats.Snapshot{At: at, Ports: []connstats.PortConns{
		{Port: 3000, Kind: connstats.KindApp, Established: established, States: map[string]int{"ESTABLISHED": established}, Clients: []connstats.ClientCount{}},
		{Port: 5432, Kind: connstats.KindDB, Established: 2, States: map[string]int{"ESTABLISHED": 2},
			Clients: []connstats.ClientCount{{Process: "loja", Conns: 2}}},
	}}
}

func TestConnectionsSinceRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 3 {
		require.NoError(t, s.AddConnSnapshot(ctx, connSnapshot(at.Add(time.Duration(i)*5*time.Second), i)))
	}
	require.NoError(t, s.AddConnSnapshot(ctx, connstats.Snapshot{At: at.Add(time.Minute)})) // sem portas: nada a gravar

	got, err := s.ConnectionsSince(ctx, at.Add(time.Second), 10)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, connSnapshot(at.Add(5*time.Second), 1), got[0])
	assert.Equal(t, at.Add(10*time.Second), got[1].At)

	last, err := s.ConnectionsSince(ctx, at, 1)
	require.NoError(t, err)
	require.Len(t, last, 1)
	assert.Equal(t, 2, last[0].Ports[0].Established)

	require.NoError(t, s.PruneConnections(ctx, at.Add(6*time.Second)))
	got, err = s.ConnectionsSince(ctx, at, 10)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestConnectionsBetweenKeepsPeakPerStep(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, n := range []int{1, 4, 2, 7} {
		require.NoError(t, s.AddConnSnapshot(ctx, connSnapshot(at.Add(time.Duration(i)*30*time.Second), n)))
	}

	got, err := s.ConnectionsBetween(ctx, at, at.Add(90*time.Second), time.Minute)
	require.NoError(t, err)
	require.Len(t, got, 2, "a coleta de 90 s fica de fora")
	assert.Equal(t, at, got[0].At)
	require.Len(t, got[0].Ports, 2)
	assert.Equal(t, 3000, got[0].Ports[0].Port)
	assert.Equal(t, 4, got[0].Ports[0].Established)
	assert.Equal(t, 2, got[0].Ports[1].Established)
	assert.Equal(t, at.Add(time.Minute), got[1].At)
	assert.Equal(t, 2, got[1].Ports[0].Established)
}
