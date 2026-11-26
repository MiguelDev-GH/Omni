package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/capture"
)

func TestCapturesRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	c, err := s.CreateCapture(ctx, capture.Capture{StartedAt: t0, Duration: 60, IP: "10.0.0.1", Port: 3000, Protocol: "HTTP"})
	require.NoError(t, err)
	assert.Equal(t, capture.FileName(c.ID), c.File)
	assert.Equal(t, capture.StatusRecording, c.Status)

	got, err := s.GetCapture(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, c, got)
	assert.Nil(t, got.FinishedAt)

	end := t0.Add(time.Minute)
	c.Status, c.Packets, c.Bytes, c.Dropped, c.FinishedAt = capture.StatusDone, 10, 2048, 1, &end
	require.NoError(t, s.FinishCapture(ctx, c))
	got, err = s.GetCapture(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, c, got)

	_, err = s.GetCapture(ctx, c.ID+1)
	assert.ErrorIs(t, err, capture.ErrNotFound)
	assert.ErrorIs(t, s.FinishCapture(ctx, capture.Capture{ID: c.ID + 1}), capture.ErrNotFound)
}

func TestCapturesListPruneAndInterrupted(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	old, err := s.CreateCapture(ctx, capture.Capture{StartedAt: t0, Duration: 10})
	require.NoError(t, err)
	old.Status = capture.StatusDone
	require.NoError(t, s.FinishCapture(ctx, old))
	running, err := s.CreateCapture(ctx, capture.Capture{StartedAt: t0.Add(time.Second), Duration: 10})
	require.NoError(t, err)
	recent, err := s.CreateCapture(ctx, capture.Capture{StartedAt: t0.Add(time.Hour), Duration: 10})
	require.NoError(t, err)

	list, err := s.ListCaptures(ctx)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, []int64{recent.ID, running.ID, old.ID}, []int64{list[0].ID, list[1].ID, list[2].ID})

	before, err := s.CapturesBefore(ctx, t0.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, before, 1, "a captura gravando nunca é apagada pela retenção")
	assert.Equal(t, old.ID, before[0].ID)

	require.NoError(t, s.FailInterruptedCaptures(ctx))
	got, err := s.GetCapture(ctx, running.ID)
	require.NoError(t, err)
	assert.Equal(t, capture.StatusFailed, got.Status)
	assert.NotEmpty(t, got.Error)
	assert.NotNil(t, got.FinishedAt)

	require.NoError(t, s.DeleteCapture(ctx, old.ID))
	assert.ErrorIs(t, s.DeleteCapture(ctx, old.ID), capture.ErrNotFound)
}
