package alerts

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/MiguelDev-GH/omni/internal/target"
)

// EventAlert é o tipo do evento de WebSocket que leva um Alert.
const EventAlert = "ALERTA"

// Tipos de evento que o Watcher observa. São os mesmos que o frontend
// recebe, documentados no README.
const (
	eventStatus  = "STATUS_ALVOS"
	eventHTTP    = "METRICAS_HTTP"
	eventDB      = "METRICAS_BANCO"
	eventSystem  = "METRICAS_SISTEMA"
	watchBuffer  = 512
	namesRefresh = 10 * time.Second
)

// Volume mínimo numa janela para as taxas de erro e o p95 valerem: com
// poucas requisições, uma única falha viraria "50% de erro".
const (
	minHTTPRequests = 20
	minDBQueries    = 20
)

// A taxa de falhas das sondas DNS e SMTP é calculada sobre as últimas
// probeWindow verificações de cada porta, e só depois de probeMinSamples.
const (
	probeWindow     = 10
	probeMinSamples = 5
)

// TargetLister é o que o Watcher precisa do armazenamento para dar nome
// aos recursos.
type TargetLister interface {
	ListTargets(ctx context.Context) ([]target.Target, error)
}

// rawEvent guarda o evento já em JSON: serializar na hora do Feed garante
// que o motor que publicou pode reaproveitar os próprios mapas e fatias sem
// corrida com a goroutine do Watcher.
type rawEvent struct {
	typ  string
	data []byte
}

// Watcher recebe os eventos publicados pelos motores, extrai as métricas e
// as entrega ao Engine numa goroutine própria.
type Watcher struct {
	engine  *Engine
	targets TargetLister
	events  chan rawEvent

	names   map[int64]string
	namesAt time.Time
	// probes guarda, por recurso, se cada uma das últimas verificações falhou.
	probes map[string][]bool
}

// NewWatcher cria um Watcher. Chame Run para começar a processar.
func NewWatcher(e *Engine, tl TargetLister) *Watcher {
	return &Watcher{
		engine:  e,
		targets: tl,
		events:  make(chan rawEvent, watchBuffer),
		names:   map[int64]string{},
		probes:  map[string][]bool{},
	}
}

// Feed entrega um evento sem bloquear. Eventos de outros tipos são
// ignorados; com o buffer cheio, o evento é descartado.
func (w *Watcher) Feed(eventType string, data any) {
	switch eventType {
	case eventStatus, eventHTTP, eventDB, eventSystem:
	default:
		return
	}
	b, err := json.Marshal(data)
	if err != nil {
		slog.Error("alertas: serializar evento", "tipo", eventType, "erro", err)
		return
	}
	select {
	case w.events <- rawEvent{typ: eventType, data: b}:
	default:
		slog.Warn("alertas: buffer cheio, evento descartado", "tipo", eventType)
	}
}

// Run processa os eventos até ctx ser cancelado.
func (w *Watcher) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-w.events:
			w.handle(ctx, ev, time.Now().UTC())
		}
	}
}

// Formatos dos eventos, iguais ao JSON enviado ao frontend.
type (
	statusEvent struct {
		Results []struct {
			TargetID   int64     `json:"alvo_id"`
			Port       int       `json:"porta"`
			Kind       string    `json:"tipo"`
			Up         bool      `json:"no_ar"`
			StatusCode int       `json:"status_http"`
			LatencyMS  float64   `json:"latencia_ms"`
			Err        string    `json:"erro"`
			At         time.Time `json:"hora"`
			Degraded   bool      `json:"degradado"`
			Detail     string    `json:"detalhe"`
			StartTLS   *bool     `json:"starttls"`
		} `json:"resultados"`
	}
	httpEvent struct {
		Requests uint64            `json:"requisicoes"`
		ByStatus map[string]uint64 `json:"status"`
		P95MS    float64           `json:"p95_ms"`
	}
	dbEvent struct {
		Queries uint64  `json:"consultas"`
		Errors  uint64  `json:"erros"`
		P95MS   float64 `json:"p95_ms"`
	}
	systemEvent struct {
		CPU    float64   `json:"cpu"`
		Memory float64   `json:"memoria"`
		Disk   float64   `json:"disco"`
		At     time.Time `json:"hora"`
	}
)

// decode lê o evento pelo mesmo JSON que vai ao frontend, sem depender dos
// tipos internos de cada motor.
func decode(data []byte, v any) error { return json.Unmarshal(data, v) }

func (w *Watcher) handle(ctx context.Context, ev rawEvent, now time.Time) {
	switch ev.typ {
	case eventStatus:
		var s statusEvent
		if err := decode(ev.data, &s); err != nil {
			slog.Error("alertas: ler STATUS_ALVOS", "erro", err)
			return
		}
		w.refreshNames(ctx, now)
		for _, r := range s.Results {
			at := r.At
			if at.IsZero() {
				at = now
			}
			service, latencyRule := ServiceWeb, RuleProbeLatency
			switch r.Kind {
			case "tcp":
				service = ServiceDB
			case "dns":
				service, latencyRule = ServiceDNS, RuleDNSLatency
			case "smtp":
				service, latencyRule = ServiceSMTP, RuleSMTPLatency
			}
			subject := w.subject(r.TargetID, r.Port)
			value, detail := float64(AvailabilityUp), ""
			switch {
			case !r.Up:
				value, detail = AvailabilityDown, r.Err
			case r.Degraded:
				value, detail = AvailabilityDegraded, r.Detail
			case r.Kind == "http" && r.StatusCode >= 500:
				value, detail = AvailabilityDegraded, "HTTP "+strconv.Itoa(r.StatusCode)
			}
			w.engine.Observe(Sample{Rule: RuleAvailability, Service: service, Subject: subject,
				Value: value, Detail: detail, At: at})
			if r.Up {
				w.engine.Observe(Sample{Rule: latencyRule, Service: service, Subject: subject,
					Value: r.LatencyMS, At: at})
			}
			if service == ServiceDNS || service == ServiceSMTP {
				if rate, n, ok := w.failureRate(subject, value != AvailabilityUp); ok {
					w.engine.Observe(Sample{Rule: RuleProbeErrors, Service: service, Subject: subject, Value: rate,
						Detail: fmt.Sprintf("%d de %d verificações recentes falharam", n, len(w.probes[subject])), At: at})
				}
			}
			if r.StartTLS != nil {
				v := 0.0
				if !*r.StartTLS {
					v = 1
				}
				w.engine.Observe(Sample{Rule: RuleSMTPNoTLS, Service: service, Subject: subject, Value: v, At: at})
			}
		}
	case eventHTTP:
		var h httpEvent
		if err := decode(ev.data, &h); err != nil {
			slog.Error("alertas: ler METRICAS_HTTP", "erro", err)
			return
		}
		if h.Requests < minHTTPRequests {
			return
		}
		rate := float64(h.ByStatus["5xx"]) / float64(h.Requests) * 100
		detail := fmt.Sprintf("%d de %d requisições nos últimos 10 s", h.ByStatus["5xx"], h.Requests)
		w.engine.Observe(Sample{Rule: RuleHTTP5xx, Value: rate, Detail: detail, At: now})
		w.engine.Observe(Sample{Rule: RuleHTTPP95, Value: h.P95MS, At: now})
	case eventDB:
		var d dbEvent
		if err := decode(ev.data, &d); err != nil {
			slog.Error("alertas: ler METRICAS_BANCO", "erro", err)
			return
		}
		if d.Queries < minDBQueries {
			return
		}
		rate := float64(d.Errors) / float64(d.Queries) * 100
		detail := fmt.Sprintf("%d de %d consultas nos últimos 10 s", d.Errors, d.Queries)
		w.engine.Observe(Sample{Rule: RuleDBErrors, Value: rate, Detail: detail, At: now})
		w.engine.Observe(Sample{Rule: RuleDBP95, Value: d.P95MS, At: now})
	case eventSystem:
		var s systemEvent
		if err := decode(ev.data, &s); err != nil {
			slog.Error("alertas: ler METRICAS_SISTEMA", "erro", err)
			return
		}
		at := s.At
		if at.IsZero() {
			at = now
		}
		w.engine.Observe(Sample{Rule: RuleCPU, Value: s.CPU, At: at})
		w.engine.Observe(Sample{Rule: RuleMemory, Value: s.Memory, At: at})
		w.engine.Observe(Sample{Rule: RuleDisk, Value: s.Disk, At: at})
	}
}

// failureRate registra a verificação mais recente de subject e devolve a
// porcentagem de falhas na janela e quantas falharam. ok é falso enquanto
// a janela tem menos de probeMinSamples verificações.
func (w *Watcher) failureRate(subject string, failed bool) (float64, int, bool) {
	win := append(w.probes[subject], failed)
	if len(win) > probeWindow {
		win = win[len(win)-probeWindow:]
	}
	w.probes[subject] = win
	if len(win) < probeMinSamples {
		return 0, 0, false
	}
	n := 0
	for _, f := range win {
		if f {
			n++
		}
	}
	return float64(n) / float64(len(win)) * 100, n, true
}

// subject monta "nome:porta", ou "alvo 3:porta" se o nome não for conhecido.
func (w *Watcher) subject(id int64, port int) string {
	name, ok := w.names[id]
	if !ok {
		name = "alvo " + strconv.FormatInt(id, 10)
	}
	return name + ":" + strconv.Itoa(port)
}

// refreshNames relê os nomes dos alvos no máximo a cada namesRefresh e
// esquece o estado das portas de alvos removidos.
func (w *Watcher) refreshNames(ctx context.Context, now time.Time) {
	if w.targets == nil || (!w.namesAt.IsZero() && now.Sub(w.namesAt) < namesRefresh) {
		return
	}
	list, err := w.targets.ListTargets(ctx)
	if err != nil {
		slog.Error("alertas: listar alvos", "erro", err)
		return
	}
	w.namesAt = now
	names := make(map[int64]string, len(list))
	current := make(map[string]bool)
	for _, t := range list {
		names[t.ID] = t.Name
		for _, p := range t.AppPorts {
			current[t.Name+":"+strconv.Itoa(p)] = true
		}
		for _, p := range []int{t.DBPort, t.DNSPort, t.SMTPPort} {
			if p != 0 {
				current[t.Name+":"+strconv.Itoa(p)] = true
			}
		}
	}
	for subject := range w.probes {
		if !current[subject] {
			delete(w.probes, subject)
		}
	}
	w.names = names
	// Só portas de alvo ("nome:porta") são esquecidas. Métricas globais não
	// têm recurso, e os recursos de segurança (IPs, contas, arquivos) são
	// cuidados pelos próprios detectores.
	w.engine.Forget(func(service, subject string) bool {
		switch service {
		case ServiceWeb, ServiceDB, ServiceDNS, ServiceSMTP:
			return strings.Contains(subject, ":") && !current[subject]
		}
		return false
	})
}
