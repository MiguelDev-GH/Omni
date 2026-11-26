package load

import (
	"bufio"
	"context"
	"encoding/binary"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSMTP aceita conexões e conta as mensagens entregues (DATA terminado).
func fakeSMTP(t *testing.T) (string, *atomic.Int64) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	var delivered atomic.Int64
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = c.Close() }()
				r := bufio.NewReader(c)
				reply := func(s string) { _, _ = c.Write([]byte(s + "\r\n")) }
				reply("220 fake")
				inData := false
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					line = strings.TrimRight(line, "\r\n")
					if inData {
						if line == "." {
							inData = false
							delivered.Add(1)
							reply("250 queued")
						}
						continue
					}
					switch strings.ToUpper(strings.SplitN(line, " ", 2)[0]) {
					case "EHLO", "HELO":
						reply("250 fake")
					case "DATA":
						inData = true
						reply("354 go")
					case "QUIT":
						reply("221 bye")
						return
					default:
						reply("250 ok")
					}
				}
			}()
		}
	}()
	return ln.Addr().String(), &delivered
}

func TestMailLoadSendsAtRate(t *testing.T) {
	addr, delivered := fakeSMTP(t)
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	sent, failed := RunMail(ctx, MailOptions{Addr: addr, PerMinute: 120})
	assert.Zero(t, failed)
	assert.GreaterOrEqual(t, sent, 2, "120 por minuto são 2 por segundo")
	assert.LessOrEqual(t, sent, 4)
	assert.Equal(t, int64(sent), delivered.Load())
}

func TestMailLoadCountsFailures(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close()) // ninguém escuta: conexão recusada
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	sent, failed := RunMail(ctx, MailOptions{Addr: addr, PerMinute: 120})
	assert.Zero(t, sent)
	assert.Positive(t, failed)
}

func TestDNSQueryPacket(t *testing.T) {
	p := dnsQuery(0x1234, "loja.local")
	require.Len(t, p, 12+len("\x04loja\x05local\x00")+4)
	assert.Equal(t, uint16(0x1234), binary.BigEndian.Uint16(p[0:]))
	assert.Equal(t, uint16(0x0100), binary.BigEndian.Uint16(p[2:]), "só RD ligado")
	assert.Equal(t, uint16(1), binary.BigEndian.Uint16(p[4:]), "uma pergunta")
	assert.Equal(t, "\x04loja\x05local\x00", string(p[12:24]))
	assert.Equal(t, []byte{0, 1, 0, 1}, p[24:], "tipo A, classe IN")
}

func TestDNSLoadQueries(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = pc.Close() })
	var got atomic.Int64
	go func() {
		buf := make([]byte, 512)
		for {
			n, from, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			got.Add(1)
			resp := append([]byte(nil), buf[:n]...)
			resp[2] |= 0x80 // QR: é uma resposta
			_, _ = pc.WriteTo(resp, from)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 1100*time.Millisecond)
	defer cancel()
	sent, answered := RunDNS(ctx, DNSOptions{Addr: pc.LocalAddr().String(), Name: "loja.local", Rate: 5})
	assert.GreaterOrEqual(t, sent, 4)
	assert.LessOrEqual(t, sent, 7)
	assert.Equal(t, sent, answered)
	assert.Equal(t, int64(sent), got.Load())
}
