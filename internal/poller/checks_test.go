package poller

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hostPort(t *testing.T, rawURL string) (string, int) {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	return u.Hostname(), port
}

func TestCheckHTTPUp(t *testing.T) {
	var ua atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua.Store(r.Header.Get("User-Agent"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	host, port := hostPort(t, srv.URL)

	res := CheckHTTP(context.Background(), NewHTTPClient(time.Second), 7, host, port)

	assert.True(t, res.Up)
	assert.Equal(t, 200, res.StatusCode)
	assert.Equal(t, "http", res.Kind)
	assert.Equal(t, int64(7), res.TargetID)
	assert.Equal(t, port, res.Port)
	assert.GreaterOrEqual(t, res.LatencyMS, 0.0)
	assert.Equal(t, "Omni-Poller/1", ua.Load())
	assert.Equal(t, time.UTC, res.At.Location())
}

func TestCheckHTTPServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	host, port := hostPort(t, srv.URL)

	res := CheckHTTP(context.Background(), NewHTTPClient(time.Second), 1, host, port)

	assert.True(t, res.Up)
	assert.Equal(t, 500, res.StatusCode)
}

func TestCheckHTTPDown(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	host, port := hostPort(t, srv.URL)
	srv.Close()

	res := CheckHTTP(context.Background(), NewHTTPClient(time.Second), 1, host, port)

	assert.False(t, res.Up)
	assert.NotEmpty(t, res.Err)
	assert.Equal(t, 0, res.StatusCode)
}

func TestCheckHTTPTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-time.After(time.Second):
		}
	}))
	defer srv.Close()
	defer close(release)
	host, port := hostPort(t, srv.URL)

	start := time.Now()
	res := CheckHTTP(context.Background(), NewHTTPClient(50*time.Millisecond), 1, host, port)

	assert.False(t, res.Up)
	assert.NotEmpty(t, res.Err)
	assert.Less(t, time.Since(start), 500*time.Millisecond)
}

func TestCheckHTTPDoesNotFollowRedirects(t *testing.T) {
	var hits atomic.Int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
	}))
	defer second.Close()
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, second.URL, http.StatusFound)
	}))
	defer first.Close()
	host, port := hostPort(t, first.URL)

	res := CheckHTTP(context.Background(), NewHTTPClient(time.Second), 1, host, port)

	assert.True(t, res.Up)
	assert.Equal(t, 302, res.StatusCode)
	assert.Equal(t, int32(0), hits.Load())
}

func TestCheckHTTPIPv6(t *testing.T) {
	ln, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skip("sem IPv6")
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	res := CheckHTTP(context.Background(), NewHTTPClient(time.Second), 1, "::1", port)

	assert.True(t, res.Up, res.Err)
	assert.Equal(t, 200, res.StatusCode)
}

func TestCheckTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port

	res := CheckTCP(context.Background(), 3, "127.0.0.1", port, time.Second)
	assert.True(t, res.Up, res.Err)
	assert.Equal(t, "tcp", res.Kind)
	assert.Equal(t, int64(3), res.TargetID)
	assert.Equal(t, port, res.Port)
	assert.Equal(t, time.UTC, res.At.Location())

	require.NoError(t, ln.Close())
	res = CheckTCP(context.Background(), 3, "127.0.0.1", port, time.Second)
	assert.False(t, res.Up)
	assert.NotEmpty(t, res.Err)
}
