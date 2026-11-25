package l7

import (
	"bytes"
	"strconv"
	"strings"
	"time"

	"github.com/gopacket/gopacket"

	"github.com/MiguelDev-GH/omni/internal/clientinfo"
)

const (
	// maxHeaderBytes limita o cabeçalho de uma mensagem; acima disso o fluxo
	// é tratado como não HTTP e o buffer é descartado.
	maxHeaderBytes = 64 << 10
	// maxPending limita as requisições sem resposta guardadas por conexão.
	maxPending = 128
	// InternalUserAgent é o prefixo do User-Agent do tráfego gerado pelo
	// próprio Omni (Poller e Arsenal), que fica fora das métricas.
	InternalUserAgent = "Omni-"
)

// HTTPExchange é uma requisição pareada com a sua resposta. Não guarda
// cabeçalhos, query string nem corpo, que podem conter segredos.
type HTTPExchange struct {
	Method   string        `json:"metodo"`
	Path     string        `json:"caminho"`
	Route    string        `json:"rota"`
	Status   int           `json:"status"`
	Latency  time.Duration `json:"latencia_ns"`
	ClientIP string        `json:"ip_cliente"`
	// OriginIP é o IP de quem fez a requisição: o do X-Forwarded-For
	// quando a conexão vem de um proxy local, senão o próprio ClientIP.
	OriginIP string `json:"ip_origem"`
	// User é o usuário de uma autenticação Basic; a senha é descartada.
	User     string    `json:"usuario,omitempty"`
	Internal bool      `json:"interno"`
	At       time.Time `json:"hora"`
}

// HTTPAssembler remonta as conexões das portas de aplicação e emite um
// HTTPExchange por resposta. Não é seguro para uso concorrente.
type HTTPAssembler struct {
	sa *streamAssembler
}

// NewHTTPAssembler decodifica HTTP/1.1 nas portas appPorts e chama emit a
// cada requisição respondida, na mesma goroutine de Add.
func NewHTTPAssembler(appPorts []int, emit func(HTTPExchange)) *HTTPAssembler {
	return &HTTPAssembler{sa: newStreamAssembler(appPorts, func(ip string) connParser {
		return &httpConn{clientIP: ip, emit: emit}
	})}
}

// Add processa um pacote capturado.
func (h *HTTPAssembler) Add(p gopacket.Packet) { h.sa.add(p) }

// Flush libera segmentos presos por buracos e fecha fluxos ociosos, contando a partir de now.
func (h *HTTPAssembler) Flush(now time.Time) { h.sa.flush(now) }

type pendingRequest struct {
	method, path string
	originIP     string
	user         string
	internal     bool
	at           time.Time
}

// httpHalf é o estado de um sentido da conexão.
type httpHalf struct {
	buf        []byte
	at         time.Time // chegada do primeiro byte da mensagem atual
	bodyLeft   int64     // bytes de corpo (ou de chunk) ainda a pular
	chunk      chunkState
	untilClose bool // corpo sem tamanho: vai até o fim da conexão
}

type chunkState int

const (
	chunkNone chunkState = iota
	chunkSize
	chunkTrailer
)

func (h *httpHalf) reset() { *h = httpHalf{} }

type httpConn struct {
	clientIP string
	emit     func(HTTPExchange)
	req      httpHalf
	resp     httpHalf
	pending  []pendingRequest
	upgraded bool // depois de um 101, o fluxo deixa de ser HTTP
}

func (c *httpConn) lost() {
	c.req.reset()
	c.resp.reset()
	c.pending = nil
}

func (c *httpConn) data(fromClient bool, b []byte, at time.Time) {
	if c.upgraded {
		return
	}
	h := &c.resp
	if fromClient {
		h = &c.req
	}
	if len(h.buf) == 0 {
		h.at = at
	}
	h.buf = append(h.buf, b...)
	if fromClient {
		c.parse(h, at, true)
	} else {
		c.parse(h, at, false)
	}
}

// parse consome as mensagens completas de h, requisições se isReq.
func (c *httpConn) parse(h *httpHalf, now time.Time, isReq bool) {
	onHead := c.response
	if isReq {
		onHead = c.request
	}
	for !c.upgraded {
		if h.untilClose {
			h.buf = h.buf[:0]
			return
		}
		if h.bodyLeft > 0 {
			n := min(int64(len(h.buf)), h.bodyLeft)
			h.buf, h.bodyLeft = h.buf[n:], h.bodyLeft-n
			if h.bodyLeft > 0 {
				return
			}
			continue
		}
		if h.chunk != chunkNone {
			if !h.nextChunk() {
				return
			}
			continue
		}
		if len(h.buf) == 0 {
			return
		}
		end := bytes.Index(h.buf, []byte("\r\n\r\n"))
		if end < 0 {
			if len(h.buf) > maxHeaderBytes {
				c.lost()
			}
			return
		}
		msg := h.buf
		h.buf = msg[end+4:]
		if !onHead(h, msg[:end]) {
			// Lixo antes da mensagem (captura iniciada no meio de outra):
			// recomeça no próximo início de mensagem, se houver.
			i := resync(msg, isReq)
			if i < 0 {
				h.reset()
				return
			}
			h.buf = msg[i:]
			continue
		}
		h.at = now
	}
}

var methods = []string{"GET ", "POST ", "PUT ", "DELETE ", "PATCH ", "HEAD ", "OPTIONS "}

// resync devolve a posição, depois do primeiro byte, em que parece começar uma
// requisição (isReq) ou resposta HTTP em b, ou -1.
func resync(b []byte, isReq bool) int {
	best := -1
	markers := []string{"HTTP/1."}
	if isReq {
		markers = methods
	}
	for _, m := range markers {
		if i := bytes.Index(b[1:], []byte(m)); i >= 0 && (best < 0 || i+1 < best) {
			best = i + 1
		}
	}
	return best
}

// nextChunk avança uma linha do corpo chunked; devolve false se faltam bytes.
func (h *httpHalf) nextChunk() bool {
	i := bytes.Index(h.buf, []byte("\r\n"))
	if i < 0 {
		if len(h.buf) > maxHeaderBytes {
			h.reset()
		}
		return false
	}
	line := string(h.buf[:i])
	h.buf = h.buf[i+2:]
	if h.chunk == chunkTrailer {
		if line == "" {
			h.chunk = chunkNone
		}
		return true
	}
	if j := strings.IndexByte(line, ';'); j >= 0 {
		line = line[:j]
	}
	size, err := strconv.ParseInt(strings.TrimSpace(line), 16, 64)
	if err != nil || size < 0 {
		h.reset()
		return false
	}
	if size == 0 {
		h.chunk = chunkTrailer
	} else {
		h.bodyLeft = size + 2 // dados e o CRLF que fecha o chunk
	}
	return true
}

// headers lê só os cabeçalhos que interessam; os demais valores, como
// Authorization e Cookie, nunca saem desta função.
type headers struct {
	contentLength int64
	chunked       bool
	userAgent     string
	forwardedFor  string
	basicUser     string
}

func parseHeaders(lines []string) (headers, bool) {
	hd := headers{contentLength: -1}
	for _, l := range lines {
		name, value, ok := strings.Cut(l, ":")
		if !ok {
			return hd, false
		}
		value = strings.TrimSpace(value)
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "content-length":
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil || n < 0 {
				return hd, false
			}
			hd.contentLength = n
		case "transfer-encoding":
			hd.chunked = strings.Contains(strings.ToLower(value), "chunked")
		case "user-agent":
			hd.userAgent = value
		case "x-forwarded-for":
			// Repetido, o cabeçalho vale como uma lista só (RFC 9110, 5.3).
			if hd.forwardedFor != "" {
				value = hd.forwardedFor + ", " + value
			}
			hd.forwardedFor = value
		case "authorization":
			hd.basicUser = clientinfo.BasicUser(value)
		}
	}
	return hd, true
}

// setBody prepara h para pular o corpo descrito por hd.
func (h *httpHalf) setBody(hd headers, untilCloseIfUnknown bool) {
	switch {
	case hd.chunked:
		h.chunk = chunkSize
	case hd.contentLength > 0:
		h.bodyLeft = hd.contentLength
	case hd.contentLength < 0 && untilCloseIfUnknown:
		h.untilClose = true
	}
}

func (c *httpConn) request(h *httpHalf, head []byte) bool {
	lines := strings.Split(string(head), "\r\n")
	parts := strings.Split(lines[0], " ")
	if len(parts) != 3 || !strings.HasPrefix(parts[2], "HTTP/1.") || !isMethod(parts[0]) {
		return false
	}
	hd, ok := parseHeaders(lines[1:])
	if !ok {
		return false
	}
	path := requestPath(parts[1])
	if len(c.pending) == maxPending {
		c.pending = c.pending[1:]
	}
	c.pending = append(c.pending, pendingRequest{
		method:   parts[0],
		path:     path,
		originIP: clientinfo.OriginIP(c.clientIP, hd.forwardedFor),
		user:     hd.basicUser,
		internal: strings.HasPrefix(hd.userAgent, InternalUserAgent),
		at:       h.at,
	})
	h.setBody(hd, false)
	return true
}

func (c *httpConn) response(h *httpHalf, head []byte) bool {
	lines := strings.Split(string(head), "\r\n")
	version, rest, _ := strings.Cut(lines[0], " ")
	codeStr, _, _ := strings.Cut(rest, " ")
	status, err := strconv.Atoi(codeStr)
	if !strings.HasPrefix(version, "HTTP/1.") || err != nil || len(codeStr) != 3 || status < 100 {
		return false
	}
	hd, ok := parseHeaders(lines[1:])
	if !ok {
		return false
	}
	if status < 200 && status != 101 {
		return true // resposta informativa: a final ainda vem
	}
	if len(c.pending) == 0 {
		// Resposta de uma requisição anterior ao início da captura.
		h.setBody(hd, status != 204 && status != 304)
		return true
	}
	req := c.pending[0]
	c.pending = c.pending[1:]
	c.emit(HTTPExchange{
		Method:   req.method,
		Path:     req.path,
		Route:    NormalizeRoute(req.path),
		Status:   status,
		Latency:  max(h.at.Sub(req.at), 0),
		ClientIP: c.clientIP,
		OriginIP: req.originIP,
		User:     req.user,
		Internal: req.internal,
		At:       req.at.UTC(),
	})
	switch {
	case status == 101:
		c.upgraded = true
	case req.method == "HEAD" || status == 204 || status == 304:
	default:
		h.setBody(hd, true)
	}
	return true
}

// requestPath tira a query string e, na forma absoluta, o esquema e o host.
func requestPath(target string) string {
	if i := strings.Index(target, "://"); i >= 0 {
		rest := target[i+3:]
		if j := strings.IndexByte(rest, '/'); j >= 0 {
			target = rest[j:]
		} else {
			target = "/"
		}
	}
	if i := strings.IndexAny(target, "?#"); i >= 0 {
		target = target[:i]
	}
	if len(target) > maxRouteLen {
		target = target[:maxRouteLen]
	}
	return target
}

func isMethod(s string) bool {
	if s == "" || len(s) > 16 {
		return false
	}
	for _, c := range s {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}
