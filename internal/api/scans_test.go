package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/orchestrator"
	"github.com/MiguelDev-GH/omni/internal/store"
	"github.com/MiguelDev-GH/omni/internal/target"
	"github.com/MiguelDev-GH/omni/internal/ws"
)

type stubRunner struct {
	available error
	stdout    []byte
}

func (s stubRunner) Run(context.Context, orchestrator.Spec, io.Writer) ([]byte, error) {
	return s.stdout, nil
}
func (s stubRunner) Available(context.Context) error { return s.available }

type nopPub struct{}

func (nopPub) Publish(ws.Event) {}

func newScanServer(t *testing.T, r orchestrator.Runner) (*Server, *store.Store, target.Target) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omni.db"))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	o := orchestrator.New(ctx, r, st, nopPub{})
	t.Cleanup(func() {
		cancel()
		o.Wait()
		_ = st.Close()
	})
	tg, err := st.CreateTarget(context.Background(), target.Target{Name: "api", Host: "127.0.0.1", AppPorts: []int{3000}})
	require.NoError(t, err)
	srv := NewServer(st, tok)
	srv.Handle("POST /api/varreduras", StartScanHandler(o))
	srv.Handle("GET /api/varreduras", ListScansHandler(st))
	srv.Handle("GET /api/varreduras/{id}", GetScanHandler(st))
	srv.Handle("GET /api/varreduras/{id}/achados", ScanFindingsHandler(st))
	srv.Handle("GET /api/ferramentas", ToolsHandler())
	return srv, st, tg
}

func TestScanDockerUnavailable(t *testing.T) {
	srv, _, tg := newScanServer(t, stubRunner{available: orchestrator.ErrDockerIndisponivel})
	rec := do(t, srv, "POST", "/api/varreduras", `{"alvo_id": `+itoa(tg.ID)+`, "ferramentas": ["nmap"]}`, true)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, `{"erro":"Docker indisponível"}`, rec.Body.String())

	rec = do(t, srv, "GET", "/api/saude", "", false)
	assert.Equal(t, http.StatusOK, rec.Code)
	rec = do(t, srv, "GET", "/api/varreduras", "", true)
	assert.JSONEq(t, `[]`, rec.Body.String())
}

func TestScanValidation(t *testing.T) {
	srv, st, tg := newScanServer(t, stubRunner{})
	id := itoa(tg.ID)
	cases := []struct {
		body   string
		status int
		field  string
	}{
		{`{"alvo_id": ` + id + `, "ferramentas": ["nmap", "bash"]}`, http.StatusBadRequest, "ferramentas"},
		{`{"alvo_id": ` + id + `, "ferramentas": []}`, http.StatusBadRequest, "ferramentas"},
		{`{"ferramentas": ["nmap"]}`, http.StatusBadRequest, "alvo_id"},
		{`{"alvo_id": 999, "ferramentas": ["nmap"]}`, http.StatusNotFound, ""},
		{`{"alvo_id": "1; rm -rf /"}`, http.StatusBadRequest, ""},
	}
	for _, c := range cases {
		rec := do(t, srv, "POST", "/api/varreduras", c.body, true)
		assert.Equal(t, c.status, rec.Code, c.body)
		if c.field != "" {
			assert.Equal(t, c.field, decode[map[string]string](t, rec)["campo"], c.body)
		}
	}
	list, err := st.ListScans(context.Background(), 0)
	require.NoError(t, err)
	assert.Empty(t, list)

	rec := do(t, srv, "POST", "/api/varreduras", `{"alvo_id": 1, "ferramentas": ["nmap"]}`, false)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestScanLifecycle(t *testing.T) {
	xml, err := os.ReadFile("../arsenal/testdata/nmap.xml")
	require.NoError(t, err)
	srv, st, tg := newScanServer(t, stubRunner{stdout: xml})

	rec := do(t, srv, "POST", "/api/varreduras", `{"alvo_id": `+itoa(tg.ID)+`, "ferramentas": ["nmap"]}`, true)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	scanID := int64(decode[map[string]float64](t, rec)["id"])
	require.Eventually(t, func() bool {
		sc, err := st.GetScan(context.Background(), scanID)
		return err == nil && sc.Status == store.ScanDone
	}, 2*time.Second, 5*time.Millisecond)

	rec = do(t, srv, "GET", "/api/varreduras?alvo_id="+itoa(tg.ID), "", true)
	require.Equal(t, http.StatusOK, rec.Code)
	scans := decode[[]map[string]any](t, rec)
	require.Len(t, scans, 1)
	assert.Equal(t, map[string]any{"info": 2.0}, scans[0]["achados"])

	rec = do(t, srv, "GET", "/api/varreduras/"+itoa(scanID)+"/achados", "", true)
	require.Equal(t, http.StatusOK, rec.Code)
	found := decode[[]map[string]any](t, rec)
	require.Len(t, found, 2)
	assert.Equal(t, "nmap", found[0]["ferramenta"])

	rec = do(t, srv, "GET", "/api/varreduras/999", "", true)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	rec = do(t, srv, "GET", "/api/varreduras/999/achados", "", true)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	rec = do(t, srv, "GET", "/api/varreduras?alvo_id=x", "", true)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = do(t, srv, "GET", "/api/ferramentas", "", true)
	require.Equal(t, http.StatusOK, rec.Code)
	tools := decode[[]map[string]any](t, rec)
	assert.Equal(t, "nmap", tools[0]["nome"])
}

func TestScanConflict(t *testing.T) {
	srv, _, tg := newScanServer(t, &blockingRunner{release: make(chan struct{})})
	body := `{"alvo_id": ` + itoa(tg.ID) + `, "ferramentas": ["nmap"]}`
	rec := do(t, srv, "POST", "/api/varreduras", body, true)
	require.Equal(t, http.StatusAccepted, rec.Code)
	rec = do(t, srv, "POST", "/api/varreduras", body, true)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

type blockingRunner struct{ release chan struct{} }

func (b *blockingRunner) Run(ctx context.Context, _ orchestrator.Spec, _ io.Writer) ([]byte, error) {
	select {
	case <-b.release:
	case <-ctx.Done():
	}
	return nil, errors.New("cancelado")
}
func (b *blockingRunner) Available(context.Context) error { return nil }

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
