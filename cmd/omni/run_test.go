package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MiguelDev-GH/omni/internal/config"
	"github.com/MiguelDev-GH/omni/internal/poller"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tok = "0123456789abcdef0123456789abcdef"

// startRun roda run numa goroutine e devolve a URL base e o canal com o retorno.
// Sem PollInterval na cfg, usa 50 ms.
func startRun(t *testing.T, cfg config.Config) (context.CancelFunc, string, <-chan error) {
	t.Helper()
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 50 * time.Millisecond
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- run(ctx, cfg, ln) }()
	return cancel, "http://" + ln.Addr().String(), done
}

// wsConfig é a configuração dos testes que usam poller e WebSocket.
func wsConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{
		DBPath:         filepath.Join(t.TempDir(), "omni.db"),
		Token:          tok,
		PollInterval:   50 * time.Millisecond,
		AllowedOrigins: []string{"http://127.0.0.1:8080"},
	}
}

// send faz uma requisição e devolve status e corpo; não usa require, para
// poder rodar dentro de require.Eventually.
func send(method, url, body string, auth bool) (int, string, error) {
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	if auth {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	return resp.StatusCode, strings.TrimSpace(string(b)), err
}

// do é send esperando o servidor subir.
func do(t *testing.T, method, url, body string, auth bool) (int, string) {
	t.Helper()
	var (
		code int
		resp string
	)
	require.Eventually(t, func() bool {
		var err error
		code, resp, err = send(method, url, body, auth)
		return err == nil
	}, 2*time.Second, 20*time.Millisecond)
	return code, resp
}

// createTarget cadastra um alvo apontando para app e devolve o id.
func createTarget(t *testing.T, base string, app *httptest.Server) int64 {
	t.Helper()
	port := app.Listener.Addr().(*net.TCPAddr).Port
	body := fmt.Sprintf(`{"nome":"api","host":"127.0.0.1","portas_app":[%d]}`, port)
	code, resp := do(t, "POST", base+"/api/alvos", body, true)
	require.Equal(t, http.StatusCreated, code, resp)
	var created struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(resp), &created))
	return created.ID
}

// dialWS pega um ticket e abre o WebSocket sem cabeçalho Origin.
func dialWS(t *testing.T, base string) *websocket.Conn {
	t.Helper()
	code, body := do(t, "POST", base+"/api/ws/ticket", "", true)
	require.Equal(t, http.StatusOK, code, body)
	var tk struct {
		Ticket string `json:"ticket"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &tk))
	u := "ws://" + strings.TrimPrefix(base, "http://") + "/ws?ticket=" + url.QueryEscape(tk.Ticket)
	conn, resp, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	_ = resp.Body.Close()
	t.Cleanup(func() { _ = conn.Close() })
	return conn
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
	cancel, base, done := startRun(t, config.Config{DBPath: filepath.Join(t.TempDir(), "omni.db"), Token: tok})
	code, _ := get(t, base+"/api/saude", false)
	assert.Equal(t, http.StatusOK, code)
	cancel()
	assert.NoError(t, waitDone(t, done))
}

func TestRunServesAuthenticatedTargets(t *testing.T) {
	cancel, base, done := startRun(t, config.Config{DBPath: filepath.Join(t.TempDir(), "omni.db"), Token: tok})
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
	_, base, done := startRun(t, config.Config{DBPath: path, Token: tok})
	err := waitDone(t, done)
	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
	_, dialErr := net.Dial("tcp", strings.TrimPrefix(base, "http://"))
	assert.Error(t, dialErr)
}

func TestRunWiresHubAndPoller(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer app.Close()
	cancel, base, done := startRun(t, wsConfig(t))
	id := createTarget(t, base, app)
	conn := dialWS(t, base)

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	for {
		var ev struct {
			Type string            `json:"tipo"`
			Data poller.StatusData `json:"dados"`
		}
		require.NoError(t, conn.ReadJSON(&ev))
		if ev.Type != poller.EventStatus {
			continue
		}
		require.Len(t, ev.Data.Results, 1)
		assert.Equal(t, id, ev.Data.Results[0].TargetID)
		assert.True(t, ev.Data.Results[0].Up)
		break
	}
	cancel()
	assert.NoError(t, waitDone(t, done))
}

func TestRunServesHistory(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer app.Close()
	cancel, base, done := startRun(t, wsConfig(t))
	id := createTarget(t, base, app)

	histURL := fmt.Sprintf("%s/api/alvos/%d/historico", base, id)
	require.Eventually(t, func() bool {
		code, body, err := send("GET", histURL, "", true)
		if err != nil || code != http.StatusOK {
			return false
		}
		var list []poller.Result
		return json.Unmarshal([]byte(body), &list) == nil && len(list) >= 1
	}, 2*time.Second, 50*time.Millisecond)
	cancel()
	assert.NoError(t, waitDone(t, done))
}

func TestRunShutsDownWithOpenWebSocket(t *testing.T) {
	cancel, base, done := startRun(t, wsConfig(t))
	conn := dialWS(t, base)
	cancel()
	assert.NoError(t, waitDone(t, done))

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	var err error
	for err == nil {
		_, _, err = conn.ReadMessage()
	}
	var netErr net.Error
	assert.False(t, errors.As(err, &netErr) && netErr.Timeout(), "esperava fechamento, veio timeout: %v", err)
}

func TestWebSocketRequiresTicket(t *testing.T) {
	cancel, base, done := startRun(t, wsConfig(t))
	code, _ := do(t, "GET", base+"/ws", "", false)
	assert.Equal(t, http.StatusUnauthorized, code)
	code, _ = do(t, "POST", base+"/api/ws/ticket", "", false)
	assert.Equal(t, http.StatusUnauthorized, code)
	cancel()
	assert.NoError(t, waitDone(t, done))
}
