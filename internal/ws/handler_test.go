package ws

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const allowedOrigin = "http://127.0.0.1:8080"

func newWSServer(t *testing.T, h *Hub, tk *Tickets) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(Handler(h, tk, []string{allowedOrigin}))
	t.Cleanup(srv.Close)
	t.Cleanup(h.Close)
	return srv
}

func wsURL(srv *httptest.Server, ticket string) string {
	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	if ticket != "" {
		u += "?ticket=" + ticket
	}
	return u
}

func issue(t *testing.T, tk *Tickets) string {
	t.Helper()
	ticket, err := tk.Issue()
	require.NoError(t, err)
	return ticket
}

func dial(t *testing.T, url, origin string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	hdr := http.Header{}
	if origin != "" {
		hdr.Set("Origin", origin)
	}
	conn, resp, err := websocket.DefaultDialer.Dial(url, hdr)
	if resp != nil {
		t.Cleanup(func() { _ = resp.Body.Close() })
	}
	if conn != nil {
		t.Cleanup(func() { _ = conn.Close() })
	}
	return conn, resp, err
}

func connect(t *testing.T, srv *httptest.Server, tk *Tickets) *websocket.Conn {
	t.Helper()
	conn, _, err := dial(t, wsURL(srv, issue(t, tk)), allowedOrigin)
	require.NoError(t, err)
	return conn
}

// waitClients espera o hub registrar n clientes.
func waitClients(t *testing.T, h *Hub, n int) {
	t.Helper()
	require.Eventually(t, func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		return len(h.clients) == n
	}, time.Second, 5*time.Millisecond)
}

func TestRejectsForeignOrigin(t *testing.T) {
	tk := NewTickets(30 * time.Second)
	srv := newWSServer(t, NewHub(4), tk)
	ticket := issue(t, tk)

	_, resp, err := dial(t, wsURL(srv, ticket), "https://evil.example")
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "origem não permitida", body["erro"])

	assert.True(t, tk.Consume(ticket))
}

func TestRejectsMissingOrExpiredTicket(t *testing.T) {
	tk := NewTickets(30 * time.Second)
	srv := newWSServer(t, NewHub(4), tk)

	for _, ticket := range []string{"", "inexistente"} {
		_, resp, err := dial(t, wsURL(srv, ticket), allowedOrigin)
		require.Error(t, err, ticket)
		require.NotNil(t, resp, ticket)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, ticket)
		var body map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, "ticket inválido", body["erro"])
	}

	short := NewTickets(10 * time.Millisecond)
	srv = newWSServer(t, NewHub(4), short)
	ticket := issue(t, short)
	time.Sleep(50 * time.Millisecond) // deixa o ticket vencer
	_, resp, err := dial(t, wsURL(srv, ticket), allowedOrigin)
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestRejectsWhenHubClosed(t *testing.T) {
	tk := NewTickets(30 * time.Second)
	h := NewHub(4)
	srv := newWSServer(t, h, tk)
	h.Close()

	_, resp, err := dial(t, wsURL(srv, issue(t, tk)), "")
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestRejectsNonGet(t *testing.T) {
	tk := NewTickets(30 * time.Second)
	srv := newWSServer(t, NewHub(4), tk)
	resp, err := http.Post(srv.URL+"/ws?ticket="+issue(t, tk), "text/plain", nil)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestDeliversEvents(t *testing.T) {
	tk := NewTickets(30 * time.Second)
	h := NewHub(4)
	srv := newWSServer(t, h, tk)
	conn := connect(t, srv, tk)
	waitClients(t, h, 1)

	h.Publish(Event{Type: "TESTE", Data: map[string]int{"n": 1}})

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, `{"tipo":"TESTE","dados":{"n":1}}`, string(msg))
}

func TestSlowClientDoesNotBlockOthers(t *testing.T) {
	tk := NewTickets(30 * time.Second)
	h := NewHub(4)
	srv := newWSServer(t, h, tk)
	_ = connect(t, srv, tk) // cliente A: nunca lê
	b := connect(t, srv, tk)
	waitClients(t, h, 2)

	for i := 1; i <= 100; i++ {
		h.Publish(Event{Type: "TESTE", Data: i})
	}

	require.NoError(t, b.SetReadDeadline(time.Now().Add(time.Second)))
	for {
		_, msg, err := b.ReadMessage()
		require.NoError(t, err)
		var ev struct {
			Data int `json:"dados"`
		}
		require.NoError(t, json.Unmarshal(msg, &ev))
		if ev.Data == 100 {
			return
		}
	}
}

func TestHubCloseDisconnectsClients(t *testing.T) {
	tk := NewTickets(30 * time.Second)
	h := NewHub(4)
	srv := newWSServer(t, h, tk)
	conn := connect(t, srv, tk)
	waitClients(t, h, 1)

	h.Close()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
	_, _, err := conn.ReadMessage()
	require.Error(t, err)
	var ne net.Error
	if errors.As(err, &ne) {
		assert.False(t, ne.Timeout(), "o cliente deveria ser desconectado, não expirar")
	}
}

func TestTicketHandler(t *testing.T) {
	tk := NewTickets(30 * time.Second)
	rec := httptest.NewRecorder()
	TicketHandler(tk).ServeHTTP(rec, httptest.NewRequest("POST", "/api/ws/ticket", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Len(t, body["ticket"], 64)
	assert.True(t, tk.Consume(body["ticket"]))
}
