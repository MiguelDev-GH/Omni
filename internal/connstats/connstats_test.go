package connstats

import (
	"context"
	"net"
	"testing"

	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/target"
)

func conn(lport, rport uint32, status string, pid int32) gnet.ConnectionStat {
	return gnet.ConnectionStat{
		Laddr:  gnet.Addr{IP: "127.0.0.1", Port: lport},
		Raddr:  gnet.Addr{IP: "127.0.0.1", Port: rport},
		Status: status,
		Pid:    pid,
	}
}

var names = map[int32]string{10: "loja", 11: "psql"}

func nameOf(pid int32) string { return names[pid] }

func TestSummarizeCountsServerSideByState(t *testing.T) {
	conns := []gnet.ConnectionStat{
		{Laddr: gnet.Addr{IP: "0.0.0.0", Port: 3000}, Status: "LISTEN"},
		conn(3000, 50001, "ESTABLISHED", 1),
		conn(3000, 50002, "ESTABLISHED", 1),
		conn(3000, 50003, "ESTABLISHED", 1),
		conn(3000, 50004, "TIME_WAIT", 0),
		conn(8080, 50005, "ESTABLISHED", 1), // outra porta
	}
	got := Summarize(conns, []PortRole{{Port: 3000, Kind: KindApp}}, nameOf)
	require.Len(t, got, 1)
	assert.Equal(t, 3000, got[0].Port)
	assert.Equal(t, KindApp, got[0].Kind)
	assert.Equal(t, 3, got[0].Established)
	assert.Equal(t, map[string]int{"ESTABLISHED": 3, "TIME_WAIT": 1}, got[0].States)
}

func TestSummarizeGroupsClientPoolsByProcess(t *testing.T) {
	conns := []gnet.ConnectionStat{
		conn(40001, 5432, "ESTABLISHED", 10),
		conn(40002, 5432, "ESTABLISHED", 10),
		conn(40003, 5432, "ESTABLISHED", 10),
		conn(40004, 5432, "ESTABLISHED", 10),
		conn(40005, 5432, "ESTABLISHED", 11),
		conn(40006, 5432, "TIME_WAIT", 11), // fora do pool: não está aberta
	}
	got := Summarize(conns, []PortRole{{Port: 5432, Kind: KindDB}}, nameOf)
	require.Len(t, got, 1)
	assert.Equal(t, []ClientCount{{Process: "loja", Conns: 4}, {Process: "psql", Conns: 1}}, got[0].Clients)
}

func TestSummarizeUnknownPIDGroupsAsEmptyName(t *testing.T) {
	called := false
	got := Summarize([]gnet.ConnectionStat{conn(40001, 5432, "ESTABLISHED", 0)}, []PortRole{{Port: 5432, Kind: KindDB}},
		func(int32) string { called = true; return "x" })
	assert.False(t, called, "PID 0 não tem processo para consultar")
	assert.Equal(t, []ClientCount{{Process: "", Conns: 1}}, got[0].Clients)
}

func TestSummarizeAlwaysListsEveryRole(t *testing.T) {
	got := Summarize(nil, []PortRole{{Port: 3000, Kind: KindApp}, {Port: 5432, Kind: KindDB}}, nameOf)
	require.Len(t, got, 2)
	assert.Equal(t, 0, got[1].Established)
	assert.Equal(t, map[string]int{}, got[1].States)
	assert.Equal(t, []ClientCount{}, got[1].Clients)
}

func TestSummarizeCapsClients(t *testing.T) {
	var conns []gnet.ConnectionStat
	for pid := int32(100); pid < 115; pid++ {
		conns = append(conns, conn(uint32(40000+pid), 5432, "ESTABLISHED", pid))
	}
	got := Summarize(conns, []PortRole{{Port: 5432, Kind: KindDB}}, func(pid int32) string { return string(rune('a' + pid - 100)) })
	assert.Len(t, got[0].Clients, maxClients)
}

func TestRolesFromTargets(t *testing.T) {
	ts := []target.Target{
		{AppPorts: []int{3000, 3001}, DBPort: 5432, SMTPPort: 1025, DNSPort: 1053},
		{AppPorts: []int{3000}, DBPort: 5432},
	}
	assert.Equal(t, []PortRole{
		{Port: 3000, Kind: KindApp}, {Port: 3001, Kind: KindApp}, {Port: 5432, Kind: KindDB}, {Port: 1025, Kind: KindSMTP},
	}, Roles(ts))
}

// TestSampleReadsRealConnections abre uma conexão de verdade no loopback e
// confere que a coleta a encontra dos dois lados.
func TestSampleReadsRealConnections(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err == nil {
			accepted <- c
		}
	}()
	c, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	defer func() { _ = (<-accepted).Close() }()

	port := ln.Addr().(*net.TCPAddr).Port
	snap, err := sample(context.Background(), func(context.Context) ([]target.Target, error) {
		return []target.Target{{AppPorts: []int{port}}}, nil
	})
	require.NoError(t, err)
	require.Len(t, snap.Ports, 1)
	assert.Equal(t, 1, snap.Ports[0].Established)
	require.Len(t, snap.Ports[0].Clients, 1)
	assert.Equal(t, 1, snap.Ports[0].Clients[0].Conns)
	assert.NotEmpty(t, snap.Ports[0].Clients[0].Process, "o próprio processo do teste é visível")
}
