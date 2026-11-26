// Package l7 decodifica protocolos de aplicação (HTTP/1.1 e PostgreSQL) a
// partir dos fluxos TCP remontados pelo sniffer.
package l7

import (
	"slices"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/reassembly"
)

const (
	// pageSize é o tamanho das páginas que o reassembly usa para guardar
	// segmentos fora de ordem.
	pageSize = 1900
	// maxPagesPerConn limita a 4 MiB o que fica em buffer por conexão.
	maxPagesPerConn = (4 << 20) / pageSize
	// maxPagesTotal limita a 64 MiB o buffer somando todas as conexões.
	maxPagesTotal = (64 << 20) / pageSize
	// IdleTimeout é quanto tempo um fluxo sem pacotes fica aberto.
	IdleTimeout = 2 * time.Minute
	// gapWait é quanto o assembler espera um segmento perdido antes de pular.
	gapWait = 5 * time.Second
)

// connParser interpreta os bytes remontados de uma conexão TCP.
type connParser interface {
	// data recebe bytes em ordem de um dos sentidos; at é o horário do
	// primeiro pacote do trecho.
	data(fromClient bool, b []byte, at time.Time)
	// lost avisa que bytes se perderam e o estado parcial não vale mais.
	lost()
}

// streamAssembler remonta as conexões TCP de portas de servidor conhecidas e
// entrega os bytes de cada uma a um connParser. Não é seguro para uso
// concorrente.
type streamAssembler struct {
	asm       *reassembly.Assembler
	ports     []int
	newParser func(clientIP string) connParser
}

func newStreamAssembler(serverPorts []int, newParser func(clientIP string) connParser) *streamAssembler {
	sa := &streamAssembler{ports: slices.Clone(serverPorts), newParser: newParser}
	sa.asm = reassembly.NewAssembler(reassembly.NewStreamPool(sa))
	sa.asm.MaxBufferedPagesPerConnection = maxPagesPerConn
	sa.asm.MaxBufferedPagesTotal = maxPagesTotal
	return sa
}

func (sa *streamAssembler) isServer(port layers.TCPPort) bool {
	return slices.Contains(sa.ports, int(port))
}

// add entrega p ao reassembly se for TCP de ou para uma porta de servidor.
func (sa *streamAssembler) add(p gopacket.Packet) {
	tcp, ok := p.TransportLayer().(*layers.TCP)
	net := p.NetworkLayer()
	if !ok || net == nil || (!sa.isServer(tcp.SrcPort) && !sa.isServer(tcp.DstPort)) {
		return
	}
	sa.asm.AssembleWithContext(net.NetworkFlow(), tcp, &captureContext{p.Metadata().CaptureInfo})
}

// flush libera os segmentos que esperam um buraco há mais de gapWait e fecha
// as conexões sem pacotes há mais de IdleTimeout, contando a partir de now.
func (sa *streamAssembler) flush(now time.Time) {
	sa.asm.FlushWithOptions(reassembly.FlushOptions{T: now.Add(-gapWait), TC: now.Add(-IdleTimeout)})
}

// New cria o stream de uma conexão nova (reassembly.StreamFactory).
func (sa *streamAssembler) New(netFlow, _ gopacket.Flow, tcp *layers.TCP, _ reassembly.AssemblerContext) reassembly.Stream {
	// O sentido "cliente para servidor" do reassembly é o do primeiro pacote
	// visto, que pode ser a resposta do servidor numa captura iniciada no meio.
	firstIsClient := sa.isServer(tcp.DstPort) || !sa.isServer(tcp.SrcPort)
	clientIP := netFlow.Src().String()
	if !firstIsClient {
		clientIP = netFlow.Dst().String()
	}
	return &stream{firstIsClient: firstIsClient, p: sa.newParser(clientIP)}
}

type stream struct {
	firstIsClient bool
	p             connParser
}

// Accept aceita todos os pacotes e força o início do fluxo mesmo sem SYN, para
// acompanhar conexões abertas antes da captura.
func (s *stream) Accept(_ *layers.TCP, _ gopacket.CaptureInfo, _ reassembly.TCPFlowDirection, _ reassembly.Sequence, start *bool, _ reassembly.AssemblerContext) bool {
	*start = true
	return true
}

func (s *stream) ReassembledSG(sg reassembly.ScatterGather, _ reassembly.AssemblerContext) {
	dir, _, _, skip := sg.Info()
	if skip != 0 {
		s.p.lost()
	}
	n, _ := sg.Lengths()
	if n == 0 {
		return
	}
	fromClient := (dir == reassembly.TCPDirClientToServer) == s.firstIsClient
	s.p.data(fromClient, sg.Fetch(n), sg.CaptureInfo(0).Timestamp)
}

// closer é implementado pelos parsers que precisam saber quando a conexão
// terminou (FIN, RST ou fechada por ociosidade).
type closer interface{ closed() }

func (s *stream) ReassemblyComplete(reassembly.AssemblerContext) bool {
	if c, ok := s.p.(closer); ok {
		c.closed()
	}
	return true
}

type captureContext struct{ ci gopacket.CaptureInfo }

func (c *captureContext) GetCaptureInfo() gopacket.CaptureInfo { return c.ci }
