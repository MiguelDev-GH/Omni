package poller

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/target"
)

// fakeDNSServer responde toda consulta com o rcode dado e, se ip não for
// vazio, um registro A apontando para ip.
func fakeDNSServer(t *testing.T, rcode int, ip string) int {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = pc.Close() })
	go func() {
		buf := make([]byte, 512)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			resp := append([]byte(nil), buf[:n]...)
			binary.BigEndian.PutUint16(resp[2:], 0x8180|uint16(rcode))
			if ip != "" {
				binary.BigEndian.PutUint16(resp[6:], 1)
				resp = append(resp, 0xC0, 12, 0, 1, 0, 1, 0, 0, 0, 60, 0, 4)
				resp = append(resp, net.ParseIP(ip).To4()...)
			}
			_, _ = pc.WriteTo(resp, addr)
		}
	}()
	return pc.LocalAddr().(*net.UDPAddr).Port
}

// fakeSMTPServer aceita conexões e responde com STARTTLS anunciado e o
// RCPT TO com rcpt.
func fakeSMTPServer(t *testing.T, rcpt string) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				reply := func(l string) { _, _ = io.WriteString(conn, l+"\r\n") }
				reply("220 fake ESMTP")
				r := bufio.NewReader(conn)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					switch up := strings.ToUpper(strings.TrimSpace(line)); {
					case strings.HasPrefix(up, "EHLO"):
						reply("250-fake")
						reply("250 STARTTLS")
					case strings.HasPrefix(up, "RCPT"):
						reply(rcpt)
					case up == "QUIT":
						reply("221 tchau")
						return
					default:
						reply("250 ok")
					}
				}
			}()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestCheckDNS(t *testing.T) {
	ok := CheckDNS(context.Background(), 1, "127.0.0.1", fakeDNSServer(t, 0, "127.0.0.1"), "loja.local")
	assert.Equal(t, KindDNS, ok.Kind)
	assert.True(t, ok.Up)
	assert.False(t, ok.Degraded)
	assert.Equal(t, "NOERROR: 127.0.0.1", ok.Detail)
	assert.Nil(t, ok.StartTLS)

	nx := CheckDNS(context.Background(), 1, "127.0.0.1", fakeDNSServer(t, 3, ""), "loja.local")
	assert.True(t, nx.Up)
	assert.True(t, nx.Degraded)
	assert.Equal(t, "NXDOMAIN", nx.Detail)

	down := CheckDNS(context.Background(), 1, "127.0.0.1", closedPort(t), "loja.local")
	assert.False(t, down.Up)
	assert.NotEmpty(t, down.Err)
}

func TestCheckSMTP(t *testing.T) {
	ok := CheckSMTP(context.Background(), 1, "127.0.0.1", fakeSMTPServer(t, "250 ok"), "loja.local")
	assert.Equal(t, KindSMTP, ok.Kind)
	assert.True(t, ok.Up)
	assert.False(t, ok.Degraded)
	require.NotNil(t, ok.StartTLS)
	assert.True(t, *ok.StartTLS)

	rejected := CheckSMTP(context.Background(), 1, "127.0.0.1", fakeSMTPServer(t, "550 não"), "loja.local")
	assert.True(t, rejected.Up)
	assert.True(t, rejected.Degraded)
	assert.Contains(t, rejected.Detail, "rcpt recusado")

	down := CheckSMTP(context.Background(), 1, "127.0.0.1", closedPort(t), "")
	assert.False(t, down.Up)
	assert.Nil(t, down.StartTLS)
}

func TestRunOnceChecksDNSAndSMTP(t *testing.T) {
	srv, appPort := newUpServer(t)
	defer srv.Close()
	dnsPort := fakeDNSServer(t, 0, "127.0.0.1")
	smtpPort := fakeSMTPServer(t, "250 ok")
	st := &fakeStore{targets: []target.Target{{
		ID: 7, Name: "loja", Host: "127.0.0.1", AppPorts: []int{appPort}, Domain: "loja.local",
		DNSPort: dnsPort, SMTPPort: smtpPort,
	}}}
	pub := &fakePub{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, NewScheduler(st, pub, time.Hour).RunOnce(ctx))

	stored := st.stored()
	require.Len(t, stored, 3)
	kinds := []string{stored[0].Kind, stored[1].Kind, stored[2].Kind}
	assert.Equal(t, []string{KindHTTP, KindDNS, KindSMTP}, kinds)
	for _, r := range stored {
		assert.True(t, r.Up, "%s na porta %d deveria estar no ar", r.Kind, r.Port)
	}
	assert.Empty(t, pub.ofType(EventDown))
}
