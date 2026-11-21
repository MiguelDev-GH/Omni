// Package testutil monta pacotes e arquivos .pcap para os testes do sniffer.
package testutil

import (
	"encoding/binary"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"
)

// Start é o horário do primeiro pacote gravado por WritePCAP; os seguintes
// avançam 1 ms cada.
var Start = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

// WritePCAP grava pkts (quadros Ethernet) num arquivo .pcap.
func WritePCAP(t testing.TB, path string, pkts ...[]byte) {
	t.Helper()
	WritePCAPLink(t, path, layers.LinkTypeEthernet, pkts...)
}

// WritePCAPLink grava pkts num arquivo .pcap com o tipo de enlace dado.
func WritePCAPLink(t testing.TB, path string, link layers.LinkType, pkts ...[]byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	w := pcapgo.NewWriter(f)
	if err := w.WriteFileHeader(65535, link); err != nil {
		t.Fatal(err)
	}
	for i, p := range pkts {
		ci := gopacket.CaptureInfo{Timestamp: Start.Add(time.Duration(i) * time.Millisecond), CaptureLength: len(p), Length: len(p)}
		if err := w.WritePacket(ci, p); err != nil {
			t.Fatal(err)
		}
	}
}

// TCP monta um quadro Ethernet com IPv4 ou IPv6 (conforme src) e TCP. flags
// aceita as letras S, A, F, R e P, como em "SA".
func TCP(src, dst string, sport, dport uint16, flags string, payload []byte) []byte {
	return TCPSeq(src, dst, sport, dport, flags, 0, 0, payload)
}

// TCPSeq é como TCP, com os números de sequência e de confirmação.
func TCPSeq(src, dst string, sport, dport uint16, flags string, seq, ack uint32, payload []byte) []byte {
	tcp := &layers.TCP{
		SrcPort: layers.TCPPort(sport), DstPort: layers.TCPPort(dport), Window: 65535, Seq: seq, Ack: ack,
		SYN: strings.Contains(flags, "S"), ACK: strings.Contains(flags, "A"),
		FIN: strings.Contains(flags, "F"), RST: strings.Contains(flags, "R"), PSH: strings.Contains(flags, "P"),
	}
	return build(src, dst, layers.IPProtocolTCP, tcp, payload)
}

// UDP monta um quadro Ethernet com IPv4 ou IPv6 (conforme src) e UDP.
func UDP(src, dst string, sport, dport uint16, payload []byte) []byte {
	udp := &layers.UDP{SrcPort: layers.UDPPort(sport), DstPort: layers.UDPPort(dport)}
	return build(src, dst, layers.IPProtocolUDP, udp, payload)
}

// ToSLL troca o cabeçalho Ethernet de frame pelo cabeçalho LINUX_SLL, usado
// nas capturas na interface "any".
func ToSLL(frame []byte) []byte {
	sll := make([]byte, 16, 16+len(frame)-14)
	binary.BigEndian.PutUint16(sll[0:], 0) // pacote destinado a este host
	binary.BigEndian.PutUint16(sll[2:], 1) // ARPHRD_ETHER
	binary.BigEndian.PutUint16(sll[4:], 6) // tamanho do endereço
	copy(sll[6:], frame[6:12])             // endereço de origem
	copy(sll[14:], frame[12:14])           // EtherType
	return append(sll, frame[14:]...)
}

type transport interface {
	gopacket.SerializableLayer
	SetNetworkLayerForChecksum(gopacket.NetworkLayer) error
}

func build(src, dst string, proto layers.IPProtocol, tr transport, payload []byte) []byte {
	srcIP, dstIP := net.ParseIP(src), net.ParseIP(dst)
	eth := &layers.Ethernet{
		SrcMAC: net.HardwareAddr{0, 1, 2, 3, 4, 5}, DstMAC: net.HardwareAddr{0, 1, 2, 3, 4, 6},
	}
	var ip gopacket.SerializableLayer
	if v4 := srcIP.To4(); v4 != nil {
		eth.EthernetType = layers.EthernetTypeIPv4
		l := &layers.IPv4{Version: 4, TTL: 64, Protocol: proto, SrcIP: v4, DstIP: dstIP.To4()}
		_ = tr.SetNetworkLayerForChecksum(l)
		ip = l
	} else {
		eth.EthernetType = layers.EthernetTypeIPv6
		l := &layers.IPv6{Version: 6, HopLimit: 64, NextHeader: proto, SrcIP: srcIP, DstIP: dstIP}
		_ = tr.SetNetworkLayerForChecksum(l)
		ip = l
	}
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	if err := gopacket.SerializeLayers(buf, opts, eth, ip, tr, gopacket.Payload(payload)); err != nil {
		panic(err)
	}
	return buf.Bytes()
}
