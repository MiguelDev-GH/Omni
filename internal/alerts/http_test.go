package alerts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler(t *testing.T) {
	e := NewEngine(testRules())
	e.Observe(Sample{Rule: "up", Subject: "loja:3000", Value: AvailabilityDown, At: t0})

	rec := httptest.NewRecorder()
	Handler(e, EmailInfo{Enabled: true, Recipients: 2}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/alertas", nil))

	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status %d, content-type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var got struct {
		Estado    []map[string]any `json:"estado"`
		Historico []map[string]any `json:"historico"`
		Regras    []map[string]any `json:"regras"`
		Email     map[string]any   `json:"email"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Estado) != 1 || got.Estado[0]["nivel"] != "vermelho" || got.Estado[0]["recurso"] != "loja:3000" {
		t.Errorf("estado = %v", got.Estado)
	}
	if len(got.Historico) != 1 || got.Historico[0]["nivel_anterior"] != "verde" {
		t.Errorf("histórico = %v", got.Historico)
	}
	if len(got.Regras) != 2 {
		t.Errorf("regras = %v", got.Regras)
	}
	if got.Email["ativo"] != true || got.Email["destinatarios"] != float64(2) {
		t.Errorf("email = %v", got.Email)
	}
}
