package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/alerts"
)

func alertAt(at time.Time, level alerts.Level) alerts.Alert {
	return alerts.Alert{
		Rule: alerts.RuleAvailability, Service: alerts.ServiceWeb, Name: "Disponibilidade",
		Subject: "loja:3000", Level: level, Previous: alerts.Green, Value: 1, Threshold: 1,
		Unit: "", Detail: "sem resposta", At: at,
	}
}

func TestAlertsRoundTripAndFilter(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, lvl := range []alerts.Level{alerts.Yellow, alerts.Red, alerts.Green} {
		_, err := s.AddAlert(ctx, alertAt(t0.Add(time.Duration(i)*time.Hour), lvl))
		require.NoError(t, err)
	}

	got, err := s.AlertsBetween(ctx, t0.Add(30*time.Minute), t0.Add(2*time.Hour), 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, alerts.Red, got[0].Level)
	assert.Equal(t, alerts.Green, got[0].Previous)
	assert.Equal(t, "sem resposta", got[0].Detail)
	assert.Equal(t, t0.Add(time.Hour), got[0].At)
	assert.NotZero(t, got[0].ID)

	all, err := s.AlertsBetween(ctx, t0, t0.Add(24*time.Hour), 10)
	require.NoError(t, err)
	require.Len(t, all, 3)
	assert.Equal(t, alerts.Green, all[0].Level, "mais novo primeiro")

	require.NoError(t, s.PruneAlerts(ctx, t0.Add(90*time.Minute)))
	left, err := s.AlertsBetween(ctx, t0, t0.Add(24*time.Hour), 10)
	require.NoError(t, err)
	assert.Len(t, left, 1)
}
