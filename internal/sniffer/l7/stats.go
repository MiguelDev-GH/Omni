package l7

import (
	"math"
	"sort"
	"time"
)

const (
	// maxSamples limita as latências exatas guardadas por grupo numa janela;
	// acima disso só o histograma continua contando.
	maxSamples = 1000
	// maxGroups limita as rotas ou consultas distintas de uma janela; o
	// excedente é somado num grupo "outras".
	maxGroups = 100
	// Others é o nome do grupo que soma o excedente de maxGroups.
	Others = "(outras)"
)

// latencyBounds são os limites superiores, em ms, das faixas do histograma:
// de 0,1 ms a cerca de 100 s, crescendo 25% por faixa.
var latencyBounds = func() []float64 {
	var b []float64
	for v := 0.1; v < 100_000; v *= 1.25 {
		b = append(b, math.Round(v*1000)/1000)
	}
	return b
}()

// Histogram conta latências por faixa de latencyBounds; a última posição
// conta o que passa do maior limite. Serve para somar percentis de janelas
// diferentes, que não podem ser combinados diretamente.
type Histogram []uint64

func newHistogram() Histogram { return make(Histogram, len(latencyBounds)+1) }

func (h Histogram) observe(ms float64) {
	h[sort.SearchFloat64s(latencyBounds, ms)]++
}

// Merge soma o histograma o em h, ajustando o tamanho se preciso.
func (h Histogram) Merge(o Histogram) Histogram {
	if len(h) < len(o) {
		h = append(h, make(Histogram, len(o)-len(h))...)
	}
	for i, n := range o {
		h[i] += n
	}
	return h
}

// Quantile devolve o limite superior da faixa que contém o quantil q (0 a 1).
func (h Histogram) Quantile(q float64) float64 {
	var total uint64
	for _, n := range h {
		total += n
	}
	if total == 0 {
		return 0
	}
	rank := uint64(math.Ceil(q * float64(total)))
	var seen uint64
	for i, n := range h {
		seen += n
		if seen >= max(rank, 1) {
			if i < len(latencyBounds) {
				return latencyBounds[i]
			}
			return latencyBounds[len(latencyBounds)-1]
		}
	}
	return 0
}

// Percentile devolve o percentil q (0 a 1) de samples pelo método do posto
// mais próximo. Ordena samples.
func Percentile(samples []float64, q float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	sort.Float64s(samples)
	rank := int(math.Ceil(q*float64(len(samples)))) - 1
	return samples[min(max(rank, 0), len(samples)-1)]
}

// group acumula as latências de uma rota ou consulta numa janela.
type group struct {
	count, errors uint64
	samples       []float64
	hist          Histogram
}

func (g *group) observe(d time.Duration, isErr bool) {
	ms := durationMS(d)
	if g.hist == nil {
		g.hist = newHistogram()
	}
	g.count++
	if isErr {
		g.errors++
	}
	if len(g.samples) < maxSamples {
		g.samples = append(g.samples, ms)
	}
	g.hist.observe(ms)
}

func (g *group) merge(o *group) {
	g.count += o.count
	g.errors += o.errors
	g.samples = append(g.samples, o.samples[:min(len(o.samples), maxSamples-min(len(g.samples), maxSamples))]...)
	g.hist = g.hist.Merge(o.hist)
}

// quantiles devolve p50 e p95 exatos das amostras.
func (g *group) quantiles() (p50, p95 float64) {
	return round3(Percentile(g.samples, 0.5)), round3(Percentile(g.samples, 0.95))
}

// topGroups devolve as chaves dos maxGroups grupos com mais ocorrências e o
// excedente somado num só grupo (nil se não houver).
func topGroups(m map[string]*group) ([]string, *group) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]].count != m[keys[j]].count {
			return m[keys[i]].count > m[keys[j]].count
		}
		return keys[i] < keys[j]
	})
	if len(keys) <= maxGroups {
		return keys, nil
	}
	rest := &group{hist: newHistogram()}
	for _, k := range keys[maxGroups:] {
		rest.merge(m[k])
	}
	return keys[:maxGroups], rest
}

func durationMS(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
