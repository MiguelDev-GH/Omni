package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
	"time"
)

// heloName é o nome que a sonda usa no EHLO.
const heloName = "omni-sonda.localhost"

// SMTPResult é o resultado de uma conversa de teste com um servidor SMTP.
type SMTPResult struct {
	// Up indica que o servidor aceitou a conexão e mandou a saudação 220.
	Up bool
	// Code é o código da resposta que falhou (por exemplo 550), ou 0.
	Code int
	// Stage diz em que passo a conversa falhou: "conexao", "saudacao",
	// "ehlo", "mail", "rcpt" ou "".
	Stage string
	// StartTLS indica que o servidor oferece STARTTLS no EHLO.
	StartTLS bool
	// RcptTested indica que o RCPT TO para postmaster foi testado.
	RcptTested bool
	LatencyMS  float64
	Err        string
}

// OK indica que todos os passos testados foram aceitos.
func (r SMTPResult) OK() bool { return r.Up && r.Err == "" }

// Detail resume o resultado para a interface.
func (r SMTPResult) Detail() string {
	switch {
	case !r.Up:
		return r.Err
	case r.Err != "":
		return fmt.Sprintf("%s recusado: %s", r.Stage, r.Err)
	case r.RcptTested:
		return "aceita destinatário postmaster"
	default:
		return "saudação e EHLO aceitos"
	}
}

// ProbeSMTP conversa com o servidor sem enviar e-mail: espera a saudação
// 220, manda EHLO e, se domain não for vazio, testa MAIL FROM:<> e
// RCPT TO:<postmaster@domain> e desfaz com RSET. Todo domínio que recebe
// e-mail precisa aceitar postmaster (RFC 5321, seção 4.5.1), então ele
// serve de destinatário de teste sem incomodar ninguém.
func ProbeSMTP(ctx context.Context, host string, port int, domain string, timeout time.Duration) SMTPResult {
	start := time.Now()
	var res SMTPResult
	done := func(stage string, err error) SMTPResult {
		res.LatencyMS = latencyMS(time.Since(start))
		if err != nil {
			res.Stage = stage
			res.Err = err.Error()
			var tpErr *textproto.Error
			if errors.As(err, &tpErr) {
				res.Code = tpErr.Code
			}
		}
		return res
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return done("conexao", err)
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	// smtp.NewClient lê a saudação e exige o código 220.
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return done("saudacao", err)
	}
	defer func() { _ = c.Close() }()
	res.Up = true

	if err := c.Hello(heloName); err != nil {
		return done("ehlo", err)
	}
	res.StartTLS, _ = c.Extension("STARTTLS")

	if domain != "" {
		if err := c.Mail(""); err != nil {
			return done("mail", err)
		}
		res.RcptTested = true
		if err := c.Rcpt("postmaster@" + domain); err != nil {
			return done("rcpt", err)
		}
		if err := c.Reset(); err != nil {
			return done("rset", err)
		}
	}
	_ = c.Quit()
	return done("", nil)
}
