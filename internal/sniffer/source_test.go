package sniffer

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gopacket/gopacket/layers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/sniffer/testutil"
	"github.com/MiguelDev-GH/omni/internal/target"
)

func collect(t *testing.T, s Source) []int {
	t.Helper()
	var sizes []int
	for p := range s.Packets(context.Background()) {
		sizes = append(sizes, len(p.Data()))
	}
	return sizes
}

func TestOpenFileReadsAllPackets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.pcap")
	testutil.WritePCAP(t, path,
		testutil.TCP("10.0.0.1", "10.0.0.2", 40000, 3000, "S", nil),
		testutil.TCP("10.0.0.2", "10.0.0.1", 3000, 40000, "SA", nil),
		testutil.UDP("10.0.0.1", "10.0.0.53", 5353, 53, []byte("q")),
	)
	s, err := OpenFile(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	assert.Len(t, collect(t, s), 3)
	assert.Equal(t, CaptureStats{Received: 3}, s.Stats())
}

func TestOpenFileMissing(t *testing.T) {
	_, err := OpenFile(filepath.Join(t.TempDir(), "nao-existe.pcap"))
	assert.Error(t, err)
}

func TestOpenFileDecodesLinuxSLL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sll.pcap")
	testutil.WritePCAPLink(t, path, layers.LinkTypeLinuxSLL,
		testutil.ToSLL(testutil.TCP("127.0.0.1", "127.0.0.1", 40000, 3000, "PA", []byte("GET / HTTP/1.1\r\n\r\n"))))
	s, err := OpenFile(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	var n int
	for p := range s.Packets(context.Background()) {
		n++
		tcp, ok := p.Layer(layers.LayerTypeTCP).(*layers.TCP)
		require.True(t, ok, "pacote sem camada TCP: %v", p)
		assert.Equal(t, layers.TCPPort(3000), tcp.DstPort)
		assert.Equal(t, "GET / HTTP/1.1\r\n\r\n", string(tcp.Payload))
	}
	assert.Equal(t, 1, n)
}

func TestBPFForTargets(t *testing.T) {
	ts := []target.Target{
		{AppPorts: []int{3000, 8080}, DBPort: 5432},
		{AppPorts: []int{3000}},
	}
	assert.Equal(t, "tcp port 3000 or tcp port 5432 or tcp port 8080", BPFForTargets(ts))
	assert.Equal(t, "", BPFForTargets(nil))
}
