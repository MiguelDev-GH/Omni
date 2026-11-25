package l7

import "time"

// DNSWindow resume as consultas DNS de um intervalo, normalmente 10 s.
type DNSWindow struct {
	Start    time.Time         `json:"inicio"`
	Queries  uint64            `json:"consultas"`
	Failures uint64            `json:"falhas"`
	ByRCode  map[string]uint64 `json:"rcodes"`
	// P50MS e P95MS são o tempo de resolução das consultas respondidas.
	P50MS float64 `json:"p50_ms"`
	P95MS float64 `json:"p95_ms"`
}

// DNSStats agrega os DNSExchange da janela atual. Não é seguro para uso
// concorrente.
type DNSStats struct {
	start    time.Time
	queries  uint64
	failures uint64
	byRCode  map[string]uint64
	answered group
}

func NewDNSStats() *DNSStats {
	s := &DNSStats{}
	s.reset(time.Time{})
	return s
}

func (s *DNSStats) reset(start time.Time) {
	s.start = start
	s.queries, s.failures = 0, 0
	s.byRCode = map[string]uint64{}
	s.answered = group{}
}

// Add conta ex na janela.
func (s *DNSStats) Add(ex DNSExchange) {
	s.queries++
	if ex.Failed {
		s.failures++
	}
	s.byRCode[ex.RCode]++
	if ex.RCode != rcodeTimeout {
		s.answered.observe(ex.Latency, ex.Failed)
	}
}

// Flush devolve a janela iniciada no Flush anterior e começa outra em now.
// A primeira janela começa em now.
func (s *DNSStats) Flush(now time.Time) DNSWindow {
	start := s.start
	if start.IsZero() {
		start = now
	}
	w := DNSWindow{Start: start.UTC(), Queries: s.queries, Failures: s.failures, ByRCode: s.byRCode}
	w.P50MS, w.P95MS = s.answered.quantiles()
	s.reset(now)
	return w
}
