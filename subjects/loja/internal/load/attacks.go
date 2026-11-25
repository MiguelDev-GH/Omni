package load

import (
	"fmt"
	"math/rand/v2"
	"net/http"
)

// Os ataques simulados mandam X-Forwarded-For como se passassem pelo proxy
// reverso: a carga fala direto com a porta da loja, em 127.0.0.1, que é a
// mesma conexão que o Caddy faria. Os IPs são dos blocos reservados para
// documentação (RFC 5737), que nunca pertencem a ninguém de verdade.
const (
	bruteForceIP = "203.0.113.66"
	sprayingIP   = "198.51.100.23"
)

var sprayUsers = []string{
	"ana", "bia", "carlos", "admin", "root", "suporte", "financeiro", "joao", "maria", "pedro",
	"lucas", "julia", "rafael", "camila", "bruno", "larissa", "teste", "contato", "vendas", "ti",
}

func login(user, pass, ip string) Request {
	return Request{Method: http.MethodPost, Path: "/entrar", User: user, Password: pass, ForwardedFor: ip}
}

// Attacks são as misturas de tráfego malicioso da opção -ataque.
var Attacks = map[string][]Kind{
	// forca-bruta: um IP tentando senhas diferentes na mesma conta.
	"forca-bruta": {{"forca_bruta", 1, func(r *rand.Rand) Request {
		return login("ana", fmt.Sprintf("senha%04d", r.IntN(10000)), bruteForceIP)
	}}},
	// spraying: um IP tentando uma senha comum em muitas contas.
	"spraying": {{"spraying", 1, func(r *rand.Rand) Request {
		return login(sprayUsers[r.IntN(len(sprayUsers))], "123456", sprayingIP)
	}}},
	// ddos: muitas origens pedindo páginas comuns ao mesmo tempo.
	"ddos": {{"ddos", 1, func(r *rand.Rand) Request {
		req := get(fmt.Sprintf("/produtos?limite=20&pagina=%d", between(r, 1, 3)))
		req.ForwardedFor = fmt.Sprintf("203.0.113.%d", between(r, 1, 254))
		return req
	}}},
}
