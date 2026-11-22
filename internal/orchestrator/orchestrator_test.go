package orchestrator

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/arsenal"
	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/store"
	"github.com/MiguelDev-GH/omni/internal/target"
	"github.com/MiguelDev-GH/omni/internal/ws"
)

type fakeRunner struct {
	mu        sync.Mutex
	specs     []Spec
	stdout    []byte
	err       error
	available error
	block     chan struct{} // se não nil, Run espera ele fechar
}

func (f *fakeRunner) Run(ctx context.Context, s Spec, logs io.Writer) ([]byte, error) {
	f.mu.Lock()
	f.specs = append(f.specs, s)
	block, out, err := f.block, f.stdout, f.err
	f.mu.Unlock()
	_, _ = io.WriteString(logs, "Starting Nmap\nlinha sem quebra")
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return out, err
}

func (f *fakeRunner) Available(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.available
}

type fakePub struct {
	mu     sync.Mutex
	events []ws.Event
}

func (p *fakePub) Publish(ev ws.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, ev)
}

func (p *fakePub) ofType(typ string) []ws.Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []ws.Event
	for _, ev := range p.events {
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
}

func setup(t *testing.T, r *fakeRunner) (*Orchestrator, *store.Store, *fakePub, target.Target) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omni.db"))
	require.NoError(t, err)
	tg, err := st.CreateTarget(context.Background(), target.Target{Name: "api", Host: "127.0.0.1", AppPorts: []int{3000}, DBPort: 5432})
	require.NoError(t, err)
	pub := &fakePub{}
	ctx, cancel := context.WithCancel(context.Background())
	o := New(ctx, r, st, pub)
	o.logEvery = 10 * time.Millisecond
	t.Cleanup(func() {
		cancel()
		o.Wait()
		_ = st.Close()
	})
	return o, st, pub, tg
}

func nmapXML(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("../arsenal/testdata/nmap.xml")
	require.NoError(t, err)
	return b
}

func waitStatus(t *testing.T, st *store.Store, id int64, status string) store.Scan {
	t.Helper()
	var sc store.Scan
	require.Eventually(t, func() bool {
		var err error
		sc, err = st.GetScan(context.Background(), id)
		return err == nil && sc.Status == status
	}, 2*time.Second, 5*time.Millisecond)
	return sc
}

func TestStartRecordsFindings(t *testing.T) {
	r := &fakeRunner{stdout: nmapXML(t)}
	o, st, pub, tg := setup(t, r)

	id, err := o.Start(context.Background(), tg.ID, []string{"nmap", "nmap"}, nil)
	require.NoError(t, err)
	sc := waitStatus(t, st, id, store.ScanDone)
	assert.Equal(t, []string{"nmap"}, sc.Tools)
	assert.Equal(t, 2, sc.Counts[findings.Info])

	r.mu.Lock()
	require.Len(t, r.specs, 1)
	assert.Equal(t, arsenal.NmapImage, r.specs[0].Image)
	assert.Equal(t, []string{"-sV", "-Pn", "-oX", "-", "-p", "3000,5432", "--", "127.0.0.1"}, r.specs[0].Args)
	r.mu.Unlock()

	require.Eventually(t, func() bool { return len(pub.ofType(EventDone)) == 1 }, time.Second, 5*time.Millisecond)
	done := pub.ofType(EventDone)[0].Data.(map[string]any)
	assert.Equal(t, 2, done["achados"])
	assert.Equal(t, 2, done["novos_desde_ultimo"])
	assert.NotEmpty(t, pub.ofType(EventStarted))

	var lines []string
	for _, ev := range pub.ofType(EventLog) {
		lines = append(lines, ev.Data.(map[string]any)["linhas"].([]string)...)
	}
	assert.Contains(t, lines, "Starting Nmap")
	assert.Contains(t, lines, "linha sem quebra")
	assert.True(t, strings.HasPrefix(lines[0], "$ nmap -sV"))

	// A segunda varredura igual não tem achados novos.
	id2, err := o.Start(context.Background(), tg.ID, []string{"nmap"}, nil)
	require.NoError(t, err)
	waitStatus(t, st, id2, store.ScanDone)
	require.Eventually(t, func() bool { return len(pub.ofType(EventDone)) == 2 }, time.Second, 5*time.Millisecond)
	assert.Equal(t, 0, pub.ofType(EventDone)[1].Data.(map[string]any)["novos_desde_ultimo"])
}

func TestStartSavesReport(t *testing.T) {
	r := &fakeRunner{stdout: nmapXML(t)}
	o, st, _, tg := setup(t, r)
	id, err := o.Start(context.Background(), tg.ID, []string{"nmap"}, nil)
	require.NoError(t, err)
	waitStatus(t, st, id, store.ScanDone)
	report, ok, err := st.Report(context.Background(), id, "nmap")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Contains(t, string(report), `"estado":"closed"`)
}

func TestReportErrorKeepsFindings(t *testing.T) {
	// Saída do testssl cortada antes do fim: os achados são lidos, mas o
	// relatório, que precisa do objeto final scanTime, não.
	cut := `{"id": "cert_chain_of_trust", "port": "443", "severity": "CRITICAL", "finding": "failed (self signed)."}`
	r := &fakeRunner{stdout: []byte(cut)}
	o, st, pub, tg := setup(t, r)
	id, err := o.Start(context.Background(), tg.ID, []string{"testssl"}, nil)
	require.NoError(t, err)
	sc := waitStatus(t, st, id, store.ScanDone)
	assert.Equal(t, 1, sc.Counts[findings.Critical])
	_, ok, err := st.Report(context.Background(), id, "testssl")
	require.NoError(t, err)
	assert.False(t, ok)

	require.Eventually(t, func() bool {
		for _, ev := range pub.ofType(EventLog) {
			for _, l := range ev.Data.(map[string]any)["linhas"].([]string) {
				if strings.HasPrefix(l, "relatório da ferramenta indisponível: ") {
					return true
				}
			}
		}
		return false
	}, time.Second, 5*time.Millisecond)
}

func TestStartOptions(t *testing.T) {
	o, st, _, tg := setup(t, &fakeRunner{stdout: nmapXML(t)})
	ctx := context.Background()
	var oe *arsenal.OptionError

	_, err := o.Start(ctx, tg.ID, []string{"nmap"}, map[string]arsenal.Options{"nmap": {"portas": "1-65535"}})
	require.ErrorAs(t, err, &oe)
	assert.Equal(t, "opcoes.nmap.portas", oe.Field())

	_, err = o.Start(ctx, tg.ID, []string{"nmap"}, map[string]arsenal.Options{"nuclei": {}})
	require.ErrorAs(t, err, &oe)
	assert.Equal(t, "opcoes.nuclei", oe.Field())

	list, err := st.ListScans(ctx, 0)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestStartSendsToolFiles(t *testing.T) {
	r := &fakeRunner{stdout: []byte(`{"commandline":"ffuf","results":[],"config":{"url":"http://127.0.0.1:3000/FUZZ"}}`)}
	o, st, _, tg := setup(t, r)
	ctx := context.Background()

	_, err := o.Start(ctx, tg.ID, []string{"ffuf"}, map[string]arsenal.Options{"ffuf": {"lista": "../../etc/passwd"}})
	var oe *arsenal.OptionError
	require.ErrorAs(t, err, &oe)
	assert.Equal(t, "opcoes.ffuf.lista", oe.Field())

	id, err := o.Start(ctx, tg.ID, []string{"ffuf"}, map[string]arsenal.Options{"ffuf": {"lista": "media"}})
	require.NoError(t, err)
	waitStatus(t, st, id, store.ScanDone)
	r.mu.Lock()
	defer r.mu.Unlock()
	require.Len(t, r.specs, 1)
	assert.Contains(t, r.specs[0].Args, "/omni/lista.txt")
	assert.Greater(t, len(r.specs[0].Files["/omni/lista.txt"]), 100_000, "lista média")
}

func TestRunnerErrorMarksScanFailed(t *testing.T) {
	r := &fakeRunner{err: errors.New("a ferramenta terminou com código 1")}
	o, st, pub, tg := setup(t, r)
	id, err := o.Start(context.Background(), tg.ID, []string{"nmap"}, nil)
	require.NoError(t, err)
	sc := waitStatus(t, st, id, store.ScanFailed)
	assert.Contains(t, sc.Error, "nmap: a ferramenta terminou com código 1")
	require.Eventually(t, func() bool { return len(pub.ofType(EventFailed)) == 1 }, time.Second, 5*time.Millisecond)
}

func TestParseErrorMarksScanFailed(t *testing.T) {
	r := &fakeRunner{stdout: []byte("não é xml")}
	o, st, _, tg := setup(t, r)
	id, err := o.Start(context.Background(), tg.ID, []string{"nmap"}, nil)
	require.NoError(t, err)
	sc := waitStatus(t, st, id, store.ScanFailed)
	assert.Contains(t, sc.Error, "XML do nmap")
}

func TestDockerUnavailableCreatesNoScan(t *testing.T) {
	r := &fakeRunner{available: ErrDockerIndisponivel}
	o, st, pub, tg := setup(t, r)
	_, err := o.Start(context.Background(), tg.ID, []string{"nmap"}, nil)
	require.ErrorIs(t, err, ErrDockerIndisponivel)
	list, err := st.ListScans(context.Background(), 0)
	require.NoError(t, err)
	assert.Empty(t, list)
	assert.Equal(t, StateUnavailable, o.DockerState())
	evs := pub.ofType(EventEngines)
	require.NotEmpty(t, evs)
	assert.Equal(t, map[string]string{"docker": StateUnavailable}, evs[len(evs)-1].Data)
}

func TestStartValidation(t *testing.T) {
	r := &fakeRunner{}
	o, st, _, tg := setup(t, r)
	ctx := context.Background()

	_, err := o.Start(ctx, tg.ID, nil, nil)
	assert.ErrorIs(t, err, ErrSemFerramentas)
	_, err = o.Start(ctx, tg.ID, []string{"nmap", "rm -rf"}, nil)
	assert.ErrorIs(t, err, ErrFerramentaDesconhecida)
	_, err = o.Start(ctx, 999, []string{"nmap"}, nil)
	assert.ErrorIs(t, err, store.ErrNotFound)

	lan, err := st.CreateTarget(ctx, target.Target{Name: "lan", Host: "10.0.0.5", AppPorts: []int{80}})
	require.NoError(t, err)
	_, err = o.Start(ctx, lan.ID, []string{"nmap"}, nil)
	assert.NoError(t, err, "o host do próprio alvo está no escopo")

	r.mu.Lock()
	assert.Len(t, r.specs, 0, "nenhum container antes da validação")
	r.mu.Unlock()
}

func TestOneScanPerTargetAndShutdown(t *testing.T) {
	r := &fakeRunner{block: make(chan struct{})}
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omni.db"))
	require.NoError(t, err)
	defer func() { _ = st.Close() }()
	tg, err := st.CreateTarget(context.Background(), target.Target{Name: "api", Host: "127.0.0.1", AppPorts: []int{3000}})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	o := New(ctx, r, st, &fakePub{})

	id, err := o.Start(context.Background(), tg.ID, []string{"nmap"}, nil)
	require.NoError(t, err)
	waitStatus(t, st, id, store.ScanRunning)
	_, err = o.Start(context.Background(), tg.ID, []string{"nmap"}, nil)
	assert.ErrorIs(t, err, ErrVarreduraEmAndamento)

	cancel()
	o.Wait()
	sc := waitStatus(t, st, id, store.ScanFailed)
	assert.Contains(t, sc.Error, "interrompida")
}

func TestTimeout(t *testing.T) {
	r := &fakeRunner{block: make(chan struct{})}
	o, st, _, tg := setup(t, r)
	o.timeout = 20 * time.Millisecond
	id, err := o.Start(context.Background(), tg.ID, []string{"nmap"}, nil)
	require.NoError(t, err)
	sc := waitStatus(t, st, id, store.ScanFailed)
	assert.Contains(t, sc.Error, "tempo limite")
}

func TestLogBatcherTruncates(t *testing.T) {
	pub := &fakePub{}
	l := newLogBatcher(pub, 1, "nmap", time.Hour)
	_, _ = io.WriteString(l, strings.Repeat("x", maxLineLen+10)+"\n")
	for range maxLogLines + 5 {
		_, _ = io.WriteString(l, "a\n")
	}
	l.Close()
	var lines []string
	for _, ev := range pub.ofType(EventLog) {
		lines = append(lines, ev.Data.(map[string]any)["linhas"].([]string)...)
	}
	require.Len(t, lines, maxLogLines+1)
	assert.Len(t, lines[0], maxLineLen)
	assert.Equal(t, "(saída truncada)", lines[len(lines)-1])
}
