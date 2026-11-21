package probe

import (
	"bufio"
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// smtpScript configura as respostas do servidor falso.
type smtpScript struct {
	banner   string
	starttls bool
	rcpt     string
}

// fakeSMTPServer devolve a porta e uma função que lista os comandos
// recebidos depois que o servidor encerra a conversa.
func fakeSMTPServer(t *testing.T, s smtpScript) (int, func() []string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	var cmds []string
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		reply := func(l string) { _, _ = io.WriteString(conn, l+"\r\n") }
		reply(s.banner)
		r := bufio.NewReader(conn)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.TrimRight(line, "\r\n")
			cmds = append(cmds, cmd)
			switch up := strings.ToUpper(cmd); {
			case strings.HasPrefix(up, "EHLO"):
				if s.starttls {
					reply("250-fake")
					reply("250 STARTTLS")
				} else {
					reply("250 fake")
				}
			case strings.HasPrefix(up, "MAIL FROM"):
				reply("250 ok")
			case strings.HasPrefix(up, "RCPT TO"):
				reply(s.rcpt)
			case up == "RSET":
				reply("250 ok")
			case up == "QUIT":
				reply("221 tchau")
				return
			default:
				reply("502 não")
			}
		}
	}()
	wait := func() []string {
		select {
		case <-finished:
		case <-time.After(2 * time.Second):
			t.Fatal("o servidor falso não terminou")
		}
		return cmds
	}
	return ln.Addr().(*net.TCPAddr).Port, wait
}

func TestProbeSMTPFullConversation(t *testing.T) {
	port, cmds := fakeSMTPServer(t, smtpScript{banner: "220 fake ESMTP", starttls: true, rcpt: "250 ok"})
	r := ProbeSMTP(context.Background(), "127.0.0.1", port, "loja.local", time.Second)
	if !r.OK() || !r.StartTLS || !r.RcptTested {
		t.Fatalf("resultado = %+v", r)
	}
	if r.Detail() != "aceita destinatário postmaster" {
		t.Errorf("Detail() = %q", r.Detail())
	}
	got := strings.Join(cmds(), "|")
	for _, want := range []string{"EHLO omni-sonda.localhost", "MAIL FROM:<>", "RCPT TO:<postmaster@loja.local>", "RSET"} {
		if !strings.Contains(got, want) {
			t.Errorf("comando %q ausente em %q", want, got)
		}
	}
	if strings.Contains(got, "DATA") {
		t.Error("a sonda não deve enviar e-mail (DATA)")
	}
}

func TestProbeSMTPWithoutDomainSkipsRcpt(t *testing.T) {
	port, cmds := fakeSMTPServer(t, smtpScript{banner: "220 fake", rcpt: "250 ok"})
	r := ProbeSMTP(context.Background(), "127.0.0.1", port, "", time.Second)
	if !r.OK() || r.StartTLS || r.RcptTested || r.Detail() != "saudação e EHLO aceitos" {
		t.Fatalf("resultado = %+v", r)
	}
	if got := cmds(); strings.Contains(strings.Join(got, "|"), "RCPT") {
		t.Errorf("RCPT enviado sem domínio: %v", got)
	}
}

func TestProbeSMTPRcptRejected(t *testing.T) {
	port, _ := fakeSMTPServer(t, smtpScript{banner: "220 fake", rcpt: "550 5.1.1 usuário desconhecido"})
	r := ProbeSMTP(context.Background(), "127.0.0.1", port, "loja.local", time.Second)
	if !r.Up || r.OK() || r.Code != 550 || r.Stage != "rcpt" {
		t.Fatalf("resultado = %+v", r)
	}
	if !strings.HasPrefix(r.Detail(), "rcpt recusado: ") {
		t.Errorf("Detail() = %q", r.Detail())
	}
}

func TestProbeSMTPBadBanner(t *testing.T) {
	port, _ := fakeSMTPServer(t, smtpScript{banner: "554 vá embora"})
	r := ProbeSMTP(context.Background(), "127.0.0.1", port, "", time.Second)
	if r.Up || r.Stage != "saudacao" || r.Code != 554 {
		t.Fatalf("resultado = %+v", r)
	}
}

func TestProbeSMTPConnectionRefused(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	r := ProbeSMTP(context.Background(), "127.0.0.1", port, "", time.Second)
	if r.Up || r.Stage != "conexao" || r.Err == "" {
		t.Fatalf("resultado = %+v", r)
	}
}

func TestProbeSMTPSilentServerTimesOut(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		c, err := ln.Accept()
		if err == nil {
			time.Sleep(time.Second)
			_ = c.Close()
		}
	}()
	start := time.Now()
	r := ProbeSMTP(context.Background(), "127.0.0.1", ln.Addr().(*net.TCPAddr).Port, "", 200*time.Millisecond)
	if r.Up || time.Since(start) > 900*time.Millisecond {
		t.Fatalf("resultado = %+v em %v", r, time.Since(start))
	}
}
