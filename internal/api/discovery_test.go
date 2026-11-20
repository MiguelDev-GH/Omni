package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/MiguelDev-GH/omni/internal/discovery"
	"github.com/stretchr/testify/assert"
)

func newDiscoveryServer(t *testing.T, list func(context.Context) ([]discovery.Listener, error)) *Server {
	t.Helper()
	srv := newTestServer(t)
	srv.Handle("GET /api/descoberta", DiscoveryHandler(list, 8080))
	return srv
}

func TestDiscoveryReturnsSuggestions(t *testing.T) {
	srv := newDiscoveryServer(t, func(context.Context) ([]discovery.Listener, error) {
		return []discovery.Listener{{Address: "127.0.0.1", Port: 3000, PID: 10, Process: "node"}}, nil
	})
	rec := do(t, srv, "GET", "/api/descoberta", "", true)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[{"processo":"node","pid":10,"host":"127.0.0.1","tipo":"app","portas":[3000]}]`, rec.Body.String())
}

func TestDiscoveryEmptyIsArray(t *testing.T) {
	srv := newDiscoveryServer(t, func(context.Context) ([]discovery.Listener, error) { return nil, nil })
	rec := do(t, srv, "GET", "/api/descoberta", "", true)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[]`, rec.Body.String())
}

func TestDiscoveryError(t *testing.T) {
	srv := newDiscoveryServer(t, func(context.Context) ([]discovery.Listener, error) {
		return nil, errors.New("permissão negada")
	})
	rec := do(t, srv, "GET", "/api/descoberta", "", true)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"erro":"não foi possível listar os serviços"}`, rec.Body.String())
}

func TestDiscoveryRequiresToken(t *testing.T) {
	srv := newDiscoveryServer(t, func(context.Context) ([]discovery.Listener, error) { return nil, nil })
	assert.Equal(t, http.StatusUnauthorized, do(t, srv, "GET", "/api/descoberta", "", false).Code)
}
