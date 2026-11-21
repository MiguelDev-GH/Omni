package testutil

import (
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
)

// Conn monta os quadros de uma conexão TCP entre um cliente e um servidor,
// mantendo os números de sequência de cada lado.
type Conn struct {
	Client, Server         string
	ClientPort, ServerPort uint16
	cseq, sseq             uint32
}

// NewConn cria uma conexão com números de sequência iniciais fixos.
func NewConn(client string, cport uint16, server string, sport uint16) *Conn {
	return &Conn{Client: client, Server: server, ClientPort: cport, ServerPort: sport, cseq: 1000, sseq: 5000}
}

// Handshake devolve SYN, SYN-ACK e ACK.
func (c *Conn) Handshake() [][]byte {
	syn := TCPSeq(c.Client, c.Server, c.ClientPort, c.ServerPort, "S", c.cseq, 0, nil)
	synAck := TCPSeq(c.Server, c.Client, c.ServerPort, c.ClientPort, "SA", c.sseq, c.cseq+1, nil)
	c.cseq++
	c.sseq++
	ack := TCPSeq(c.Client, c.Server, c.ClientPort, c.ServerPort, "A", c.cseq, c.sseq, nil)
	return [][]byte{syn, synAck, ack}
}

// FromClient devolve um segmento do cliente com payload e avança a sequência.
func (c *Conn) FromClient(payload []byte) []byte {
	f := TCPSeq(c.Client, c.Server, c.ClientPort, c.ServerPort, "PA", c.cseq, c.sseq, payload)
	c.cseq += uint32(len(payload))
	return f
}

// FromServer devolve um segmento do servidor com payload e avança a sequência.
func (c *Conn) FromServer(payload []byte) []byte {
	f := TCPSeq(c.Server, c.Client, c.ServerPort, c.ClientPort, "PA", c.sseq, c.cseq, payload)
	c.sseq += uint32(len(payload))
	return f
}

// Packet decodifica um quadro Ethernet com o horário de captura at.
func Packet(frame []byte, at time.Time) gopacket.Packet {
	p := gopacket.NewPacket(frame, layers.LayerTypeEthernet, gopacket.Default)
	md := p.Metadata()
	md.Timestamp = at
	md.CaptureLength = len(frame)
	md.Length = len(frame)
	return p
}
