//go:build integration

package orchestrator

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
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

// TestDockerRunnerFfuf roda o ffuf de verdade com uma lista curta copiada
// para o container, contra um servidor no loopback que só tem /.env.
func TestDockerRunnerFfuf(t *testing.T) {
	r, err := NewDockerRunner()
	require.NoError(t, err)
	if err := r.Available(context.Background()); err != nil {
		t.Skip(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/.env" {
			http.NotFound(w, req)
			return
		}
		_, _ = w.Write([]byte("SEGREDO=1\n"))
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port

	args, err := arsenal.FfufArgs(target.Target{Name: "t", Host: "127.0.0.1", AppPorts: []int{port}}, nil)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var logs bytes.Buffer
	files := map[string][]byte{"/omni/lista.txt": []byte(".env\nadmin\nnada\n")}
	out, err := r.Run(ctx, Spec{Image: arsenal.FfufImage, Args: args, Files: files}, &logs)
	require.NoError(t, err, logs.String())
	list, err := arsenal.ParseFfufJSON(bytes.NewReader(out))
	require.NoError(t, err)
	require.Len(t, list, 1, logs.String())
	assert.Equal(t, "Caminho encontrado: /.env", list[0].Title)
	assert.Equal(t, port, list[0].Port)
	_, err = arsenal.FfufReportFrom(out)
	require.NoError(t, err)
}

// TestDockerRunnerNikto roda o Nikto de verdade contra um servidor no
// loopback que expõe /.env e lê o JSON que ele grava no container.
func TestDockerRunnerNikto(t *testing.T) {
	r, err := NewDockerRunner()
	require.NoError(t, err)
	if err := r.Available(context.Background()); err != nil {
		t.Skip(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Server", "Teste/1.0")
		if req.URL.Path != "/" && req.URL.Path != "/.env" {
			http.NotFound(w, req)
			return
		}
		_, _ = w.Write([]byte("SEGREDO=1\n"))
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port

	tool, _ := arsenal.Lookup("nikto")
	args, err := tool.Args(target.Target{Name: "t", Host: "127.0.0.1", AppPorts: []int{port}}, nil)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var logs bytes.Buffer
	out, err := r.Run(ctx, Spec{Image: tool.Image, Args: args, OutputFile: tool.OutputFile}, &logs)
	require.NoError(t, err, logs.String())
	require.Contains(t, string(out), arsenal.OutputFileMarker, logs.String())
	raw, err := tool.Report(out)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"servidor":"Teste/1.0"`)
	assert.Contains(t, string(raw), `"url":"/.env"`)
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
