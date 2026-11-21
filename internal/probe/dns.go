// Package probe faz as verificações ativas de DNS e SMTP do Poller. Só usa a
// biblioteca padrão: o cliente DNS monta e lê os pacotes byte a byte, como
// descrito na RFC 1035.
package probe

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// Tipos de registro e classe usados na consulta.
const (
	typeA    = 1
	typeAAAA = 28
	classIN  = 1
)

// Códigos de resposta (RCODE) do cabeçalho DNS.
const (
	RCodeNoError  = 0
	RCodeFormErr  = 1
	RCodeServFail = 2
	RCodeNXDomain = 3
	RCodeNotImp   = 4
	RCodeRefused  = 5
)

var rcodeNames = map[int]string{
	RCodeNoError: "NOERROR", RCodeFormErr: "FORMERR", RCodeServFail: "SERVFAIL",
	RCodeNXDomain: "NXDOMAIN", RCodeNotImp: "NOTIMP", RCodeRefused: "REFUSED",
}

// RCodeName devolve o nome do código de resposta, como "NXDOMAIN".
func RCodeName(rcode int) string {
	if n, ok := rcodeNames[rcode]; ok {
		return n
	}
	return "RCODE" + strconv.Itoa(rcode)
}

// maxUDPResponse é o maior pacote UDP aceito; respostas maiores chegam
// truncadas (bit TC) e são repetidas por TCP.
const maxUDPResponse = 4096

var (
	errShortPacket = errors.New("resposta DNS curta demais")
	errBadPointer  = errors.New("ponteiro de compressão inválido na resposta DNS")
)

// DNSResult é o resultado de uma consulta.
type DNSResult struct {
	// Up indica que o servidor respondeu, com qualquer RCODE.
	Up        bool
	RCode     int
	Answers   []string // endereços IPv4 e IPv6 das respostas
	Truncated bool     // a resposta UDP veio truncada e foi repetida por TCP
	LatencyMS float64
	Err       string
}

// OK indica uma resolução bem-sucedida: NOERROR com pelo menos uma resposta.
func (r DNSResult) OK() bool { return r.Up && r.RCode == RCodeNoError && len(r.Answers) > 0 }

// Detail resume o resultado para a interface, como "NOERROR: 127.0.0.1".
func (r DNSResult) Detail() string {
	if !r.Up {
		return r.Err
	}
	if r.RCode != RCodeNoError {
		return RCodeName(r.RCode)
	}
	if len(r.Answers) == 0 {
		return "NOERROR sem registros A/AAAA"
	}
	return "NOERROR: " + strings.Join(r.Answers, ", ")
}

func latencyMS(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// QueryDNS pergunta ao servidor host:port pelo registro A de name. Usa UDP
// e repete por TCP quando a resposta vem truncada.
func QueryDNS(ctx context.Context, host string, port int, name string, timeout time.Duration) DNSResult {
	start := time.Now()
	var res DNSResult
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	query, id, err := buildQuery(name, typeA)
	if err != nil {
		res.Err = err.Error()
		return res
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	resp, err := exchangeUDP(ctx, addr, query, id)
	if err == nil && isTruncated(resp) {
		res.Truncated = true
		resp, err = exchangeTCP(ctx, addr, query, id)
	}
	res.LatencyMS = latencyMS(time.Since(start))
	if err != nil {
		res.Err = err.Error()
		return res
	}
	rcode, answers, err := parseResponse(resp, id, name)
	if err != nil {
		res.Err = err.Error()
		return res
	}
	res.Up = true
	res.RCode = rcode
	res.Answers = answers
	return res
}

// buildQuery monta uma consulta com ID aleatório e recursão pedida (RD).
// O ID aleatório, junto com a porta de origem aleatória que o sistema
// escolhe, dificulta que um atacante forje a resposta (envenenamento de cache).
func buildQuery(name string, qtype uint16) ([]byte, uint16, error) {
	var idb [2]byte
	if _, err := rand.Read(idb[:]); err != nil {
		return nil, 0, err
	}
	id := binary.BigEndian.Uint16(idb[:])
	qname, err := encodeName(name)
	if err != nil {
		return nil, 0, err
	}
	msg := make([]byte, 12, 12+len(qname)+4)
	binary.BigEndian.PutUint16(msg[0:], id)
	binary.BigEndian.PutUint16(msg[2:], 0x0100) // RD = 1
	binary.BigEndian.PutUint16(msg[4:], 1)      // QDCOUNT
	msg = append(msg, qname...)
	msg = binary.BigEndian.AppendUint16(msg, qtype)
	msg = binary.BigEndian.AppendUint16(msg, classIN)
	return msg, id, nil
}

// encodeName converte "loja.local" em 4loja5local0.
func encodeName(name string) ([]byte, error) {
	name = strings.TrimSuffix(strings.ToLower(name), ".")
	if name == "" || len(name) > 253 {
		return nil, fmt.Errorf("nome DNS inválido: %q", name)
	}
	var out []byte
	for _, label := range strings.Split(name, ".") {
		if label == "" || len(label) > 63 {
			return nil, fmt.Errorf("nome DNS inválido: %q", name)
		}
		out = append(out, byte(len(label)))
		out = append(out, label...)
	}
	return append(out, 0), nil
}

func isTruncated(resp []byte) bool { return len(resp) >= 4 && resp[2]&0x02 != 0 }

// exchangeUDP envia a consulta e espera uma resposta com o mesmo ID,
// ignorando pacotes que não batem (possíveis respostas forjadas ou atrasadas).
func exchangeUDP(ctx context.Context, addr string, query []byte, id uint16) ([]byte, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	if _, err := conn.Write(query); err != nil {
		return nil, err
	}
	buf := make([]byte, maxUDPResponse)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return nil, err
		}
		if n >= 12 && binary.BigEndian.Uint16(buf) == id && buf[2]&0x80 != 0 {
			return append([]byte(nil), buf[:n]...), nil
		}
	}
}

// exchangeTCP usa o formato de DNS sobre TCP: cada mensagem vem precedida
// do tamanho em 2 bytes.
func exchangeTCP(ctx context.Context, addr string, query []byte, id uint16) ([]byte, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	framed := binary.BigEndian.AppendUint16(nil, uint16(len(query)))
	if _, err := conn.Write(append(framed, query...)); err != nil {
		return nil, err
	}
	var lenb [2]byte
	if _, err := io.ReadFull(conn, lenb[:]); err != nil {
		return nil, err
	}
	resp := make([]byte, binary.BigEndian.Uint16(lenb[:]))
	if _, err := io.ReadFull(conn, resp); err != nil {
		return nil, err
	}
	if len(resp) < 12 || binary.BigEndian.Uint16(resp) != id {
		return nil, errors.New("resposta DNS com ID diferente da consulta")
	}
	return resp, nil
}

// parseResponse confere a pergunta ecoada, lê o RCODE e extrai os endereços
// A e AAAA da seção de respostas.
func parseResponse(msg []byte, id uint16, name string) (int, []string, error) {
	if len(msg) < 12 {
		return 0, nil, errShortPacket
	}
	if binary.BigEndian.Uint16(msg) != id {
		return 0, nil, errors.New("resposta DNS com ID diferente da consulta")
	}
	flags := binary.BigEndian.Uint16(msg[2:])
	if flags&0x8000 == 0 {
		return 0, nil, errors.New("pacote DNS não é uma resposta")
	}
	rcode := int(flags & 0x000F)
	qd := int(binary.BigEndian.Uint16(msg[4:]))
	an := int(binary.BigEndian.Uint16(msg[6:]))

	off := 12
	for i := 0; i < qd; i++ {
		qname, next, err := readName(msg, off)
		if err != nil {
			return 0, nil, err
		}
		if next+4 > len(msg) {
			return 0, nil, errShortPacket
		}
		if i == 0 && !strings.EqualFold(qname, strings.TrimSuffix(name, ".")) {
			return 0, nil, fmt.Errorf("resposta DNS para outro nome: %q", qname)
		}
		off = next + 4
	}

	var answers []string
	for i := 0; i < an; i++ {
		_, next, err := readName(msg, off)
		if err != nil {
			return 0, nil, err
		}
		if next+10 > len(msg) {
			return 0, nil, errShortPacket
		}
		rtype := binary.BigEndian.Uint16(msg[next:])
		rdlen := int(binary.BigEndian.Uint16(msg[next+8:]))
		rdata := next + 10
		if rdata+rdlen > len(msg) {
			return 0, nil, errShortPacket
		}
		switch {
		case rtype == typeA && rdlen == 4, rtype == typeAAAA && rdlen == 16:
			answers = append(answers, net.IP(msg[rdata:rdata+rdlen]).String())
		}
		off = rdata + rdlen
	}
	return rcode, answers, nil
}

// readName lê um nome a partir de off, seguindo ponteiros de compressão
// (RFC 1035, seção 4.1.4). Devolve o nome e o deslocamento logo depois dele
// na posição original. Limita os saltos para um pacote malicioso com
// ponteiros em ciclo não travar a leitura.
func readName(msg []byte, off int) (string, int, error) {
	var labels []string
	next := -1
	for jumps := 0; ; {
		if off >= len(msg) {
			return "", 0, errShortPacket
		}
		l := int(msg[off])
		switch {
		case l == 0:
			if next < 0 {
				next = off + 1
			}
			return strings.Join(labels, "."), next, nil
		case l&0xC0 == 0xC0:
			if off+1 >= len(msg) {
				return "", 0, errShortPacket
			}
			if jumps++; jumps > 16 {
				return "", 0, errBadPointer
			}
			if next < 0 {
				next = off + 2
			}
			off = int(binary.BigEndian.Uint16(msg[off:]) & 0x3FFF)
		case l&0xC0 != 0:
			return "", 0, errBadPointer
		default:
			if off+1+l > len(msg) {
				return "", 0, errShortPacket
			}
			labels = append(labels, string(msg[off+1:off+1+l]))
			off += 1 + l
		}
	}
}
