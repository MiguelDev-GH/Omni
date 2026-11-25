// Package clientinfo extrai de uma requisição HTTP quem a fez: o IP de
// origem real atrás de um proxy reverso e o usuário de uma autenticação
// Basic. Nunca devolve a senha.
package clientinfo

import (
	"encoding/base64"
	"net"
	"strings"
	"unicode"
)

// maxUserLen limita o tamanho do usuário aceito, para um cabeçalho
// gigante não virar uma chave enorme nos contadores.
const maxUserLen = 64

// OriginIP devolve o IP de quem fez a requisição. O cabeçalho
// X-Forwarded-For só é usado quando a conexão vem de um proxy local
// (loopback), porque qualquer cliente pode escrever esse cabeçalho. Mesmo
// assim, só o último endereço da lista vale: é o que o proxy local
// acrescentou. Os anteriores vieram do próprio cliente e podem ser falsos.
func OriginIP(peer, forwardedFor string) string {
	ip := net.ParseIP(peer)
	if ip == nil || !ip.IsLoopback() || forwardedFor == "" {
		return peer
	}
	parts := strings.Split(forwardedFor, ",")
	last := strings.TrimSpace(parts[len(parts)-1])
	if parsed := net.ParseIP(last); parsed != nil {
		return parsed.String()
	}
	return peer
}

// BasicUser devolve o usuário de um cabeçalho "Authorization: Basic ...",
// ou "" se o cabeçalho não for Basic ou for inválido. A senha é descartada
// aqui mesmo e nunca sai desta função.
func BasicUser(authorization string) string {
	scheme, encoded, ok := strings.Cut(strings.TrimSpace(authorization), " ")
	if !ok || !strings.EqualFold(scheme, "Basic") {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return ""
	}
	user, _, ok := strings.Cut(string(raw), ":")
	if !ok || user == "" || len(user) > maxUserLen {
		return ""
	}
	for _, r := range user {
		if !unicode.IsPrint(r) {
			return ""
		}
	}
	return user
}
