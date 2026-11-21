package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MiguelDev-GH/omni/internal/target"
)

type fakeTargets struct {
	list  []target.Target
	err   error
	calls int
}

func (f *fakeTargets) ListTargets(context.Context) ([]target.Target, error) {
	f.calls++
	return f.list, f.err
}

func newTestWatcher(rules []Rule, tl TargetLister) (*Watcher, *[]Alert) {
	e := NewEngine(rules)
	got := collect(e)
	return NewWatcher(e, tl), got
}

// mkEvent serializa os dados como o Feed faz.
func mkEvent(typ string, data any) rawEvent {
	b, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return rawEvent{typ: typ, data: b}
}

func lojaTargets() *fakeTargets {
	return &fakeTargets{list: []target.Target{{ID: 1, Name: "loja", AppPorts: []int{3000}, DBPort: 5433}}}
}

// statusPayload imita o poller.StatusData publicado pelo Poller.
func statusPayload(results ...map[string]any) map[string]any {
	return map[string]any{"resultados": results}
}

func TestWatcherAvailabilityFromStatus(t *testing.T) {
	w, got := newTestWatcher(DefaultRules(), lojaTargets())
	ev := mkEvent(eventStatus, statusPayload(
		map[string]any{"alvo_id": 1, "porta": 3000, "tipo": "http", "no_ar": false, "erro": "connection refused", "hora": t0},
		map[string]any{"alvo_id": 1, "porta": 5433, "tipo": "tcp", "no_ar": true, "latencia_ms": 0.3, "hora": t0},
	))
	w.handle(context.Background(), ev, t0)

	if len(*got) != 1 {
		t.Fatalf("alertas = %+v, quer 1", *got)
	}
	a := (*got)[0]
	if a.Subject != "loja:3000" || a.Level != Red || a.Service != ServiceWeb || a.Detail != "connection refused" {
		t.Errorf("alerta = %+v", a)
	}
	statuses, _ := w.engine.Snapshot()
	var dbSeen bool
	for _, s := range statuses {
		if s.Subject == "loja:5433" && s.Rule == RuleAvailability {
			dbSeen = s.Service == ServiceDB && s.Level == Green
		}
	}
	if !dbSeen {
		t.Errorf("porta do banco ausente ou com serviço errado: %+v", statuses)
	}
}

func TestWatcherHTTP5xxIsDegraded(t *testing.T) {
	w, got := newTestWatcher(DefaultRules(), lojaTargets())
	w.handle(context.Background(), mkEvent(eventStatus, statusPayload(
		map[string]any{"alvo_id": 1, "porta": 3000, "tipo": "http", "no_ar": true, "status_http": 503, "latencia_ms": 2, "hora": t0},
	)), t0)
	if len(*got) != 1 || (*got)[0].Level != Yellow || (*got)[0].Detail != "HTTP 503" {
		t.Fatalf("alertas = %+v", *got)
	}
}

func TestWatcherUnknownTargetName(t *testing.T) {
	w, got := newTestWatcher(DefaultRules(), &fakeTargets{err: errors.New("banco travado")})
	w.handle(context.Background(), mkEvent(eventStatus, statusPayload(
		map[string]any{"alvo_id": 7, "porta": 80, "tipo": "http", "no_ar": false, "hora": t0},
	)), t0)
	if len(*got) != 1 || (*got)[0].Subject != "alvo 7:80" {
		t.Fatalf("alertas = %+v", *got)
	}
}

func TestWatcherHTTPWindow(t *testing.T) {
	w, got := newTestWatcher(DefaultRules(), nil)
	window := func(total, errs uint64, p95 float64) rawEvent {
		return mkEvent(eventHTTP, map[string]any{
			"inicio": t0, "requisicoes": total, "status": map[string]uint64{"2xx": total - errs, "5xx": errs}, "p95_ms": p95,
		})
	}
	// 40% de erro durante 30 s (uma janela por segundo).
	for i := 0; i <= 30; i++ {
		w.handle(context.Background(), window(100, 40, 50), t0.Add(time.Duration(i)*time.Second))
	}
	if len(*got) != 1 || (*got)[0].Rule != RuleHTTP5xx || (*got)[0].Level != Red {
		t.Fatalf("alertas = %+v", *got)
	}
	if !strings.Contains((*got)[0].Detail, "40 de 100") {
		t.Errorf("detalhe = %q", (*got)[0].Detail)
	}
}

func TestWatcherIgnoresSmallWindows(t *testing.T) {
	w, got := newTestWatcher(DefaultRules(), nil)
	for i := 0; i <= 60; i++ {
		w.handle(context.Background(), mkEvent(eventHTTP, map[string]any{
			"requisicoes": 2, "status": map[string]uint64{"5xx": 2}, "p95_ms": 9000,
		}), t0.Add(time.Duration(i)*time.Second))
		w.handle(context.Background(), mkEvent(eventDB, map[string]any{
			"consultas": 3, "erros": 3, "p95_ms": 9000,
		}), t0.Add(time.Duration(i)*time.Second))
	}
	statuses, _ := w.engine.Snapshot()
	if len(*got) != 0 || len(statuses) != 0 {
		t.Fatalf("janelas pequenas geraram alerta: %+v", *got)
	}
}

func TestWatcherDBAndSystem(t *testing.T) {
	w, got := newTestWatcher(DefaultRules(), nil)
	for i := 0; i <= 60; i++ {
		at := t0.Add(time.Duration(i) * time.Second)
		w.handle(context.Background(), mkEvent(eventDB, map[string]any{
			"consultas": 100, "erros": 1, "p95_ms": 2500,
		}), at)
		w.handle(context.Background(), mkEvent(eventSystem, map[string]any{
			"cpu": 97.5, "memoria": 10, "disco": 90, "hora": at,
		}), at)
	}
	levels := map[string]Level{}
	for _, a := range *got {
		levels[a.Rule] = a.Level
	}
	want := map[string]Level{RuleDBP95: Red, RuleCPU: Red, RuleDisk: Yellow}
	for rule, lvl := range want {
		if levels[rule] != lvl {
			t.Errorf("%s = %q, quer %q (alertas: %+v)", rule, levels[rule], lvl, *got)
		}
	}
	if _, ok := levels[RuleDBErrors]; ok {
		t.Errorf("1%% de erro não deveria alertar")
	}
	if _, ok := levels[RuleMemory]; ok {
		t.Errorf("memória em 10%% não deveria alertar")
	}
}

func TestWatcherForgetsRemovedTargets(t *testing.T) {
	tl := lojaTargets()
	w, _ := newTestWatcher(DefaultRules(), tl)
	down := mkEvent(eventStatus, statusPayload(
		map[string]any{"alvo_id": 1, "porta": 3000, "tipo": "http", "no_ar": false, "hora": t0},
	))
	w.handle(context.Background(), down, t0)
	w.engine.Observe(Sample{Rule: RuleCPU, Value: 10, At: t0})

	tl.list = nil // o alvo foi apagado
	w.handle(context.Background(), mkEvent(eventStatus, statusPayload()), t0.Add(namesRefresh))

	statuses, _ := w.engine.Snapshot()
	if len(statuses) != 1 || statuses[0].Rule != RuleCPU {
		t.Fatalf("estados = %+v, quer só a CPU", statuses)
	}
}

func TestWatcherCachesNames(t *testing.T) {
	tl := lojaTargets()
	w, _ := newTestWatcher(DefaultRules(), tl)
	ev := mkEvent(eventStatus, statusPayload())
	w.handle(context.Background(), ev, t0)
	w.handle(context.Background(), ev, t0.Add(time.Second))
	w.handle(context.Background(), ev, t0.Add(namesRefresh))
	if tl.calls != 2 {
		t.Fatalf("ListTargets chamado %d vezes, quer 2", tl.calls)
	}
}

func TestWatcherFeedAndRun(t *testing.T) {
	w, got := newTestWatcher(DefaultRules(), lojaTargets())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()

	w.Feed("SCAN_LOG", map[string]any{"linhas": []string{"x"}}) // ignorado
	w.Feed(eventStatus, statusPayload(
		map[string]any{"alvo_id": 1, "porta": 3000, "tipo": "http", "no_ar": false, "hora": t0},
	))
	deadline := time.After(2 * time.Second)
	for {
		statuses, _ := w.engine.Snapshot()
		if len(statuses) > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("o evento não foi processado")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	<-done
	if len(*got) != 1 {
		t.Fatalf("alertas = %+v", *got)
	}
}

func TestWatcherFeedNeverBlocks(t *testing.T) {
	w, _ := newTestWatcher(DefaultRules(), nil) // sem Run
	done := make(chan struct{})
	go func() {
		for i := 0; i < watchBuffer*2; i++ {
			w.Feed(eventSystem, map[string]any{"cpu": 1})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Feed bloqueou com o buffer cheio")
	}
}
