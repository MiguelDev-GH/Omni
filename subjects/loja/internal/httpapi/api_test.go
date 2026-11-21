package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MiguelDev-GH/omni/subjects/loja/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStore guarda os produtos em memória e registra as chamadas.
type fakeStore struct {
	down       bool
	hang       bool
	outOfStock bool
	products   map[int64]store.Product

	listArgs  [2]int
	searchArg string
	sleepArg  time.Duration
	getCalled bool
}

func newFake() *fakeStore {
	return &fakeStore{products: map[int64]store.Product{1: {ID: 1, Name: "Produto 01", PriceCents: 1137, Stock: 100}}}
}

func (f *fakeStore) Ping(context.Context) error {
	if f.down {
		return store.ErrUnavailable
	}
	return nil
}

func (f *fakeStore) ListProducts(_ context.Context, limit, offset int) ([]store.Product, error) {
	if f.down {
		return nil, store.ErrUnavailable
	}
	f.listArgs = [2]int{limit, offset}
	return []store.Product{f.products[1]}, nil
}

func (f *fakeStore) GetProduct(ctx context.Context, id int64) (store.Product, error) {
	f.getCalled = true
	if f.hang {
		<-ctx.Done()
		return store.Product{}, ctx.Err()
	}
	if f.down {
		return store.Product{}, store.ErrUnavailable
	}
	p, ok := f.products[id]
	if !ok {
		return store.Product{}, store.ErrNotFound
	}
	return p, nil
}

func (f *fakeStore) SearchProducts(_ context.Context, q string) ([]store.Product, error) {
	if f.down {
		return nil, store.ErrUnavailable
	}
	f.searchArg = q
	return []store.Product{}, nil
}

func (f *fakeStore) CreateOrder(_ context.Context, productID int64, qty int) (store.Order, error) {
	switch {
	case f.down:
		return store.Order{}, store.ErrUnavailable
	case f.outOfStock:
		return store.Order{}, store.ErrOutOfStock
	}
	if _, ok := f.products[productID]; !ok {
		return store.Order{}, store.ErrNotFound
	}
	return store.Order{ID: 7, ProductID: productID, Quantity: qty}, nil
}

func (f *fakeStore) GetOrder(_ context.Context, id int64) (store.Order, error) {
	if f.down {
		return store.Order{}, store.ErrUnavailable
	}
	if id != 7 {
		return store.Order{}, store.ErrNotFound
	}
	return store.Order{ID: 7, ProductID: 1, Quantity: 2}, nil
}

func (f *fakeStore) Sleep(_ context.Context, d time.Duration) error {
	if f.down {
		return store.ErrUnavailable
	}
	f.sleepArg = d
	return nil
}

func (f *fakeStore) Broken(context.Context) error {
	if f.down {
		return store.ErrUnavailable
	}
	return assert.AnError
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errorOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Erro string `json:"erro"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	return body.Erro
}

func TestRoot(t *testing.T) {
	rec := do(t, New(newFake()), "GET", "/", "")
	assert.Equal(t, 200, rec.Code)
	assert.JSONEq(t, `{"servico":"loja","status":"ok"}`, rec.Body.String())
	assert.Equal(t, "Loja/0.1", rec.Header().Get("Server"))
	assert.Equal(t, "Go", rec.Header().Get("X-Powered-By"))
	assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
}

func TestRootIgnoresDB(t *testing.T) {
	f := newFake()
	f.down = true
	assert.Equal(t, 200, do(t, New(f), "GET", "/", "").Code)
}

func TestHealth(t *testing.T) {
	rec := do(t, New(newFake()), "GET", "/saude", "")
	assert.Equal(t, 200, rec.Code)
	assert.JSONEq(t, `{"banco":"ok"}`, rec.Body.String())
}

func TestDataRoutesReturn503WhenDBDown(t *testing.T) {
	f := newFake()
	f.down = true
	h := New(f)
	for _, r := range []struct{ method, path, body string }{
		{"GET", "/saude", ""},
		{"GET", "/produtos", ""},
		{"GET", "/produtos/1", ""},
		{"GET", "/produtos/busca?q=a", ""},
		{"POST", "/pedidos", `{"produto_id":1,"quantidade":1}`},
		{"GET", "/pedidos/1", ""},
		{"GET", "/lento?ms=0", ""},
		{"GET", "/erro", ""},
	} {
		rec := do(t, h, r.method, r.path, r.body)
		assert.Equal(t, 503, rec.Code, r.path)
		assert.Equal(t, "banco indisponível", errorOf(t, rec), r.path)
	}
}

func TestGetProduct(t *testing.T) {
	h := New(newFake())
	rec := do(t, h, "GET", "/produtos/1", "")
	assert.Equal(t, 200, rec.Code)
	assert.JSONEq(t, `{"id":1,"nome":"Produto 01","preco_centavos":1137,"estoque":100}`, rec.Body.String())
	rec = do(t, h, "GET", "/produtos/2", "")
	assert.Equal(t, 404, rec.Code)
	assert.Equal(t, "não encontrado", errorOf(t, rec))
}

func TestBadIDs(t *testing.T) {
	h := New(newFake())
	for _, p := range []string{"/produtos/abc", "/produtos/-1", "/produtos/0", "/produtos/99999999999999999999", "/produtos/2147483648", "/pedidos/2147483648", "/pedidos/x"} {
		rec := do(t, h, "GET", p, "")
		assert.Equal(t, 400, rec.Code, p)
		assert.Equal(t, "id inválido", errorOf(t, rec), p)
	}
}

func TestProductsPagination(t *testing.T) {
	f := newFake()
	h := New(f)
	assert.Equal(t, 200, do(t, h, "GET", "/produtos", "").Code)
	assert.Equal(t, [2]int{20, 0}, f.listArgs)
	assert.Equal(t, 200, do(t, h, "GET", "/produtos?limite=5&pagina=2", "").Code)
	assert.Equal(t, [2]int{5, 5}, f.listArgs)
	for _, q := range []string{"?limite=0", "?limite=101", "?pagina=0", "?limite=x"} {
		assert.Equal(t, 400, do(t, h, "GET", "/produtos"+q, "").Code, q)
	}
}

func TestSearchRouteNotShadowed(t *testing.T) {
	f := newFake()
	h := New(f)
	assert.Equal(t, 200, do(t, h, "GET", "/produtos/busca?q=01", "").Code)
	assert.Equal(t, "01", f.searchArg)
	assert.False(t, f.getCalled)
	assert.Equal(t, 400, do(t, h, "GET", "/produtos/busca", "").Code)
	assert.Equal(t, 400, do(t, h, "GET", "/produtos/busca?q="+strings.Repeat("a", 101), "").Code)
	assert.Equal(t, 400, do(t, h, "GET", "/produtos/busca?q=%ff", "").Code)
	assert.Equal(t, 400, do(t, h, "GET", "/produtos/busca?q=a%00b", "").Code)
}

func TestCreateOrderValidation(t *testing.T) {
	f := newFake()
	h := New(f)
	for _, body := range []string{`{`, `{"produto_id":1,"quantidade":0}`, `{"produto_id":1,"quantidade":11}`,
		`{"produto_id":0,"quantidade":1}`, `{"produto_id":2147483648,"quantidade":1}`, `{"produto_id":1,"quantidade":1,"x":"` + strings.Repeat("a", 5<<10) + `"}`} {
		assert.Equal(t, 400, do(t, h, "POST", "/pedidos", body).Code, body[:min(len(body), 40)])
	}
	rec := do(t, h, "POST", "/pedidos", `{"produto_id":9,"quantidade":1}`)
	assert.Equal(t, 404, rec.Code)

	rec = do(t, h, "POST", "/pedidos", `{"produto_id":1,"quantidade":2}`)
	assert.Equal(t, 201, rec.Code)
	assert.Equal(t, "/pedidos/7", rec.Header().Get("Location"))
	assert.Contains(t, rec.Body.String(), `"quantidade":2`)

	f.outOfStock = true
	rec = do(t, h, "POST", "/pedidos", `{"produto_id":1,"quantidade":1}`)
	assert.Equal(t, 409, rec.Code)
	assert.Equal(t, "estoque insuficiente", errorOf(t, rec))
}

func TestGetOrder(t *testing.T) {
	h := New(newFake())
	assert.Equal(t, 200, do(t, h, "GET", "/pedidos/7", "").Code)
	assert.Equal(t, 404, do(t, h, "GET", "/pedidos/8", "").Code)
}

func TestSlowClampsMS(t *testing.T) {
	f := newFake()
	h := New(f)
	rec := do(t, h, "GET", "/lento?ms=999999", "")
	assert.Equal(t, 200, rec.Code)
	assert.Equal(t, 2*time.Second, f.sleepArg)
	assert.JSONEq(t, `{"dormiu_ms":2000}`, rec.Body.String())
	do(t, h, "GET", "/lento?ms=-5", "")
	assert.Equal(t, time.Duration(0), f.sleepArg)
	do(t, h, "GET", "/lento", "")
	assert.Equal(t, 500*time.Millisecond, f.sleepArg)
	assert.Equal(t, 400, do(t, h, "GET", "/lento?ms=abc", "").Code)
}

func TestErrorRoute(t *testing.T) {
	rec := do(t, New(newFake()), "GET", "/erro", "")
	assert.Equal(t, 500, rec.Code)
	assert.Equal(t, "falha proposital no banco", errorOf(t, rec))
}

func TestUnknownRoute(t *testing.T) {
	rec := do(t, New(newFake()), "GET", "/rota-inexistente", "")
	assert.Equal(t, 404, rec.Code)
	assert.Equal(t, "rota não encontrada", errorOf(t, rec))
}

func TestHangingDBReturns503(t *testing.T) {
	old := queryTimeout
	queryTimeout = 50 * time.Millisecond
	defer func() { queryTimeout = old }()
	f := newFake()
	f.hang = true
	start := time.Now()
	rec := do(t, New(f), "GET", "/produtos/1", "")
	assert.Less(t, time.Since(start), time.Second)
	assert.Equal(t, 503, rec.Code)
	assert.Equal(t, "banco indisponível", errorOf(t, rec))
}
