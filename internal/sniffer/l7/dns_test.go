package l7

import (
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/sniffer/testutil"
)

// dnsMsg serializa uma pergunta (resp false) ou resposta DNS do tipo A.
func dnsMsg(t *testing.T, id uint16, resp bool, rcode layers.DNSResponseCode) []byte {
	t.Helper()
	d := &layers.DNS{
		ID: id, QR: resp, OpCode: layers.DNSOpCodeQuery, RD: true, RA: resp, ResponseCode: rcode,
		Questions: []layers.DNSQuestion{{Name: []byte("loja.local"), Type: layers.DNSTypeA, Class: layers.DNSClassIN}},
	}
	if resp && rcode == layers.DNSResponseCodeNoErr {
		d.Answers = []layers.DNSResourceRecord{{
			Name: []byte("loja.local"), Type: layers.DNSTypeA, Class: layers.DNSClassIN, TTL: 60, IP: []byte{127, 0, 0, 1},
		}}
	}
	buf := gopacket.NewSerializeBuffer()
	require.NoError(t, d.SerializeTo(buf, gopacket.SerializeOptions{FixLengths: true}))
	return buf.Bytes()
}

const dnsPort = 1053

func query(t *testing.T, cport, id uint16) []byte {
	return testutil.UDP("127.0.0.1", "127.0.0.1", cport, dnsPort, dnsMsg(t, id, false, 0))
}

func answer(t *testing.T, cport, id uint16, rcode layers.DNSResponseCode) []byte {
	return testutil.UDP("127.0.0.1", "127.0.0.1", dnsPort, cport, dnsMsg(t, id, true, rcode))
}

func newTracker() (*DNSTracker, *[]DNSExchange) {
	var got []DNSExchange
	return NewDNSTracker([]int{dnsPort}, func(ex DNSExchange) { got = append(got, ex) }), &got
}

func TestDNSTrackerPairsQueryAndResponse(t *testing.T) {
	tr, got := newTracker()
	tr.Add(testutil.Packet(query(t, 40000, 7), t0))
	tr.Add(testutil.Packet(answer(t, 40000, 7, layers.DNSResponseCodeNoErr), t0.Add(3*time.Millisecond)))
	require.Len(t, *got, 1)
	ex := (*got)[0]
	assert.Equal(t, "NOERROR", ex.RCode)
	assert.False(t, ex.Failed)
	assert.Equal(t, 3*time.Millisecond, ex.Latency)
	assert.Equal(t, t0, ex.At)
}

func TestDNSTrackerNXDOMAINIsFailure(t *testing.T) {
	tr, got := newTracker()
	tr.Add(testutil.Packet(query(t, 40000, 9), t0))
	tr.Add(testutil.Packet(answer(t, 40000, 9, layers.DNSResponseCodeNXDomain), t0.Add(time.Millisecond)))
	require.Len(t, *got, 1)
	assert.Equal(t, "NXDOMAIN", (*got)[0].RCode)
	assert.True(t, (*got)[0].Failed)
}

func TestDNSTrackerSameIDFromDifferentClients(t *testing.T) {
	tr, got := newTracker()
	tr.Add(testutil.Packet(query(t, 40000, 5), t0))
	tr.Add(testutil.Packet(query(t, 40001, 5), t0.Add(time.Millisecond)))
	tr.Add(testutil.Packet(answer(t, 40001, 5, layers.DNSResponseCodeNoErr), t0.Add(2*time.Millisecond)))
	tr.Add(testutil.Packet(answer(t, 40000, 5, layers.DNSResponseCodeNoErr), t0.Add(4*time.Millisecond)))
	require.Len(t, *got, 2)
	assert.Equal(t, time.Millisecond, (*got)[0].Latency)
	assert.Equal(t, 4*time.Millisecond, (*got)[1].Latency)
}

func TestDNSTrackerTimeoutOnFlush(t *testing.T) {
	tr, got := newTracker()
	tr.Add(testutil.Packet(query(t, 40000, 1), t0))
	tr.Add(testutil.Packet(query(t, 40000, 2), t0.Add(4*time.Second)))
	tr.Flush(t0.Add(6 * time.Second))
	require.Len(t, *got, 1)
	assert.Equal(t, "TIMEOUT", (*got)[0].RCode)
	assert.True(t, (*got)[0].Failed)

	// A resposta atrasada da consulta que já expirou é ignorada.
	tr.Add(testutil.Packet(answer(t, 40000, 1, layers.DNSResponseCodeNoErr), t0.Add(7*time.Second)))
	assert.Len(t, *got, 1)
}

func TestDNSTrackerIgnoresOtherPortsAndGarbage(t *testing.T) {
	tr, got := newTracker()
	assert.NotPanics(t, func() {
		tr.Add(testutil.Packet(testutil.UDP("127.0.0.1", "127.0.0.1", 40000, dnsPort, []byte{0xde, 0xad}), t0))
		tr.Add(testutil.Packet(testutil.UDP("127.0.0.1", "127.0.0.1", 40000, 9999, dnsMsg(t, 3, false, 0)), t0))
		tr.Add(testutil.Packet(testutil.TCP("127.0.0.1", "127.0.0.1", 40000, dnsPort, "PA", []byte("x")), t0))
		tr.Add(testutil.Packet(answer(t, 40000, 77, layers.DNSResponseCodeNoErr), t0)) // resposta sem pergunta
	})
	tr.Flush(t0.Add(time.Minute))
	assert.Empty(t, *got)
}

func TestDNSTrackerBoundsPending(t *testing.T) {
	tr, got := newTracker()
	for i := range maxDNSPending + 1 {
		tr.Add(testutil.Packet(query(t, uint16(10000+i%50000), uint16(i)), t0.Add(time.Duration(i)*time.Microsecond)))
	}
	assert.Len(t, tr.pending, maxDNSPending)
	require.Len(t, *got, 1)
	assert.Equal(t, "TIMEOUT", (*got)[0].RCode)
	assert.Equal(t, t0, (*got)[0].At, "a mais antiga sai primeiro")
}

func TestDNSStatsWindow(t *testing.T) {
	s := NewDNSStats()
	s.Flush(t0)
	for i := 1; i <= 10; i++ {
		s.Add(DNSExchange{RCode: "NOERROR", Latency: time.Duration(i) * time.Millisecond})
	}
	s.Add(DNSExchange{RCode: "NXDOMAIN", Failed: true, Latency: time.Millisecond})
	s.Add(DNSExchange{RCode: "TIMEOUT", Failed: true})
	w := s.Flush(t0.Add(10 * time.Second))
	assert.Equal(t, t0, w.Start)
	assert.Equal(t, uint64(12), w.Queries)
	assert.Equal(t, uint64(2), w.Failures)
	assert.Equal(t, map[string]uint64{"NOERROR": 10, "NXDOMAIN": 1, "TIMEOUT": 1}, w.ByRCode)
	assert.Equal(t, 5.0, w.P50MS, "o p50 só conta as respostas, não os timeouts")
	assert.Equal(t, 10.0, w.P95MS)

	next := s.Flush(t0.Add(20 * time.Second))
	assert.Equal(t, uint64(0), next.Queries)
	assert.Equal(t, map[string]uint64{}, next.ByRCode)
}
