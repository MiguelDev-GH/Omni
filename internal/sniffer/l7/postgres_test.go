package l7

import (
	"encoding/binary"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/sniffer/testutil"
)

// msg monta uma mensagem tipada do protocolo do PostgreSQL.
func msg(typ byte, parts ...[]byte) []byte {
	var body []byte
	for _, p := range parts {
		body = append(body, p...)
	}
	out := []byte{typ, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(out[1:], uint32(len(body)+4))
	return append(out, body...)
}

func cstr(s string) []byte { return append([]byte(s), 0) }

// untyped monta StartupMessage ou SSLRequest.
func untyped(code uint32, body []byte) []byte {
	out := make([]byte, 8)
	binary.BigEndian.PutUint32(out, uint32(len(body)+8))
	binary.BigEndian.PutUint32(out[4:], code)
	return append(out, body...)
}

func startup() []byte {
	return untyped(pgProtocol3, append(append(cstr("user"), cstr("app")...), 0))
}

var ready = msg('Z', []byte("I"))

func pgConnFrames() *testutil.Conn { return testutil.NewConn("127.0.0.1", 50000, "127.0.0.1", 5432) }

func runPG(t *testing.T, frames ...timed) []DBQuery {
	t.Helper()
	var got []DBQuery
	a := NewPostgresAssembler([]int{5432}, func(q DBQuery) { got = append(got, q) })
	for _, f := range frames {
		a.Add(testutil.Packet(f.frame, t0.Add(f.at)))
	}
	return got
}

// login devolve o handshake TCP, a inicialização e a autenticação por senha.
func login(c *testutil.Conn) []timed {
	frames := at(0, c.Handshake()...)
	return append(frames,
		timed{c.FromClient(startup()), 0},
		timed{c.FromServer(msg('R', []byte{0, 0, 0, 3})), 0}, // pede senha em texto
		timed{c.FromClient(msg('p', cstr("senhaSecreta"))), 0},
		timed{c.FromServer(append(msg('R', []byte{0, 0, 0, 0}), ready...)), 0},
	)
}

func TestPGSimpleQuery(t *testing.T) {
	c := pgConnFrames()
	frames := append(login(c),
		timed{c.FromClient(msg('Q', cstr("SELECT * FROM u WHERE id = 42"))), 10 * time.Millisecond},
		timed{c.FromServer(append(append(msg('T', []byte{0, 0}), msg('C', cstr("SELECT 1"))...), ready...)), 14 * time.Millisecond},
	)
	got := runPG(t, frames...)
	require.Len(t, got, 1)
	assert.Equal(t, "SELECT * FROM u WHERE id = ?", got[0].Query)
	assert.Equal(t, "SELECT", got[0].Kind)
	assert.Equal(t, 4*time.Millisecond, got[0].Latency)
	assert.False(t, got[0].Error)

	b, err := json.Marshal(got)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "senhaSecreta")
}

func TestPGExtendedQuery(t *testing.T) {
	c := pgConnFrames()
	parse := msg('P', cstr(""), cstr("INSERT INTO t (a) VALUES ($1)"), []byte{0, 0})
	bind := msg('B', cstr(""), cstr(""), []byte{0, 0, 0, 0, 0, 0})
	exec := msg('E', cstr(""), []byte{0, 0, 0, 0})
	sync := msg('S')
	var batch []byte
	for _, m := range [][]byte{parse, bind, exec, sync} {
		batch = append(batch, m...)
	}
	frames := append(login(c),
		timed{c.FromClient(batch), 0},
		timed{c.FromServer(append(append(append(msg('1'), msg('2')...), msg('C', cstr("INSERT 0 1"))...), ready...)), 3 * time.Millisecond},
	)
	got := runPG(t, frames...)
	require.Len(t, got, 1)
	assert.Equal(t, "INSERT INTO t (a) VALUES ($1)", got[0].Query)
	assert.Equal(t, "INSERT", got[0].Kind)
	assert.Equal(t, 3*time.Millisecond, got[0].Latency)
}

func TestPGNamedStatementReused(t *testing.T) {
	c := pgConnFrames()
	parse := msg('P', cstr("s1"), cstr("SELECT nome FROM u WHERE id = $1"), []byte{0, 0})
	execOnce := append(append(msg('B', cstr("p"), cstr("s1"), []byte{0, 0, 0, 0, 0, 0}), msg('E', cstr("p"), []byte{0, 0, 0, 0})...), msg('S')...)
	resp := append(append(msg('2'), msg('C', cstr("SELECT 1"))...), ready...)
	frames := append(login(c),
		timed{c.FromClient(append(parse, execOnce...)), 0},
		timed{c.FromServer(append(msg('1'), resp...)), time.Millisecond},
		timed{c.FromClient(execOnce), 2 * time.Millisecond},
		timed{c.FromServer(resp), 3 * time.Millisecond},
	)
	got := runPG(t, frames...)
	require.Len(t, got, 2)
	assert.Equal(t, got[0].Query, got[1].Query)
}

func TestPGErrorResponse(t *testing.T) {
	c := pgConnFrames()
	frames := append(login(c),
		timed{c.FromClient(msg('Q', cstr("SELECT * FROM nao_existe"))), 0},
		timed{c.FromServer(append(msg('E', cstr("SERROR"), cstr("Mrelation does not exist"), []byte{0}), ready...)), time.Millisecond},
	)
	got := runPG(t, frames...)
	require.Len(t, got, 1)
	assert.True(t, got[0].Error)
}

func TestPGSSLAcceptedIsIgnored(t *testing.T) {
	c := pgConnFrames()
	frames := at(0, c.Handshake()...)
	frames = append(frames,
		timed{c.FromClient(untyped(pgSSLRequest, nil)), 0},
		timed{c.FromServer([]byte("S")), 0},
		timed{c.FromClient([]byte{0x16, 0x03, 0x01, 0x00, 0x05, 1, 2, 3, 4, 5}), 0},
		timed{c.FromServer([]byte{0x16, 0x03, 0x03, 0x00, 0x02, 1, 2}), 0},
	)
	assert.Empty(t, runPG(t, frames...))
}

func TestPGSSLRefusedContinuesInPlainText(t *testing.T) {
	c := pgConnFrames()
	frames := at(0, c.Handshake()...)
	frames = append(frames,
		timed{c.FromClient(untyped(pgSSLRequest, nil)), 0},
		timed{c.FromServer([]byte("N")), 0},
		timed{c.FromClient(startup()), 0},
		timed{c.FromServer(append(msg('R', []byte{0, 0, 0, 0}), ready...)), 0},
		timed{c.FromClient(msg('Q', cstr("BEGIN"))), 0},
		timed{c.FromServer(append(msg('C', cstr("BEGIN")), ready...)), 0},
	)
	got := runPG(t, frames...)
	require.Len(t, got, 1)
	assert.Equal(t, "BEGIN", got[0].Kind)
}

func TestPGCaptureStartedMidConnection(t *testing.T) {
	c := pgConnFrames()
	c.Handshake()
	got := runPG(t,
		timed{c.FromClient(msg('Q', cstr("UPDATE t SET a = 1"))), 0},
		timed{c.FromServer(append(msg('C', cstr("UPDATE 1")), ready...)), time.Millisecond},
	)
	require.Len(t, got, 1)
	assert.Equal(t, "UPDATE t SET a = ?", got[0].Query)
}

func TestPGGarbageIsIgnored(t *testing.T) {
	c := pgConnFrames()
	frames := at(0, c.Handshake()...)
	frames = append(frames, timed{c.FromClient([]byte("GET / HTTP/1.1\r\n\r\n")), 0}, timed{c.FromServer([]byte("HTTP/1.1 200 OK\r\n\r\n")), 0})
	assert.Empty(t, runPG(t, frames...))
}

func TestPostgresTxFailedOncePerAbortedBlock(t *testing.T) {
	c := pgConnFrames()
	inTx := msg('Z', []byte("T"))
	failedTx := msg('Z', []byte("E"))
	frames := append(login(c),
		timed{c.FromClient(msg('Q', cstr("BEGIN"))), time.Millisecond},
		timed{c.FromServer(append(msg('C', cstr("BEGIN")), inTx...)), 2 * time.Millisecond},
		timed{c.FromClient(msg('Q', cstr("INSERT INTO t VALUES (1)"))), 3 * time.Millisecond},
		timed{c.FromServer(append(msg('E', cstr("SERROR")), failedTx...)), 4 * time.Millisecond},
		timed{c.FromClient(msg('Q', cstr("SELECT 1"))), 5 * time.Millisecond},
		timed{c.FromServer(append(msg('E', cstr("SERROR")), failedTx...)), 6 * time.Millisecond},
		timed{c.FromClient(msg('Q', cstr("ROLLBACK"))), 7 * time.Millisecond},
		timed{c.FromServer(append(msg('C', cstr("ROLLBACK")), ready...)), 8 * time.Millisecond},
		// Uma segunda transação que falha conta de novo.
		timed{c.FromClient(msg('Q', cstr("BEGIN"))), 9 * time.Millisecond},
		timed{c.FromServer(append(msg('C', cstr("BEGIN")), inTx...)), 10 * time.Millisecond},
		timed{c.FromClient(msg('Q', cstr("UPDATE t SET a = 1"))), 11 * time.Millisecond},
		timed{c.FromServer(append(msg('E', cstr("SERROR")), failedTx...)), 12 * time.Millisecond},
	)
	var failures []time.Time
	a := NewPostgresAssembler([]int{5432}, func(DBQuery) {})
	a.OnTxFailed(func(at time.Time) { failures = append(failures, at) })
	for _, f := range frames {
		a.Add(testutil.Packet(f.frame, t0.Add(f.at)))
	}
	require.Len(t, failures, 2)
	assert.Equal(t, t0.Add(4*time.Millisecond), failures[0])
	assert.Equal(t, t0.Add(12*time.Millisecond), failures[1])
}
