package api

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/alerts"
	"github.com/MiguelDev-GH/omni/internal/poller"
	"github.com/MiguelDev-GH/omni/internal/sniffer"
	"github.com/MiguelDev-GH/omni/internal/sniffer/l7"
	"github.com/MiguelDev-GH/omni/internal/store"
	"github.com/MiguelDev-GH/omni/internal/target"
)

func TestReportStep(t *testing.T) {
	assert.Equal(t, 10*time.Second, reportStep(30*time.Minute))
	assert.Equal(t, 20*time.Second, reportStep(time.Hour))
	assert.Equal(t, 290*time.Second, reportStep(24*time.Hour))
	assert.Equal(t, 2020*time.Second, reportStep(168*time.Hour))
}

func TestBucketHTTPWeightsPercentiles(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := bucketHTTP([]l7.HTTPWindow{
		{Start: t0, Requests: 1, P95MS: 100, ByStatus: map[string]uint64{"2xx": 1}},
		{Start: t0.Add(10 * time.Second), Requests: 3, P95MS: 20, ByStatus: map[string]uint64{"2xx": 2, "5xx": 1}},
		{Start: t0.Add(time.Minute), Requests: 2, P95MS: 5},
	}, t0, time.Minute)
	require.Len(t, got, 2)
	assert.Equal(t, uint64(4), got[0].Requests)
	assert.InDelta(t, 40, got[0].P95MS, 0.001)
	assert.Equal(t, map[string]uint64{"2xx": 3, "5xx": 1}, got[0].ByStatus)
}

func TestBucketChecksMarksSlotDownOnAnyFailure(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := bucketChecks([]poller.Result{
		{Port: 80, Kind: "http", Up: true, LatencyMS: 10, At: t0},
		{Port: 80, Kind: "http", Up: false, LatencyMS: 30, Err: "timeout", At: t0.Add(30 * time.Second)},
		{Port: 80, Kind: "http", Up: true, LatencyMS: 5, At: t0.Add(time.Minute)},
	}, t0, time.Minute)
	require.Len(t, got, 2)
	assert.False(t, got[0].Up)
	assert.Equal(t, "timeout", got[0].Err)
	assert.InDelta(t, 20, got[0].LatencyMS, 0.001)
	assert.True(t, got[1].Up)
}

func TestReportHandlerBuildsWholeReport(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "omni.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	srv := NewServer(st, tok)
	srv.Handle("GET /api/alvos/{id}/relatorio", ReportHandler(st))

	tg, err := st.CreateTarget(ctx, target.Target{Name: "loja", Host: "127.0.0.1", AppPorts: []int{3000}})
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	for i, up := range []bool{true, true, false, true} {
		require.NoError(t, st.InsertCheckResult(ctx, poller.Result{TargetID: tg.ID, Port: 3000, Kind: "http", Up: up, LatencyMS: 10, At: now.Add(-time.Duration(i+1) * time.Minute)}))
	}
	require.NoError(t, st.InsertTrafficWindow(ctx, sniffer.Window{Start: now.Add(-time.Minute), Bytes: 100, ByProtocol: map[string]uint64{"tcp": 100}, TopSources: []sniffer.SourceCount{}}))
	_, err = st.AddAlert(ctx, alerts.Alert{Rule: alerts.RuleAvailability, Level: alerts.Red, Previous: alerts.Green, At: now.Add(-2 * time.Minute)})
	require.NoError(t, err)
	_, err = st.CreateScan(ctx, tg.ID, []string{"nmap"}, nil)
	require.NoError(t, err)

	rec := do(t, srv, "GET", "/api/alvos/"+itoa(tg.ID)+"/relatorio?desde="+rfc(now.Add(-time.Hour))+"&ate="+rfc(now.Add(time.Minute)), "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var rep Report
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rep))
	assert.Equal(t, "loja", rep.Target.Name)
	assert.Equal(t, 1, rep.AlertCounts[alerts.Red])
	require.Len(t, rep.Uptime, 1)
	assert.InDelta(t, 75, rep.Uptime[0].Percent, 0.001)
	assert.Len(t, rep.Traffic, 1)
	require.Len(t, rep.Scans, 1)
	assert.Equal(t, []string{"nmap"}, rep.Scans[0].Tools)

	rec = do(t, srv, "GET", "/api/alvos/999/relatorio", "", true)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
