package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/dbsize"
)

func TestDBSizesRoundTripAndPrune(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, s.AddDBSizes(ctx, []dbsize.Sample{
		{Name: "loja", Bytes: 100, At: at},
		{Name: "postgres", Bytes: 7, At: at},
	}))
	require.NoError(t, s.AddDBSizes(ctx, []dbsize.Sample{{Name: "loja", Bytes: 150, At: at.Add(5 * time.Minute)}}))
	require.NoError(t, s.AddDBSizes(ctx, nil))

	got, err := s.DBSizesSince(ctx, at)
	require.NoError(t, err)
	assert.Equal(t, []dbsize.Sample{
		{Name: "loja", Bytes: 100, At: at},
		{Name: "postgres", Bytes: 7, At: at},
		{Name: "loja", Bytes: 150, At: at.Add(5 * time.Minute)},
	}, got)

	require.NoError(t, s.PruneDBSizes(ctx, at.Add(time.Minute)))
	got, err = s.DBSizesSince(ctx, at)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestDBSizesBetweenKeepsLargestPerStep(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, s.AddDBSizes(ctx, []dbsize.Sample{{Name: "loja", Bytes: 100, At: at}}))
	require.NoError(t, s.AddDBSizes(ctx, []dbsize.Sample{{Name: "loja", Bytes: 120, At: at.Add(30 * time.Second)}}))
	require.NoError(t, s.AddDBSizes(ctx, []dbsize.Sample{{Name: "loja", Bytes: 90, At: at.Add(time.Minute)}}))

	got, err := s.DBSizesBetween(ctx, at, at.Add(2*time.Minute), time.Minute)
	require.NoError(t, err)
	assert.Equal(t, []dbsize.Sample{
		{Name: "loja", Bytes: 120, At: at},
		{Name: "loja", Bytes: 90, At: at.Add(time.Minute)},
	}, got)
}
