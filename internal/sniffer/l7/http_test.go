package l7

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/sniffer/testutil"
)

var t0 = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

// feed entrega os quadros ao assembler, o i-ésimo em t0 + offsets[i].
type timed struct {
	frame []byte
	at    time.Duration
}

func runHTTP(t *testing.T, frames ...timed) []HTTPExchange {
	t.Helper()
	var got []HTTPExchange
	a := NewHTTPAssembler([]int{3000}, func(ex HTTPExchange) { got = append(got, ex) })
	for _, f := range frames {
		a.Add(testutil.Packet(f.frame, t0.Add(f.at)))
	}
	return got
}

func at(d time.Duration, frames ...[]byte) []timed {
	out := make([]timed, len(frames))
	for i, f := range frames {
		out[i] = timed{f, d}
	}
	return out
}

func conn() *testutil.Conn { return testutil.NewConn("10.0.0.1", 40000, "10.0.0.2", 3000) }

func TestHTTPSimpleExchange(t *testing.T) {
	c := conn()
	frames := at(0, c.Handshake()...)
	frames = append(frames, timed{c.FromClient([]byte("GET /users/42?x=1 HTTP/1.1\r\nHost: a\r\n\r\n")), 10 * time.Millisecond})
	frames = append(frames, timed{c.FromServer([]byte("HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nhello")), 35 * time.Millisecond})

	got := runHTTP(t, frames...)
	require.Len(t, got, 1)
	ex := got[0]
	assert.Equal(t, "GET", ex.Method)
	assert.Equal(t, "/users/42", ex.Path)
	assert.Equal(t, "/users/:id", ex.Route)
	assert.Equal(t, 200, ex.Status)
	assert.Equal(t, 25*time.Millisecond, ex.Latency)
	assert.Equal(t, "10.0.0.1", ex.ClientIP)
	assert.False(t, ex.Internal)
	assert.Equal(t, t0.Add(10*time.Millisecond), ex.At)
}

func TestHTTPPipelinedRequestsPairInOrder(t *testing.T) {
	c := conn()
	frames := at(0, c.Handshake()...)
	frames = append(frames,
		timed{c.FromClient([]byte("GET /a HTTP/1.1\r\n\r\nPOST /b HTTP/1.1\r\nContent-Length: 3\r\n\r\nabc")), time.Millisecond},
		timed{c.FromServer([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")), 5 * time.Millisecond},
		timed{c.FromServer([]byte("HTTP/1.1 201 Created\r\nTransfer-Encoding: chunked\r\n\r\n3\r\nabc\r\n0\r\n\r\n")), 9 * time.Millisecond},
	)
	got := runHTTP(t, frames...)
	require.Len(t, got, 2)
	assert.Equal(t, [2]any{"GET", 200}, [2]any{got[0].Method, got[0].Status})
	assert.Equal(t, [2]any{"POST", 201}, [2]any{got[1].Method, got[1].Status})
	assert.Equal(t, "/b", got[1].Path)
	assert.Equal(t, 8*time.Millisecond, got[1].Latency)
}

func TestHTTPResponseSplitInThreeSegments(t *testing.T) {
	c := conn()
	frames := at(0, c.Handshake()...)
	frames = append(frames,
		timed{c.FromClient([]byte("GET / HTTP/1.1\r\n\r\n")), 0},
		timed{c.FromServer([]byte("HTTP/1.1 404 Not")), 2 * time.Millisecond},
		timed{c.FromServer([]byte(" Found\r\nContent-Length: 10\r\n\r\n12345")), 3 * time.Millisecond},
		timed{c.FromServer([]byte("67890")), 4 * time.Millisecond},
		timed{c.FromClient([]byte("GET /x HTTP/1.1\r\n\r\n")), 5 * time.Millisecond},
		timed{c.FromServer([]byte("HTTP/1.1 500 Erro\r\nContent-Length: 0\r\n\r\n")), 6 * time.Millisecond},
	)
	got := runHTTP(t, frames...)
	require.Len(t, got, 2)
	assert.Equal(t, 404, got[0].Status)
	assert.Equal(t, 2*time.Millisecond, got[0].Latency)
	assert.Equal(t, 500, got[1].Status)
	assert.Equal(t, "/x", got[1].Path)
}

func TestHTTPOutOfOrderSegments(t *testing.T) {
	c := conn()
	frames := at(0, c.Handshake()...)
	req1 := c.FromClient([]byte("GET /primeira HTTP/1.1\r\n"))
	req2 := c.FromClient([]byte("Host: a\r\n\r\n"))
	resp := c.FromServer([]byte("HTTP/1.1 204 No Content\r\n\r\n"))
	frames = append(frames, timed{req2, time.Millisecond}, timed{req1, 2 * time.Millisecond}, timed{resp, 4 * time.Millisecond})

	got := runHTTP(t, frames...)
	require.Len(t, got, 1)
	assert.Equal(t, "/primeira", got[0].Path)
	assert.Equal(t, 204, got[0].Status)
}

func TestHTTPIgnoresTLS(t *testing.T) {
	c := testutil.NewConn("10.0.0.1", 40000, "10.0.0.2", 443)
	frames := at(0, c.Handshake()...)
	hello := []byte{0x16, 0x03, 0x01, 0x00, 0x2f, 0x01, 0x00, 0x00, 0x2b, 0x03, 0x03}
	frames = append(frames, timed{c.FromClient(hello), 0}, timed{c.FromServer([]byte{0x16, 0x03, 0x03, 0x00, 0x10}), 0})
	var got []HTTPExchange
	a := NewHTTPAssembler([]int{443}, func(ex HTTPExchange) { got = append(got, ex) })
	for _, f := range frames {
		a.Add(testutil.Packet(f.frame, t0))
	}
	assert.Empty(t, got)
}

func TestHTTPCaptureStartedMidConnection(t *testing.T) {
	c := conn()
	c.Handshake() // não capturado
	got := runHTTP(t,
		timed{c.FromServer([]byte("lo da resposta anterior")), 0},
		timed{c.FromClient([]byte("GET /ok HTTP/1.1\r\n\r\n")), time.Millisecond},
		timed{c.FromServer([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")), 2 * time.Millisecond},
	)
	require.Len(t, got, 1)
	assert.Equal(t, "/ok", got[0].Path)
}

func TestHTTPInternalUserAgent(t *testing.T) {
	c := conn()
	frames := at(0, c.Handshake()...)
	frames = append(frames,
		timed{c.FromClient([]byte("GET / HTTP/1.1\r\nUser-Agent: Omni-Poller/1\r\n\r\n")), 0},
		timed{c.FromServer([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")), time.Millisecond},
	)
	got := runHTTP(t, frames...)
	require.Len(t, got, 1)
	assert.True(t, got[0].Internal)
}

func TestNoSecretsLeak(t *testing.T) {
	c := conn()
	frames := at(0, c.Handshake()...)
	frames = append(frames,
		timed{c.FromClient([]byte("POST /login?senha=segredo123 HTTP/1.1\r\nAuthorization: Bearer segredo123\r\n" +
			"Cookie: sessao=abc\r\nContent-Length: 16\r\n\r\nsenha=segredo123")), 0},
		timed{c.FromServer([]byte("HTTP/1.1 200 OK\r\nSet-Cookie: sessao=abc\r\nContent-Length: 0\r\n\r\n")), time.Millisecond},
	)
	got := runHTTP(t, frames...)
	require.Len(t, got, 1)
	b, err := json.Marshal(got)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "segredo123")
	assert.NotContains(t, string(b), "abc")

	s := NewHTTPStats()
	s.Add(got[0])
	b, err = json.Marshal(s.Flush(t0.Add(time.Second)))
	require.NoError(t, err)
	assert.NotContains(t, string(b), "segredo123")
	assert.NotContains(t, string(b), "abc")
}

func TestHTTPUpgradeStopsParsing(t *testing.T) {
	c := conn()
	frames := at(0, c.Handshake()...)
	frames = append(frames,
		timed{c.FromClient([]byte("GET /ws HTTP/1.1\r\nUpgrade: websocket\r\n\r\n")), 0},
		timed{c.FromServer([]byte("HTTP/1.1 101 Switching Protocols\r\n\r\n")), time.Millisecond},
		timed{c.FromClient([]byte("GET /falso HTTP/1.1\r\n\r\n")), 2 * time.Millisecond},
		timed{c.FromServer([]byte("HTTP/1.1 200 OK\r\n\r\n")), 3 * time.Millisecond},
	)
	got := runHTTP(t, frames...)
	require.Len(t, got, 1)
	assert.Equal(t, 101, got[0].Status)
}

func TestHTTPOriginIPAndBasicUserBehindProxy(t *testing.T) {
	c := testutil.NewConn("127.0.0.1", 40000, "127.0.0.1", 3000)
	frames := at(0, c.Handshake()...)
	req := "POST /entrar HTTP/1.1\r\nHost: loja\r\n" +
		"X-Forwarded-For: 1.1.1.1\r\nX-Forwarded-For: 203.0.113.9\r\n" +
		"Authorization: Basic YW5hOnNlbmhhLWVycmFkYQ==\r\nContent-Length: 0\r\n\r\n" // ana:senha-errada
	frames = append(frames,
		timed{c.FromClient([]byte(req)), time.Millisecond},
		timed{c.FromServer([]byte("HTTP/1.1 401 Unauthorized\r\nContent-Length: 0\r\n\r\n")), 2 * time.Millisecond},
	)
	got := runHTTP(t, frames...)
	require.Len(t, got, 1)
	assert.Equal(t, "127.0.0.1", got[0].ClientIP)
	assert.Equal(t, "203.0.113.9", got[0].OriginIP)
	assert.Equal(t, "ana", got[0].User)
	assert.Equal(t, 401, got[0].Status)
}

func TestHTTPForwardedForIgnoredFromRemoteClient(t *testing.T) {
	c := conn() // 10.0.0.1, sem proxy no meio
	frames := at(0, c.Handshake()...)
	frames = append(frames,
		timed{c.FromClient([]byte("GET / HTTP/1.1\r\nX-Forwarded-For: 127.0.0.1\r\n\r\n")), time.Millisecond},
		timed{c.FromServer([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")), 2 * time.Millisecond},
	)
	got := runHTTP(t, frames...)
	require.Len(t, got, 1)
	assert.Equal(t, "10.0.0.1", got[0].OriginIP)
	assert.Empty(t, got[0].User)
}
