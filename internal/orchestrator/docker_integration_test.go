//go:build integration

package orchestrator

import (
	"bytes"
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/arsenal"
	"github.com/MiguelDev-GH/omni/internal/target"
)

// TestDockerRunnerNmap roda o Nmap de verdade contra uma porta aberta no
// loopback: go test -tags integration ./internal/orchestrator
func TestDockerRunnerNmap(t *testing.T) {
	r, err := NewDockerRunner()
	require.NoError(t, err)
	if err := r.Available(context.Background()); err != nil {
		t.Skip(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port

	args, err := arsenal.NmapArgs(target.Target{Name: "t", Host: "127.0.0.1", AppPorts: []int{port}}, nil)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var logs bytes.Buffer
	out, err := r.Run(ctx, Spec{Image: arsenal.NmapImage, Args: args}, &logs)
	require.NoError(t, err, logs.String())
	list, err := arsenal.ParseNmapXML(bytes.NewReader(out))
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.True(t, strings.HasPrefix(list[0].Title, "Porta "+strconv.Itoa(port)+"/tcp aberta"), list[0].Title)
}

func TestDockerRunnerExitCodeAndCleanup(t *testing.T) {
	r, err := NewDockerRunner()
	require.NoError(t, err)
	if err := r.Available(context.Background()); err != nil {
		t.Skip(err)
	}
	var logs bytes.Buffer
	_, err = r.Run(context.Background(), Spec{Image: arsenal.NmapImage, Args: []string{"--opcao-invalida"}}, &logs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "código")
	assert.Contains(t, logs.String(), "unrecognized option")
}
