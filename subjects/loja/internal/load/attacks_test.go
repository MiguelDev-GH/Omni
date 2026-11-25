package load

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttackMixes(t *testing.T) {
	var mu sync.Mutex
	seen := map[string][]*http.Request{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen[r.URL.Path] = append(seen[r.URL.Path], r.Clone(context.Background()))
		mu.Unlock()
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	for name, mix := range Attacks {
		st := Run(context.Background(), srv.Client(), mix,
			Options{BaseURL: srv.URL, Rate: 200, Duration: 200 * time.Millisecond, Workers: 4}, 1)
		assert.Positive(t, st.Total, name)
	}

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, seen["/entrar"])
	users, ips := map[string]bool{}, map[string]bool{}
	for _, r := range seen["/entrar"] {
		assert.Equal(t, http.MethodPost, r.Method)
		u, _, ok := r.BasicAuth()
		require.True(t, ok)
		users[u] = true
		ips[r.Header.Get("X-Forwarded-For")] = true
	}
	assert.True(t, users["ana"], "forca-bruta mira a conta ana")
	assert.Greater(t, len(users), 3, "spraying testa várias contas")
	assert.Equal(t, map[string]bool{bruteForceIP: true, sprayingIP: true}, ips)

	require.NotEmpty(t, seen["/produtos"])
	origins := map[string]bool{}
	for _, r := range seen["/produtos"] {
		xff := r.Header.Get("X-Forwarded-For")
		assert.True(t, strings.HasPrefix(xff, "203.0.113."), xff)
		origins[xff] = true
	}
	assert.Greater(t, len(origins), 5, "ddos vem de muitas origens")
}
