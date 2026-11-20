package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

const maxBodyBytes = 1 << 20

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("escrever resposta", "erro", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"erro": msg})
}

// decodeJSON lê exatamente um objeto JSON do corpo, com limite de tamanho.
// Quando ok é false, status e msg descrevem a resposta de erro.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) (status int, msg string, ok bool) {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	err := dec.Decode(v)
	if err == nil {
		err = dec.Decode(&struct{}{})
		if errors.Is(err, io.EOF) {
			return 0, "", true
		}
		if err == nil {
			err = errors.New("dados depois do objeto")
		}
	}
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return http.StatusRequestEntityTooLarge, "corpo muito grande", false
	}
	return http.StatusBadRequest, "json inválido", false
}
