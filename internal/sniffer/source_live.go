//go:build cgo

package sniffer

import (
	"fmt"
	"strings"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/pcap"
)

const (
	liveSnaplen = 1600
	// liveTimeout limita quanto uma leitura espera, para a captura notar o cancelamento.
	liveTimeout = 500 * time.Millisecond
)

// OpenLive captura ao vivo em iface ("any" para todas) com o filtro BPF dado.
// Falta de permissão devolve um erro que envolve ErrSemPermissao.
func OpenLive(iface string, bpf string) (Source, error) {
	h, err := pcap.OpenLive(iface, liveSnaplen, false, liveTimeout)
	if err != nil {
		if isPermissionError(err) {
			return nil, fmt.Errorf("%w: %v", ErrSemPermissao, err)
		}
		return nil, fmt.Errorf("abrir captura em %s: %w", iface, err)
	}
	if err := h.SetBPFFilter(bpf); err != nil {
		h.Close()
		return nil, fmt.Errorf("filtro de captura %q: %w", bpf, err)
	}
	return &packetSource{
		src: gopacket.NewPacketSource(h, h.LinkType()),
		stats: func() CaptureStats {
			s, err := h.Stats()
			if err != nil {
				return CaptureStats{}
			}
			return CaptureStats{Received: uint64(s.PacketsReceived), Dropped: uint64(s.PacketsDropped + s.PacketsIfDropped)}
		},
		close: func() error { h.Close(); return nil },
	}, nil
}

func isPermissionError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "permission") || strings.Contains(msg, "not permitted")
}
