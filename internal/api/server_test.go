package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MiguelDev-GH/omni/internal/store"
	"github.com/MiguelDev-GH/omni/internal/target"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tok = "0123456789abcdef0123456789abcdef"

const validBody = `{"nome":"api","host":"127.0.0.1","portas_app":[3000]}`

func newTestServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omni.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	return NewServer(st, tok)
}

func do(t *testing.T, srv http.Handler, method, path, body string, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if auth {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &v), rec.Body.String())
	return v
}

func create(t *testing.T, srv http.Handler, body string) target.Target {
	t.Helper()
	rec := do(t, srv, "POST", "/api/alvos", body, true)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decode[target.Target](t, rec)
}

func listLen(t *testing.T, srv http.Handler) int {
	t.Helper()
	return len(decode[[]target.Target](t, do(t, srv, "GET", "/api/alvos", "", true)))
}

func TestHealthNeedsNoToken(t *testing.T) {
	rec := do(t, newTestServer(t), "GET", "/api/saude", "", false)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestAlvosRequireToken(t *testing.T) {
	srv := newTestServer(t)

	rec := do(t, srv, "GET", "/api/alvos", "", false)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"erro":"não autorizado"}`, rec.Body.String())
	assert.Equal(t, "Bearer", rec.Header().Get("WWW-Authenticate"))

	for _, h := range []string{"Bearer errado", "Basic " + tok} {
		req := httptest.NewRequest("GET", "/api/alvos", nil)
		req.Header.Set("Authorization", h)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, h)
	}
}

func TestUnknownAPIRouteNeedsToken(t *testing.T) {
	srv := newTestServer(t)
	assert.Equal(t, http.StatusUnauthorized, do(t, srv, "GET", "/api/nada", "", false).Code)
	assert.Equal(t, http.StatusNotFound, do(t, srv, "GET", "/api/nada", "", true).Code)
}

func TestListEmptyReturnsArray(t *testing.T) {
	rec := do(t, newTestServer(t), "GET", "/api/alvos", "", true)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "[]", strings.TrimSpace(rec.Body.String()))
}

func TestCreateTarget(t *testing.T) {
	srv := newTestServer(t)
	got := create(t, srv, `{"nome":" api ","host":"127.0.0.1","portas_app":[3000]}`)
	assert.Positive(t, got.ID)
	assert.Equal(t, "api", got.Name)
	assert.Equal(t, 1, listLen(t, srv))
}

func TestCreateTargetRejectsShellHost(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, "POST", "/api/alvos", `{"nome":"api","host":"127.0.0.1; rm -rf /","portas_app":[3000]}`, true)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "host", decode[map[string]string](t, rec)["campo"])
	assert.Equal(t, 0, listLen(t, srv))
}

func TestCreateTargetRejectsUnknownField(t *testing.T) {
	rec := do(t, newTestServer(t), "POST", "/api/alvos", `{"nome":"api","host":"127.0.0.1","portas_app":[3000],"extra":1}`, true)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"erro":"json inválido"}`, rec.Body.String())
}

func TestCreateTargetRejectsMalformedJSON(t *testing.T) {
	srv := newTestServer(t)
	for _, body := range []string{"", "{", validBody + validBody} {
		rec := do(t, srv, "POST", "/api/alvos", body, true)
		assert.Equal(t, http.StatusBadRequest, rec.Code, body)
		assert.JSONEq(t, `{"erro":"json inválido"}`, rec.Body.String(), body)
	}
	assert.Equal(t, 0, listLen(t, srv))
}

func TestCreateTargetRejectsHugeBody(t *testing.T) {
	body := `{"nome":"` + strings.Repeat("a", 2<<20) + `"}`
	rec := do(t, newTestServer(t), "POST", "/api/alvos", body, true)
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.JSONEq(t, `{"erro":"corpo muito grande"}`, rec.Body.String())
}

func TestGetTarget(t *testing.T) {
	srv := newTestServer(t)
	created := create(t, srv, validBody)
	rec := do(t, srv, "GET", fmt.Sprintf("/api/alvos/%d", created.ID), "", true)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, created, decode[target.Target](t, rec))
}

func TestTargetNotFound(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, "GET", "/api/alvos/999", "", true)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"erro":"alvo não encontrado"}`, rec.Body.String())
	for _, id := range []string{"abc", "0"} {
		rec := do(t, srv, "GET", "/api/alvos/"+id, "", true)
		assert.Equal(t, http.StatusBadRequest, rec.Code, id)
		assert.JSONEq(t, `{"erro":"id inválido"}`, rec.Body.String(), id)
	}
}

func TestUpdateTargetUsesPathID(t *testing.T) {
	srv := newTestServer(t)
	a := create(t, srv, validBody)
	b := create(t, srv, `{"nome":"outro","host":"127.0.0.1","portas_app":[4000]}`)

	body := fmt.Sprintf(`{"id":%d,"nome":"novo","host":"127.0.0.1","portas_app":[3000]}`, b.ID)
	rec := do(t, srv, "PUT", fmt.Sprintf("/api/alvos/%d", a.ID), body, true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decode[target.Target](t, rec)
	assert.Equal(t, a.ID, got.ID)
	assert.Equal(t, "novo", got.Name)

	rec = do(t, srv, "GET", fmt.Sprintf("/api/alvos/%d", b.ID), "", true)
	assert.Equal(t, "outro", decode[target.Target](t, rec).Name)
}

func TestUpdateTargetValidates(t *testing.T) {
	srv := newTestServer(t)
	a := create(t, srv, validBody)

	rec := do(t, srv, "PUT", fmt.Sprintf("/api/alvos/%d", a.ID), `{"nome":"api","host":"127.0.0.1","portas_app":[70000]}`, true)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "portas_app", decode[map[string]string](t, rec)["campo"])

	rec = do(t, srv, "PUT", "/api/alvos/999", validBody, true)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDeleteTarget(t *testing.T) {
	srv := newTestServer(t)
	a := create(t, srv, validBody)
	path := fmt.Sprintf("/api/alvos/%d", a.ID)

	rec := do(t, srv, "DELETE", path, "", true)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())

	assert.Equal(t, http.StatusNotFound, do(t, srv, "GET", path, "", true).Code)
	assert.Equal(t, http.StatusNotFound, do(t, srv, "DELETE", path, "", true).Code)
}

func TestHandleIsBehindAuth(t *testing.T) {
	srv := newTestServer(t)
	srv.Handle("GET /api/extra", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	assert.Equal(t, http.StatusUnauthorized, do(t, srv, "GET", "/api/extra", "", false).Code)
	assert.Equal(t, http.StatusOK, do(t, srv, "GET", "/api/extra", "", true).Code)
}

func TestHandlePublicSkipsAuth(t *testing.T) {
	srv := newTestServer(t)
	srv.HandlePublic("GET /ws", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	assert.Equal(t, http.StatusOK, do(t, srv, "GET", "/ws", "", false).Code)
}
