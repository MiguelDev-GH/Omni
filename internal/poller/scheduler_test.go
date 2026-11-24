package poller

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/target"
	"github.com/MiguelDev-GH/omni/internal/ws"
)

type fakeStore struct {
	mu      sync.Mutex
	targets []target.Target
	results []Result
	failFor map[int64]bool
	prunes  []time.Time
}

func (f *fakeStore) PruneChecks(_ context.Context, before time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prunes = append(f.prunes, before)
	return nil
}

func (f *fakeStore) ListTargets(context.Context) ([]target.Target, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]target.Target(nil), f.targets...), nil
}

func (f *fakeStore) InsertCheckResult(_ context.Context, r Result) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failFor[r.TargetID] {
		return errors.New("falha simulada")
	}
	f.results = append(f.results, r)
	return nil
}

func (f *fakeStore) setTargets(ts []target.Target) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.targets = ts
}

func (f *fakeStore) stored() []Result {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Result(nil), f.results...)
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

func newUpServer(t *testing.T) (*httptest.Server, int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	_, port := hostPort(t, srv.URL)
	return srv, port
}

// closedPort devolve uma porta local onde nada está escutando.
func closedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	return port
}

func TestRunOnceEmitsDownOnlyOnTransition(t *testing.T) {
	srv, port := newUpServer(t)
	t.Cleanup(srv.Close)
	st := &fakeStore{targets: []target.Target{{ID: 1, Name: "api", Host: "127.0.0.1", AppPorts: []int{port}}}}
	pub := &fakePub{}
	s := NewScheduler(st, pub, time.Hour)
	ctx := context.Background()

	require.NoError(t, s.RunOnce(ctx))
	assert.Empty(t, pub.ofType("ALERTA_QUEDA"))

	srv.Close()
	require.NoError(t, s.RunOnce(ctx))
	downs := pub.ofType("ALERTA_QUEDA")
	require.Len(t, downs, 1)
	down, ok := downs[0].Data.(DownAlert)
	require.True(t, ok)
	assert.Equal(t, int64(1), down.TargetID)
	assert.Equal(t, "api", down.Target)
	assert.Equal(t, port, down.Port)
	assert.NotEmpty(t, down.Err)
	assert.False(t, down.At.IsZero())

	require.NoError(t, s.RunOnce(ctx))
	assert.Len(t, pub.ofType("ALERTA_QUEDA"), 1)
	assert.Empty(t, pub.ofType("SERVICO_RESTAURADO"))

	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		t.Skipf("não foi possível reabrir a porta %d: %v", port, err)
	}
	back := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go func() { _ = back.Serve(ln) }()
	defer func() { _ = back.Close() }()

	require.NoError(t, s.RunOnce(ctx))
	ups := pub.ofType("SERVICO_RESTAURADO")
	require.Len(t, ups, 1)
	up, ok := ups[0].Data.(RestoredAlert)
	require.True(t, ok)
	assert.Equal(t, int64(1), up.TargetID)
	assert.Equal(t, "api", up.Target)
	assert.Equal(t, port, up.Port)
	assert.False(t, up.At.IsZero())
	assert.Len(t, pub.ofType("ALERTA_QUEDA"), 1)
}

func TestRunOnceAlertsWhenFirstSeenDown(t *testing.T) {
	port := closedPort(t)
	st := &fakeStore{targets: []target.Target{{ID: 3, Name: "fora", Host: "127.0.0.1", AppPorts: []int{port}}}}
	pub := &fakePub{}

	require.NoError(t, NewScheduler(st, pub, time.Hour).RunOnce(context.Background()))

	downs := pub.ofType("ALERTA_QUEDA")
	require.Len(t, downs, 1)
	down, ok := downs[0].Data.(DownAlert)
	require.True(t, ok)
	assert.Equal(t, port, down.Port)
}

func TestRunOncePublishesStatusAndStores(t *testing.T) {
	srvA, portA := newUpServer(t)
	defer srvA.Close()
	srvB, portB := newUpServer(t)
	defer srvB.Close()
	db, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	dbPort := db.Addr().(*net.TCPAddr).Port

	st := &fakeStore{targets: []target.Target{{
		ID: 5, Name: "app", Host: "127.0.0.1", AppPorts: []int{portA, portB}, DBPort: dbPort,
	}}}
	pub := &fakePub{}

	require.NoError(t, NewScheduler(st, pub, time.Hour).RunOnce(context.Background()))

	assert.Len(t, st.stored(), 3)
	statuses := pub.ofType("STATUS_ALVOS")
	require.Len(t, statuses, 1)
	data, ok := statuses[0].Data.(StatusData)
	require.True(t, ok)
	require.Len(t, data.Results, 3)
	assert.Equal(t, []int{portA, portB, dbPort},
		[]int{data.Results[0].Port, data.Results[1].Port, data.Results[2].Port})
	assert.Equal(t, []string{"http", "http", "tcp"},
		[]string{data.Results[0].Kind, data.Results[1].Kind, data.Results[2].Kind})
	for _, r := range data.Results {
		assert.True(t, r.Up, "porta %d deveria estar no ar", r.Port)
	}
}

func TestRunOnceContinuesAfterInsertError(t *testing.T) {
	srv, port := newUpServer(t)
	defer srv.Close()
	st := &fakeStore{
		targets: []target.Target{
			{ID: 1, Name: "um", Host: "127.0.0.1", AppPorts: []int{port}},
			{ID: 2, Name: "dois", Host: "127.0.0.1", AppPorts: []int{port}},
		},
		failFor: map[int64]bool{1: true},
	}
	pub := &fakePub{}

	require.NoError(t, NewScheduler(st, pub, time.Hour).RunOnce(context.Background()))

	stored := st.stored()
	require.Len(t, stored, 1)
	assert.Equal(t, int64(2), stored[0].TargetID)
	statuses := pub.ofType("STATUS_ALVOS")
	require.Len(t, statuses, 1)
	data, ok := statuses[0].Data.(StatusData)
	require.True(t, ok)
	results := data.Results
	require.Len(t, results, 2)
	assert.Equal(t, int64(1), results[0].TargetID)
	assert.Equal(t, int64(2), results[1].TargetID)
}

func TestRunOnceForgetsRemovedTargets(t *testing.T) {
	port := closedPort(t)
	tg := target.Target{ID: 9, Name: "volta", Host: "127.0.0.1", AppPorts: []int{port}}
	st := &fakeStore{targets: []target.Target{tg}}
	pub := &fakePub{}
	s := NewScheduler(st, pub, time.Hour)
	ctx := context.Background()

	require.NoError(t, s.RunOnce(ctx))
	require.Len(t, pub.ofType("ALERTA_QUEDA"), 1)
	require.Len(t, pub.ofType("STATUS_ALVOS"), 1)

	st.setTargets(nil)
	require.NoError(t, s.RunOnce(ctx))
	assert.Len(t, pub.ofType("STATUS_ALVOS"), 1)

	st.setTargets([]target.Target{tg})
	require.NoError(t, s.RunOnce(ctx))
	assert.Len(t, pub.ofType("ALERTA_QUEDA"), 2)
}

func TestRunStopsOnCancel(t *testing.T) {
	srv, port := newUpServer(t)
	defer srv.Close()
	st := &fakeStore{targets: []target.Target{{ID: 1, Name: "api", Host: "127.0.0.1", AppPorts: []int{port}}}}
	pub := &fakePub{}
	s := NewScheduler(st, pub, 20*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	require.Eventually(t, func() bool { return len(pub.ofType("STATUS_ALVOS")) >= 2 },
		2*time.Second, 5*time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run não retornou após o cancelamento")
	}
}

func TestRunOnceDiscardsRoundWhenCanceled(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var enterOnce sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		enterOnce.Do(func() { close(entered) })
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	_, port := hostPort(t, srv.URL)

	st := &fakeStore{targets: []target.Target{{ID: 1, Name: "api", Host: "127.0.0.1", AppPorts: []int{port}}}}
	pub := &fakePub{}
	s := NewScheduler(st, pub, time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- s.RunOnce(ctx) }()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("o handler não foi chamado")
	}
	cancel()

	select {
	case err := <-errCh:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("RunOnce não retornou após o cancelamento")
	}
	assert.Empty(t, st.stored())
	pub.mu.Lock()
	defer pub.mu.Unlock()
	assert.Empty(t, pub.events)
}

func TestRunOnceSeparatesStateByKind(t *testing.T) {
	// Aceita TCP e fecha a conexão logo em seguida: CheckTCP vê no ar e
	// CheckHTTP vê fora do ar.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port

	st := &fakeStore{targets: []target.Target{{
		ID: 4, Name: "mesma", Host: "127.0.0.1", AppPorts: []int{port}, DBPort: port,
	}}}
	pub := &fakePub{}
	s := NewScheduler(st, pub, time.Hour)
	ctx := context.Background()

	require.NoError(t, s.RunOnce(ctx))
	downs := pub.ofType("ALERTA_QUEDA")
	require.Len(t, downs, 1)
	down, ok := downs[0].Data.(DownAlert)
	require.True(t, ok)
	assert.Equal(t, port, down.Port)
	statuses := pub.ofType("STATUS_ALVOS")
	require.Len(t, statuses, 1)
	data, ok := statuses[0].Data.(StatusData)
	require.True(t, ok)
	require.Len(t, data.Results, 2)
	assert.Equal(t, "http", data.Results[0].Kind)
	assert.False(t, data.Results[0].Up)
	assert.Equal(t, "tcp", data.Results[1].Kind)
	assert.True(t, data.Results[1].Up)

	require.NoError(t, s.RunOnce(ctx))
	assert.Len(t, pub.ofType("ALERTA_QUEDA"), 1)
	assert.Empty(t, pub.ofType("SERVICO_RESTAURADO"))
}

func TestRunOncePrunesAtMostOncePerHour(t *testing.T) {
	st := &fakeStore{}
	s := NewScheduler(st, &fakePub{}, time.Hour).WithRetention(48 * time.Hour)
	start := time.Now()
	require.NoError(t, s.RunOnce(context.Background()))
	require.NoError(t, s.RunOnce(context.Background()))
	st.mu.Lock()
	defer st.mu.Unlock()
	require.Len(t, st.prunes, 1)
	assert.WithinDuration(t, start.Add(-48*time.Hour), st.prunes[0], time.Minute)
}

func TestRunOnceWithoutRetentionDoesNotPrune(t *testing.T) {
	st := &fakeStore{}
	require.NoError(t, NewScheduler(st, &fakePub{}, time.Hour).RunOnce(context.Background()))
	assert.Empty(t, st.prunes)
}
