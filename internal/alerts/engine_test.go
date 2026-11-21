package alerts

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)

func testRules() []Rule {
	return []Rule{
		{ID: "p95", Service: ServiceWeb, Name: "p95", Unit: "ms", Warn: 100, Crit: 500, For: 30 * time.Second},
		{ID: "up", Service: ServiceWeb, Name: "Disponibilidade", Warn: AvailabilityDegraded, Crit: AvailabilityDown},
	}
}

// collect registra um Listener e devolve um ponteiro para os alertas recebidos.
func collect(e *Engine) *[]Alert {
	var got []Alert
	e.OnAlert(func(a Alert) { got = append(got, a) })
	return &got
}

func feed(e *Engine, rule, subject string, start time.Time, every time.Duration, values ...float64) time.Time {
	at := start
	for _, v := range values {
		e.Observe(Sample{Rule: rule, Subject: subject, Value: v, At: at})
		at = at.Add(every)
	}
	return at
}

func TestLevelFor(t *testing.T) {
	r := Rule{Warn: 10, Crit: 20}
	cases := map[float64]Level{0: Green, 9.99: Green, 10: Yellow, 19: Yellow, 20: Red, 1e9: Red}
	for v, want := range cases {
		if got := r.LevelFor(v); got != want {
			t.Errorf("LevelFor(%v) = %s, quer %s", v, got, want)
		}
	}
}

func TestImmediateRuleAlertsOnFirstSample(t *testing.T) {
	e := NewEngine(testRules())
	got := collect(e)
	e.Observe(Sample{Rule: "up", Subject: "loja:3000", Value: AvailabilityDown, Detail: "connection refused", At: t0})

	if len(*got) != 1 {
		t.Fatalf("alertas = %d, quer 1", len(*got))
	}
	a := (*got)[0]
	if a.Level != Red || a.Previous != Green || a.Subject != "loja:3000" || a.Detail != "connection refused" {
		t.Errorf("alerta inesperado: %+v", a)
	}
	if a.Threshold != AvailabilityDown || a.ID != 1 {
		t.Errorf("limite %v, id %d", a.Threshold, a.ID)
	}
}

func TestGreenFirstSampleDoesNotAlert(t *testing.T) {
	e := NewEngine(testRules())
	got := collect(e)
	feed(e, "up", "loja:3000", t0, time.Second, 0, 0, 0)
	if len(*got) != 0 {
		t.Fatalf("alertas = %+v, quer nenhum", *got)
	}
}

func TestSpikeShorterThanForIsIgnored(t *testing.T) {
	e := NewEngine(testRules())
	got := collect(e)
	// 20 s acima do limite e depois volta ao normal.
	at := feed(e, "p95", "", t0, 10*time.Second, 600, 600, 600)
	feed(e, "p95", "", at, 10*time.Second, 10, 10, 10, 10)
	if len(*got) != 0 {
		t.Fatalf("alertas = %+v, quer nenhum", *got)
	}
}

func TestSustainedBreachEscalatesAndRecovers(t *testing.T) {
	e := NewEngine(testRules())
	got := collect(e)

	at := feed(e, "p95", "", t0, 10*time.Second, 200, 200, 200, 200) // amarelo por 30 s
	if len(*got) != 1 || (*got)[0].Level != Yellow || (*got)[0].Threshold != 100 {
		t.Fatalf("depois do amarelo: %+v", *got)
	}

	at = feed(e, "p95", "", at, 10*time.Second, 900, 900, 900, 900) // vermelho por 30 s
	if len(*got) != 2 || (*got)[1].Level != Red || (*got)[1].Previous != Yellow {
		t.Fatalf("depois do vermelho: %+v", *got)
	}

	// Ao voltar ao normal, só fica verde depois de For.
	at = feed(e, "p95", "", at, 10*time.Second, 10, 10, 10)
	if len(*got) != 2 {
		t.Fatalf("voltou ao verde cedo demais: %+v", *got)
	}
	feed(e, "p95", "", at, 10*time.Second, 10)
	if len(*got) != 3 || (*got)[2].Level != Green || (*got)[2].Previous != Red {
		t.Fatalf("depois da recuperação: %+v", *got)
	}
}

func TestDirectJumpToRed(t *testing.T) {
	e := NewEngine(testRules())
	got := collect(e)
	feed(e, "p95", "", t0, 10*time.Second, 900, 900, 900, 900)
	if len(*got) != 1 || (*got)[0].Level != Red || (*got)[0].Previous != Green {
		t.Fatalf("alertas = %+v, quer um vermelho direto", *got)
	}
}

func TestStaleGapResetsTimers(t *testing.T) {
	e := NewEngine(testRules())
	got := collect(e)
	e.Observe(Sample{Rule: "p95", Value: 900, At: t0})
	// A próxima amostra ruim chega muito depois: não conta como 1 h ruim.
	e.Observe(Sample{Rule: "p95", Value: 900, At: t0.Add(time.Hour)})
	if len(*got) != 0 {
		t.Fatalf("alertas = %+v, quer nenhum", *got)
	}
}

func TestSubjectsAreIndependent(t *testing.T) {
	e := NewEngine(testRules())
	got := collect(e)
	e.Observe(Sample{Rule: "up", Subject: "a:1", Value: AvailabilityDown, At: t0})
	e.Observe(Sample{Rule: "up", Subject: "b:2", Value: AvailabilityUp, At: t0})
	statuses, _ := e.Snapshot()
	if len(*got) != 1 || len(statuses) != 2 {
		t.Fatalf("alertas %d, estados %d", len(*got), len(statuses))
	}
	if statuses[0].Subject != "a:1" || statuses[0].Level != Red {
		t.Errorf("o mais grave deveria vir primeiro: %+v", statuses)
	}
}

func TestServiceOverride(t *testing.T) {
	e := NewEngine(testRules())
	got := collect(e)
	e.Observe(Sample{Rule: "up", Service: ServiceDB, Subject: "loja:5433", Value: AvailabilityDown, At: t0})
	if (*got)[0].Service != ServiceDB {
		t.Errorf("serviço = %q, quer %q", (*got)[0].Service, ServiceDB)
	}
}

func TestUnknownRuleIgnored(t *testing.T) {
	e := NewEngine(testRules())
	got := collect(e)
	e.Observe(Sample{Rule: "nao-existe", Value: 1e9, At: t0})
	statuses, _ := e.Snapshot()
	if len(*got) != 0 || len(statuses) != 0 {
		t.Fatalf("regra desconhecida gerou estado: %+v %+v", *got, statuses)
	}
}

func TestHistoryNewestFirstAndBounded(t *testing.T) {
	e := NewEngine(testRules())
	at := t0
	for i := 0; i < historyLimit+10; i++ {
		v := float64(AvailabilityDown)
		if i%2 == 1 {
			v = AvailabilityUp
		}
		e.Observe(Sample{Rule: "up", Subject: "x:1", Value: v, At: at})
		at = at.Add(time.Second)
	}
	_, history := e.Snapshot()
	if len(history) != historyLimit {
		t.Fatalf("histórico = %d, quer %d", len(history), historyLimit)
	}
	if history[0].ID <= history[1].ID {
		t.Errorf("histórico fora de ordem: %d antes de %d", history[0].ID, history[1].ID)
	}
}

func TestForget(t *testing.T) {
	e := NewEngine(testRules())
	e.Observe(Sample{Rule: "up", Subject: "a:1", Value: AvailabilityDown, At: t0})
	e.Observe(Sample{Rule: "p95", Value: 900, At: t0})
	e.Forget(func(s string) bool { return s == "a:1" })
	statuses, _ := e.Snapshot()
	if len(statuses) != 1 || statuses[0].Rule != "p95" {
		t.Fatalf("estados = %+v", statuses)
	}
}

func TestAlertTitle(t *testing.T) {
	cases := []struct {
		a    Alert
		want string
	}{
		{Alert{Name: "Disponibilidade", Subject: "loja:3000", Level: Red}, "CRÍTICO: Disponibilidade em loja:3000"},
		{Alert{Name: "Uso de CPU", Service: ServiceSystem, Level: Yellow}, "Atenção: Uso de CPU em sistema"},
		{Alert{Name: "p95", Subject: "web", Level: Green}, "p95 normalizado: web"},
	}
	for _, c := range cases {
		if got := c.a.Title(); got != c.want {
			t.Errorf("Title() = %q, quer %q", got, c.want)
		}
	}
}

func TestDefaultRulesAreConsistent(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range DefaultRules() {
		if seen[r.ID] {
			t.Errorf("regra repetida: %s", r.ID)
		}
		seen[r.ID] = true
		if r.Warn > r.Crit {
			t.Errorf("%s: limite amarelo %v maior que o vermelho %v", r.ID, r.Warn, r.Crit)
		}
		if r.Name == "" || r.Service == "" {
			t.Errorf("%s sem nome ou serviço", r.ID)
		}
	}
}
