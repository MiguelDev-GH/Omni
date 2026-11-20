package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MiguelDev-GH/omni/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tok = "0123456789abcdef0123456789abcdef"

// startRun roda run numa goroutine e devolve a URL base e o canal com o retorno.
func startRun(t *testing.T, dbPath string) (context.CancelFunc, string, <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- run(ctx, config.Config{DBPath: dbPath, Token: tok}, ln) }()
	return cancel, "http://" + ln.Addr().String(), done
}

func get(t *testing.T, url string, auth bool) (int, string) {
	t.Helper()
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)
	if auth {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	var resp *http.Response
	require.Eventually(t, func() bool {
		resp, err = http.DefaultClient.Do(req)
		return err == nil
	}, 2*time.Second, 20*time.Millisecond)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, strings.TrimSpace(string(body))
}

func waitDone(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("run não retornou em 2 s")
		return nil
	}
}

func TestRunServesAndShutsDown(t *testing.T) {
	cancel, base, done := startRun(t, filepath.Join(t.TempDir(), "omni.db"))
	code, _ := get(t, base+"/api/saude", false)
	assert.Equal(t, http.StatusOK, code)
	cancel()
	assert.NoError(t, waitDone(t, done))
}

func TestRunServesAuthenticatedTargets(t *testing.T) {
	cancel, base, done := startRun(t, filepath.Join(t.TempDir(), "omni.db"))
	code, body := get(t, base+"/api/alvos", true)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "[]", body)
	code, _ = get(t, base+"/api/alvos", false)
	assert.Equal(t, http.StatusUnauthorized, code)
	cancel()
	assert.NoError(t, waitDone(t, done))
}

func TestRunFailsOnBadDBPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nao-existe", "omni.db")
	_, base, done := startRun(t, path)
	err := waitDone(t, done)
	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
	_, dialErr := net.Dial("tcp", strings.TrimPrefix(base, "http://"))
	assert.Error(t, dialErr)
}
