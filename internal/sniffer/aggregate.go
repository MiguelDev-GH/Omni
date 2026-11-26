package sniffer

import (
	"net"
	"sort"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
)

// maxTopSources limita a lista de IPs de origem de cada janela.
const maxTopSources = 10

// Window resume o tráfego de um intervalo, normalmente um segundo.
type Window struct {
	Start      time.Time         `json:"inicio"`
	Bytes      uint64            `json:"bytes"`
	Packets    uint64            `json:"pacotes"`
	Dropped    uint64            `json:"descartados"`
	ByProtocol map[string]uint64 `json:"protocolos"`
	TopSources []SourceCount     `json:"top_origens"`
}

// SourceCount é o volume enviado por um IP de origem.
type SourceCount struct {
	IP      string `json:"ip"`
	Bytes   uint64 `json:"bytes"`
	Packets uint64 `json:"pacotes"`
}

// Aggregator soma os pacotes da janela atual. Não é seguro para uso
// concorrente; o motor o usa de uma única goroutine.
type Aggregator struct {
	roles      PortRoles
	start      time.Time
	bytes      uint64
	packets    uint64
	byProtocol map[string]uint64
	sources    map[string]*SourceCount
}

// NewAggregator cria um agregador que classifica as portas dos alvos pelo
// papel de cada uma (veja Classify).
func NewAggregator(roles PortRoles) *Aggregator {
	a := &Aggregator{roles: roles}
	a.reset(time.Time{})
	return a
}

func (a *Aggregator) reset(start time.Time) {
	a.start = start
	a.bytes, a.packets = 0, 0
	a.byProtocol = map[string]uint64{}
	a.sources = map[string]*SourceCount{}
}

// Add conta p na janela atual. Pacotes sem camada IP são ignorados.
func (a *Aggregator) Add(p gopacket.Packet) {
	var src net.IP
	switch ip := p.NetworkLayer().(type) {
	case *layers.IPv4:
		src = ip.SrcIP
	case *layers.IPv6:
		src = ip.SrcIP
	default:
		return
	}
	size := uint64(p.Metadata().Length)
	if size == 0 {
		size = uint64(len(p.Data()))
	}

	proto := "IP"
	switch tr := p.TransportLayer().(type) {
	case *layers.TCP:
		proto = Classify(uint16(tr.SrcPort), uint16(tr.DstPort), false, a.roles)
	case *layers.UDP:
		proto = Classify(uint16(tr.SrcPort), uint16(tr.DstPort), true, a.roles)
	}

	a.bytes += size
	a.packets++
	a.byProtocol[proto] += size
	key := src.String()
	sc := a.sources[key]
	if sc == nil {
		sc = &SourceCount{IP: key}
		a.sources[key] = sc
	}
	sc.Bytes += size
	sc.Packets++
}

// Flush devolve a janela iniciada no Flush anterior e começa outra em now.
// A primeira janela começa em now menos um segundo.
func (a *Aggregator) Flush(now time.Time) Window {
	start := a.start
	if start.IsZero() {
		start = now.Add(-time.Second)
	}
	w := Window{
		Start:      start.UTC(),
		Bytes:      a.bytes,
		Packets:    a.packets,
		ByProtocol: a.byProtocol,
		TopSources: make([]SourceCount, 0, len(a.sources)),
	}
	for _, sc := range a.sources {
		w.TopSources = append(w.TopSources, *sc)
	}
	sort.Slice(w.TopSources, func(i, j int) bool {
		if w.TopSources[i].Bytes != w.TopSources[j].Bytes {
			return w.TopSources[i].Bytes > w.TopSources[j].Bytes
		}
		return w.TopSources[i].IP < w.TopSources[j].IP
	})
	if len(w.TopSources) > maxTopSources {
		w.TopSources = w.TopSources[:maxTopSources]
	}
	a.reset(now)
	return w
}
