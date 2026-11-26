package sniffer

import (
	"encoding/binary"
	"testing"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/stretchr/testify/assert"

	"github.com/MiguelDev-GH/omni/internal/sniffer/testutil"
	"github.com/MiguelDev-GH/omni/internal/ws"
)

func TestSummaryNeverHasHeaders(t *testing.T) {
	body := "POST /login?next=/conta HTTP/1.1\r\nHost: loja\r\nAuthorization: Bearer x\r\n" +
		"Content-Type: application/json\r\n\r\n{\"senha\":\"segredo\"}"
	p := decode(testutil.TCP("10.0.0.1", "10.0.0.2", 40000, 3000, "PA", []byte(body)))

	s := Summarize(p, PortRoles{App: []int{3000}}, nil)

	assert.Equal(t, "POST /login", s.Info)
	assert.Equal(t, "HTTP", s.Protocol)
	assert.Equal(t, "10.0.0.1", s.Src)
	assert.Equal(t, "10.0.0.2", s.Dst)
	assert.Equal(t, 40000, s.SrcPort)
	assert.Equal(t, 3000, s.DstPort)
	assert.Equal(t, len(p.Data()), s.Length)
	for _, f := range []string{s.Info, s.Flags, s.Protocol} {
		assert.NotContains(t, f, "Bearer")
		assert.NotContains(t, f, "segredo")
		assert.NotContains(t, f, "next")
	}
}

func TestSummaryHTTPRouteAndStatus(t *testing.T) {
	app := PortRoles{App: []int{3000}}
	req := decode(testutil.TCP("10.0.0.1", "10.0.0.2", 40000, 3000, "PA", []byte("GET /produtos/42 HTTP/1.1\r\n\r\n")))
	assert.Equal(t, "GET /produtos/:id", Summarize(req, app, nil).Info)

	resp := decode(testutil.TCP("10.0.0.2", "10.0.0.1", 3000, 40000, "PA", []byte("HTTP/1.1 404 Not Found\r\nSet-Cookie: s=1\r\n\r\n")))
	assert.Equal(t, "HTTP 404", Summarize(resp, app, nil).Info)

	cont := decode(testutil.TCP("10.0.0.1", "10.0.0.2", 40000, 3000, "PA", []byte("Authorization: Bearer x\r\n\r\n")))
	assert.Empty(t, Summarize(cont, app, nil).Info)
}

func TestSummaryTCPFlags(t *testing.T) {
	for flags, want := range map[string]string{"S": "SYN", "SA": "SYN,ACK", "R": "RST", "PA": "PSH,ACK", "FA": "FIN,ACK"} {
		p := decode(testutil.TCP("10.0.0.1", "10.0.0.2", 40000, 3000, flags, nil))
		assert.Equal(t, want, Summarize(p, PortRoles{App: []int{3000}}, nil).Flags, flags)
	}
}

func TestSummaryTCPSequence(t *testing.T) {
	p := decode(testutil.TCPSeq("10.0.0.1", "10.0.0.2", 40000, 3000, "PA", 1000, 7000, []byte("GET / HTTP/1.1\r\n\r\n")))
	s := Summarize(p, PortRoles{App: []int{3000}}, nil)
	assert.Equal(t, uint32(1000), s.Seq)
	assert.Equal(t, uint32(7000), s.Ack)
	assert.Equal(t, 18, s.Payload)
}

func TestSummaryPayloadFromIPHeaderWhenTruncated(t *testing.T) {
	frame := testutil.TCPSeq("10.0.0.1", "10.0.0.2", 3000, 40000, "PA", 1, 1, make([]byte, 5000))
	p := gopacket.NewPacket(frame[:1600], layers.LinkTypeEthernet, gopacket.Default)
	assert.Equal(t, 5000, Summarize(p, PortRoles{App: []int{3000}}, nil).Payload,
		"a carga sai do cabeçalho IP, não dos bytes capturados")
}

func TestSummaryUDPWithoutKnownPort(t *testing.T) {
	p := decode(testutil.UDP("10.0.0.1", "10.0.0.2", 5000, 6000, []byte("x")))
	s := Summarize(p, PortRoles{}, nil)
	assert.Equal(t, "UDP", s.Protocol)
	assert.Empty(t, s.Flags)
	assert.Equal(t, 6000, s.DstPort)
}

func TestSummaryPostgresMessageType(t *testing.T) {
	q := []byte{'Q', 0, 0, 0, 0}
	q = append(q, []byte("SELECT senha FROM usuarios\x00")...)
	binary.BigEndian.PutUint32(q[1:], uint32(len(q)-1))
	p := decode(testutil.TCP("10.0.0.1", "10.0.0.2", 40000, 15432, "PA", q))

	s := Summarize(p, PortRoles{}, []int{15432})
	assert.Equal(t, "PostgreSQL", s.Protocol)
	assert.Equal(t, "Query", s.Info)

	ready := []byte{'Z', 0, 0, 0, 5, 'I'}
	p = decode(testutil.TCP("10.0.0.2", "10.0.0.1", 15432, 40000, "PA", ready))
	assert.Equal(t, "ReadyForQuery", Summarize(p, PortRoles{}, []int{15432}).Info)

	ssl := make([]byte, 8)
	binary.BigEndian.PutUint32(ssl, 8)
	binary.BigEndian.PutUint32(ssl[4:], 80877103)
	p = decode(testutil.TCP("10.0.0.1", "10.0.0.2", 40000, 15432, "PA", ssl))
	assert.Equal(t, "SSLRequest", Summarize(p, PortRoles{}, []int{15432}).Info)
}

func TestSummaryMatchesFilter(t *testing.T) {
	s := PacketSummary{Src: "10.0.0.1", Dst: "10.0.0.2", SrcPort: 40000, DstPort: 3000, Protocol: "HTTP"}
	assert.True(t, s.MatchesFilter(ws.InspectorFilter{}))
	assert.True(t, s.MatchesFilter(ws.InspectorFilter{IP: "10.0.0.2", Port: 3000, Protocol: "http"}))
	assert.False(t, s.MatchesFilter(ws.InspectorFilter{IP: "10.0.0.9"}))
	assert.False(t, s.MatchesFilter(ws.InspectorFilter{Port: 5432}))
	assert.False(t, s.MatchesFilter(ws.InspectorFilter{Protocol: "UDP"}))
}
