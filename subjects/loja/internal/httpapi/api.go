// Package httpapi expõe a API HTTP da loja cobaia.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/MiguelDev-GH/omni/subjects/loja/internal/store"
)

const (
	maxBody     = 4 << 10
	maxSearch   = 100
	maxSleepMS  = 2000
	defSleepMS  = 500
	defLimit    = 20
	maxLimit    = 100
	maxQuantity = 10
)

type api struct {
	s store.Store
}

// New devolve o handler com todas as rotas da loja.
func New(s store.Store) http.Handler {
	a := api{s: s}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", a.root)
	mux.HandleFunc("GET /saude", a.health)
	mux.HandleFunc("GET /produtos", a.listProducts)
	mux.HandleFunc("GET /produtos/busca", a.searchProducts)
	mux.HandleFunc("GET /produtos/{id}", a.getProduct)
	mux.HandleFunc("POST /pedidos", a.createOrder)
	mux.HandleFunc("GET /pedidos/{id}", a.getOrder)
	mux.HandleFunc("GET /lento", a.slow)
	mux.HandleFunc("GET /erro", a.broken)
	registerFlaws(mux)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotFound, errorBody("rota não encontrada"))
	})
	return withHeaders(mux)
}

// withHeaders anuncia servidor e tecnologia, de propósito, para o Nuclei.
func withHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "Loja/0.1")
		w.Header().Set("X-Powered-By", "Go")
		next.ServeHTTP(w, r)
	})
}

func (a api) root(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"servico": "loja", "status": "ok"})
}

func (a api) health(w http.ResponseWriter, r *http.Request) {
	if err := a.s.Ping(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"banco": "ok"})
}

func (a api) listProducts(w http.ResponseWriter, r *http.Request) {
	limit, ok1 := intParam(r, "limite", defLimit, 1, maxLimit)
	page, ok2 := intParam(r, "pagina", 1, 1, 1<<20)
	if !ok1 || !ok2 {
		writeJSON(w, http.StatusBadRequest, errorBody("parâmetro inválido"))
		return
	}
	list, err := a.s.ListProducts(r.Context(), limit, (page-1)*limit)
	respond(w, http.StatusOK, list, err)
}

func (a api) searchProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" || len([]rune(q)) > maxSearch {
		writeJSON(w, http.StatusBadRequest, errorBody("q deve ter de 1 a 100 caracteres"))
		return
	}
	list, err := a.s.SearchProducts(r.Context(), q)
	respond(w, http.StatusOK, list, err)
}

func (a api) getProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	p, err := a.s.GetProduct(r.Context(), id)
	respond(w, http.StatusOK, p, err)
}

func (a api) createOrder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ProductID int64 `json:"produto_id"`
		Quantity  int   `json:"quantidade"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("JSON inválido"))
		return
	}
	if in.ProductID <= 0 || in.Quantity < 1 || in.Quantity > maxQuantity {
		writeJSON(w, http.StatusBadRequest, errorBody("produto_id deve ser positivo e quantidade de 1 a 10"))
		return
	}
	o, err := a.s.CreateOrder(r.Context(), in.ProductID, in.Quantity)
	if err == nil {
		w.Header().Set("Location", "/pedidos/"+strconv.FormatInt(o.ID, 10))
	}
	respond(w, http.StatusCreated, o, err)
}

func (a api) getOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	o, err := a.s.GetOrder(r.Context(), id)
	respond(w, http.StatusOK, o, err)
}

func (a api) slow(w http.ResponseWriter, r *http.Request) {
	ms := defSleepMS
	if v := r.URL.Query().Get("ms"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorBody("ms deve ser um número"))
			return
		}
		ms = min(max(n, 0), maxSleepMS)
	}
	err := a.s.Sleep(r.Context(), time.Duration(ms)*time.Millisecond)
	respond(w, http.StatusOK, map[string]int{"dormiu_ms": ms}, err)
}

func (a api) broken(w http.ResponseWriter, r *http.Request) {
	err := a.s.Broken(r.Context())
	if err == nil || errors.Is(err, store.ErrUnavailable) {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusInternalServerError, errorBody("falha proposital no banco"))
}

// pathID lê o {id} da rota; responde 400 e devolve false se for inválido.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, errorBody("id inválido"))
		return 0, false
	}
	return id, true
}

// intParam lê um inteiro da query string em [lo, hi], com padrão quando ausente.
func intParam(r *http.Request, name string, def, lo, hi int) (int, bool) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def, true
	}
	n, err := strconv.Atoi(v)
	return n, err == nil && n >= lo && n <= hi
}

func respond(w http.ResponseWriter, status int, v any, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, status, v)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case err == nil:
		writeJSON(w, http.StatusInternalServerError, errorBody("erro interno"))
	case errors.Is(err, store.ErrUnavailable):
		writeJSON(w, http.StatusServiceUnavailable, errorBody(store.ErrUnavailable.Error()))
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errorBody(store.ErrNotFound.Error()))
	case errors.Is(err, store.ErrOutOfStock):
		writeJSON(w, http.StatusConflict, errorBody(store.ErrOutOfStock.Error()))
	default:
		slog.Error("erro inesperado", "erro", err)
		writeJSON(w, http.StatusInternalServerError, errorBody("erro interno"))
	}
}

func errorBody(msg string) map[string]string { return map[string]string{"erro": msg} }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
