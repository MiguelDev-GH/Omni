package capture

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/sniffer"
	"github.com/MiguelDev-GH/omni/internal/sniffer/testutil"
	"github.com/MiguelDev-GH/omni/internal/target"
	"github.com/MiguelDev-GH/omni/internal/ws"
)

// memStore guarda as capturas em memória.
type memStore struct {
	mu   sync.Mutex
	next int64
	rows map[int64]Capture
}

func newMemStore() *memStore { return &memStore{rows: map[int64]Capture{}} }

func (m *memStore) CreateCapture(_ context.Context, c Capture) (Capture, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.next++
	c.ID, c.File, c.Status = m.next, FileName(m.next), StatusRecording
	m.rows[c.ID] = c
	return c, nil
}

func (m *memStore) FinishCapture(_ context.Context, c Capture) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rows[c.ID]; !ok {
		return ErrNotFound
	}
	m.rows[c.ID] = c
	return nil
}

func (m *memStore) FailInterruptedCaptures(context.Context) error { return nil }

func (m *memStore) ListCaptures(context.Context) ([]Capture, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []Capture
	for _, c := range m.rows {
		list = append(list, c)
	}
	return list, nil
}

func (m *memStore) CapturesBefore(_ context.Context, before time.Time) ([]Capture, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []Capture
	for _, c := range m.rows {
		if c.StartedAt.Before(before) && c.Status != StatusRecording {
			list = append(list, c)
		}
	}
	return list, nil
}

func (m *memStore) GetCapture(_ context.Context, id int64) (Capture, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.rows[id]
	if !ok {
		return Capture{}, ErrNotFound
	}
	return c, nil
}

func (m *memStore) DeleteCapture(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rows[id]; !ok {
		return ErrNotFound
	}
	delete(m.rows, id)
	return nil
}

func (m *memStore) setStarted(id int64, at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.rows[id]
	c.StartedAt = at
	m.rows[id] = c
}

// fakeSniffer entrega aos taps os pacotes passados a send.
type fakeSniffer struct {
	mu    sync.Mutex
	state string
	taps  map[int]sniffer.RawTap
	next  int
}

func newFakeSniffer() *fakeSniffer {
	return &fakeSniffer{state: sniffer.StateActive, taps: map[int]sniffer.RawTap{}}
}

func (f *fakeSniffer) AddRawTap(fn sniffer.RawTap) func() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	id := f.next
	f.taps[id] = fn
	return func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		delete(f.taps, id)
	}
}

func (f *fakeSniffer) State() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}

func (f *fakeSniffer) tapCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.taps)
}

func (f *fakeSniffer) send(frames ...[]byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, b := range frames {
		p := gopacket.NewPacket(b, layers.LinkTypeEthernet, gopacket.Default)
		ts := testutil.Start.Add(time.Duration(i) * time.Millisecond)
		p.Metadata().CaptureInfo = gopacket.CaptureInfo{Timestamp: ts, CaptureLength: len(b), Length: len(b)}
		for _, tap := range f.taps {
			tap(p, layers.LinkTypeEthernet)
		}
	}
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

func (p *fakePub) types() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []string
	for _, ev := range p.events {
		out = append(out, ev.Type)
	}
	return out
}

var appTargets = func(context.Context) ([]target.Target, error) {
	return []target.Target{{ID: 1, Name: "api", Host: "127.0.0.1", AppPorts: []int{3000}}}, nil
}

func newRecorder(t *testing.T) (*Recorder, *memStore, *fakeSniffer, *fakePub) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	st, snif, pub := newMemStore(), newFakeSniffer(), &fakePub{}
	r := New(ctx, filepath.Join(t.TempDir(), "capturas"), st, snif, pub, appTargets, 24*time.Hour)
	r.minDur = 10 * time.Millisecond
	t.Cleanup(func() {
		cancel()
		r.Wait()
	})
	return r, st, snif, pub
}

func waitDone(t *testing.T, st *memStore, id int64) Capture {
	t.Helper()
	var c Capture
	require.Eventually(t, func() bool {
		var err error
		c, err = st.GetCapture(context.Background(), id)
		return err == nil && c.Status != StatusRecording
	}, 2*time.Second, 5*time.Millisecond)
	return c
}

func httpFrame(sport, dport uint16) []byte {
	return testutil.TCP("10.0.0.1", "10.0.0.2", sport, dport, "PA", []byte("GET /produtos/7 HTTP/1.1\r\n\r\n"))
}

func TestRecordWritesReadablePcap(t *testing.T) {
	r, st, snif, pub := newRecorder(t)
	ctx := context.Background()
	id, err := r.Start(ctx, 150*time.Millisecond, 0)
	require.NoError(t, err)
	snif.send(httpFrame(40000, 3000), httpFrame(40001, 3000))

	c := waitDone(t, st, id)
	assert.Equal(t, StatusDone, c.Status)
	assert.Equal(t, 2, c.Packets)
	assert.NotNil(t, c.FinishedAt)
	assert.Zero(t, snif.tapCount(), "o tap sai junto com a gravação")
	assert.Equal(t, []string{EventStarted, EventFinished}, pub.types())

	path := filepath.Join(r.dir, c.File)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	assert.Equal(t, info.Size(), c.Bytes)

	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	pr, err := pcapgo.NewReader(f)
	require.NoError(t, err)
	assert.Equal(t, layers.LinkTypeEthernet, pr.LinkType())
	data, ci, err := pr.ReadPacketData()
	require.NoError(t, err)
	assert.Equal(t, httpFrame(40000, 3000), data)
	assert.Equal(t, testutil.Start, ci.Timestamp.UTC())
}

func TestRecordFiltersByPort(t *testing.T) {
	r, st, snif, _ := newRecorder(t)
	id, err := r.Start(context.Background(), 100*time.Millisecond, 5432)
	require.NoError(t, err)
	snif.send(httpFrame(40000, 3000), httpFrame(40001, 5432), httpFrame(5432, 40002))
	assert.Equal(t, 2, waitDone(t, st, id).Packets)
}

func TestRecordStopsAtSizeLimit(t *testing.T) {
	r, st, snif, _ := newRecorder(t)
	frame := httpFrame(40000, 3000)
	r.maxFile = headerBytes + 2*int64(recordHeaderBytes+len(frame))
	id, err := r.Start(context.Background(), 5*time.Second, 0)
	require.NoError(t, err)
	snif.send(frame, frame, frame, frame)

	c := waitDone(t, st, id)
	assert.Equal(t, StatusDone, c.Status, "o limite encerra a captura como concluída")
	assert.Equal(t, 2, c.Packets)
	assert.Equal(t, r.maxFile, c.Bytes)
}

func TestStartRejectsSecondCaptureAndBadInput(t *testing.T) {
	r, st, _, _ := newRecorder(t)
	ctx := context.Background()
	_, err := r.Start(ctx, time.Millisecond, 0)
	assert.ErrorIs(t, err, ErrDuracaoInvalida)
	_, err = r.Start(ctx, 301*time.Second, 0)
	assert.ErrorIs(t, err, ErrDuracaoInvalida)
	_, err = r.Start(ctx, time.Second, 70000)
	assert.ErrorIs(t, err, ErrPortaInvalida)

	id, err := r.Start(ctx, 100*time.Millisecond, 0)
	require.NoError(t, err)
	_, err = r.Start(ctx, time.Second, 0)
	assert.ErrorIs(t, err, ErrEmAndamento)
	assert.ErrorIs(t, r.Delete(ctx, id), ErrAindaGravando)
	_, _, err = r.Open(ctx, id)
	assert.ErrorIs(t, err, ErrAindaGravando)

	waitDone(t, st, id)
	_, err = r.Start(ctx, 100*time.Millisecond, 0)
	assert.NoError(t, err, "a vez é liberada quando a captura termina")
}

func TestStartRefusesWithoutSnifferOrSpace(t *testing.T) {
	r, _, snif, _ := newRecorder(t)
	ctx := context.Background()
	snif.state = sniffer.StateNoPermission
	_, err := r.Start(ctx, time.Second, 0)
	assert.ErrorIs(t, err, ErrSnifferInativo)

	snif.state = sniffer.StateActive
	require.NoError(t, os.MkdirAll(r.dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(r.dir, "outra.pcap"), make([]byte, 64), 0o600))
	r.maxDir = 64
	_, err = r.Start(ctx, time.Second, 0)
	assert.ErrorIs(t, err, ErrDiretorioCheio)
}

func TestEmptyCaptureIsStillAValidFile(t *testing.T) {
	r, st, _, _ := newRecorder(t)
	id, err := r.Start(context.Background(), 20*time.Millisecond, 0)
	require.NoError(t, err)
	c := waitDone(t, st, id)
	assert.Equal(t, StatusDone, c.Status)
	assert.Equal(t, int64(headerBytes), c.Bytes)

	list, _, err := r.Packets(context.Background(), id, 0)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestPacketsSummarizesPages(t *testing.T) {
	r, st, snif, _ := newRecorder(t)
	id, err := r.Start(context.Background(), 200*time.Millisecond, 0)
	require.NoError(t, err)
	frames := make([][]byte, PageSize+3)
	for i := range frames {
		frames[i] = httpFrame(40000, 3000)
	}
	snif.send(frames...)
	require.Equal(t, PageSize+3, waitDone(t, st, id).Packets)

	first, _, err := r.Packets(context.Background(), id, 0)
	require.NoError(t, err)
	require.Len(t, first, PageSize)
	assert.Equal(t, "HTTP", first[0].Protocol)
	assert.Equal(t, "GET /produtos/:id", first[0].Info, "o resumo nunca traz o identificador da rota")
	assert.Equal(t, testutil.Start, first[0].Time)

	second, _, err := r.Packets(context.Background(), id, 1)
	require.NoError(t, err)
	assert.Len(t, second, 3)
	beyond, _, err := r.Packets(context.Background(), id, 9)
	require.NoError(t, err)
	assert.Empty(t, beyond)
}

func TestDeleteAndRetentionRemoveFileAndRow(t *testing.T) {
	r, st, _, _ := newRecorder(t)
	ctx := context.Background()
	var ids []int64
	for range 2 {
		id, err := r.Start(ctx, 10*time.Millisecond, 0)
		require.NoError(t, err)
		waitDone(t, st, id)
		ids = append(ids, id)
	}
	pathOf := func(id int64) string { return filepath.Join(r.dir, FileName(id)) }

	require.NoError(t, r.Delete(ctx, ids[0]))
	assert.NoFileExists(t, pathOf(ids[0]))
	_, err := st.GetCapture(ctx, ids[0])
	assert.ErrorIs(t, err, ErrNotFound)
	assert.ErrorIs(t, r.Delete(ctx, ids[0]), ErrNotFound)

	st.setStarted(ids[1], time.Now().Add(-25*time.Hour))
	r.Prune(ctx, time.Now())
	assert.NoFileExists(t, pathOf(ids[1]))
	_, err = st.GetCapture(ctx, ids[1])
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestOpenRefusesMissingFileAndPathsOutsideDir(t *testing.T) {
	r, st, _, _ := newRecorder(t)
	ctx := context.Background()
	id, err := r.Start(ctx, 10*time.Millisecond, 0)
	require.NoError(t, err)
	c := waitDone(t, st, id)
	require.NoError(t, os.Remove(filepath.Join(r.dir, c.File)))
	_, _, err = r.Open(ctx, id)
	assert.ErrorIs(t, err, ErrArquivoIndisponivel)

	c.File = "../omni.db"
	require.NoError(t, st.FinishCapture(ctx, c))
	_, _, err = r.Open(ctx, id)
	assert.ErrorIs(t, err, ErrArquivoIndisponivel)
}
