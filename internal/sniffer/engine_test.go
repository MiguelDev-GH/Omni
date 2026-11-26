package sniffer

import (
	"context"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
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
	db      []l7.DBWindow
	dns     []l7.DNSWindow
	smtp    []l7.SMTPWindow
	pruned  []time.Time
}

func (f *fakeStore) InsertDNSWindow(_ context.Context, w l7.DNSWindow) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dns = append(f.dns, w)
	return nil
}

func (f *fakeStore) InsertSMTPWindow(_ context.Context, w l7.SMTPWindow) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.smtp = append(f.smtp, w)
	return nil
}

func (f *fakeStore) savedDNS() []l7.DNSWindow {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]l7.DNSWindow(nil), f.dns...)
}

func (f *fakeStore) savedSMTP() []l7.SMTPWindow {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]l7.SMTPWindow(nil), f.smtp...)
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

func (f *fakeStore) InsertDBWindow(_ context.Context, w l7.DBWindow) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.db = append(f.db, w)
	return nil
}

func (f *fakeStore) savedDB() []l7.DBWindow {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]l7.DBWindow(nil), f.db...)
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

func TestEngineRecordsDBWindows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pg.pcap")
	c := testutil.NewConn("127.0.0.1", 50000, "127.0.0.1", 5432)
	query := []byte{'Q', 0, 0, 0, 0}
	query = append(query, "SELECT * FROM u WHERE email = 'a@b.c'\x00"...)
	query[4] = byte(len(query) - 1)
	frames := c.Handshake()
	frames = append(frames,
		c.FromClient(query),
		c.FromServer([]byte{'C', 0, 0, 0, 13, 'S', 'E', 'L', 'E', 'C', 'T', ' ', '1', 0, 'Z', 0, 0, 0, 5, 'I'}),
	)
	testutil.WritePCAP(t, path, frames...)

	st := &fakeStore{targets: []target.Target{{ID: 1, AppPorts: []int{3000}, DBPort: 5432}}}
	pub := &fakePub{}
	e := NewEngine(func(string) (Source, error) { return OpenFile(path) }, st, pub)
	startEngine(t, e)

	require.Eventually(t, func() bool { return len(st.savedDB()) > 0 }, 2*time.Second, 10*time.Millisecond)
	w := st.savedDB()[0]
	assert.Equal(t, uint64(1), w.Queries)
	require.Len(t, w.Top, 1)
	assert.Equal(t, "SELECT * FROM u WHERE email = ?", w.Top[0].Query)
	assert.NotEmpty(t, pub.ofType(EventDB))
}

func dnsPayload(t *testing.T, id uint16, resp bool) []byte {
	t.Helper()
	d := &layers.DNS{ID: id, QR: resp, RD: true, Questions: []layers.DNSQuestion{
		{Name: []byte("loja.local"), Type: layers.DNSTypeA, Class: layers.DNSClassIN},
	}}
	buf := gopacket.NewSerializeBuffer()
	require.NoError(t, d.SerializeTo(buf, gopacket.SerializeOptions{FixLengths: true}))
	return buf.Bytes()
}

func TestEnginePublishesDNSAndSMTPWindows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dns-smtp.pcap")
	frames := [][]byte{
		testutil.UDP("127.0.0.1", "127.0.0.1", 40000, 1053, dnsPayload(t, 1, false)),
		testutil.UDP("127.0.0.1", "127.0.0.1", 1053, 40000, dnsPayload(t, 1, true)),
	}
	c := testutil.NewConn("127.0.0.1", 41000, "127.0.0.1", 1025)
	frames = append(frames, c.Handshake()...)
	frames = append(frames,
		c.FromServer([]byte("220 ok\r\n")),
		c.FromClient([]byte("MAIL FROM:<a@b.c>\r\n")), c.FromServer([]byte("250 ok\r\n")),
		c.FromClient([]byte("RCPT TO:<d@e.f>\r\n")), c.FromServer([]byte("250 ok\r\n")),
		c.FromClient([]byte("DATA\r\n")), c.FromServer([]byte("354 go\r\n")),
		c.FromClient([]byte("x\r\n.\r\n")), c.FromServer([]byte("250 queued\r\n")),
	)
	testutil.WritePCAP(t, path, frames...)

	st := &fakeStore{targets: []target.Target{{ID: 1, AppPorts: []int{3000}, Domain: "loja.local", DNSPort: 1053, SMTPPort: 1025}}}
	pub := &fakePub{}
	e := NewEngine(func(string) (Source, error) { return OpenFile(path) }, st, pub)
	startEngine(t, e)

	require.Eventually(t, func() bool { return len(st.savedDNS()) > 0 && len(st.savedSMTP()) > 0 }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, uint64(1), st.savedDNS()[0].Queries)
	assert.Equal(t, uint64(1), st.savedSMTP()[0].Delivered)
	assert.NotEmpty(t, pub.ofType(EventDNS))
	assert.NotEmpty(t, pub.ofType(EventSMTP))
	var dnsBytes uint64
	for _, w := range st.savedWindows() {
		dnsBytes += w.ByProtocol["DNS"]
	}
	assert.Positive(t, dnsBytes, "a porta DNS do alvo é classificada como DNS")
}

func TestEngineTapReceivesSummaries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.pcap")
	req := testutil.TCP("10.0.0.1", "10.0.0.2", 40000, 3000, "PA", []byte("GET /produtos/7 HTTP/1.1\r\nCookie: s=1\r\n\r\n"))
	testutil.WritePCAP(t, path, req)

	st := &fakeStore{targets: appTarget}
	e := NewEngine(func(string) (Source, error) { return OpenFile(path) }, st, &fakePub{})
	var mu sync.Mutex
	var got []PacketSummary
	e.SetTap(func(s PacketSummary) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, s)
	})
	startEngine(t, e)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(got) > 0
	}, 2*time.Second, 10*time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "GET /produtos/:id", got[0].Info)
	assert.Equal(t, testutil.Start, got[0].Time)
}

func TestEngineRawTapsReceivePacketsUntilRemoved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.pcap")
	req := testutil.TCP("10.0.0.1", "10.0.0.2", 40000, 3000, "PA", []byte("GET / HTTP/1.1\r\n\r\n"))
	testutil.WritePCAP(t, path, req)

	st := &fakeStore{targets: appTarget}
	e := NewEngine(func(string) (Source, error) { return OpenFile(path) }, st, &fakePub{})
	var mu sync.Mutex
	var links []layers.LinkType
	var kept, removed int
	e.AddRawTap(func(p gopacket.Packet, link layers.LinkType) {
		mu.Lock()
		defer mu.Unlock()
		kept++
		links = append(links, link)
	})
	remove := e.AddRawTap(func(gopacket.Packet, layers.LinkType) {
		mu.Lock()
		defer mu.Unlock()
		removed++
	})
	remove()
	remove()
	startEngine(t, e)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return kept > 0
	}, 2*time.Second, 10*time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	assert.Zero(t, removed, "o tap removido não recebe pacotes")
	assert.Equal(t, layers.LinkTypeEthernet, links[0], "o tipo de enlace vem da fonte")
}
