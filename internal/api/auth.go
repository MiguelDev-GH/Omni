package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// requireToken só deixa passar requisições com "Authorization: Bearer <token>".
func requireToken(token string, next http.Handler) http.Handler {
	want := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(got), want) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, http.StatusUnauthorized, "não autorizado")
			return
		}
		next.ServeHTTP(w, r)
	})
}
