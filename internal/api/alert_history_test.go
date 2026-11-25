package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/alerts"
	"github.com/MiguelDev-GH/omni/internal/store"
)

func newAlertHistoryServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omni.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	srv := NewServer(st, tok)
	srv.Handle("GET /api/alertas/historico", AlertHistoryHandler(st))
	return srv, st
}

func rfc(t time.Time) string { return url.QueryEscape(t.UTC().Format(time.RFC3339)) }

func TestAlertHistoryFiltersByRange(t *testing.T) {
	srv, st := newAlertHistoryServer(t)
	now := time.Now().UTC().Truncate(time.Second)
	for _, at := range []time.Time{now.Add(-48 * time.Hour), now.Add(-time.Hour)} {
		_, err := st.AddAlert(context.Background(), alerts.Alert{Rule: alerts.RuleCPU, Service: alerts.ServiceSystem, Level: alerts.Red, Previous: alerts.Green, At: at})
		require.NoError(t, err)
	}

	rec := do(t, srv, "GET", "/api/alertas/historico", "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got []alerts.Alert
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Len(t, got, 1, "padrão são as últimas 24 h")

	rec = do(t, srv, "GET", "/api/alertas/historico?desde="+rfc(now.Add(-72*time.Hour))+"&ate="+rfc(now), "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Len(t, got, 2)
}

func TestAlertHistoryRejectsBadRanges(t *testing.T) {
	srv, _ := newAlertHistoryServer(t)
	now := time.Now()
	for name, q := range map[string]string{
		"maior que 168h": "?desde=" + rfc(now.Add(-169*time.Hour)) + "&ate=" + rfc(now),
		"invertido":      "?desde=" + rfc(now) + "&ate=" + rfc(now.Add(-time.Hour)),
		"desde ruim":     "?desde=ontem",
		"ate ruim":       "?ate=hoje",
	} {
		rec := do(t, srv, "GET", "/api/alertas/historico"+q, "", true)
		assert.Equal(t, http.StatusBadRequest, rec.Code, name)
	}
}
