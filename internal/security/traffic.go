// Package security detecta ataques no tráfego HTTP (brute-force, password
// spraying, excesso de requisições por IP e picos de tráfego) e alterações
// em arquivos de configuração. Os resultados viram amostras do motor de
// alertas, no serviço "seguranca".
package security

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/MiguelDev-GH/omni/internal/alerts"
)

const (
	// window é o período das contagens: falhas e requisições "por minuto".
	window = time.Minute
	// bucket é a resolução das contagens dentro da janela.
	bucket = 10 * time.Second
	// TickEvery é a frequência com que as contagens viram amostras.
	TickEvery = 5 * time.Second
	// maxTracked limita quantos IPs e contas ficam em memória. Num ataque
	// com IPs forjados, o próprio detector não pode virar o alvo.
	maxTracked = 10000
	// spikeMinPerMin é o volume mínimo para o pico de tráfego valer: sair
	// de 2 para 10 requisições por minuto não é DDoS.
	spikeMinPerMin = 600
	// spikeWarmup é quantos ticks a linha de base precisa antes de valer.
	spikeWarmup = 12
	// spikeAlpha é o peso de cada tick novo na média móvel exponencial.
	spikeAlpha = 0.05
)

// Request é o que o detector precisa de cada requisição observada.
type Request struct {
	IP     string // IP de origem (já resolvido atrás do proxy)
	User   string // usuário da autenticação Basic, se houver
	Status int
	At     time.Time
}

// failed indica uma tentativa de login recusada.
func (r Request) failed() bool { return r.Status == 401 }

// counter conta eventos por chave em baldes de 10 s cobrindo 1 minuto.
type counter struct {
	buckets map[string]*[6]slot
}

type slot struct {
	start time.Time
	n     int
}

func newCounter() *counter { return &counter{buckets: map[string]*[6]slot{}} }

func slotStart(at time.Time) time.Time { return at.Truncate(bucket) }

// add soma um evento de key em at. Devolve falso se a chave é nova e o
// limite de chaves já foi atingido.
func (c *counter) add(key string, at time.Time) bool {
	b, ok := c.buckets[key]
	if !ok {
		if len(c.buckets) >= maxTracked {
			return false
		}
		b = &[6]slot{}
		c.buckets[key] = b
	}
	start := slotStart(at)
	i := int(start.Unix()/int64(bucket/time.Second)) % len(b)
	if !b[i].start.Equal(start) {
		b[i] = slot{start: start}
	}
	b[i].n++
	return true
}

// total soma os eventos de key no último minuto até now.
func (c *counter) total(key string, now time.Time) int {
	b, ok := c.buckets[key]
	if !ok {
		return 0
	}
	n := 0
	for _, s := range b {
		if !s.start.IsZero() && now.Sub(s.start) < window {
			n += s.n
		}
	}
	return n
}

// Traffic recebe as requisições do sniffer e, a cada tick, entrega ao motor
// de alertas as contagens por IP e por conta e a razão do pico de tráfego.
// É seguro para uso concorrente.
type Traffic struct {
	engine *alerts.Engine

	mu       sync.Mutex
	failIP   *counter
	failUser *counter
	reqIP    *counter
	// accounts guarda, por IP, as contas que falharam e quando.
	accounts map[string]map[string]time.Time
	total    *counter
	// reported marca os recursos que já têm amostra no motor, para mandar
	// um zero quando a atividade acaba e o nível poder voltar ao verde.
	reported map[string]bool
	dropped  int

	baseline float64 // requisições por minuto em tráfego normal
	ticks    int
}

// NewTraffic cria o detector.
func NewTraffic(e *alerts.Engine) *Traffic {
	return &Traffic{
		engine:   e,
		failIP:   newCounter(),
		failUser: newCounter(),
		reqIP:    newCounter(),
		total:    newCounter(),
		accounts: map[string]map[string]time.Time{},
		reported: map[string]bool{},
	}
}

// Observe registra uma requisição. Não bloqueia.
func (t *Traffic) Observe(r Request) {
	if r.IP == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.total.add("", r.At)
	if !t.reqIP.add(r.IP, r.At) {
		t.dropped++
	}
	if !r.failed() {
		return
	}
	t.failIP.add(r.IP, r.At)
	if r.User == "" {
		return
	}
	t.failUser.add(r.User, r.At)
	acc, ok := t.accounts[r.IP]
	if !ok {
		if len(t.accounts) >= maxTracked {
			t.dropped++
			return
		}
		acc = map[string]time.Time{}
		t.accounts[r.IP] = acc
	}
	if _, seen := acc[r.User]; seen || len(acc) < 1000 {
		acc[r.User] = r.At
	}
}

// Run entrega as amostras a cada TickEvery até ctx ser cancelado.
func (t *Traffic) Run(ctx context.Context) {
	tk := time.NewTicker(TickEvery)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tk.C:
			t.Tick(now.UTC())
		}
	}
}

func ipSubject(ip string) string      { return "IP " + ip }
func userSubject(u string) string     { return "conta " + u }
func reportKey(rule, s string) string { return rule + "|" + s }

// Tick calcula as contagens do último minuto e as entrega ao motor.
func (t *Traffic) Tick(now time.Time) {
	t.mu.Lock()
	samples := t.collect(now)
	if t.dropped > 0 {
		slog.Warn("segurança: limite de IPs e contas acompanhados atingido", "descartados", t.dropped)
		t.dropped = 0
	}
	t.mu.Unlock()
	for _, s := range samples {
		t.engine.Observe(s)
	}
}

func (t *Traffic) collect(now time.Time) []alerts.Sample {
	var out []alerts.Sample
	seen := map[string]bool{}
	emit := func(rule, subject string, value float64, detail string) {
		key := reportKey(rule, subject)
		seen[key] = true
		if value == 0 && !t.reported[key] {
			return // nada a dizer sobre quem nunca passou de zero
		}
		t.reported[key] = value != 0
		out = append(out, alerts.Sample{Rule: rule, Service: alerts.ServiceSecurity, Subject: subject,
			Value: value, Detail: detail, At: now})
	}

	for ip := range t.reqIP.buckets {
		n := t.reqIP.total(ip, now)
		emit(alerts.RuleIPRate, ipSubject(ip), float64(n), "")
		if n == 0 {
			delete(t.reqIP.buckets, ip)
		}
	}
	for ip := range t.failIP.buckets {
		n := t.failIP.total(ip, now)
		emit(alerts.RuleAuthFailIP, ipSubject(ip), float64(n), fmt.Sprintf("%d respostas 401 no último minuto", n))
		if n == 0 {
			delete(t.failIP.buckets, ip)
		}
	}
	for user := range t.failUser.buckets {
		n := t.failUser.total(user, now)
		emit(alerts.RuleAuthFailUser, userSubject(user), float64(n), fmt.Sprintf("%d senhas erradas no último minuto", n))
		if n == 0 {
			delete(t.failUser.buckets, user)
		}
	}
	for ip, acc := range t.accounts {
		for user, at := range acc {
			if now.Sub(at) >= window {
				delete(acc, user)
			}
		}
		emit(alerts.RuleSpraying, ipSubject(ip), float64(len(acc)), fmt.Sprintf("%d contas diferentes no último minuto", len(acc)))
		if len(acc) == 0 {
			delete(t.accounts, ip)
		}
	}
	// Recursos que sumiram de todos os mapas e ainda estavam acima de zero.
	for key, active := range t.reported {
		if !seen[key] {
			if active {
				rule, subject := splitKey(key)
				out = append(out, alerts.Sample{Rule: rule, Service: alerts.ServiceSecurity, Subject: subject, At: now})
			}
			delete(t.reported, key)
		}
	}

	if s, ok := t.spike(now); ok {
		out = append(out, s)
	}
	return out
}

func splitKey(key string) (string, string) {
	for i := 0; i < len(key); i++ {
		if key[i] == '|' {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}

// spike compara o volume do último minuto com a linha de base, uma média
// móvel exponencial que só aprende com tráfego normal: durante um ataque
// ela fica congelada, senão o próprio ataque viraria "o normal".
func (t *Traffic) spike(now time.Time) (alerts.Sample, bool) {
	current := float64(t.total.total("", now))
	t.ticks++
	if t.ticks == 1 || t.baseline == 0 {
		t.baseline = current
		return alerts.Sample{}, false
	}
	ratio := 0.0
	if current >= spikeMinPerMin {
		ratio = current / max(t.baseline, 1)
	}
	if t.ticks <= spikeWarmup {
		t.baseline += spikeAlpha * 4 * (current - t.baseline) // aprende mais rápido no início
		return alerts.Sample{}, false
	}
	if ratio < 3 {
		t.baseline += spikeAlpha * (current - t.baseline)
	}
	return alerts.Sample{
		Rule: alerts.RuleTrafficSpike, Service: alerts.ServiceSecurity, Value: round1(ratio), At: now,
		Detail: fmt.Sprintf("%.0f requisições no último minuto; o normal é %.0f", current, t.baseline),
	}, true
}

func round1(v float64) float64 { return float64(int(v*10+0.5)) / 10 }
