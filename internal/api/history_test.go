package api

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/MiguelDev-GH/omni/internal/poller"
	"github.com/MiguelDev-GH/omni/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newHistoryServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omni.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	srv := NewServer(st, tok)
	srv.Handle("GET /api/alvos/{id}/historico", HistoryHandler(st))
	return srv, st
}

func TestHistoryReturnsResults(t *testing.T) {
	srv, st := newHistoryServer(t)
	tg := create(t, srv, validBody)
	now := time.Now().UTC()
	for i := 0; i < 2; i++ {
		require.NoError(t, st.InsertCheckResult(context.Background(), poller.Result{
			TargetID: tg.ID, Port: 3000, Kind: "tcp", Up: true, LatencyMS: 1, At: now.Add(-time.Duration(i) * time.Minute),
		}))
	}
	rec := do(t, srv, "GET", fmt.Sprintf("/api/alvos/%d/historico", tg.ID), "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decode[[]map[string]any](t, rec)
	require.Len(t, got, 2)
	for _, k := range []string{"alvo_id", "porta", "no_ar", "hora"} {
		assert.Contains(t, got[0], k)
	}
}

func TestHistoryEmptyIsArray(t *testing.T) {
	srv, _ := newHistoryServer(t)
	tg := create(t, srv, validBody)
	rec := do(t, srv, "GET", fmt.Sprintf("/api/alvos/%d/historico", tg.ID), "", true)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "[]", rec.Body.String()[:2])
	assert.JSONEq(t, `[]`, rec.Body.String())
}

func TestHistoryErrors(t *testing.T) {
	srv, _ := newHistoryServer(t)
	tg := create(t, srv, validBody)

	rec := do(t, srv, "GET", fmt.Sprintf("/api/alvos/%d/historico?desde=ontem", tg.ID), "", true)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"erro":"desde inválido"}`, rec.Body.String())

	rec = do(t, srv, "GET", "/api/alvos/999/historico", "", true)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"erro":"alvo não encontrado"}`, rec.Body.String())

	rec = do(t, srv, "GET", "/api/alvos/abc/historico", "", true)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"erro":"id inválido"}`, rec.Body.String())

	rec = do(t, srv, "GET", fmt.Sprintf("/api/alvos/%d/historico", tg.ID), "", false)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
