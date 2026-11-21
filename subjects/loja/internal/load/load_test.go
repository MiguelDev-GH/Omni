package load

import (
	"context"
	"encoding/json"
	"math"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPickRespectsWeights(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	total := 0
	for _, k := range DefaultMix {
		total += k.Weight
	}
	counts := map[string]int{}
	const n = 100_000
	for range n {
		counts[Pick(DefaultMix, r).Name]++
	}
	for _, k := range DefaultMix {
		want := float64(k.Weight) / float64(total)
		got := float64(counts[k.Name]) / n
		assert.LessOrEqual(t, math.Abs(got-want), 0.01, k.Name)
	}
}

func TestDefaultMixPaths(t *testing.T) {
	prefixes := map[string]string{
		"listar": "GET /produtos?limite=20&pagina=", "produto": "GET /produtos/", "busca": "GET /produtos/busca?q=",
		"pedido": "POST /pedidos", "ver_pedido": "GET /pedidos/", "lento": "GET /lento?ms=", "erro": "GET /erro",
		"inexistente": "GET /rota-inexistente", "raiz": "GET /",
	}
	weights := map[string]int{"listar": 30, "produto": 25, "busca": 10, "pedido": 10, "ver_pedido": 5,
		"lento": 5, "erro": 5, "inexistente": 5, "raiz": 5}
	require.Len(t, DefaultMix, len(prefixes))
	r := rand.New(rand.NewPCG(3, 4))
	for _, k := range DefaultMix {
		assert.Equal(t, weights[k.Name], k.Weight, k.Name)
		for range 200 {
			req := k.Build(r)
			line := req.Method + " " + req.Path
			assert.True(t, strings.HasPrefix(line, prefixes[k.Name]), line)
			if k.Name == "pedido" {
				var body struct {
					ProductID int64 `json:"produto_id"`
					Quantity  int   `json:"quantidade"`
				}
				require.NoError(t, json.Unmarshal(req.Body, &body))
				assert.True(t, body.ProductID >= 1 && body.ProductID <= 50)
				assert.True(t, body.Quantity >= 1 && body.Quantity <= 3)
			}
		}
	}
}

func TestRunHitsServer(t *testing.T) {
	var mu sync.Mutex
	agents := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		agents[r.UserAgent()]++
		mu.Unlock()
		if r.URL.Path == "/erro" {
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()

	st := Run(context.Background(), srv.Client(), DefaultMix,
		Options{BaseURL: srv.URL, Rate: 200, Duration: 300 * time.Millisecond, Workers: 4}, 1)
	assert.GreaterOrEqual(t, st.Total, 30)
	assert.LessOrEqual(t, st.Total, 70)
	assert.Equal(t, 0, st.Failures)
	sum := 0
	for _, n := range st.ByStatus {
		sum += n
	}
	assert.Equal(t, st.Total-st.Failures, sum)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, map[string]int{"Loja-Carga/1": st.Total}, agents)
}

func TestRunCountsConnectionFailures(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	st := Run(context.Background(), &http.Client{Timeout: time.Second}, DefaultMix,
		Options{BaseURL: url, Rate: 100, Duration: 200 * time.Millisecond, Workers: 2}, 1)
	assert.Positive(t, st.Total)
	assert.Equal(t, st.Total, st.Failures)
}
