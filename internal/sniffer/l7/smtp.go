package l7

import (
	"bytes"
	"strconv"
	"time"

	"github.com/gopacket/gopacket"
)

// Resultados de uma transação SMTP.
const (
	OutcomeDelivered = "entregue"
	OutcomeRejected  = "rejeitado"
	// OutcomeAborted é uma transação que a conexão abandonou no meio: o
	// cliente desistiu (RSET, QUIT) ou a conexão caiu ou ficou ociosa.
	OutcomeAborted = "abortado"
)

const (
	// maxSMTPLine limita o buffer de uma linha de comando ou resposta; uma
	// linha maior é descartada.
	maxSMTPLine = 4096
	// maxSMTPPending limita os comandos esperando resposta (pipelining).
	maxSMTPPending = 100
)

// SMTPExchange é uma transação SMTP (do MAIL FROM ao fim) vista no tráfego.
// Endereços, cabeçalhos e corpo nunca saem do decodificador.
type SMTPExchange struct {
	Outcome    string        `json:"resultado"` // OutcomeDelivered, OutcomeRejected ou OutcomeAborted
	Code       int           `json:"codigo"`    // resposta final (250, 550...); 0 quando abortada
	Recipients int           `json:"destinatarios"`
	Latency    time.Duration `json:"latencia_ns"` // do fim do DATA até a resposta; 0 sem DATA
	At         time.Time     `json:"hora"`        // hora do MAIL FROM
}

// SMTPAssembler remonta as conexões das portas SMTP e emite um SMTPExchange
// por transação. Conexões com STARTTLS são ignoradas depois do aceite, e o
// conteúdo do AUTH nunca é lido. Não é seguro para uso concorrente.
type SMTPAssembler struct {
	sa *streamAssembler
}

// NewSMTPAssembler chama emit a cada transação, na mesma goroutine de Add e Flush.
func NewSMTPAssembler(smtpPorts []int, emit func(SMTPExchange)) *SMTPAssembler {
	return &SMTPAssembler{sa: newStreamAssembler(smtpPorts, func(string) connParser {
		return &smtpConn{emit: emit}
	})}
}

// Add processa um pacote capturado.
func (a *SMTPAssembler) Add(p gopacket.Packet) { a.sa.add(p) }

// Flush libera segmentos presos por buracos e fecha fluxos ociosos, contando
// a partir de now; transações abertas em fluxos fechados contam como abortadas.
func (a *SMTPAssembler) Flush(now time.Time) { a.sa.flush(now) }

type smtpCmd struct {
	verb string // "MAIL", "RCPT", "DATA", "DOT" (fim do DATA), "STARTTLS" ou "OUTRO"
	at   time.Time
}

type smtpTx struct {
	at         time.Time
	recipients int // RCPT aceitos
	rcptTried  int
	lastCode   int // último código de recusa de um RCPT
	dataEnd    time.Time
}

type smtpConn struct {
	emit       func(SMTPExchange)
	client     []byte
	server     []byte
	pending    []smtpCmd
	tx         *smtpTx
	inData     bool
	tail       []byte // últimos bytes do corpo, para achar o terminador entre segmentos
	authActive bool   // dentro de um AUTH: as linhas do cliente não são lidas
	dead       bool   // STARTTLS aceito: o resto é criptografado
}

func (c *smtpConn) lost() {
	*c = smtpConn{emit: c.emit}
}

// closed é chamado quando o fluxo termina (FIN, RST ou ocioso).
func (c *smtpConn) closed() {
	c.finishTx()
}

func (c *smtpConn) data(fromClient bool, b []byte, at time.Time) {
	if c.dead {
		return
	}
	if fromClient {
		c.fromClient(b, at)
	} else {
		c.fromServer(b, at)
	}
}

func (c *smtpConn) fromClient(b []byte, at time.Time) {
	for len(b) > 0 && !c.dead {
		if c.inData {
			n, done := c.scanBody(b)
			b = b[n:]
			if !done {
				return
			}
			c.inData = false
			if c.tx != nil {
				c.tx.dataEnd = at
			}
			c.push(smtpCmd{verb: "DOT", at: at})
			continue
		}
		line, rest, ok := nextLine(&c.client, b)
		b = rest
		if !ok {
			return
		}
		c.command(line, at)
	}
}

// scanBody consome o corpo até o terminador "\r\n.\r\n" (o CRLF antes do
// ponto pode ser o fim da linha DATA). Devolve quantos bytes de b pertencem
// ao corpo e se o terminador foi encontrado. Só os últimos bytes ficam guardados.
func (c *smtpConn) scanBody(b []byte) (int, bool) {
	for i, ch := range b {
		c.tail = append(c.tail, ch)
		if len(c.tail) > 5 {
			c.tail = c.tail[len(c.tail)-5:]
		}
		if bytes.Equal(c.tail, []byte("\r\n.\r\n")) {
			c.tail = nil
			return i + 1, true
		}
	}
	return len(b), false
}

// nextLine junta b ao buffer e devolve a primeira linha completa, sem o CRLF,
// e o que sobrou de b depois dela. Sem linha completa, guarda b no buffer.
func nextLine(buf *[]byte, b []byte) (line, rest []byte, ok bool) {
	i := bytes.IndexByte(b, '\n')
	if i < 0 {
		*buf = append(*buf, b...)
		if len(*buf) > maxSMTPLine {
			*buf = nil
		}
		return nil, nil, false
	}
	line = append(*buf, b[:i]...)
	*buf = nil
	return bytes.TrimRight(line, "\r"), b[i+1:], true
}

func (c *smtpConn) command(line []byte, at time.Time) {
	if c.authActive {
		return
	}
	verb := line
	if i := bytes.IndexByte(line, ' '); i >= 0 {
		verb = line[:i]
	}
	if i := bytes.IndexByte(verb, ':'); i >= 0 {
		verb = verb[:i]
	}
	switch string(bytes.ToUpper(verb)) {
	case "MAIL":
		c.push(smtpCmd{verb: "MAIL", at: at})
	case "RCPT":
		c.push(smtpCmd{verb: "RCPT", at: at})
	case "DATA":
		c.push(smtpCmd{verb: "DATA", at: at})
	case "STARTTLS":
		c.push(smtpCmd{verb: "STARTTLS", at: at})
	case "AUTH":
		// As linhas seguintes do cliente são credenciais até a resposta final.
		c.authActive = true
	case "RSET", "QUIT":
		c.finishTx()
		c.push(smtpCmd{verb: "OUTRO", at: at})
	default:
		c.push(smtpCmd{verb: "OUTRO", at: at})
	}
}

func (c *smtpConn) push(cmd smtpCmd) {
	if len(c.pending) == maxSMTPPending {
		c.pending = c.pending[1:]
	}
	c.pending = append(c.pending, cmd)
}

func (c *smtpConn) fromServer(b []byte, at time.Time) {
	for len(b) > 0 && !c.dead {
		line, rest, ok := nextLine(&c.server, b)
		b = rest
		if !ok {
			return
		}
		if len(line) < 3 {
			continue
		}
		code, err := strconv.Atoi(string(line[:3]))
		if err != nil || (len(line) > 3 && line[3] == '-') {
			continue // linha inválida ou continuação de uma resposta de várias linhas
		}
		c.reply(code, at)
	}
}

func (c *smtpConn) reply(code int, at time.Time) {
	if c.authActive {
		if code != 334 {
			c.authActive = false
		}
		return
	}
	if len(c.pending) == 0 {
		return // saudação 220 ou resposta sem comando visto
	}
	cmd := c.pending[0]
	c.pending = c.pending[1:]
	ok := code >= 200 && code < 300
	switch cmd.verb {
	case "MAIL":
		if ok {
			c.tx = &smtpTx{at: cmd.at}
		} else {
			c.emit(SMTPExchange{Outcome: OutcomeRejected, Code: code, At: cmd.at.UTC()})
		}
	case "RCPT":
		if c.tx == nil {
			return
		}
		c.tx.rcptTried++
		if ok {
			c.tx.recipients++
		} else {
			c.tx.lastCode = code
		}
	case "DATA":
		if c.tx == nil {
			return
		}
		if code == 354 {
			c.inData = true
			c.tail = []byte("\r\n")
			return
		}
		c.done(OutcomeRejected, code, 0)
	case "DOT":
		if c.tx == nil {
			return
		}
		outcome := OutcomeRejected
		if ok {
			outcome = OutcomeDelivered
		}
		c.done(outcome, code, max(at.Sub(c.tx.dataEnd), 0))
	case "STARTTLS":
		if code == 220 {
			c.dead = true
			c.tx = nil
		}
	}
}

// done emite a transação aberta e a encerra.
func (c *smtpConn) done(outcome string, code int, latency time.Duration) {
	tx := c.tx
	c.tx = nil
	c.emit(SMTPExchange{Outcome: outcome, Code: code, Recipients: tx.recipients, Latency: latency, At: tx.at.UTC()})
}

// finishTx encerra uma transação que não chegou ao fim: rejeitada quando
// todos os destinatários foram recusados, abortada nos outros casos.
func (c *smtpConn) finishTx() {
	if c.tx == nil {
		return
	}
	if c.tx.rcptTried > 0 && c.tx.recipients == 0 {
		c.done(OutcomeRejected, c.tx.lastCode, 0)
		return
	}
	c.done(OutcomeAborted, 0, 0)
}
