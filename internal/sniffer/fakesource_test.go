package sniffer

import (
	"context"

	"github.com/gopacket/gopacket"
)

// blockingSource não entrega pacotes; o canal só fecha quando ctx acaba.
type blockingSource struct{}

func newBlockingSource() *blockingSource { return &blockingSource{} }

func (*blockingSource) Packets(ctx context.Context) <-chan gopacket.Packet {
	c := make(chan gopacket.Packet)
	go func() {
		<-ctx.Done()
		close(c)
	}()
	return c
}
func (*blockingSource) Stats() CaptureStats { return CaptureStats{} }
func (*blockingSource) Close() error        { return nil }
