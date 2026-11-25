package security

import (
	"strings"
	"testing"
	"time"

	"github.com/MiguelDev-GH/omni/internal/alerts"
)

var t0 = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

func newTestTraffic() (*Traffic, *alerts.Engine, *[]alerts.Alert) {
	e := alerts.NewEngine(alerts.DefaultRules())
	var got []alerts.Alert
	e.OnAlert(func(a alerts.Alert) { got = append(got, a) })
	return NewTraffic(e), e, &got
}

func level(e *alerts.Engine, rule, subject string) (alerts.Level, float64, bool) {
	statuses, _ := e.Snapshot()
	for _, s := range statuses {
		if s.Rule == rule && s.Subject == subject {
			return s.Level, s.Value, true
		}
	}
	return "", 0, false
}

func TestBruteForceByIP(t *testing.T) {
	tr, e, got := newTestTraffic()
	for i := 0; i < 35; i++ {
		tr.Observe(Request{IP: "203.0.113.9", User: "ana", Status: 401, At: t0.Add(time.Duration(i) * time.Second)})
	}
	tr.Tick(t0.Add(36 * time.Second))

	lvl, v, ok := level(e, alerts.RuleAuthFailIP, "IP 203.0.113.9")
	if !ok || lvl != alerts.Red || v != 35 {
		t.Fatalf("falhas por IP: nível %q valor %v (ok=%v)", lvl, v, ok)
	}
	if lvl, _, _ := level(e, alerts.RuleAuthFailUser, "conta ana"); lvl != alerts.Red {
		t.Errorf("falhas por conta: %q", lvl)
	}
	if lvl, _, _ := level(e, alerts.RuleSpraying, "IP 203.0.113.9"); lvl != alerts.Green {
		t.Errorf("uma conta só não é spraying: %q", lvl)
	}
	var found bool
	for _, a := range *got {
		if a.Rule == alerts.RuleAuthFailIP && strings.Contains(a.Detail, "35 respostas 401") {
			found = true
		}
	}
	if !found {
		t.Errorf("alerta sem detalhe: %+v", *got)
	}
}

func TestPasswordSpraying(t *testing.T) {
	tr, e, _ := newTestTraffic()
	users := []string{"ana", "bia", "carlos", "davi", "eva", "fabio", "gil", "hugo", "iris", "joao", "kai"}
	for i, u := range users {
		tr.Observe(Request{IP: "198.51.100.7", User: u, Status: 401, At: t0.Add(time.Duration(i) * time.Second)})
	}
	tr.Tick(t0.Add(15 * time.Second))
	lvl, v, _ := level(e, alerts.RuleSpraying, "IP 198.51.100.7")
	if lvl != alerts.Red || v != 11 {
		t.Fatalf("spraying: %q %v", lvl, v)
	}
	// Cada conta falhou uma vez só: não é brute-force por conta.
	if lvl, _, _ := level(e, alerts.RuleAuthFailUser, "conta ana"); lvl != alerts.Green {
		t.Errorf("conta ana: %q", lvl)
	}
}

func TestSuccessfulLoginsAreNotFailures(t *testing.T) {
	tr, e, got := newTestTraffic()
	for i := 0; i < 50; i++ {
		tr.Observe(Request{IP: "10.0.0.1", User: "ana", Status: 200, At: t0.Add(time.Duration(i) * time.Second)})
	}
	tr.Tick(t0.Add(51 * time.Second))
	if _, _, ok := level(e, alerts.RuleAuthFailIP, "IP 10.0.0.1"); ok {
		t.Error("200 contado como falha")
	}
	if len(*got) != 0 {
		t.Errorf("alertas inesperados: %+v", *got)
	}
}

func TestFailuresExpireAndLevelRecovers(t *testing.T) {
	tr, e, got := newTestTraffic()
	for i := 0; i < 12; i++ {
		tr.Observe(Request{IP: "203.0.113.9", User: "ana", Status: 401, At: t0.Add(time.Duration(i) * time.Second)})
	}
	tr.Tick(t0.Add(13 * time.Second))
	if lvl, _, _ := level(e, alerts.RuleAuthFailIP, "IP 203.0.113.9"); lvl != alerts.Yellow {
		t.Fatalf("depois do ataque: %q", lvl)
	}
	// Dois minutos depois as falhas saíram da janela.
	tr.Tick(t0.Add(2 * time.Minute))
	if lvl, v, _ := level(e, alerts.RuleAuthFailIP, "IP 203.0.113.9"); lvl != alerts.Green || v != 0 {
		t.Fatalf("depois de 2 min: %q %v", lvl, v)
	}
	last := (*got)[len(*got)-1]
	if last.Level != alerts.Green {
		t.Errorf("último alerta: %+v", last)
	}
	// No tick seguinte não há mais nada a dizer sobre esse IP.
	tr.Tick(t0.Add(2*time.Minute + 5*time.Second))
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.reported) != 0 || len(tr.failIP.buckets) != 0 || len(tr.accounts) != 0 {
		t.Errorf("estado não foi limpo: reported=%v", tr.reported)
	}
}

func TestRequestsPerIP(t *testing.T) {
	tr, e, _ := newTestTraffic()
	// 120 requisições por segundo durante 50 s = 6000 no minuto.
	for s := 0; s < 50; s++ {
		for i := 0; i < 120; i++ {
			tr.Observe(Request{IP: "192.0.2.1", Status: 200, At: t0.Add(time.Duration(s) * time.Second)})
		}
	}
	tr.Tick(t0.Add(50 * time.Second))
	if lvl, v, _ := level(e, alerts.RuleIPRate, "IP 192.0.2.1"); lvl != alerts.Red || v != 6000 {
		t.Fatalf("taxa por IP: %q %v", lvl, v)
	}
}

func TestCounterWindow(t *testing.T) {
	c := newCounter()
	for i := 0; i < 70; i++ {
		c.add("x", t0.Add(time.Duration(i)*time.Second))
	}
	// Em t0+70s, a janela de 1 minuto cobre os baldes a partir de t0+20s.
	if got := c.total("x", t0.Add(70*time.Second)); got < 50 || got > 60 {
		t.Fatalf("total = %d, quer entre 50 e 60", got)
	}
	if got := c.total("nada", t0); got != 0 {
		t.Errorf("chave ausente = %d", got)
	}
}

func TestTrackedKeysAreBounded(t *testing.T) {
	tr, _, _ := newTestTraffic()
	for i := 0; i < maxTracked+50; i++ {
		tr.Observe(Request{IP: "10.0." + itoa(i/256) + "." + itoa(i%256), Status: 401, At: t0})
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.reqIP.buckets) > maxTracked || len(tr.failIP.buckets) > maxTracked {
		t.Fatalf("passou do limite: %d IPs", len(tr.reqIP.buckets))
	}
	if tr.dropped == 0 {
		t.Error("descartes não contados")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// feedRate simula rate requisições por segundo de vários IPs por d.
func feedRate(tr *Traffic, from time.Time, d time.Duration, rate int) time.Time {
	at := from
	for ; at.Before(from.Add(d)); at = at.Add(time.Second) {
		for i := 0; i < rate; i++ {
			tr.Observe(Request{IP: "10.1.0." + itoa(i%200), Status: 200, At: at})
		}
		if at.Sub(from)%TickEvery == 0 {
			tr.Tick(at)
		}
	}
	return at
}

func TestTrafficSpike(t *testing.T) {
	tr, e, _ := newTestTraffic()
	// 3 minutos de tráfego normal (20/s = 1200/min) para aprender a base.
	at := feedRate(tr, t0, 3*time.Minute, 20)
	if lvl, _, ok := level(e, alerts.RuleTrafficSpike, ""); !ok || lvl != alerts.Green {
		t.Fatalf("antes do ataque: %q (ok=%v)", lvl, ok)
	}
	// Ataque de 200/s, distribuído entre 200 IPs: nenhum IP sozinho passa do
	// limite por IP, mas o total dispara.
	at = feedRate(tr, at, 90*time.Second, 200)
	lvl, v, _ := level(e, alerts.RuleTrafficSpike, "")
	if lvl != alerts.Red || v < 6 {
		t.Fatalf("durante o ataque: %q %v", lvl, v)
	}
	if tr.baseline > 2000 {
		t.Errorf("a base aprendeu o ataque: %v", tr.baseline)
	}
	// O tráfego volta ao normal.
	feedRate(tr, at, 2*time.Minute, 20)
	if lvl, _, _ := level(e, alerts.RuleTrafficSpike, ""); lvl != alerts.Green {
		t.Fatalf("depois do ataque: %q", lvl)
	}
}

func TestSpikeIgnoresSmallVolumes(t *testing.T) {
	tr, e, _ := newTestTraffic()
	at := feedRate(tr, t0, 3*time.Minute, 1)
	feedRate(tr, at, time.Minute, 8) // 8x mais, mas só 480/min
	if lvl, v, _ := level(e, alerts.RuleTrafficSpike, ""); lvl != alerts.Green || v != 0 {
		t.Fatalf("pico pequeno: %q %v", lvl, v)
	}
}

func TestEmptyIPIgnored(t *testing.T) {
	tr, _, _ := newTestTraffic()
	tr.Observe(Request{Status: 401, At: t0})
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.reqIP.buckets) != 0 {
		t.Error("requisição sem IP contada")
	}
}
