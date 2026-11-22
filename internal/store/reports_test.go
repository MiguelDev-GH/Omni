package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	tg := newScanTarget(t, s)
	sc, err := s.CreateScan(ctx, tg.ID, []string{"nmap", "nuclei"}, nil)
	require.NoError(t, err)

	_, ok, err := s.Report(ctx, sc.ID, "nmap")
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, s.SaveReport(ctx, sc.ID, "nmap", json.RawMessage(`{"hosts":[]}`)))
	require.NoError(t, s.SaveReport(ctx, sc.ID, "nmap", json.RawMessage(`{"hosts":[{"endereco":"127.0.0.1"}]}`)))
	got, ok, err := s.Report(ctx, sc.ID, "nmap")
	require.NoError(t, err)
	require.True(t, ok)
	assert.JSONEq(t, `{"hosts":[{"endereco":"127.0.0.1"}]}`, string(got))

	_, ok, err = s.Report(ctx, sc.ID, "nuclei")
	require.NoError(t, err)
	assert.False(t, ok)

	// Varreduras antigas, sem opções gravadas, voltam com o mapa vazio.
	old, err := s.GetScan(ctx, sc.ID)
	require.NoError(t, err)
	assert.Equal(t, map[string]map[string]string{}, old.Options)
}
