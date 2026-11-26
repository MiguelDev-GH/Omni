package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/mailqueue"
	"github.com/MiguelDev-GH/omni/internal/sniffer/l7"
)

func TestDNSHandler(t *testing.T) {
	srv, st := newTrafficServer(t)
	srv.Handle("GET /api/dns", DNSHandler(st))
	now := time.Now().UTC().Truncate(time.Second)
	ctx := context.Background()
	require.NoError(t, st.InsertDNSWindow(ctx, l7.DNSWindow{Start: now.Add(-time.Hour), Queries: 1, ByRCode: map[string]uint64{}}))
	require.NoError(t, st.InsertDNSWindow(ctx, l7.DNSWindow{Start: now.Add(-time.Minute), Queries: 5, Failures: 1,
		ByRCode: map[string]uint64{"NOERROR": 4, "SERVFAIL": 1}}))

	rec := do(t, srv, "GET", "/api/dns", "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decode[[]map[string]any](t, rec)
	require.Len(t, got, 1, "sem desde, só os últimos 15 minutos")
	assert.ElementsMatch(t, []string{"inicio", "consultas", "falhas", "rcodes", "p50_ms", "p95_ms"}, keys(got[0]))

	rec = do(t, srv, "GET", "/api/dns?desde="+sinceAgo(2*time.Hour), "", true)
	assert.Len(t, decode[[]map[string]any](t, rec), 2)
}

func TestSMTPHandler(t *testing.T) {
	srv, st := newTrafficServer(t)
	srv.Handle("GET /api/smtp", SMTPHandler(st))
	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, st.InsertSMTPWindow(context.Background(), l7.SMTPWindow{Start: now.Add(-time.Minute), Attempts: 3, Delivered: 3,
		ByCode: map[string]uint64{}}))

	rec := do(t, srv, "GET", "/api/smtp", "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decode[[]map[string]any](t, rec)
	require.Len(t, got, 1)
	assert.ElementsMatch(t, []string{"inicio", "tentativas", "entregues", "rejeitados", "abortados", "codigos", "p50_ms", "p95_ms"}, keys(got[0]))

	rec = do(t, srv, "GET", "/api/smtp?desde=x", "", true)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSMTPQueueHandler(t *testing.T) {
	srv, st := newTrafficServer(t)
	srv.Handle("GET /api/smtp/fila", SMTPQueueHandler(st, true))
	now := time.Now().UTC().Truncate(time.Second)
	ctx := context.Background()
	require.NoError(t, st.AddQueueSnapshot(ctx, mailqueue.Snapshot{At: now.Add(-time.Hour), Messages: 1}))
	require.NoError(t, st.AddQueueSnapshot(ctx, mailqueue.Snapshot{At: now.Add(-time.Minute), Messages: 12}))

	rec := do(t, srv, "GET", "/api/smtp/fila", "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decode[map[string]any](t, rec)
	assert.Equal(t, true, got["configurado"])
	counts := got["contagens"].([]any)
	require.Len(t, counts, 1)
	assert.Equal(t, 12.0, counts[0].(map[string]any)["mensagens"])

	off, _ := newTrafficServer(t)
	off.Handle("GET /api/smtp/fila", SMTPQueueHandler(st, false))
	got = decode[map[string]any](t, do(t, off, "GET", "/api/smtp/fila", "", true))
	assert.Equal(t, false, got["configurado"])
}
