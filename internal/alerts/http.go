package alerts

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// EmailInfo descreve o canal de e-mail para a interface, sem expor
// endereços nem credenciais.
type EmailInfo struct {
	Enabled    bool `json:"ativo"`
	Recipients int  `json:"destinatarios"`
}

// Overview é a resposta de GET /api/alertas.
type Overview struct {
	Statuses []Status  `json:"estado"`
	History  []Alert   `json:"historico"`
	Rules    []Rule    `json:"regras"`
	Email    EmailInfo `json:"email"`
}

// Handler responde GET /api/alertas com o nível atual de cada recurso, o
// histórico, as regras e o estado do e-mail.
func Handler(e *Engine, email EmailInfo) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		statuses, history := e.Snapshot()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(Overview{
			Statuses: statuses, History: history, Rules: e.Rules(), Email: email,
		}); err != nil {
			slog.Error("alertas: escrever resposta", "erro", err)
		}
	})
}
