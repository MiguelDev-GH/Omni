package api

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/capture"
	"github.com/MiguelDev-GH/omni/internal/sniffer"
	"github.com/MiguelDev-GH/omni/internal/sniffer/testutil"
	"github.com/MiguelDev-GH/omni/internal/store"
	"github.com/MiguelDev-GH/omni/internal/target"
)

// tapSniffer entrega aos taps os quadros passados a send.
type tapSniffer struct {
	mu   sync.Mutex
	taps []sniffer.RawTap
}

func (s *tapSniffer) AddRawTap(fn sniffer.RawTap) func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taps = append(s.taps, fn)
	n := len(s.taps) - 1
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.taps[n] = nil
	}
}

func (*tapSniffer) State() string { return sniffer.StateActive }

func (s *tapSniffer) send(frame []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := gopacket.NewPacket(frame, layers.LinkTypeEthernet, gopacket.Default)
	p.Metadata().CaptureInfo = gopacket.CaptureInfo{Timestamp: testutil.Start, CaptureLength: len(frame), Length: len(frame)}
	for _, tap := range s.taps {
		if tap != nil {
			tap(p, layers.LinkTypeEthernet)
		}
	}
}

// newCaptureServer devolve o servidor com as rotas de capturas e uma função
// que grava uma captura com um pacote HTTP e espera o fim dela.
func newCaptureServer(t *testing.T) (*Server, string, func() int64) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omni.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	_, err = st.CreateTarget(context.Background(), target.Target{Name: "api", Host: "127.0.0.1", AppPorts: []int{3000}})
	require.NoError(t, err)
	dir := filepath.Join(t.TempDir(), "capturas")
	snif := &tapSniffer{}

	ctx, cancel := context.WithCancel(context.Background())
	rec := capture.New(ctx, dir, st, snif, nopPub{}, st.ListTargets, 24*time.Hour)
	t.Cleanup(func() {
		cancel()
		rec.Wait()
	})
	srv := NewServer(st, tok)
	srv.Handle("POST /api/capturas", StartCaptureHandler(rec))
	srv.Handle("GET /api/capturas", ListCapturesHandler(rec))
	srv.Handle("GET /api/capturas/{id}/arquivo", CaptureFileHandler(rec))
	srv.Handle("GET /api/capturas/{id}/pacotes", CapturePacketsHandler(rec))
	srv.Handle("DELETE /api/capturas/{id}", DeleteCaptureHandler(rec))

	// recordOne só funciona uma vez por servidor: o fim da gravação vem do
	// cancelamento do contexto do gravador.
	recordOne := func() int64 {
		resp := do(t, srv, "POST", "/api/capturas", `{"duracao_s": 60, "filtro": {"ip": "", "porta": 0, "protocolo": ""}}`, true)
		require.Equal(t, http.StatusAccepted, resp.Code, resp.Body.String())
		id := decode[map[string]int64](t, resp)["id"]
		snif.send(testutil.TCP("10.0.0.1", "10.0.0.2", 40000, 3000, "PA", []byte("GET /produtos/7 HTTP/1.1\r\n\r\n")))
		// Dá tempo de o pacote sair do buffer antes de encerrar a gravação.
		time.Sleep(50 * time.Millisecond)
		cancel()
		rec.Wait()
		return id
	}
	return srv, dir, recordOne
}

func TestCaptureLifecycle(t *testing.T) {
	srv, dir, recordOne := newCaptureServer(t)
	id := recordOne()
	path := "/api/capturas/" + itoa(id)

	list := decode[[]capture.Capture](t, do(t, srv, "GET", "/api/capturas", "", true))
	require.Len(t, list, 1)
	assert.Equal(t, capture.StatusDone, list[0].Status)
	assert.Equal(t, 1, list[0].Packets)

	file := do(t, srv, "GET", path+"/arquivo", "", true)
	require.Equal(t, http.StatusOK, file.Code)
	assert.Equal(t, "application/vnd.tcpdump.pcap", file.Header().Get("Content-Type"))
	assert.Contains(t, file.Header().Get("Content-Disposition"), "attachment")
	onDisk, err := os.ReadFile(filepath.Join(dir, capture.FileName(id)))
	require.NoError(t, err)
	assert.True(t, bytes.Equal(onDisk, file.Body.Bytes()), "o download devolve os bytes do arquivo")

	page := decode[capturePage](t, do(t, srv, "GET", path+"/pacotes?pagina=0", "", true))
	require.Len(t, page.Packets, 1)
	assert.Equal(t, "GET /produtos/:id", page.Packets[0].Info)
	assert.Equal(t, 1, page.Total)
	assert.Equal(t, capture.PageSize, page.PerPage)
	assert.Equal(t, http.StatusBadRequest, do(t, srv, "GET", path+"/pacotes?pagina=-1", "", true).Code)

	assert.Equal(t, http.StatusNoContent, do(t, srv, "DELETE", path, "", true).Code)
	assert.Empty(t, decode[[]capture.Capture](t, do(t, srv, "GET", "/api/capturas", "", true)))
	assert.NoFileExists(t, filepath.Join(dir, capture.FileName(id)))
}

func TestCaptureMissingIDIs404(t *testing.T) {
	srv, _, _ := newCaptureServer(t)
	for _, req := range [][2]string{{"GET", "/api/capturas/99/arquivo"}, {"GET", "/api/capturas/99/pacotes"}, {"DELETE", "/api/capturas/99"}} {
		resp := do(t, srv, req[0], req[1], "", true)
		assert.Equal(t, http.StatusNotFound, resp.Code, req[1])
	}
	assert.Equal(t, http.StatusBadRequest, do(t, srv, "GET", "/api/capturas/abc/arquivo", "", true).Code)
}

func TestStartCaptureValidatesAndRejectsSecond(t *testing.T) {
	srv, _, _ := newCaptureServer(t)
	resp := do(t, srv, "POST", "/api/capturas", `{"duracao_s": 5}`, true)
	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, "duracao_s", decode[map[string]string](t, resp)["campo"])
	resp = do(t, srv, "POST", "/api/capturas", `{"duracao_s": 60, "filtro": {"porta": 70000}}`, true)
	assert.Equal(t, "filtro", decode[map[string]string](t, resp)["campo"])
	assert.Equal(t, http.StatusBadRequest, do(t, srv, "POST", "/api/capturas", `{"duracao": 60}`, true).Code)

	require.Equal(t, http.StatusAccepted, do(t, srv, "POST", "/api/capturas", `{"duracao_s": 60, "filtro": {"ip": "", "porta": 0, "protocolo": ""}}`, true).Code)
	assert.Equal(t, http.StatusConflict, do(t, srv, "POST", "/api/capturas", `{"duracao_s": 60, "filtro": {"ip": "", "porta": 0, "protocolo": ""}}`, true).Code)
	assert.Equal(t, http.StatusConflict, do(t, srv, "GET", "/api/capturas/1/arquivo", "", true).Code,
		"o arquivo só sai depois do fim da gravação")
	assert.Equal(t, http.StatusConflict, do(t, srv, "DELETE", "/api/capturas/1", "", true).Code)
	assert.Equal(t, http.StatusUnauthorized, do(t, srv, "GET", "/api/capturas", "", false).Code)
}
