package register

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const token = "token-de-teste-com-tamanho-suficiente"

// fakeOmni imita GET e POST /api/alvos do Omni, inclusive a recusa de campos
// desconhecidos.
type fakeOmni struct {
	mu        sync.Mutex
	targets   []Target
	posts     int
	postError string
}

func (f *fakeOmni) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer "+token {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"erro":"token inválido"}`))
		return
	}
	switch r.Method + " " + r.URL.Path {
	case "GET /api/alvos":
		_ = json.NewEncoder(w).Encode(f.targets)
	case "POST /api/alvos":
		f.posts++
		if f.postError != "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"erro": f.postError})
			return
		}
		var t Target
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&t); err != nil || t.ID != 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		t.ID = int64(len(f.targets) + 1)
		f.targets = append(f.targets, t)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(t)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestRegisterCreatesMissing(t *testing.T) {
	f := &fakeOmni{}
	srv := httptest.NewServer(f)
	defer srv.Close()
	created, existing, err := Register(context.Background(), srv.Client(), srv.URL, token, Targets)
	require.NoError(t, err)
	assert.Equal(t, []string{"loja", "loja-tls"}, created)
	assert.Empty(t, existing)
	require.Len(t, f.targets, 2)
	assert.Equal(t, Target{ID: 1, Name: "loja", Host: "127.0.0.1", AppPorts: []int{3000}, DBPort: 5433}, f.targets[0])
	assert.Equal(t, Target{ID: 2, Name: "loja-tls", Host: "127.0.0.1", AppPorts: []int{8443}}, f.targets[1])
}

func TestRegisterIsIdempotent(t *testing.T) {
	f := &fakeOmni{}
	srv := httptest.NewServer(f)
	defer srv.Close()
	_, _, err := Register(context.Background(), srv.Client(), srv.URL, token, Targets)
	require.NoError(t, err)
	created, existing, err := Register(context.Background(), srv.Client(), srv.URL, token, Targets)
	require.NoError(t, err)
	assert.Empty(t, created)
	assert.Equal(t, []string{"loja", "loja-tls"}, existing)
	assert.Len(t, f.targets, 2)
}

func TestRegisterReportsBadToken(t *testing.T) {
	f := &fakeOmni{}
	srv := httptest.NewServer(f)
	defer srv.Close()
	_, _, err := Register(context.Background(), srv.Client(), srv.URL, "errado", Targets)
	assert.ErrorIs(t, err, ErrBadToken)
	assert.Equal(t, 0, f.posts)
}

func TestRegisterSurfacesValidationError(t *testing.T) {
	f := &fakeOmni{postError: "portas_app: porta repetida"}
	srv := httptest.NewServer(f)
	defer srv.Close()
	_, _, err := Register(context.Background(), srv.Client(), srv.URL, token, Targets)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Contains(t, err.Error(), "porta repetida")
}

func TestTargetJSONHasNoID(t *testing.T) {
	b, err := json.Marshal(Targets[1])
	require.NoError(t, err)
	assert.False(t, bytes.Contains(b, []byte(`"id"`)), string(b))
	assert.False(t, bytes.Contains(b, []byte(`"porta_banco"`)), string(b))
}
