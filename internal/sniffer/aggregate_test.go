package sniffer

import (
	"fmt"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/sniffer/testutil"
)

func decode(frame []byte) gopacket.Packet {
	return gopacket.NewPacket(frame, layers.LinkTypeEthernet, gopacket.Default)
}

func TestClassify(t *testing.T) {
	app := PortRoles{App: []int{3000}}
	cases := []struct {
		src, dst uint16
		udp      bool
		want     string
	}{
		{40000, 3000, false, "HTTP"},
		{3000, 40000, false, "HTTP"},
		{40000, 80, false, "HTTP"},
		{40000, 443, false, "HTTPS"},
		{40000, 53, true, "DNS"},
		{53, 40000, true, "DNS"},
		{40000, 5432, false, "PostgreSQL"},
		{3306, 40000, false, "MySQL"},
		{40000, 25, false, "SMTP"},
		{40000, 465, false, "SMTP"},
		{40000, 587, false, "SMTP"},
		{40000, 9999, false, "TCP"},
		{40000, 9999, true, "UDP"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%d-%d-%v", c.src, c.dst, c.udp), func(t *testing.T) {
			assert.Equal(t, c.want, Classify(c.src, c.dst, c.udp, app))
		})
	}
}

func TestAggregatorFlush(t *testing.T) {
	a := NewAggregator(PortRoles{App: []int{3000}})
	big := testutil.TCP("10.0.0.1", "10.0.0.2", 40000, 3000, "PA", make([]byte, 500))
	db := testutil.TCP("10.0.0.3", "10.0.0.2", 40001, 5432, "PA", make([]byte, 100))
	dns := testutil.UDP("10.0.0.3", "10.0.0.53", 5353, 53, make([]byte, 20))
	for _, f := range [][]byte{big, big, db, dns} {
		a.Add(decode(f))
	}

	now := time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC)
	w := a.Flush(now)
	total := uint64(2*len(big) + len(db) + len(dns))
	assert.Equal(t, now.Add(-time.Second), w.Start)
	assert.Equal(t, total, w.Bytes)
	assert.Equal(t, uint64(4), w.Packets)
	assert.Equal(t, map[string]uint64{
		"HTTP":       uint64(2 * len(big)),
		"PostgreSQL": uint64(len(db)),
		"DNS":        uint64(len(dns)),
	}, w.ByProtocol)
	require.Len(t, w.TopSources, 2)
	assert.Equal(t, SourceCount{IP: "10.0.0.1", Bytes: uint64(2 * len(big)), Packets: 2}, w.TopSources[0])
	assert.Equal(t, SourceCount{IP: "10.0.0.3", Bytes: uint64(len(db) + len(dns)), Packets: 2}, w.TopSources[1])

	next := a.Flush(now.Add(time.Second))
	assert.Equal(t, now, next.Start)
	assert.Zero(t, next.Bytes)
	assert.Zero(t, next.Packets)
	assert.Empty(t, next.ByProtocol)
	assert.Empty(t, next.TopSources)
}

func TestAggregatorCountsIPv6(t *testing.T) {
	a := NewAggregator(PortRoles{App: []int{3000}})
	v4 := testutil.TCP("10.0.0.1", "10.0.0.2", 40000, 3000, "S", nil)
	v6 := testutil.TCP("fd00::1", "fd00::2", 40000, 3000, "S", nil)
	a.Add(decode(v4))
	a.Add(decode(v6))
	w := a.Flush(time.Now())
	assert.Equal(t, uint64(2), w.Packets)
	assert.Equal(t, uint64(len(v4)+len(v6)), w.ByProtocol["HTTP"])
	ips := []string{w.TopSources[0].IP, w.TopSources[1].IP}
	assert.ElementsMatch(t, []string{"10.0.0.1", "fd00::1"}, ips)
}

func TestAggregatorKeepsTopTenSources(t *testing.T) {
	a := NewAggregator(PortRoles{})
	for i := 1; i <= 15; i++ {
		f := testutil.TCP(fmt.Sprintf("10.0.0.%d", i), "10.0.0.200", 40000, 9999, "PA", make([]byte, i*10))
		a.Add(decode(f))
	}
	w := a.Flush(time.Now())
	require.Len(t, w.TopSources, maxTopSources)
	assert.Equal(t, "10.0.0.15", w.TopSources[0].IP)
	assert.Equal(t, "10.0.0.6", w.TopSources[9].IP)
}

func TestClassifyCustomDNSAndSMTPPorts(t *testing.T) {
	roles := PortRoles{App: []int{3000}, DNS: []int{1053}, SMTP: []int{1025}}
	assert.Equal(t, "DNS", Classify(40000, 1053, true, roles))
	assert.Equal(t, "DNS", Classify(1053, 40000, true, roles))
	assert.Equal(t, "SMTP", Classify(41000, 1025, false, roles))
	assert.Equal(t, "SMTP", Classify(1025, 41000, false, roles))
	assert.Equal(t, "UDP", Classify(40000, 1054, true, roles))
}
