package l7

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/sniffer/testutil"
)

const smtpPort = 1025

// smtpSession guarda a conexão e o relógio de uma conversa SMTP de teste.
type smtpSession struct {
	c      *testutil.Conn
	frames []timed
	now    time.Duration
}

func newSMTPSession() *smtpSession {
	c := testutil.NewConn("127.0.0.1", 41000, "127.0.0.1", smtpPort)
	s := &smtpSession{c: c, frames: at(0, c.Handshake()...)}
	return s.server("220 mail.loja.local ESMTP\r\n")
}

func (s *smtpSession) client(line string) *smtpSession {
	s.now += time.Millisecond
	s.frames = append(s.frames, timed{s.c.FromClient([]byte(line)), s.now})
	return s
}

func (s *smtpSession) server(line string) *smtpSession {
	s.now += time.Millisecond
	s.frames = append(s.frames, timed{s.c.FromServer([]byte(line)), s.now})
	return s
}

func (s *smtpSession) wait(d time.Duration) *smtpSession {
	s.now += d
	return s
}

func runSMTP(t *testing.T, s *smtpSession, flushAt time.Duration) []SMTPExchange {
	t.Helper()
	var got []SMTPExchange
	a := NewSMTPAssembler([]int{smtpPort}, func(ex SMTPExchange) { got = append(got, ex) })
	for _, f := range s.frames {
		a.Add(testutil.Packet(f.frame, t0.Add(f.at)))
	}
	if flushAt > 0 {
		a.Flush(t0.Add(flushAt))
	}
	return got
}

func hello(s *smtpSession) *smtpSession {
	return s.client("EHLO cliente\r\n").server("250-mail.loja.local\r\n250-PIPELINING\r\n250 SIZE 10240000\r\n")
}

func TestSMTPDelivered(t *testing.T) {
	s := hello(newSMTPSession()).
		client("MAIL FROM:<loja@loja.local>\r\n").server("250 OK\r\n").
		client("RCPT TO:<ana@exemplo.com>\r\n").server("250 OK\r\n").
		client("RCPT TO:<bia@exemplo.com>\r\n").server("250 OK\r\n").
		client("DATA\r\n").server("354 Go ahead\r\n").
		client("Subject: pedido\r\n\r\nSeu pedido saiu.\r\n.\r\n")
	end := s.now
	s.wait(39 * time.Millisecond).server("250 2.0.0 Ok: queued\r\n").client("QUIT\r\n").server("221 Bye\r\n")

	got := runSMTP(t, s, 0)
	require.Len(t, got, 1)
	ex := got[0]
	assert.Equal(t, "entregue", ex.Outcome)
	assert.Equal(t, 250, ex.Code)
	assert.Equal(t, 2, ex.Recipients)
	assert.Equal(t, s.now-2*time.Millisecond-end, ex.Latency) // do "." até o 250
	assert.Equal(t, 40*time.Millisecond, ex.Latency)
}

func TestSMTPRejectedAtRcpt(t *testing.T) {
	s := hello(newSMTPSession()).
		client("MAIL FROM:<loja@loja.local>\r\n").server("250 OK\r\n").
		client("RCPT TO:<nao-existe@exemplo.com>\r\n").server("550 5.1.1 User unknown\r\n").
		client("QUIT\r\n").server("221 Bye\r\n")
	got := runSMTP(t, s, 0)
	require.Len(t, got, 1)
	assert.Equal(t, "rejeitado", got[0].Outcome)
	assert.Equal(t, 550, got[0].Code)
	assert.Equal(t, 0, got[0].Recipients)
}

func TestSMTPRejectedAtMail(t *testing.T) {
	s := hello(newSMTPSession()).
		client("MAIL FROM:<spam@x.com>\r\n").server("421 4.7.0 Try again later\r\n")
	got := runSMTP(t, s, 0)
	require.Len(t, got, 1)
	assert.Equal(t, "rejeitado", got[0].Outcome)
	assert.Equal(t, 421, got[0].Code)
}

func TestSMTPRejectedAfterData(t *testing.T) {
	s := hello(newSMTPSession()).
		client("MAIL FROM:<a@b.c>\r\n").server("250 OK\r\n").
		client("RCPT TO:<d@e.f>\r\n").server("250 OK\r\n").
		client("DATA\r\n").server("354 Go\r\n").
		client("corpo\r\n.\r\n").server("554 5.7.1 Spam\r\n")
	got := runSMTP(t, s, 0)
	require.Len(t, got, 1)
	assert.Equal(t, "rejeitado", got[0].Outcome)
	assert.Equal(t, 554, got[0].Code)
}

func TestSMTPPipelinedCommands(t *testing.T) {
	s := hello(newSMTPSession()).
		client("MAIL FROM:<a@b.c>\r\nRCPT TO:<d@e.f>\r\nRCPT TO:<g@h.i>\r\nDATA\r\n").
		server("250 OK\r\n250 OK\r\n550 No\r\n354 Go\r\n").
		client("x\r\n.\r\n").server("250 queued\r\n")
	got := runSMTP(t, s, 0)
	require.Len(t, got, 1)
	assert.Equal(t, "entregue", got[0].Outcome)
	assert.Equal(t, 1, got[0].Recipients)
}

func TestSMTPBodyTerminatorSplitAcrossSegments(t *testing.T) {
	s := hello(newSMTPSession()).
		client("MAIL FROM:<a@b.c>\r\n").server("250 OK\r\n").
		client("RCPT TO:<d@e.f>\r\n").server("250 OK\r\n").
		client("DATA\r\n").server("354 Go\r\n").
		client("linha com MAIL FROM:<falso> no corpo\r\n.").
		client("\r\n").server("250 queued\r\n")
	got := runSMTP(t, s, 0)
	require.Len(t, got, 1)
	assert.Equal(t, "entregue", got[0].Outcome)
}

func TestSMTPDotStuffedLineIsNotTheEnd(t *testing.T) {
	s := hello(newSMTPSession()).
		client("MAIL FROM:<a@b.c>\r\n").server("250 OK\r\n").
		client("RCPT TO:<d@e.f>\r\n").server("250 OK\r\n").
		client("DATA\r\n").server("354 Go\r\n").
		client("..linha que começa com ponto\r\n").
		client(".\r\n").server("250 queued\r\n")
	got := runSMTP(t, s, 0)
	require.Len(t, got, 1)
	assert.Equal(t, "entregue", got[0].Outcome)
}

func TestSMTPStopsAfterStartTLS(t *testing.T) {
	s := hello(newSMTPSession()).
		client("STARTTLS\r\n").server("220 Ready to start TLS\r\n").
		client("\x16\x03\x01MAIL FROM:<a@b.c>\r\n").server("\x16\x03\x03250 OK\r\n")
	assert.Empty(t, runSMTP(t, s, 3*time.Minute))
}

func TestSMTPNeverReadsAuth(t *testing.T) {
	s := hello(newSMTPSession()).
		client("AUTH LOGIN\r\n").server("334 VXNlcm5hbWU6\r\n").
		client("MAIL FROM:<falso@x.y>\r\n").server("334 UGFzc3dvcmQ6\r\n").
		client("c2VuaGE=\r\n").server("235 2.7.0 Authentication successful\r\n").
		client("RSET\r\n").server("250 OK\r\n")
	assert.Empty(t, runSMTP(t, s, 3*time.Minute), "nada dentro do AUTH abre transação")

	s = hello(newSMTPSession()).
		client("AUTH PLAIN AGFuYQBzZW5oYQ==\r\n").server("235 ok\r\n").
		client("MAIL FROM:<a@b.c>\r\n").server("250 OK\r\n").
		client("RCPT TO:<d@e.f>\r\n").server("250 OK\r\n").
		client("DATA\r\n").server("354 Go\r\n").
		client("x\r\n.\r\n").server("250 queued\r\n")
	got := runSMTP(t, s, 0)
	require.Len(t, got, 1, "depois do AUTH a conversa volta a ser lida")
	assert.Equal(t, "entregue", got[0].Outcome)
}

func TestSMTPAbortedOnClose(t *testing.T) {
	s := hello(newSMTPSession()).
		client("MAIL FROM:<a@b.c>\r\n").server("250 OK\r\n").
		client("RCPT TO:<d@e.f>\r\n").server("250 OK\r\n")
	got := runSMTP(t, s, IdleTimeout+time.Minute)
	require.Len(t, got, 1)
	assert.Equal(t, "abortado", got[0].Outcome)
	assert.Equal(t, 0, got[0].Code)
	assert.Equal(t, 1, got[0].Recipients)
}

func TestSMTPRsetAfterRecipientsIsAborted(t *testing.T) {
	s := hello(newSMTPSession()).
		client("MAIL FROM:<a@b.c>\r\n").server("250 OK\r\n").
		client("RCPT TO:<d@e.f>\r\n").server("250 OK\r\n").
		client("RSET\r\n").server("250 OK\r\n")
	got := runSMTP(t, s, 0)
	require.Len(t, got, 1)
	assert.Equal(t, "abortado", got[0].Outcome)
}

func TestSMTPStatsWindow(t *testing.T) {
	s := NewSMTPStats()
	s.Flush(t0)
	for i := 1; i <= 4; i++ {
		s.Add(SMTPExchange{Outcome: OutcomeDelivered, Code: 250, Latency: time.Duration(i*10) * time.Millisecond})
	}
	s.Add(SMTPExchange{Outcome: OutcomeRejected, Code: 550})
	s.Add(SMTPExchange{Outcome: OutcomeRejected, Code: 421})
	s.Add(SMTPExchange{Outcome: OutcomeAborted})
	w := s.Flush(t0.Add(10 * time.Second))
	assert.Equal(t, t0, w.Start)
	assert.Equal(t, uint64(7), w.Attempts)
	assert.Equal(t, uint64(4), w.Delivered)
	assert.Equal(t, uint64(2), w.Rejected)
	assert.Equal(t, uint64(1), w.Aborted)
	assert.Equal(t, map[string]uint64{"550": 1, "421": 1}, w.ByCode)
	assert.Equal(t, 20.0, w.P50MS)
	assert.Equal(t, 40.0, w.P95MS)

	next := s.Flush(t0.Add(20 * time.Second))
	assert.Equal(t, uint64(0), next.Attempts)
	assert.Equal(t, map[string]uint64{}, next.ByCode)
}
