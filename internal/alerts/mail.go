package alerts

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	mailQueueSize   = 100
	mailDialTimeout = 10 * time.Second
	mailSendTimeout = 30 * time.Second
	// mailPerMinute limita os envios. Um ataque que derruba o serviço não
	// pode virar, de quebra, uma enxurrada de e-mails (e um bloqueio da
	// conta no provedor).
	mailPerMinute = 20
)

// ErrNoTLS é devolvido quando o servidor não oferece STARTTLS e o envio
// exigiria mandar a senha em texto puro pela rede.
var ErrNoTLS = errors.New("servidor SMTP sem STARTTLS: a senha seria enviada em texto puro")

// MailConfig é a configuração do envio de alertas por e-mail.
type MailConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	To       []string
	// PanelURL, se preenchido, vai no corpo do e-mail como link do painel.
	PanelURL string
}

// Enabled diz se o envio está configurado.
func (c MailConfig) Enabled() bool { return c.Host != "" && len(c.To) > 0 }

// Mailer envia alertas por e-mail numa goroutine própria, para que um
// servidor SMTP lento nunca atrase o restante do Omni.
type Mailer struct {
	cfg   MailConfig
	addr  string // endereço discado; os testes o trocam por um servidor falso
	queue chan Alert
	send  func(ctx context.Context, msg []byte) error
	now   func() time.Time

	mu          sync.Mutex
	windowStart time.Time
	sentInWin   int
	suppressed  int
}

// NewMailer cria um Mailer. Chame Run para começar a enviar.
func NewMailer(cfg MailConfig) *Mailer {
	m := &Mailer{
		cfg:   cfg,
		addr:  net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		queue: make(chan Alert, mailQueueSize),
		now:   time.Now,
	}
	m.send = m.smtpSend
	return m
}

// Notify enfileira o alerta sem bloquear. Com a fila cheia, o alerta é
// descartado e registrado no log.
func (m *Mailer) Notify(a Alert) {
	select {
	case m.queue <- a:
	default:
		slog.Warn("alertas: fila de e-mail cheia, alerta descartado", "alerta", a.Title())
	}
}

// Run envia os alertas da fila até ctx ser cancelado.
func (m *Mailer) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case a := <-m.queue:
			m.deliver(ctx, a)
		}
	}
}

func (m *Mailer) deliver(ctx context.Context, a Alert) {
	suppressed, ok := m.allow()
	if !ok {
		slog.Warn("alertas: limite de e-mails por minuto atingido", "alerta", a.Title())
		return
	}
	msg, err := m.buildMessage(a, suppressed)
	if err != nil {
		slog.Error("alertas: montar e-mail", "erro", err)
		return
	}
	sendCtx, cancel := context.WithTimeout(ctx, mailSendTimeout)
	defer cancel()
	if err := m.send(sendCtx, msg); err != nil {
		slog.Error("alertas: enviar e-mail", "alerta", a.Title(), "erro", err)
		return
	}
	slog.Info("alertas: e-mail enviado", "alerta", a.Title(), "destinatarios", len(m.cfg.To))
}

// allow aplica o limite por minuto. Devolve quantos alertas foram
// suprimidos desde o último envio, para avisar no próximo e-mail.
func (m *Mailer) allow() (int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	if now.Sub(m.windowStart) >= time.Minute {
		m.windowStart = now
		m.sentInWin = 0
	}
	if m.sentInWin >= mailPerMinute {
		m.suppressed++
		return 0, false
	}
	m.sentInWin++
	s := m.suppressed
	m.suppressed = 0
	return s, true
}

var levelLabel = map[Level]string{Green: "VERDE", Yellow: "AMARELO", Red: "VERMELHO"}

// headerSafe remove quebras de linha. Sem isso, um nome de alvo como
// "x\r\nBcc: vitima@exemplo.com" injetaria cabeçalhos no e-mail.
func headerSafe(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' {
			return ' '
		}
		return r
	}, s)
}

func formatValue(v float64, unit string) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if strings.Contains(s, ".") {
		s = strconv.FormatFloat(v, 'f', 1, 64)
	}
	switch unit {
	case "":
		return s
	case "%":
		return s + "%"
	default:
		return s + " " + unit
	}
}

// buildMessage monta o e-mail (cabeçalhos e corpo em quoted-printable).
func (m *Mailer) buildMessage(a Alert, suppressed int) ([]byte, error) {
	var body bytes.Buffer
	fmt.Fprintf(&body, "Nível: %s (antes: %s)\n", levelLabel[a.Level], levelLabel[a.Previous])
	fmt.Fprintf(&body, "Serviço: %s\n", a.Service)
	if a.Subject != "" {
		fmt.Fprintf(&body, "Recurso: %s\n", headerSafe(a.Subject))
	}
	fmt.Fprintf(&body, "Métrica: %s\n", a.Name)
	if a.Rule == RuleAvailability {
		fmt.Fprintf(&body, "Estado: %s\n", availabilityText(a.Value))
	} else {
		fmt.Fprintf(&body, "Valor: %s (limite: %s)\n", formatValue(a.Value, a.Unit), formatValue(a.Threshold, a.Unit))
	}
	if a.Detail != "" {
		fmt.Fprintf(&body, "Detalhe: %s\n", headerSafe(a.Detail))
	}
	fmt.Fprintf(&body, "Hora: %s\n", a.At.Local().Format("02/01/2006 15:04:05 MST"))
	if suppressed > 0 {
		fmt.Fprintf(&body, "\n%d alerta(s) anteriores não foram enviados por causa do limite de %d e-mails por minuto.\n",
			suppressed, mailPerMinute)
	}
	if m.cfg.PanelURL != "" {
		fmt.Fprintf(&body, "\nPainel: %s\n", m.cfg.PanelURL)
	}
	body.WriteString("\n-- \nOmni, monitoramento e segurança\n")

	id, err := messageID(m.cfg.From)
	if err != nil {
		return nil, err
	}
	subject := fmt.Sprintf("[Omni][%s] %s", levelLabel[a.Level], a.Title())

	var msg bytes.Buffer
	header := func(k, v string) { fmt.Fprintf(&msg, "%s: %s\r\n", k, headerSafe(v)) }
	header("From", m.cfg.From)
	header("To", strings.Join(m.cfg.To, ", "))
	header("Subject", mime.QEncoding.Encode("utf-8", headerSafe(subject)))
	header("Date", m.now().Format(time.RFC1123Z))
	header("Message-ID", id)
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=utf-8")
	header("Content-Transfer-Encoding", "quoted-printable")
	header("X-Omni-Nivel", string(a.Level))
	msg.WriteString("\r\n")
	qp := quotedprintable.NewWriter(&msg)
	text := strings.ReplaceAll(body.String(), "\n", "\r\n")
	if _, err := qp.Write([]byte(text)); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}
	return msg.Bytes(), nil
}

func availabilityText(v float64) string {
	switch {
	case v >= AvailabilityDown:
		return "fora do ar"
	case v >= AvailabilityDegraded:
		return "no ar, mas respondendo com erro 5xx"
	default:
		return "no ar"
	}
}

func messageID(from string) (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	domain := "omni.local"
	if at := strings.LastIndex(from, "@"); at >= 0 {
		domain = strings.Trim(from[at+1:], "> ")
	}
	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(b), domain), nil
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// smtpSend entrega msg ao servidor configurado. Na porta 465 usa TLS desde o
// início; nas outras, sobe para TLS com STARTTLS quando o servidor oferece.
// Sem TLS, só envia se não houver senha ou se o servidor for local.
func (m *Mailer) smtpSend(ctx context.Context, msg []byte) error {
	addr := m.addr
	tlsCfg := &tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}
	dialer := &net.Dialer{Timeout: mailDialTimeout}

	var conn net.Conn
	var err error
	if m.cfg.Port == 465 {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsCfg}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(mailSendTimeout)
	}
	_ = conn.SetDeadline(deadline)

	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer func() { _ = c.Close() }()

	if m.cfg.Port != 465 {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(tlsCfg); err != nil {
				return fmt.Errorf("starttls: %w", err)
			}
		} else if m.cfg.User != "" && !isLoopbackHost(m.cfg.Host) {
			return ErrNoTLS
		}
	}
	if m.cfg.User != "" {
		if err := c.Auth(smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)); err != nil {
			return fmt.Errorf("autenticação: %w", err)
		}
	}
	if err := c.Mail(bareAddress(m.cfg.From)); err != nil {
		return err
	}
	for _, to := range m.cfg.To {
		if err := c.Rcpt(bareAddress(to)); err != nil {
			return fmt.Errorf("destinatário %s: %w", to, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// bareAddress tira o nome de exibição: "Omni <a@b.c>" vira "a@b.c".
func bareAddress(s string) string {
	if i := strings.LastIndex(s, "<"); i >= 0 {
		if j := strings.LastIndex(s, ">"); j > i {
			return s[i+1 : j]
		}
	}
	return strings.TrimSpace(s)
}
