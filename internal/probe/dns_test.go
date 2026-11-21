package probe

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// dnsReply monta a resposta para query com rcode e endereços IPv4. O nome
// de cada resposta é um ponteiro de compressão para a pergunta (offset 12).
func dnsReply(query []byte, rcode int, truncated bool, ips ...string) []byte {
	// A pergunta vai do byte 12 até o fim da consulta.
	resp := append([]byte(nil), query...)
	flags := uint16(0x8180) | uint16(rcode) // QR, RD, RA
	if truncated {
		flags |= 0x0200
	}
	binary.BigEndian.PutUint16(resp[2:], flags)
	binary.BigEndian.PutUint16(resp[6:], uint16(len(ips)))
	for _, s := range ips {
		resp = append(resp, 0xC0, 12) // ponteiro para o nome da pergunta
		resp = binary.BigEndian.AppendUint16(resp, typeA)
		resp = binary.BigEndian.AppendUint16(resp, classIN)
		resp = binary.BigEndian.AppendUint32(resp, 60)
		resp = binary.BigEndian.AppendUint16(resp, 4)
		resp = append(resp, net.ParseIP(s).To4()...)
	}
	return resp
}

// fakeDNS responde cada consulta UDP com handle(consulta); nil não responde.
func fakeDNS(t *testing.T, handle func(q []byte) [][]byte) int {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	go func() {
		buf := make([]byte, 512)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			for _, r := range handle(append([]byte(nil), buf[:n]...)) {
				_, _ = pc.WriteTo(r, addr)
			}
		}
	}()
	return pc.LocalAddr().(*net.UDPAddr).Port
}

func TestQueryDNSNoError(t *testing.T) {
	port := fakeDNS(t, func(q []byte) [][]byte {
		return [][]byte{dnsReply(q, RCodeNoError, false, "127.0.0.1", "10.0.0.2")}
	})
	r := QueryDNS(context.Background(), "127.0.0.1", port, "loja.local", time.Second)
	if !r.OK() || r.RCode != RCodeNoError {
		t.Fatalf("resultado = %+v", r)
	}
	if strings.Join(r.Answers, ",") != "127.0.0.1,10.0.0.2" {
		t.Errorf("respostas = %v", r.Answers)
	}
	if r.Detail() != "NOERROR: 127.0.0.1, 10.0.0.2" {
		t.Errorf("Detail() = %q", r.Detail())
	}
}

func TestQueryDNSNXDomain(t *testing.T) {
	port := fakeDNS(t, func(q []byte) [][]byte { return [][]byte{dnsReply(q, RCodeNXDomain, false)} })
	r := QueryDNS(context.Background(), "127.0.0.1", port, "nao-existe.local", time.Second)
	if !r.Up || r.OK() || r.RCode != RCodeNXDomain || r.Detail() != "NXDOMAIN" {
		t.Fatalf("resultado = %+v", r)
	}
}

func TestQueryDNSIgnoresSpoofedID(t *testing.T) {
	port := fakeDNS(t, func(q []byte) [][]byte {
		forged := dnsReply(q, RCodeNoError, false, "6.6.6.6")
		binary.BigEndian.PutUint16(forged, binary.BigEndian.Uint16(q)+1) // ID errado
		return [][]byte{forged, dnsReply(q, RCodeNoError, false, "127.0.0.1")}
	})
	r := QueryDNS(context.Background(), "127.0.0.1", port, "loja.local", time.Second)
	if !r.OK() || len(r.Answers) != 1 || r.Answers[0] != "127.0.0.1" {
		t.Fatalf("aceitou a resposta forjada: %+v", r)
	}
}

func TestQueryDNSTimeout(t *testing.T) {
	port := fakeDNS(t, func([]byte) [][]byte { return nil })
	r := QueryDNS(context.Background(), "127.0.0.1", port, "loja.local", 200*time.Millisecond)
	if r.Up || r.Err == "" || r.Detail() != r.Err {
		t.Fatalf("resultado = %+v", r)
	}
}

func TestQueryDNSTruncatedFallsBackToTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port
	pc, err := net.ListenPacket("udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Skipf("porta UDP %d ocupada: %v", port, err)
	}
	defer func() { _ = pc.Close() }()
	go func() {
		buf := make([]byte, 512)
		n, addr, err := pc.ReadFrom(buf)
		if err == nil {
			_, _ = pc.WriteTo(dnsReply(buf[:n], RCodeNoError, true), addr)
		}
	}()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		var l [2]byte
		if _, err := io.ReadFull(conn, l[:]); err != nil {
			return
		}
		q := make([]byte, binary.BigEndian.Uint16(l[:]))
		if _, err := io.ReadFull(conn, q); err != nil {
			return
		}
		resp := dnsReply(q, RCodeNoError, false, "127.0.0.9")
		_, _ = conn.Write(append(binary.BigEndian.AppendUint16(nil, uint16(len(resp))), resp...))
	}()

	r := QueryDNS(context.Background(), "127.0.0.1", port, "grande.local", time.Second)
	if !r.OK() || !r.Truncated || r.Answers[0] != "127.0.0.9" {
		t.Fatalf("resultado = %+v", r)
	}
}

func TestEncodeName(t *testing.T) {
	got, err := encodeName("Loja.Local.")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "\x04loja\x05local\x00" {
		t.Errorf("encodeName = %q", got)
	}
	for _, bad := range []string{"", "a..b", strings.Repeat("a", 64) + ".com", strings.Repeat("a.", 130)} {
		if _, err := encodeName(bad); err == nil {
			t.Errorf("encodeName(%q) deveria falhar", bad)
		}
	}
}

func TestParseResponseRejectsWrongQuestion(t *testing.T) {
	q, id, err := buildQuery("outro.local", typeA)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := parseResponse(dnsReply(q, 0, false, "1.2.3.4"), id, "loja.local"); err == nil {
		t.Fatal("aceitou resposta para outro nome")
	}
}

func TestReadNamePointerLoop(t *testing.T) {
	// Um ponteiro que aponta para si mesmo: sem limite de saltos, o laço
	// nunca terminaria.
	msg := make([]byte, 14)
	msg[12], msg[13] = 0xC0, 12
	if _, _, err := readName(msg, 12); err != errBadPointer {
		t.Fatalf("erro = %v, quer errBadPointer", err)
	}
}

func TestParseResponseShortPackets(t *testing.T) {
	q, id, _ := buildQuery("loja.local", typeA)
	full := dnsReply(q, 0, false, "1.2.3.4")
	for n := 0; n < len(full); n++ {
		if _, _, err := parseResponse(full[:n], id, "loja.local"); err == nil {
			t.Fatalf("pacote cortado em %d bytes foi aceito", n)
		}
	}
}

func TestRCodeName(t *testing.T) {
	if RCodeName(RCodeServFail) != "SERVFAIL" || RCodeName(11) != "RCODE11" {
		t.Errorf("RCodeName errado")
	}
}
