package discovery

import (
	"context"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const own = 8080

func TestSuggestGroupsByProcess(t *testing.T) {
	got := Suggest([]Listener{
		{Address: "0.0.0.0", Port: 3000, PID: 10, Process: "node"},
		{Address: "127.0.0.1", Port: 9229, PID: 10, Process: "node"},
	}, own)
	assert.Equal(t, []Suggestion{{Process: "node", PID: 10, Host: "127.0.0.1", Kind: "app", Ports: []int{3000, 9229}}}, got)
}

func TestSuggestMergesIPv4AndIPv6(t *testing.T) {
	got := Suggest([]Listener{
		{Address: "0.0.0.0", Port: 8000, PID: 11, Process: "python"},
		{Address: "::", Port: 8000, PID: 11, Process: "python"},
	}, own)
	require.Len(t, got, 1)
	assert.Equal(t, []int{8000}, got[0].Ports)
}

func TestSuggestDetectsDatabase(t *testing.T) {
	got := Suggest([]Listener{{Address: "127.0.0.1", Port: 5432, PID: 12, Process: "postgres"}}, own)
	require.Len(t, got, 1)
	assert.Equal(t, "banco", got[0].Kind)
}

func TestSuggestUnknownProcess(t *testing.T) {
	got := Suggest([]Listener{
		{Address: "127.0.0.1", Port: 5432},
		{Address: "0.0.0.0", Port: 80},
	}, own)
	assert.Equal(t, []Suggestion{
		{Host: "127.0.0.1", Kind: "app", Ports: []int{80}},
		{Host: "127.0.0.1", Kind: "banco", Ports: []int{5432}},
	}, got)
}

func TestSuggestSkipsSystemAndSelf(t *testing.T) {
	got := Suggest([]Listener{
		{Address: "0.0.0.0", Port: 22, PID: 1, Process: "sshd"},
		{Address: "127.0.0.53", Port: 53, PID: 2, Process: "systemd-resolved"},
		{Address: "127.0.0.1", Port: own, PID: 3, Process: "omni"},
	}, own)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestSuggestKeepsSpecificAddress(t *testing.T) {
	got := Suggest([]Listener{
		{Address: "192.168.0.10", Port: 8443, PID: 20, Process: "java"},
		{Address: "::1", Port: 4000, PID: 21, Process: "deno"},
	}, own)
	require.Len(t, got, 2)
	assert.Equal(t, "::1", got[0].Host)
	assert.Equal(t, "192.168.0.10", got[1].Host)
}

func TestSuggestOrder(t *testing.T) {
	got := Suggest([]Listener{
		{Address: "127.0.0.1", Port: 3306, PID: 30, Process: "mysqld"},
		{Address: "127.0.0.1", Port: 5000, PID: 31, Process: "flask"},
		{Address: "127.0.0.1", Port: 3000, PID: 32, Process: "node"},
	}, own)
	require.Len(t, got, 3)
	assert.Equal(t, []int{3000}, got[0].Ports)
	assert.Equal(t, []int{5000}, got[1].Ports)
	assert.Equal(t, []int{3306}, got[2].Ports)
}

func TestSystemListenersRuns(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	port := ln.Addr().(*net.TCPAddr).Port

	ls, err := SystemListeners(context.Background())
	require.NoError(t, err)
	var found *Listener
	for i := range ls {
		if ls[i].Port == port {
			found = &ls[i]
		}
	}
	require.NotNil(t, found, "porta %d não listada", port)
	assert.Equal(t, int32(os.Getpid()), found.PID)
	assert.Equal(t, "127.0.0.1", found.Address)
	assert.NotEmpty(t, found.Process)
}
