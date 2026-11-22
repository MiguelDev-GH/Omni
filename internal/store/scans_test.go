package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/target"
)

func newScanTarget(t *testing.T, s *Store) target.Target {
	t.Helper()
	tg, err := s.CreateTarget(context.Background(), target.Target{Name: "api", Host: "127.0.0.1", AppPorts: []int{3000}})
	require.NoError(t, err)
	return tg
}

func TestScanRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	tg := newScanTarget(t, s)

	sc, err := s.CreateScan(ctx, tg.ID, []string{"nmap"}, map[string]map[string]string{"nmap": {"a": "b"}})
	require.NoError(t, err)
	assert.Equal(t, ScanQueued, sc.Status)
	require.NoError(t, s.UpdateScanStatus(ctx, sc.ID, ScanRunning, ""))

	ssh := findings.New("nmap", "Porta 22/tcp aberta: ssh", "OpenSSH 9.6", findings.Info, 22)
	crit := findings.New("nmap", "Teste", "", findings.Critical, 3000)
	require.NoError(t, s.AddFindings(ctx, sc.ID, []findings.Finding{ssh, ssh, crit}))
	require.NoError(t, s.UpdateScanStatus(ctx, sc.ID, ScanDone, ""))

	got, err := s.GetScan(ctx, sc.ID)
	require.NoError(t, err)
	assert.Equal(t, ScanDone, got.Status)
	assert.Equal(t, []string{"nmap"}, got.Tools)
	assert.Equal(t, map[string]map[string]string{"nmap": {"a": "b"}}, got.Options)
	require.NotNil(t, got.StartedAt)
	require.NotNil(t, got.FinishedAt)
	assert.Equal(t, map[findings.Severity]int{findings.Info: 1, findings.Critical: 1}, got.Counts)

	list, err := s.ScanFindings(ctx, sc.ID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, findings.Critical, list[0].Severity)
	assert.Equal(t, ssh, list[1])

	scans, err := s.ListScans(ctx, tg.ID)
	require.NoError(t, err)
	require.Len(t, scans, 1)
	assert.Equal(t, 2, scans[0].Counts[findings.Info]+scans[0].Counts[findings.Critical])

	_, err = s.GetScan(ctx, 999)
	assert.ErrorIs(t, err, ErrScanNotFound)
	assert.ErrorIs(t, s.UpdateScanStatus(ctx, 999, ScanDone, ""), ErrScanNotFound)
	assert.Error(t, s.UpdateScanStatus(ctx, sc.ID, "x", ""))
}

func TestPreviousScanAndInterrupted(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	tg := newScanTarget(t, s)
	mk := func(tools []string, status string) int64 {
		sc, err := s.CreateScan(ctx, tg.ID, tools, nil)
		require.NoError(t, err)
		if status != ScanQueued {
			require.NoError(t, s.UpdateScanStatus(ctx, sc.ID, status, ""))
		}
		return sc.ID
	}
	first := mk([]string{"nmap"}, ScanDone)
	mk([]string{"nmap"}, ScanFailed)
	mk([]string{"nuclei"}, ScanDone)
	running := mk([]string{"nmap"}, ScanRunning)
	queued := mk([]string{"nmap"}, ScanQueued)

	prev, ok, err := s.PreviousScan(ctx, tg.ID, running, "nmap")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, first, prev.ID)
	_, ok, err = s.PreviousScan(ctx, tg.ID, first, "nmap")
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, s.FailInterruptedScans(ctx))
	for _, id := range []int64{running, queued} {
		sc, err := s.GetScan(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, ScanFailed, sc.Status)
		assert.Contains(t, sc.Error, "interrompida")
	}

	require.NoError(t, s.DeleteTarget(ctx, tg.ID))
	list, err := s.ListScans(ctx, 0)
	require.NoError(t, err)
	assert.Empty(t, list)
}
