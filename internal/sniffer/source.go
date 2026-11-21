// Package sniffer observa passivamente o tráfego das portas dos alvos.
package sniffer

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/gopacket/gopacket"

	"github.com/MiguelDev-GH/omni/internal/target"
)

// ErrSemPermissao indica que o processo não pode capturar pacotes (falta CAP_NET_RAW).
var ErrSemPermissao = errors.New("sem permissão de captura")

// Source entrega pacotes até ctx ser cancelado ou a fonte acabar; o canal é
// fechado nos dois casos. Packets deve ser chamado uma única vez.
type Source interface {
	Packets(ctx context.Context) <-chan gopacket.Packet
	Stats() CaptureStats
	Close() error
}

// CaptureStats conta os pacotes vistos pela captura desde a abertura.
// Dropped maior que zero indica métricas incompletas.
type CaptureStats struct {
	Received, Dropped uint64
}

// BPFForTargets monta o filtro de captura com as portas TCP de todos os alvos,
// sem repetição e em ordem crescente. Sem portas, devolve "".
func BPFForTargets(ts []target.Target) string {
	ports := TargetPorts(ts)
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = fmt.Sprintf("tcp port %d", p)
	}
	return strings.Join(parts, " or ")
}

// TargetPorts devolve as portas de aplicação e de banco de todos os alvos,
// sem repetição e em ordem crescente.
func TargetPorts(ts []target.Target) []int {
	seen := map[int]bool{}
	var ports []int
	add := func(p int) {
		if p > 0 && !seen[p] {
			seen[p] = true
			ports = append(ports, p)
		}
	}
	for _, t := range ts {
		for _, p := range t.AppPorts {
			add(p)
		}
		add(t.DBPort)
	}
	sort.Ints(ports)
	return ports
}

// packetSource adapta um gopacket.PacketSource à interface Source.
type packetSource struct {
	src   *gopacket.PacketSource
	stats func() CaptureStats
	close func() error
}

func (s *packetSource) Packets(ctx context.Context) <-chan gopacket.Packet {
	return s.src.PacketsCtx(ctx)
}
func (s *packetSource) Stats() CaptureStats { return s.stats() }
func (s *packetSource) Close() error        { return s.close() }
