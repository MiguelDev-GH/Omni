// Package load gera tráfego contra a loja cobaia, com uma mistura ponderada
// de sucessos, erros 4xx e 5xx e requisições lentas.
package load

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"
)

// UserAgent identifica a carga; não começa com "Omni-", então o sniffer a conta.
const UserAgent = "Loja-Carga/1"

const progressEvery = 10 * time.Second

type Request struct {
	Method string
	Path   string
	Body   []byte
	// User e Password, se User não for vazio, vão na autenticação Basic.
	User, Password string
	// ForwardedFor simula o IP de origem que um proxy reverso informaria.
	ForwardedFor string
}

type Kind struct {
	Name   string
	Weight int
	Build  func(r *rand.Rand) Request
}

func get(path string) Request { return Request{Method: http.MethodGet, Path: path} }

// between devolve um inteiro em [lo, hi].
func between(r *rand.Rand, lo, hi int) int { return lo + r.IntN(hi-lo+1) }

var searches = []string{"Produto%200", "o%201", "o%202", "inexistente"}

// DefaultMix é a mistura padrão. Produtos 51–60 e boa parte dos pedidos não
// existem, de propósito, para gerar 404.
var DefaultMix = []Kind{
	{"listar", 30, func(r *rand.Rand) Request {
		return get(fmt.Sprintf("/produtos?limite=20&pagina=%d", between(r, 1, 3)))
	}},
	{"produto", 25, func(r *rand.Rand) Request { return get(fmt.Sprintf("/produtos/%d", between(r, 1, 60))) }},
	{"busca", 10, func(r *rand.Rand) Request { return get("/produtos/busca?q=" + searches[r.IntN(len(searches))]) }},
	{"pedido", 10, func(r *rand.Rand) Request {
		body := fmt.Sprintf(`{"produto_id":%d,"quantidade":%d}`, between(r, 1, 50), between(r, 1, 3))
		return Request{Method: http.MethodPost, Path: "/pedidos", Body: []byte(body)}
	}},
	{"ver_pedido", 5, func(r *rand.Rand) Request { return get(fmt.Sprintf("/pedidos/%d", between(r, 1, 200))) }},
	{"lento", 5, func(r *rand.Rand) Request { return get(fmt.Sprintf("/lento?ms=%d", between(r, 200, 1500))) }},
	{"erro", 5, func(*rand.Rand) Request { return get("/erro") }},
	{"inexistente", 5, func(*rand.Rand) Request { return get("/rota-inexistente") }},
	{"raiz", 5, func(*rand.Rand) Request { return get("/") }},
}

// Pick sorteia um Kind com probabilidade proporcional ao peso.
func Pick(mix []Kind, r *rand.Rand) Kind {
	total := 0
	for _, k := range mix {
		total += k.Weight
	}
	n := r.IntN(total)
	for _, k := range mix {
		if n < k.Weight {
			return k
		}
		n -= k.Weight
	}
	return mix[len(mix)-1]
}

type Options struct {
	BaseURL string
	Rate    int // requisições por segundo
	// Duration zero roda até ctx acabar.
	Duration time.Duration
	Workers  int
	// Progress, se definido, recebe uma cópia das estatísticas a cada 10 s.
	Progress func(Stats)
}

type Stats struct {
	Total    int
	ByStatus map[int]int
	Failures int
}

// Run envia requisições na taxa pedida até Duration passar ou ctx acabar.
// Falhas de conexão são contadas e não param a carga.
func Run(ctx context.Context, c *http.Client, mix []Kind, o Options, seed uint64) Stats {
	if o.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.Duration)
		defer cancel()
	}

	var mu sync.Mutex
	st := Stats{ByStatus: map[int]int{}}
	jobs := make(chan Request)
	var wg sync.WaitGroup
	for range max(o.Workers, 1) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for req := range jobs {
				status, err := send(ctx, c, o.BaseURL, req)
				mu.Lock()
				st.Total++
				if err != nil {
					st.Failures++
				} else {
					st.ByStatus[status]++
				}
				mu.Unlock()
			}
		}()
	}

	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	tick := time.NewTicker(time.Second / time.Duration(max(o.Rate, 1)))
	defer tick.Stop()
	report := time.NewTicker(progressEvery)
	defer report.Stop()
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-report.C:
			if o.Progress != nil {
				mu.Lock()
				snap := Stats{Total: st.Total, Failures: st.Failures, ByStatus: maps.Clone(st.ByStatus)}
				mu.Unlock()
				o.Progress(snap)
			}
		case <-tick.C:
			select {
			case jobs <- Pick(mix, r).Build(r):
			case <-ctx.Done():
				break loop
			}
		}
	}
	close(jobs)
	wg.Wait()
	return st
}

// String resume st por classe de status.
func (st Stats) String() string {
	var c2, c4, c5 int
	for code, n := range st.ByStatus {
		switch code / 100 {
		case 2:
			c2 += n
		case 4:
			c4 += n
		case 5:
			c5 += n
		}
	}
	return fmt.Sprintf("total=%d, 2xx=%d, 4xx=%d, 5xx=%d, falhas=%d", st.Total, c2, c4, c5, st.Failures)
}

func send(ctx context.Context, c *http.Client, base string, r Request) (int, error) {
	var body io.Reader
	if r.Body != nil {
		body = bytes.NewReader(r.Body)
	}
	// A requisição usa um contexto próprio: a que está em voo termina mesmo
	// quando a carga acaba.
	req, err := http.NewRequestWithContext(context.WithoutCancel(ctx), r.Method, base+r.Path, body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", UserAgent)
	if r.User != "" {
		req.SetBasicAuth(r.User, r.Password)
	}
	if r.ForwardedFor != "" {
		req.Header.Set("X-Forwarded-For", r.ForwardedFor)
	}
	if r.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return resp.StatusCode, nil
}
