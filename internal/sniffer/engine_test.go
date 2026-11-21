package sniffer

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/sniffer/l7"
	"github.com/MiguelDev-GH/omni/internal/sniffer/testutil"
	"github.com/MiguelDev-GH/omni/internal/target"
	"github.com/MiguelDev-GH/omni/internal/ws"
)

type fakeStore struct {
	mu      sync.Mutex
	targets []target.Target
	windows []Window
	http    []l7.HTTPWindow
	pruned  []time.Time
}

func (f *fakeStore) ListTargets(context.Context) ([]target.Target, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.targets, nil
}

func (f *fakeStore) InsertTrafficWindow(_ context.Context, w Window) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.windows = append(f.windows, w)
	return nil
}

func (f *fakeStore) InsertHTTPWindow(_ context.Context, w l7.HTTPWindow) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.http = append(f.http, w)
	return nil
}

func (f *fakeStore) savedHTTP() []l7.HTTPWindow {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]l7.HTTPWindow(nil), f.http...)
}

func (f *fakeStore) PruneTraffic(_ context.Context, before time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pruned = append(f.pruned, before)
	return nil
}

func (f *fakeStore) setTargets(ts []target.Target) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.targets = ts
}

func (f *fakeStore) savedWindows() []Window {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Window(nil), f.windows...)
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

func (p *fakePub) lastState() string {
	evs := p.ofType(EventEngines)
	if len(evs) == 0 {
		return ""
	}
	return evs[len(evs)-1].Data.(map[string]string)["sniffer"]
}

var appTarget = []target.Target{{ID: 1, Name: "api", Host: "127.0.0.1", AppPorts: []int{3000}}}

func startEngine(t *testing.T, e *Engine) context.CancelFunc {
	t.Helper()
	e.window = 20 * time.Millisecond
	e.statusEvery = 50 * time.Millisecond
	e.l7Window = 30 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- e.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Error("Run não terminou em 2 s")
		}
	})
	return cancel
}

func TestEngineRecordsWindowsFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.pcap")
	req := testutil.TCP("10.0.0.1", "10.0.0.2", 40000, 3000, "PA", make([]byte, 100))
	resp := testutil.TCP("10.0.0.2", "10.0.0.1", 3000, 40000, "PA", make([]byte, 300))
	testutil.WritePCAP(t, path, req, resp, resp)

	st := &fakeStore{targets: appTarget}
	pub := &fakePub{}
	var gotBPF string
	var mu sync.Mutex
	e := NewEngine(func(bpf string) (Source, error) {
		mu.Lock()
		gotBPF = bpf
		mu.Unlock()
		return OpenFile(path)
	}, st, pub)
	startEngine(t, e)

	require.Eventually(t, func() bool { return len(st.savedWindows()) > 0 }, 2*time.Second, 10*time.Millisecond)
	w := st.savedWindows()[0]
	assert.Equal(t, uint64(3), w.Packets)
	assert.Equal(t, uint64(len(req)+2*len(resp)), w.Bytes)
	assert.Equal(t, map[string]uint64{"HTTP": w.Bytes}, w.ByProtocol)
	require.Len(t, w.TopSources, 2)
	assert.Equal(t, "10.0.0.2", w.TopSources[0].IP)

	assert.NotEmpty(t, pub.ofType(EventTraffic))
	assert.NotEmpty(t, pub.ofType(EventEngines))
	mu.Lock()
	assert.Equal(t, "tcp port 3000", gotBPF)
	mu.Unlock()
}

func TestSnifferPermissionDeniedKeepsRunning(t *testing.T) {
	st := &fakeStore{targets: appTarget}
	pub := &fakePub{}
	e := NewEngine(func(string) (Source, error) { return nil, ErrSemPermissao }, st, pub)
	startEngine(t, e)

	require.Eventually(t, func() bool { return e.State() == StateNoPermission }, 2*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool { return len(pub.ofType(EventEngines)) >= 2 }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, StateNoPermission, pub.lastState())
	assert.Empty(t, st.savedWindows())
}

func TestEngineWithoutTargetsIsInactive(t *testing.T) {
	st := &fakeStore{}
	pub := &fakePub{}
	opened := make(chan struct{}, 1)
	e := NewEngine(func(string) (Source, error) {
		opened <- struct{}{}
		return nil, ErrSemPermissao
	}, st, pub)
	startEngine(t, e)

	require.Eventually(t, func() bool { return pub.lastState() == StateInactive }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, StateInactive, e.State())
	assert.Empty(t, opened)
}

func TestEngineReopensWhenTargetsChange(t *testing.T) {
	st := &fakeStore{targets: appTarget}
	pub := &fakePub{}
	filters := make(chan string, 10)
	e := NewEngine(func(bpf string) (Source, error) {
		filters <- bpf
		return newBlockingSource(), nil
	}, st, pub)
	startEngine(t, e)

	assert.Equal(t, "tcp port 3000", <-filters)
	require.Eventually(t, func() bool { return e.State() == StateActive }, 2*time.Second, 10*time.Millisecond)
	st.setTargets([]target.Target{{ID: 1, AppPorts: []int{3000}, DBPort: 5432}})
	select {
	case bpf := <-filters:
		assert.Equal(t, "tcp port 3000 or tcp port 5432", bpf)
	case <-time.After(2 * time.Second):
		t.Fatal("o motor não reabriu a captura com o filtro novo")
	}
	assert.NotEmpty(t, pub.ofType(EventTraffic))
}

func TestEnginePrunesOldWindows(t *testing.T) {
	st := &fakeStore{}
	e := NewEngine(func(string) (Source, error) { return nil, ErrSemPermissao }, st, &fakePub{})
	before := time.Now()
	startEngine(t, e)
	require.Eventually(t, func() bool {
		st.mu.Lock()
		defer st.mu.Unlock()
		return len(st.pruned) == 1
	}, 2*time.Second, 10*time.Millisecond)
	st.mu.Lock()
	defer st.mu.Unlock()
	assert.WithinDuration(t, before.Add(-Retention), st.pruned[0], time.Second)
}

func TestEngineRecordsHTTPWindows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "http.pcap")
	c := testutil.NewConn("10.0.0.1", 40000, "10.0.0.2", 3000)
	frames := c.Handshake()
	frames = append(frames,
		c.FromClient([]byte("GET /users/7 HTTP/1.1\r\nAuthorization: Bearer segredo\r\n\r\n")),
		c.FromServer([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")),
		c.FromClient([]byte("GET / HTTP/1.1\r\nUser-Agent: Omni-Poller/1\r\n\r\n")),
		c.FromServer([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")),
	)
	testutil.WritePCAP(t, path, frames...)

	st := &fakeStore{targets: appTarget}
	pub := &fakePub{}
	e := NewEngine(func(string) (Source, error) { return OpenFile(path) }, st, pub)
	startEngine(t, e)

	require.Eventually(t, func() bool { return len(st.savedHTTP()) > 0 }, 2*time.Second, 10*time.Millisecond)
	w := st.savedHTTP()[0]
	assert.Equal(t, uint64(1), w.Requests)
	assert.Equal(t, map[string]uint64{"2xx": 1}, w.ByStatus)
	require.Len(t, w.Routes, 1)
	assert.Equal(t, "/users/:id", w.Routes[0].Route)
	assert.NotEmpty(t, pub.ofType(EventHTTP))
}
