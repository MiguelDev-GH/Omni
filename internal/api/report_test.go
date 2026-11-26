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
	"github.com/MiguelDev-GH/omni/internal/connstats"
	"github.com/MiguelDev-GH/omni/internal/dbsize"
	"github.com/MiguelDev-GH/omni/internal/mailqueue"
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
		{Start: t0.Add(10 * time.Second), Requests: 3, P95MS: 20, ByStatus: map[string]uint64{"2xx": 2, "5xx": 1}, ByCode: map[string]uint64{"503": 1}},
		{Start: t0.Add(time.Minute), Requests: 2, P95MS: 5},
	}, t0, time.Minute)
	require.Len(t, got, 2)
	assert.Equal(t, uint64(4), got[0].Requests)
	assert.InDelta(t, 40, got[0].P95MS, 0.001)
	assert.Equal(t, map[string]uint64{"2xx": 3, "5xx": 1}, got[0].ByStatus)
	assert.Equal(t, map[string]uint64{"503": 1}, got[0].ByCode)
}

func TestBucketDBSumsFailedTransactions(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := bucketDB([]l7.DBWindow{
		{Start: t0, Queries: 2, TxFailed: 1},
		{Start: t0.Add(10 * time.Second), Queries: 1, TxFailed: 2},
	}, t0, time.Minute)
	require.Len(t, got, 1)
	assert.Equal(t, uint64(3), got[0].TxFailed)
}

func TestBucketDNSAndSMTPSumPerSlot(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	dns := bucketDNS([]l7.DNSWindow{
		{Start: t0, Queries: 1, Failures: 1, P95MS: 40, ByRCode: map[string]uint64{"SERVFAIL": 1}},
		{Start: t0.Add(10 * time.Second), Queries: 3, P95MS: 0},
		{Start: t0.Add(time.Minute), Queries: 2},
	}, t0, time.Minute)
	require.Len(t, dns, 2)
	assert.Equal(t, uint64(4), dns[0].Queries)
	assert.Equal(t, uint64(1), dns[0].Failures)
	assert.InDelta(t, 10, dns[0].P95MS, 0.001)
	assert.Equal(t, map[string]uint64{"SERVFAIL": 1}, dns[0].ByRCode)

	smtp := bucketSMTP([]l7.SMTPWindow{
		{Start: t0, Attempts: 2, Delivered: 1, Rejected: 1, ByCode: map[string]uint64{"550": 1}},
		{Start: t0.Add(20 * time.Second), Attempts: 1, Aborted: 1},
	}, t0, time.Minute)
	require.Len(t, smtp, 1)
	assert.Equal(t, uint64(3), smtp[0].Attempts)
	assert.Equal(t, uint64(1), smtp[0].Delivered)
	assert.Equal(t, uint64(1), smtp[0].Rejected)
	assert.Equal(t, uint64(1), smtp[0].Aborted)
	assert.Equal(t, map[string]uint64{"550": 1}, smtp[0].ByCode)
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
	require.NoError(t, st.InsertDNSWindow(ctx, l7.DNSWindow{Start: now.Add(-time.Minute), Queries: 5, Failures: 1}))
	require.NoError(t, st.InsertSMTPWindow(ctx, l7.SMTPWindow{Start: now.Add(-time.Minute), Attempts: 2, Delivered: 2}))
	require.NoError(t, st.AddQueueSnapshot(ctx, mailqueue.Snapshot{At: now.Add(-time.Minute), Messages: 4}))
	require.NoError(t, st.AddConnSnapshot(ctx, connstats.Snapshot{At: now.Add(-time.Minute), Ports: []connstats.PortConns{{Port: 3000, Kind: connstats.KindApp, Established: 6}}}))
	require.NoError(t, st.AddDBSizes(ctx, []dbsize.Sample{{Name: "loja", Bytes: 2048, At: now.Add(-time.Minute)}}))
	_, err = st.AddAlert(ctx, alerts.Alert{Rule: alerts.RuleAvailability, Level: alerts.Red, Previous: alerts.Green, At: now.Add(-2 * time.Minute)})
	require.NoError(t, err)
	// Um ataque com muitos IPs gera mais mudanças do que a lista do relatório
	// mostra; as contagens continuam exatas.
	for i := range reportAlertRows + 50 {
		_, err = st.AddAlert(ctx, alerts.Alert{Rule: alerts.RuleAuthFailIP, Subject: "10.0.0." + itoa(int64(i)), Level: alerts.Yellow, Previous: alerts.Green, At: now.Add(-3 * time.Minute)})
		require.NoError(t, err)
	}
	_, err = st.CreateScan(ctx, tg.ID, []string{"nmap"}, nil)
	require.NoError(t, err)

	rec := do(t, srv, "GET", "/api/alvos/"+itoa(tg.ID)+"/relatorio?desde="+rfc(now.Add(-time.Hour))+"&ate="+rfc(now.Add(time.Minute)), "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var rep Report
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rep))
	assert.Equal(t, "loja", rep.Target.Name)
	assert.Equal(t, 1, rep.AlertCounts[alerts.Red])
	assert.Equal(t, reportAlertRows+50, rep.AlertCounts[alerts.Yellow])
	require.Len(t, rep.Alerts, reportAlertRows)
	assert.Equal(t, alerts.Red, rep.Alerts[0].Level, "a mais grave primeiro")
	require.Len(t, rep.AlertSummary, 2)
	assert.Equal(t, alerts.RuleAvailability, rep.AlertSummary[0].Rule)
	assert.Equal(t, reportAlertRows+50, rep.AlertSummary[1].Subjects)
	require.Len(t, rep.Uptime, 1)
	assert.InDelta(t, 75, rep.Uptime[0].Percent, 0.001)
	assert.Len(t, rep.Traffic, 1)
	require.Len(t, rep.DNS, 1)
	assert.Equal(t, uint64(5), rep.DNS[0].Queries)
	require.Len(t, rep.SMTP, 1)
	assert.Equal(t, uint64(2), rep.SMTP[0].Delivered)
	require.Len(t, rep.Queue, 1)
	assert.Equal(t, 4, rep.Queue[0].Messages)
	require.Len(t, rep.Conns, 1)
	assert.Equal(t, 6, rep.Conns[0].Ports[0].Established)
	require.Len(t, rep.DBSizes, 1)
	assert.Equal(t, int64(2048), rep.DBSizes[0].Bytes)
	require.Len(t, rep.Scans, 1)
	assert.Equal(t, []string{"nmap"}, rep.Scans[0].Tools)

	rec = do(t, srv, "GET", "/api/alvos/999/relatorio", "", true)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
