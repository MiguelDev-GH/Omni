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

func TestWorstAlertsAndSummary(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	add := func(i int, rule, subject string, lvl alerts.Level) {
		a := alertAt(t0.Add(time.Duration(i)*time.Minute), lvl)
		a.Rule, a.Subject = rule, subject
		_, err := s.AddAlert(ctx, a)
		require.NoError(t, err)
	}
	add(0, alerts.RuleAvailability, "loja:3000", alerts.Yellow)
	add(1, alerts.RuleAvailability, "loja:3000", alerts.Green)
	add(2, alerts.RuleAuthFailIP, "10.0.0.1", alerts.Red)
	add(3, alerts.RuleAuthFailIP, "10.0.0.2", alerts.Red)
	add(4, alerts.RuleAuthFailIP, "10.0.0.2", alerts.Green)

	worst, err := s.WorstAlertsBetween(ctx, t0, t0.Add(time.Hour), 3)
	require.NoError(t, err)
	require.Len(t, worst, 3)
	assert.Equal(t, alerts.Red, worst[0].Level)
	assert.Equal(t, "10.0.0.2", worst[0].Subject, "mais novo primeiro no mesmo nível")
	assert.Equal(t, alerts.Red, worst[1].Level)
	assert.Equal(t, alerts.Yellow, worst[2].Level)

	sum, err := s.AlertSummaryBetween(ctx, t0, t0.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, sum, 2)
	assert.Equal(t, alerts.RuleAuthFailIP, sum[0].Rule, "mais vermelhos primeiro")
	assert.Equal(t, 2, sum[0].Subjects)
	assert.Equal(t, map[alerts.Level]int{alerts.Green: 1, alerts.Yellow: 0, alerts.Red: 2}, sum[0].Counts)
	assert.Equal(t, map[alerts.Level]int{alerts.Green: 1, alerts.Yellow: 1, alerts.Red: 0}, sum[1].Counts)

	empty, err := s.AlertSummaryBetween(ctx, t0.Add(time.Hour), t0.Add(2*time.Hour))
	require.NoError(t, err)
	assert.Empty(t, empty)
}
