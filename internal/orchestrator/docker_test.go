package orchestrator

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDocker imita a API do Docker Engine e guarda o tar recebido.
type fakeDocker struct {
	mu      sync.Mutex
	calls   []string
	archive []byte
}

func (f *fakeDocker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	f.mu.Unlock()
	switch {
	case r.URL.Path == "/images/img:1/json":
		w.WriteHeader(http.StatusOK)
	case r.URL.Path == "/containers/create":
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"Id":"c1"}`)
	case r.Method == http.MethodPut && r.URL.Path == "/containers/c1/archive":
		if r.URL.Query().Get("path") != "/" || r.Header.Get("Content-Type") != "application/x-tar" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		b, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.archive = b
		f.mu.Unlock()
	case r.URL.Path == "/containers/c1/start":
		w.WriteHeader(http.StatusNoContent)
	case r.URL.Path == "/containers/c1/logs":
		hdr := make([]byte, 8)
		hdr[0] = 1
		binary.BigEndian.PutUint32(hdr[4:], 3)
		_, _ = w.Write(append(hdr, "ok\n"...))
	case r.URL.Path == "/containers/c1/wait":
		_, _ = io.WriteString(w, `{"StatusCode":0}`)
	case r.Method == http.MethodDelete:
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func fakeRunnerFor(t *testing.T) (*DockerRunner, *fakeDocker) {
	t.Helper()
	fd := &fakeDocker{}
	srv := httptest.NewServer(fd)
	t.Cleanup(srv.Close)
	addr := strings.TrimPrefix(srv.URL, "http://")
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", addr)
	}}
	return &DockerRunner{c: &http.Client{Transport: tr}}, fd
}

func TestRunCopiesFilesBeforeStart(t *testing.T) {
	d, fd := fakeRunnerFor(t)
	out, err := d.Run(context.Background(), Spec{Image: "img:1", Files: map[string][]byte{"/omni/lista.txt": []byte("admin\n.env\n")}}, io.Discard)
	require.NoError(t, err)
	assert.Equal(t, "ok\n", string(out))

	fd.mu.Lock()
	defer fd.mu.Unlock()
	assert.Equal(t, []string{"GET /images/img:1/json", "POST /containers/create", "PUT /containers/c1/archive",
		"POST /containers/c1/start", "GET /containers/c1/logs", "POST /containers/c1/wait", "DELETE /containers/c1"}, fd.calls)

	tr := tar.NewReader(bytes.NewReader(fd.archive))
	dir, err := tr.Next()
	require.NoError(t, err)
	assert.Equal(t, "omni/", dir.Name)
	assert.Equal(t, byte(tar.TypeDir), dir.Typeflag)
	file, err := tr.Next()
	require.NoError(t, err)
	assert.Equal(t, "omni/lista.txt", file.Name)
	assert.Equal(t, int64(0o444), file.Mode)
	body, err := io.ReadAll(tr)
	require.NoError(t, err)
	assert.Equal(t, "admin\n.env\n", string(body))
	_, err = tr.Next()
	assert.ErrorIs(t, err, io.EOF)
}

func TestRunWithoutFilesSkipsArchive(t *testing.T) {
	d, fd := fakeRunnerFor(t)
	_, err := d.Run(context.Background(), Spec{Image: "img:1"}, io.Discard)
	require.NoError(t, err)
	fd.mu.Lock()
	defer fd.mu.Unlock()
	assert.NotContains(t, fd.calls, "PUT /containers/c1/archive")
}

func TestRunRejectsBadFilePaths(t *testing.T) {
	for _, name := range []string{"lista.txt", "omni/lista.txt", "/omni/../etc/passwd", "/omni//lista.txt", "/", "/omni/..lista"} {
		d, fd := fakeRunnerFor(t)
		_, err := d.Run(context.Background(), Spec{Image: "img:1", Files: map[string][]byte{name: []byte("x")}}, io.Discard)
		require.Error(t, err, name)
		fd.mu.Lock()
		assert.Empty(t, fd.calls, "nenhum container criado para %q", name)
		fd.mu.Unlock()
	}
}
