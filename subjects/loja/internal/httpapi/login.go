package httpapi

import (
	"crypto/subtle"
	"net/http"
)

// accounts são as contas da loja cobaia. As senhas fracas e a falta de
// limite de tentativas são falhas de propósito: é o que o detector de
// brute-force do Omni precisa ver acontecer.
var accounts = map[string]string{
	"ana":    "ana123",
	"bia":    "bia123",
	"carlos": "carlos123",
	"admin":  "admin",
}

// login responde POST /entrar com autenticação Basic: 200 com a senha
// certa, 401 com a errada.
func (a api) login(w http.ResponseWriter, r *http.Request) {
	user, pass, ok := r.BasicAuth()
	want, known := accounts[user]
	// A comparação em tempo constante não deixa o tempo de resposta revelar
	// quantos caracteres da senha estão certos. Conta desconhecida também
	// passa pela comparação, para não revelar quais contas existem.
	if !known {
		want = "\x00conta-inexistente"
	}
	if !ok || subtle.ConstantTimeCompare([]byte(pass), []byte(want)) != 1 || !known {
		w.Header().Set("WWW-Authenticate", `Basic realm="loja"`)
		writeJSON(w, http.StatusUnauthorized, errorBody("usuário ou senha inválidos"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"usuario": user})
}
