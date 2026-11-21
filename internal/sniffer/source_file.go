package sniffer

import (
	"os"
	"sync/atomic"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/pcapgo"
)

// OpenFile lê pacotes de um arquivo .pcap, para testes e análise offline.
func OpenFile(path string) (Source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	r, err := pcapgo.NewReader(f)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	counted := &countingReader{r: r}
	src := gopacket.NewPacketSource(counted, r.LinkType())
	return &packetSource{
		src:   src,
		stats: func() CaptureStats { return CaptureStats{Received: counted.n.Load()} },
		close: f.Close,
	}, nil
}

// countingReader conta os pacotes lidos, já que um arquivo não tem estatísticas de captura.
type countingReader struct {
	r *pcapgo.Reader
	n atomic.Uint64
}

func (c *countingReader) ReadPacketData() ([]byte, gopacket.CaptureInfo, error) {
	data, ci, err := c.r.ReadPacketData()
	if err == nil {
		c.n.Add(1)
	}
	return data, ci, err
}
