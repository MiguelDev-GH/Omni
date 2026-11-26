package api

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/dbsize"
	"github.com/MiguelDev-GH/omni/internal/store"
)

func newDBSizeServer(t *testing.T, configured bool) (*Server, *store.Store) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omni.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	srv := NewServer(st, tok)
	srv.Handle("GET /api/banco/tamanho", DBSizeHandler(st, configured))
	return srv, st
}

func TestDBSizeHandlerNotConfigured(t *testing.T) {
	srv, _ := newDBSizeServer(t, false)
	rec := do(t, srv, "GET", "/api/banco/tamanho", "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decode[map[string]any](t, rec)
	assert.Equal(t, false, got["configurado"])
	assert.Equal(t, []any{}, got["bancos"])
}

func TestDBSizeHandlerGrowthPerDatabase(t *testing.T) {
	srv, st := newDBSizeServer(t, true)
	now := time.Now().UTC().Truncate(time.Second)
	ctx := context.Background()
	require.NoError(t, st.AddDBSizes(ctx, []dbsize.Sample{
		{Name: "velho", Bytes: 1, At: now.Add(-48 * time.Hour)}, // fora das 24 h
		{Name: "loja", Bytes: 1e9, At: now.Add(-12 * time.Hour)},
		{Name: "postgres", Bytes: 8e6, At: now.Add(-10 * time.Minute)},
	}))
	require.NoError(t, st.AddDBSizes(ctx, []dbsize.Sample{
		{Name: "loja", Bytes: 1.5e9, At: now},
		{Name: "postgres", Bytes: 8e6, At: now},
	}))

	rec := do(t, srv, "GET", "/api/banco/tamanho", "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decode[map[string]any](t, rec)
	assert.Equal(t, true, got["configurado"])
	bancos := got["bancos"].([]any)
	require.Len(t, bancos, 2)
	loja := bancos[0].(map[string]any)
	assert.Equal(t, "loja", loja["banco"])
	assert.Equal(t, 1.5e9, loja["bytes"])
	assert.InDelta(t, 1.0, loja["crescimento_gb_dia"], 1e-9)
	assert.Len(t, loja["serie"], 2)
	pg := bancos[1].(map[string]any)
	assert.Nil(t, pg["crescimento_gb_dia"], "10 minutos não bastam para estimar o crescimento")
}
