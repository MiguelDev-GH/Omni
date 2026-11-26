package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/sniffer/l7"
)

func TestDNSWindowsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	w := l7.DNSWindow{Start: at, Queries: 30, Failures: 3, ByRCode: map[string]uint64{"NOERROR": 27, "NXDOMAIN": 3}, P50MS: 1.5, P95MS: 9}
	require.NoError(t, s.InsertDNSWindow(ctx, w))
	require.NoError(t, s.InsertDNSWindow(ctx, l7.DNSWindow{Start: at.Add(10 * time.Second), Queries: 1, ByRCode: map[string]uint64{"NOERROR": 1}}))

	got, err := s.DNSSince(ctx, at, 10)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, w, got[0])

	last, err := s.DNSSince(ctx, at, 1)
	require.NoError(t, err)
	require.Len(t, last, 1)
	assert.Equal(t, uint64(1), last[0].Queries)

	require.NoError(t, s.PruneTraffic(ctx, at.Add(5*time.Second)))
	got, err = s.DNSSince(ctx, at, 10)
	require.NoError(t, err)
	assert.Len(t, got, 1, "a retenção do sniffer vale para o DNS")
}

func TestSMTPWindowsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	w := l7.SMTPWindow{Start: at, Attempts: 10, Delivered: 7, Rejected: 2, Aborted: 1,
		ByCode: map[string]uint64{"550": 2}, P50MS: 40, P95MS: 120}
	require.NoError(t, s.InsertSMTPWindow(ctx, w))

	got, err := s.SMTPSince(ctx, at, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, w, got[0])

	require.NoError(t, s.PruneTraffic(ctx, at.Add(time.Second)))
	got, err = s.SMTPSince(ctx, at, 10)
	require.NoError(t, err)
	assert.Empty(t, got)
}
