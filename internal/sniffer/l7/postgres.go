package l7

import (
	"bytes"
	"encoding/binary"
	"time"

	"github.com/gopacket/gopacket"
)

const (
	// maxPGMessage limita o tamanho de uma mensagem guardada em buffer; o
	// conteúdo de mensagens maiores é pulado sem ser lido.
	maxPGMessage = 1 << 20
	// tooLarge substitui a consulta de mensagens maiores que maxPGMessage.
	tooLarge = "(consulta grande demais)"

	pgSSLRequest    = 80877103
	pgGSSENCRequest = 80877104
	pgCancelRequest = 80877102
	pgProtocol3     = 196608
)

// DBQuery é uma consulta ao PostgreSQL pareada com a resposta. Query vem
// normalizada (NormalizeSQL), sem valores literais.
type DBQuery struct {
	Query   string        `json:"consulta"`
	Kind    string        `json:"tipo"`
	Latency time.Duration `json:"latencia_ns"`
	Error   bool          `json:"erro"`
	At      time.Time     `json:"hora"`
}

// PostgresAssembler remonta as conexões das portas do banco e emite um
// DBQuery por consulta respondida. Não é seguro para uso concorrente.
type PostgresAssembler struct {
	sa *streamAssembler
}

// NewPostgresAssembler decodifica o protocolo do PostgreSQL nas portas
// dbPorts e chama emit a cada consulta, na mesma goroutine de Add.
func NewPostgresAssembler(dbPorts []int, emit func(DBQuery)) *PostgresAssembler {
	return &PostgresAssembler{sa: newStreamAssembler(dbPorts, func(string) connParser {
		return &pgConn{emit: emit, stmts: map[string]string{}, portals: map[string]string{}}
	})}
}

// Add processa um pacote capturado.
func (a *PostgresAssembler) Add(p gopacket.Packet) { a.sa.add(p) }

// Flush libera segmentos presos por buracos e fecha fluxos ociosos, contando a partir de now.
func (a *PostgresAssembler) Flush(now time.Time) { a.sa.flush(now) }

type pgPending struct {
	query    string
	extended bool // Execute do protocolo estendido; o simples fecha no ReadyForQuery
	failed   bool
	at       time.Time
	done     time.Time // primeira resposta (CommandComplete ou ErrorResponse)
}

// pgHalf é o buffer de um sentido da conexão.
type pgHalf struct {
	buf  []byte
	skip int // bytes de uma mensagem grande ainda a pular
}

type pgConn struct {
	emit    func(DBQuery)
	client  pgHalf
	server  pgHalf
	started bool // o primeiro byte do cliente já foi visto
	// startup indica que a próxima mensagem do cliente não tem tipo
	// (StartupMessage depois de um SSLRequest recusado).
	startup bool
	// sslReply indica que o servidor deve responder com um só byte, S ou N.
	sslReply bool
	dead     bool // criptografado ou não parece PostgreSQL: ignorado
	stmts    map[string]string
	portals  map[string]string
	pending  []pgPending
}

func (c *pgConn) lost() {
	c.client, c.server = pgHalf{}, pgHalf{}
	c.pending = nil
}

func (c *pgConn) data(fromClient bool, b []byte, at time.Time) {
	if c.dead {
		return
	}
	if fromClient {
		c.fromClient(b, at)
	} else {
		c.fromServer(b, at)
	}
}

func (c *pgConn) fromClient(b []byte, at time.Time) {
	h := &c.client
	h.buf = append(h.buf, b...)
	if !c.started && len(h.buf) >= 8 {
		c.started = true
		code := binary.BigEndian.Uint32(h.buf[4:8])
		c.startup = code == pgProtocol3 || code == pgSSLRequest || code == pgGSSENCRequest || code == pgCancelRequest
	}
	for !c.dead && c.started {
		if c.startup {
			if len(h.buf) < 8 {
				return
			}
			n := int(binary.BigEndian.Uint32(h.buf[:4]))
			if n < 8 || n > maxPGMessage {
				c.dead = true
				return
			}
			if len(h.buf) < n {
				return
			}
			code := binary.BigEndian.Uint32(h.buf[4:8])
			h.buf = h.buf[n:]
			switch code {
			case pgSSLRequest, pgGSSENCRequest:
				c.sslReply = true // o StartupMessage vem depois do N
			case pgCancelRequest:
				c.dead = true
			default:
				c.startup = false // StartupMessage: usuário e banco não são lidos
			}
			continue
		}
		typ, body, ok := c.next(h, clientTypes)
		if !ok {
			return
		}
		c.clientMessage(typ, body, at)
	}
}

func (c *pgConn) fromServer(b []byte, at time.Time) {
	h := &c.server
	h.buf = append(h.buf, b...)
	for !c.dead && len(h.buf) > 0 {
		if c.sslReply {
			c.sslReply = false
			reply := h.buf[0]
			h.buf = h.buf[1:]
			if reply != 'N' {
				c.dead = true // conexão criptografada (S ou G): ilegível
				return
			}
			c.startup = true
			continue
		}
		typ, body, ok := c.next(h, serverTypes)
		if !ok {
			return
		}
		c.serverMessage(typ, body, at)
	}
}

var (
	clientTypes = []byte("QPBEDCSHXpFdcf")
	serverTypes = []byte("RSKZTDCEIn12t3sNAGHWdcfV")
)

// next tira a próxima mensagem tipada de h. Uma mensagem maior que
// maxPGMessage volta com body nil e o resto dela é pulado.
func (c *pgConn) next(h *pgHalf, types []byte) (byte, []byte, bool) {
	if h.skip > 0 {
		n := min(h.skip, len(h.buf))
		h.buf, h.skip = h.buf[n:], h.skip-n
		if h.skip > 0 {
			return 0, nil, false
		}
	}
	if len(h.buf) < 5 {
		return 0, nil, false
	}
	typ := h.buf[0]
	n := int(binary.BigEndian.Uint32(h.buf[1:5]))
	if bytes.IndexByte(types, typ) < 0 || n < 4 {
		c.dead = true // fora de sincronia ou não é PostgreSQL
		return 0, nil, false
	}
	if n > maxPGMessage {
		h.skip = n + 1
		return typ, nil, true
	}
	if len(h.buf) < n+1 {
		return 0, nil, false
	}
	body := h.buf[5 : n+1]
	h.buf = h.buf[n+1:]
	return typ, body, true
}

// cstring lê uma string terminada em zero de b e devolve o resto.
func cstring(b []byte) (string, []byte) {
	i := bytes.IndexByte(b, 0)
	if i < 0 {
		return string(b), nil
	}
	return string(b[:i]), b[i+1:]
}

func (c *pgConn) clientMessage(typ byte, body []byte, at time.Time) {
	switch typ {
	case 'Q':
		q := tooLarge
		if body != nil {
			q, _ = cstring(body)
		}
		c.push(pgPending{query: q, at: at})
	case 'P':
		if body == nil {
			return
		}
		name, rest := cstring(body)
		q, _ := cstring(rest)
		c.stmts[name] = q
	case 'B':
		if body == nil {
			return
		}
		portal, rest := cstring(body)
		stmt, _ := cstring(rest)
		c.portals[portal] = stmt
	case 'E':
		portal := ""
		if body != nil {
			portal, _ = cstring(body)
		}
		q, ok := c.stmts[c.portals[portal]]
		if !ok {
			q = "(consulta preparada antes da captura)"
		}
		c.push(pgPending{query: q, extended: true, at: at})
	case 'C':
		if len(body) > 0 && body[0] == 'S' {
			name, _ := cstring(body[1:])
			delete(c.stmts, name)
		}
	}
	// 'p' (senha e autenticação), 'd' (COPY) e os demais não são lidos.
}

func (c *pgConn) push(p pgPending) {
	if len(c.pending) == maxPending {
		c.pending = c.pending[1:]
	}
	c.pending = append(c.pending, p)
}

func (c *pgConn) serverMessage(typ byte, _ []byte, at time.Time) {
	switch typ {
	case 'C', 'I': // CommandComplete, EmptyQueryResponse
		if len(c.pending) == 0 {
			return
		}
		p := &c.pending[0]
		if p.done.IsZero() {
			p.done = at
		}
		if p.extended {
			c.finish()
		}
	case 'E': // ErrorResponse
		if len(c.pending) == 0 {
			return
		}
		p := &c.pending[0]
		p.failed = true
		if p.done.IsZero() {
			p.done = at
		}
		if p.extended {
			// Depois de um erro o servidor ignora tudo até o Sync, e os
			// Execute seguintes não terão resposta.
			c.finish()
			c.dropExtended()
		}
	case 'Z': // ReadyForQuery fecha a consulta simples
		if len(c.pending) > 0 && !c.pending[0].extended {
			p := &c.pending[0]
			if p.done.IsZero() {
				p.done = at
			}
			c.finish()
		}
		c.dropExtended()
	}
}

// finish emite a primeira consulta pendente.
func (c *pgConn) finish() {
	p := c.pending[0]
	c.pending = c.pending[1:]
	q := NormalizeSQL(p.query)
	c.emit(DBQuery{Query: q, Kind: QueryKind(q), Latency: max(p.done.Sub(p.at), 0), Error: p.failed, At: p.at.UTC()})
}

// dropExtended descarta os Execute iniciais ainda sem resposta.
func (c *pgConn) dropExtended() {
	for len(c.pending) > 0 && c.pending[0].extended {
		c.pending = c.pending[1:]
	}
}
