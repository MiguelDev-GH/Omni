package alerts

import (
	"bufio"
	"context"
	"errors"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTP é um servidor SMTP mínimo, sem TLS, que guarda as mensagens.
type fakeSMTP struct {
	ln       net.Listener
	starttls bool

	mu    sync.Mutex
	from  string
	rcpts []string
	data  string
	done  chan struct{}
}

func newFakeSMTP(t *testing.T, starttls bool) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSMTP{ln: ln, starttls: starttls, done: make(chan struct{}, 1)}
	t.Cleanup(func() { _ = ln.Close() })
	go f.serve()
	return f
}

func (f *fakeSMTP) port() int { return f.ln.Addr().(*net.TCPAddr).Port }

func (f *fakeSMTP) serve() {
	conn, err := f.ln.Accept()
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()
	r := bufio.NewReader(conn)
	reply := func(s string) { _, _ = io.WriteString(conn, s+"\r\n") }
	reply("220 fake ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.TrimRight(line, "\r\n")
		upper := strings.ToUpper(cmd)
		switch {
		case strings.HasPrefix(upper, "EHLO"):
			if f.starttls {
				reply("250-fake")
				reply("250 STARTTLS")
			} else {
				reply("250 fake")
			}
		case strings.HasPrefix(upper, "MAIL FROM:"):
			f.mu.Lock()
			f.from = strings.Trim(cmd[len("MAIL FROM:"):], "<> ")
			f.mu.Unlock()
			reply("250 ok")
		case strings.HasPrefix(upper, "RCPT TO:"):
			f.mu.Lock()
			f.rcpts = append(f.rcpts, strings.Trim(cmd[len("RCPT TO:"):], "<> "))
			f.mu.Unlock()
			reply("250 ok")
		case upper == "DATA":
			reply("354 manda")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				b.WriteString(l)
			}
			f.mu.Lock()
			f.data = b.String()
			f.mu.Unlock()
			reply("250 recebido")
			f.done <- struct{}{}
		case upper == "QUIT":
			reply("221 tchau")
			return
		default:
			reply("502 não implementado")
		}
	}
}

var sampleAlert = Alert{
	ID: 1, Rule: RuleAvailability, Service: ServiceWeb, Name: "Disponibilidade", Subject: "loja:3000",
	Level: Red, Previous: Green, Value: AvailabilityDown, Threshold: AvailabilityDown,
	Detail: "connection refused", At: t0,
}

func testMailConfig(port int) MailConfig {
	return MailConfig{
		Host: "127.0.0.1", Port: port, From: "Omni <omni@exemplo.com>",
		To: []string{"ana@exemplo.com", "bia@exemplo.com"}, PanelURL: "http://127.0.0.1:8080/alertas",
	}
}

func decodeBody(t *testing.T, raw string) (mail.Header, string) {
	t.Helper()
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("mensagem inválida: %v\n%s", err, raw)
	}
	b, err := io.ReadAll(quotedprintable.NewReader(msg.Body))
	if err != nil {
		t.Fatal(err)
	}
	return msg.Header, string(b)
}

func TestSMTPSendDeliversMessage(t *testing.T) {
	srv := newFakeSMTP(t, false)
	m := NewMailer(testMailConfig(srv.port()))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Run(ctx)

	m.Notify(sampleAlert)
	select {
	case <-srv.done:
	case <-time.After(5 * time.Second):
		t.Fatal("o servidor não recebeu o e-mail")
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.from != "omni@exemplo.com" {
		t.Errorf("MAIL FROM = %q", srv.from)
	}
	if strings.Join(srv.rcpts, ",") != "ana@exemplo.com,bia@exemplo.com" {
		t.Errorf("RCPT TO = %v", srv.rcpts)
	}
	h, body := decodeBody(t, srv.data)
	subject, err := new(mime.WordDecoder).DecodeHeader(h.Get("Subject"))
	if err != nil {
		t.Fatal(err)
	}
	if subject != "[Omni][VERMELHO] CRÍTICO: Disponibilidade em loja:3000" {
		t.Errorf("assunto = %q", subject)
	}
	if h.Get("X-Omni-Nivel") != "vermelho" {
		t.Errorf("X-Omni-Nivel = %q", h.Get("X-Omni-Nivel"))
	}
	for _, want := range []string{"Nível: VERMELHO (antes: VERDE)", "Recurso: loja:3000", "Estado: fora do ar",
		"Detalhe: connection refused", "Painel: http://127.0.0.1:8080/alertas"} {
		if !strings.Contains(body, want) {
			t.Errorf("corpo sem %q:\n%s", want, body)
		}
	}
}

func TestHeaderInjectionIsBlocked(t *testing.T) {
	m := NewMailer(testMailConfig(25))
	a := sampleAlert
	a.Subject = "loja\r\nBcc: vitima@exemplo.com"
	raw, err := m.buildMessage(a, 0)
	if err != nil {
		t.Fatal(err)
	}
	h, _ := decodeBody(t, string(raw))
	if h.Get("Bcc") != "" {
		t.Fatalf("cabeçalho injetado: Bcc = %q", h.Get("Bcc"))
	}
	if strings.Contains(string(raw), "\r\nBcc:") {
		t.Fatal("a mensagem contém uma linha Bcc")
	}
}

func TestMetricBodyShowsValueAndThreshold(t *testing.T) {
	m := NewMailer(testMailConfig(25))
	a := Alert{Rule: RuleHTTPP95, Service: ServiceWeb, Name: "Latência p95 das requisições", Level: Yellow,
		Previous: Green, Value: 1234.56, Threshold: 1000, Unit: "ms", At: t0}
	raw, err := m.buildMessage(a, 3)
	if err != nil {
		t.Fatal(err)
	}
	_, body := decodeBody(t, string(raw))
	for _, want := range []string{"Valor: 1234.6 ms (limite: 1000 ms)", "3 alerta(s) anteriores não foram enviados"} {
		if !strings.Contains(body, want) {
			t.Errorf("corpo sem %q:\n%s", want, body)
		}
	}
}

func TestRateLimit(t *testing.T) {
	m := NewMailer(testMailConfig(25))
	now := t0
	m.now = func() time.Time { return now }
	var sent int
	m.send = func(context.Context, []byte) error { sent++; return nil }

	for i := 0; i < mailPerMinute+5; i++ {
		m.deliver(context.Background(), sampleAlert)
	}
	if sent != mailPerMinute {
		t.Fatalf("enviados = %d, quer %d", sent, mailPerMinute)
	}
	now = now.Add(time.Minute)
	suppressed, ok := m.allow()
	if !ok || suppressed != 5 {
		t.Fatalf("depois de 1 min: ok=%v suprimidos=%d, quer true e 5", ok, suppressed)
	}
}

func TestNotifyNeverBlocks(t *testing.T) {
	m := NewMailer(testMailConfig(25)) // sem Run: ninguém consome a fila
	done := make(chan struct{})
	go func() {
		for i := 0; i < mailQueueSize*2; i++ {
			m.Notify(sampleAlert)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Notify bloqueou com a fila cheia")
	}
}

func TestIsLoopbackHost(t *testing.T) {
	for _, h := range []string{"localhost", "127.0.0.1", "::1"} {
		if !isLoopbackHost(h) {
			t.Errorf("%s deveria ser loopback", h)
		}
	}
	for _, h := range []string{"smtp.gmail.com", "10.0.0.5", "localhost.exemplo.com"} {
		if isLoopbackHost(h) {
			t.Errorf("%s não é loopback", h)
		}
	}
}

// remoteMailer aponta para o servidor falso, mas com um nome de host remoto:
// a decisão de segurança olha para o nome configurado.
func remoteMailer(srv *fakeSMTP, user string) *Mailer {
	cfg := testMailConfig(587)
	cfg.Host = "smtp.exemplo.com"
	cfg.User, cfg.Password = user, "segredo"
	m := NewMailer(cfg)
	m.addr = srv.ln.Addr().String()
	return m
}

func TestRemoteWithoutTLSRefusesPassword(t *testing.T) {
	srv := newFakeSMTP(t, false)
	m := remoteMailer(srv, "omni")
	if err := m.smtpSend(context.Background(), []byte("Subject: x\r\n\r\nx\r\n")); !errors.Is(err, ErrNoTLS) {
		t.Fatalf("erro = %v, quer ErrNoTLS", err)
	}
}

func TestRemoteWithoutTLSAndWithoutPasswordSends(t *testing.T) {
	srv := newFakeSMTP(t, false)
	m := remoteMailer(srv, "")
	if err := m.smtpSend(context.Background(), []byte("Subject: x\r\n\r\nx\r\n")); err != nil {
		t.Fatalf("erro = %v", err)
	}
}

func TestBareAddressAndFormat(t *testing.T) {
	if got := bareAddress("Omni <a@b.c>"); got != "a@b.c" {
		t.Errorf("bareAddress = %q", got)
	}
	if got := bareAddress(" a@b.c "); got != "a@b.c" {
		t.Errorf("bareAddress = %q", got)
	}
	cases := map[string]string{
		formatValue(12, "%"):    "12%",
		formatValue(1.26, "ms"): "1.3 ms",
		formatValue(2, ""):      "2",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("formatValue = %q, quer %q", got, want)
		}
	}
}

func TestSMTPNoTLSBody(t *testing.T) {
	m := NewMailer(testMailConfig(25))
	a := Alert{Rule: RuleSMTPNoTLS, Service: ServiceSMTP, Name: "SMTP sem STARTTLS", Subject: "loja:1025",
		Level: Yellow, Previous: Green, Value: 1, Threshold: 1, At: t0}
	raw, err := m.buildMessage(a, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, body := decodeBody(t, string(raw))
	if !strings.Contains(body, "não oferece STARTTLS") || strings.Contains(body, "Valor:") {
		t.Errorf("corpo:\n%s", body)
	}
}
