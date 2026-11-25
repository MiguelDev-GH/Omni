package l7

import (
	"net"
	"slices"
	"strconv"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
)

const (
	// dnsTimeout é quanto uma consulta espera a resposta antes de contar
	// como falha (TIMEOUT).
	dnsTimeout = 5 * time.Second
	// maxDNSPending limita as consultas sem resposta guardadas; com o limite
	// cheio, a mais antiga sai como TIMEOUT.
	maxDNSPending = 10_000

	rcodeTimeout = "TIMEOUT"
)

// DNSExchange é uma consulta DNS vista no tráfego, pareada com a resposta ou
// expirada. O nome consultado nunca sai do decodificador.
type DNSExchange struct {
	RCode   string        `json:"rcode"`  // "NOERROR", "NXDOMAIN", "SERVFAIL"...; "TIMEOUT" sem resposta
	Failed  bool          `json:"falhou"` // RCode diferente de NOERROR, ou timeout
	Latency time.Duration `json:"latencia_ns"`
	At      time.Time     `json:"hora"` // hora da consulta
}

type dnsKey struct {
	client string // ip:porta do cliente
	id     uint16
}

// DNSTracker pareia consultas e respostas DNS por UDP nas portas dnsPorts.
// DNS sobre TCP não é decodificado. Não é seguro para uso concorrente.
type DNSTracker struct {
	ports   []int
	emit    func(DNSExchange)
	pending map[dnsKey]time.Time
	// order guarda as chaves na ordem de chegada; chaves já respondidas
	// continuam até a próxima limpeza e são puladas.
	order []dnsKey
}

// NewDNSTracker chama emit a cada consulta respondida ou expirada, na mesma
// goroutine de Add e Flush.
func NewDNSTracker(dnsPorts []int, emit func(DNSExchange)) *DNSTracker {
	return &DNSTracker{ports: dnsPorts, emit: emit, pending: map[dnsKey]time.Time{}}
}

// Add processa um pacote capturado.
func (t *DNSTracker) Add(p gopacket.Packet) {
	udp, ok := p.Layer(layers.LayerTypeUDP).(*layers.UDP)
	if !ok || len(t.ports) == 0 {
		return
	}
	src, dst := endpoints(p)
	if src == nil {
		return
	}
	toServer := slices.Contains(t.ports, int(udp.DstPort))
	fromServer := slices.Contains(t.ports, int(udp.SrcPort))
	if !toServer && !fromServer {
		return
	}
	var msg layers.DNS
	if err := msg.DecodeFromBytes(udp.Payload, gopacket.NilDecodeFeedback); err != nil {
		return
	}
	at := p.Metadata().Timestamp
	switch {
	case toServer && !msg.QR:
		t.query(dnsKey{client: net.JoinHostPort(src.String(), strconv.Itoa(int(udp.SrcPort))), id: msg.ID}, at)
	case fromServer && msg.QR:
		key := dnsKey{client: net.JoinHostPort(dst.String(), strconv.Itoa(int(udp.DstPort))), id: msg.ID}
		sent, ok := t.pending[key]
		if !ok {
			return
		}
		delete(t.pending, key)
		rcode := rcodeName(msg.ResponseCode)
		t.emit(DNSExchange{RCode: rcode, Failed: msg.ResponseCode != layers.DNSResponseCodeNoErr,
			Latency: max(at.Sub(sent), 0), At: sent.UTC()})
	}
}

func (t *DNSTracker) query(key dnsKey, at time.Time) {
	if _, dup := t.pending[key]; !dup {
		for len(t.pending) >= maxDNSPending {
			t.expireOldest()
		}
		t.order = append(t.order, key)
	}
	t.pending[key] = at
}

// expireOldest emite como TIMEOUT a consulta pendente mais antiga.
func (t *DNSTracker) expireOldest() {
	for len(t.order) > 0 {
		key := t.order[0]
		t.order = t.order[1:]
		if sent, ok := t.pending[key]; ok {
			delete(t.pending, key)
			t.emit(DNSExchange{RCode: rcodeTimeout, Failed: true, At: sent.UTC()})
			return
		}
	}
}

// Flush emite como TIMEOUT as consultas sem resposta há dnsTimeout ou mais,
// contando a partir de now, e descarta as chaves já respondidas.
func (t *DNSTracker) Flush(now time.Time) {
	kept := t.order[:0]
	for _, key := range t.order {
		sent, ok := t.pending[key]
		if !ok {
			continue
		}
		if now.Sub(sent) >= dnsTimeout {
			delete(t.pending, key)
			t.emit(DNSExchange{RCode: rcodeTimeout, Failed: true, At: sent.UTC()})
			continue
		}
		kept = append(kept, key)
	}
	t.order = kept
}

// endpoints devolve os IPs de origem e destino do pacote, ou nil sem IP.
func endpoints(p gopacket.Packet) (src, dst net.IP) {
	switch ip := p.NetworkLayer().(type) {
	case *layers.IPv4:
		return ip.SrcIP, ip.DstIP
	case *layers.IPv6:
		return ip.SrcIP, ip.DstIP
	}
	return nil, nil
}

// rcodeName devolve o mnemônico do RCODE (RFC 1035 e 6895), como "NXDOMAIN".
func rcodeName(c layers.DNSResponseCode) string {
	switch c {
	case layers.DNSResponseCodeNoErr:
		return "NOERROR"
	case layers.DNSResponseCodeFormErr:
		return "FORMERR"
	case layers.DNSResponseCodeServFail:
		return "SERVFAIL"
	case layers.DNSResponseCodeNXDomain:
		return "NXDOMAIN"
	case layers.DNSResponseCodeNotImp:
		return "NOTIMP"
	case layers.DNSResponseCodeRefused:
		return "REFUSED"
	default:
		return "RCODE" + strconv.Itoa(int(c))
	}
}
