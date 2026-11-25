package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func loginAs(t *testing.T, user, pass string, basic bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/entrar", nil)
	if basic {
		req.SetBasicAuth(user, pass)
	}
	rec := httptest.NewRecorder()
	New(&fakeStore{}).ServeHTTP(rec, req)
	return rec
}

func TestLoginAcceptsRightPassword(t *testing.T) {
	rec := loginAs(t, "ana", "ana123", true)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"usuario":"ana"}`, rec.Body.String())
}

func TestLoginRejects(t *testing.T) {
	cases := map[string]*httptest.ResponseRecorder{
		"senha errada":      loginAs(t, "ana", "errada", true),
		"conta inexistente": loginAs(t, "zeca", "conta-inexistente", true),
		"senha vazia":       loginAs(t, "ana", "", true),
		"sem autenticação":  loginAs(t, "", "", false),
	}
	for name, rec := range cases {
		assert.Equal(t, http.StatusUnauthorized, rec.Code, name)
		assert.Equal(t, `Basic realm="loja"`, rec.Header().Get("WWW-Authenticate"), name)
		assert.Equal(t, "usuário ou senha inválidos", errorOf(t, rec), name)
	}
}

func TestLoginOnlyPost(t *testing.T) {
	rec := do(t, New(&fakeStore{}), http.MethodGet, "/entrar", "")
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
