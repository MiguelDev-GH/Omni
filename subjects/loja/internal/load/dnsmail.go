package load

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// MailOptions configura a carga de e-mails: mensagens de pedido enviadas ao
// servidor SMTP, como uma loja que avisa os clientes.
type MailOptions struct {
	Addr      string // host:porta do servidor SMTP
	PerMinute int
}

// RunMail envia PerMinute e-mails por minuto até ctx acabar e devolve quantos
// foram aceitos e quantos falharam. Cada e-mail usa uma conexão nova.
func RunMail(ctx context.Context, o MailOptions) (sent, failed int) {
	if o.PerMinute <= 0 {
		return 0, 0
	}
	ticker := time.NewTicker(time.Minute / time.Duration(o.PerMinute))
	defer ticker.Stop()
	for n := 1; ; n++ {
		select {
		case <-ctx.Done():
			return sent, failed
		case <-ticker.C:
		}
		to := fmt.Sprintf("cliente%d@exemplo.com", n%50)
		msg := fmt.Sprintf("From: loja@loja.local\r\nTo: %s\r\nSubject: Pedido %d confirmado\r\n\r\nSeu pedido %d foi confirmado.\r\n", to, n, n)
		if err := smtp.SendMail(o.Addr, nil, "loja@loja.local", []string{to}, []byte(msg)); err != nil {
			failed++
			continue
		}
		sent++
	}
}

// DNSOptions configura a carga de consultas DNS do tipo A.
type DNSOptions struct {
	Addr string // host:porta do servidor DNS
	Name string // nome consultado, como loja.local
	Rate int    // consultas por segundo
}

// RunDNS faz Rate consultas por segundo até ctx acabar e devolve quantas
// foram enviadas e quantas tiveram resposta em até 1 s.
func RunDNS(ctx context.Context, o DNSOptions) (sent, answered int) {
	if o.Rate <= 0 {
		return 0, 0
	}
	ticker := time.NewTicker(time.Second / time.Duration(o.Rate))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return sent, answered
		case <-ticker.C:
		}
		ok, err := askDNS(o.Addr, o.Name)
		if err != nil {
			continue
		}
		sent++
		if ok {
			answered++
		}
	}
}

// askDNS envia uma consulta por um socket UDP novo (porta de origem nova a
// cada consulta) e diz se chegou uma resposta com o mesmo ID.
func askDNS(addr, name string) (bool, error) {
	c, err := net.Dial("udp", addr)
	if err != nil {
		return false, err
	}
	defer func() { _ = c.Close() }()
	id := uint16(rand.Uint32())
	if _, err := c.Write(dnsQuery(id, name)); err != nil {
		return false, err
	}
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 512)
	n, err := c.Read(buf)
	if err != nil {
		return false, nil
	}
	return n >= 12 && binary.BigEndian.Uint16(buf) == id && buf[2]&0x80 != 0, nil
}

// dnsQuery monta uma pergunta DNS do tipo A, classe IN, com recursão pedida.
func dnsQuery(id uint16, name string) []byte {
	p := make([]byte, 12, 12+len(name)+6)
	binary.BigEndian.PutUint16(p[0:], id)
	binary.BigEndian.PutUint16(p[2:], 0x0100) // RD
	binary.BigEndian.PutUint16(p[4:], 1)      // QDCOUNT
	for _, label := range strings.Split(strings.TrimSuffix(name, "."), ".") {
		p = append(p, byte(len(label)))
		p = append(p, label...)
	}
	return append(p, 0, 0, 1, 0, 1)
}
