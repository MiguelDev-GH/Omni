package l7

import "time"

// DBWindow resume as consultas ao banco de um intervalo, normalmente 10 s.
type DBWindow struct {
	Start   time.Time         `json:"inicio"`
	Queries uint64            `json:"consultas"`
	Errors  uint64            `json:"erros"`
	ByKind  map[string]uint64 `json:"tipos"` // "SELECT", "INSERT"...
	P50MS   float64           `json:"p50_ms"`
	P95MS   float64           `json:"p95_ms"`
	Top     []QueryStats      `json:"top"`
}

// QueryStats resume uma consulta normalizada numa janela.
type QueryStats struct {
	Query  string    `json:"consulta"`
	Kind   string    `json:"tipo"`
	Count  uint64    `json:"execucoes"`
	Errors uint64    `json:"erros"`
	P50MS  float64   `json:"p50_ms"`
	P95MS  float64   `json:"p95_ms"`
	Hist   Histogram `json:"-"`
}

// DBStats agrega os DBQuery da janela atual. Não é seguro para uso concorrente.
type DBStats struct {
	start   time.Time
	all     group
	byKind  map[string]uint64
	queries map[string]*group
}

func NewDBStats() *DBStats {
	s := &DBStats{}
	s.reset(time.Time{})
	return s
}

func (s *DBStats) reset(start time.Time) {
	s.start = start
	s.all = group{}
	s.byKind = map[string]uint64{}
	s.queries = map[string]*group{}
}

// Add conta q na janela.
func (s *DBStats) Add(q DBQuery) {
	s.all.observe(q.Latency, q.Error)
	s.byKind[q.Kind]++
	g := s.queries[q.Query]
	if g == nil {
		g = &group{}
		s.queries[q.Query] = g
	}
	g.observe(q.Latency, q.Error)
}

// Flush devolve a janela iniciada no Flush anterior e começa outra em now.
// A primeira janela começa em now.
func (s *DBStats) Flush(now time.Time) DBWindow {
	start := s.start
	if start.IsZero() {
		start = now
	}
	w := DBWindow{Start: start.UTC(), Queries: s.all.count, Errors: s.all.errors, ByKind: s.byKind, Top: []QueryStats{}}
	w.P50MS, w.P95MS = s.all.quantiles()
	keys, rest := topGroups(s.queries)
	for _, k := range keys {
		w.Top = append(w.Top, queryStats(k, QueryKind(k), s.queries[k]))
	}
	if rest != nil {
		w.Top = append(w.Top, queryStats(Others, "OUTRO", rest))
	}
	s.reset(now)
	return w
}

func queryStats(q, kind string, g *group) QueryStats {
	r := QueryStats{Query: q, Kind: kind, Count: g.count, Errors: g.errors, Hist: g.hist}
	r.P50MS, r.P95MS = g.quantiles()
	return r
}
