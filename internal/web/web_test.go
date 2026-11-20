package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func builtFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":      {Data: []byte("<!doctype html>app")},
		"assets/app-1.js": {Data: []byte("js")},
	}
}

func serve(t *testing.T, fsys fs.FS, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	Handler(fsys).ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestServesIndex(t *testing.T) {
	rec := serve(t, builtFS(), "GET", "/")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rec.Body.String(), "app")
}

func TestSPAFallback(t *testing.T) {
	root := serve(t, builtFS(), "GET", "/").Body.String()
	for _, p := range []string{"/alvos", "/monitoramento/geral"} {
		rec := serve(t, builtFS(), "GET", p)
		assert.Equal(t, http.StatusOK, rec.Code, p)
		assert.Equal(t, root, rec.Body.String(), p)
	}
}

func TestServesAsset(t *testing.T) {
	rec := serve(t, builtFS(), "GET", "/assets/app-1.js")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "js", rec.Body.String())
	assert.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
}

func TestMissingAssetIs404(t *testing.T) {
	for _, p := range []string{"/assets/nao-existe.js", "/favicon.ico"} {
		assert.Equal(t, http.StatusNotFound, serve(t, builtFS(), "GET", p).Code, p)
	}
}

func TestAPIPathsNeverFallBack(t *testing.T) {
	for _, p := range []string{"/api", "/api/x", "/ws"} {
		assert.Equal(t, http.StatusNotFound, serve(t, builtFS(), "GET", p).Code, p)
	}
}

func TestRejectsPost(t *testing.T) {
	rec := serve(t, builtFS(), "POST", "/")
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, "GET, HEAD", rec.Header().Get("Allow"))
}

func TestNoTraversalOrListing(t *testing.T) {
	assert.Equal(t, http.StatusNotFound, serve(t, builtFS(), "GET", "/../go.mod").Code)
	rec := serve(t, builtFS(), "GET", "/assets/")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, serve(t, builtFS(), "GET", "/").Body.String(), rec.Body.String())
}

func TestSecurityHeaders(t *testing.T) {
	h := serve(t, builtFS(), "GET", "/").Header()
	assert.Equal(t, "default-src 'self'; connect-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'", h.Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", h.Get("X-Content-Type-Options"))
	assert.Equal(t, "no-referrer", h.Get("Referrer-Policy"))
	assert.Equal(t, "no-cache", h.Get("Cache-Control"))
}

func TestPlaceholderWithoutBuild(t *testing.T) {
	fsys := fstest.MapFS{"placeholder.html": {Data: []byte("make build")}}
	for _, p := range []string{"/", "/alvos"} {
		rec := serve(t, fsys, "GET", p)
		assert.Equal(t, http.StatusOK, rec.Code, p)
		assert.Contains(t, rec.Body.String(), "make build", p)
	}
}

func TestEmbeddedAssetsHavePlaceholder(t *testing.T) {
	_, err := fs.Stat(Assets(), "placeholder.html")
	require.NoError(t, err)
}
