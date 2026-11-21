// Package arsenal monta os argumentos das ferramentas de segurança e
// interpreta a saída delas como achados normalizados.
package arsenal

import (
	"errors"
	"net"
	"strings"

	"github.com/MiguelDev-GH/omni/internal/target"
)

// ErrForaDoEscopo indica um host que o Arsenal não pode varrer.
var ErrForaDoEscopo = errors.New("host fora do escopo do alvo")

// UserAgent marca as requisições do Arsenal, que o sniffer tira das métricas.
const UserAgent = "Omni-Arsenal/1"

// InScope diz se host pode ser varrido para o alvo t: endereços de loopback,
// localhost, o Host do alvo, o Domain do alvo e os subdomínios dele.
func InScope(t target.Target, host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	if host == "localhost" || host == t.Host {
		return true
	}
	return t.Domain != "" && (host == t.Domain || strings.HasSuffix(host, "."+t.Domain))
}

// checkTarget valida o alvo e confirma que o host dele está no escopo.
func checkTarget(t target.Target) error {
	if err := t.Validate(); err != nil {
		return err
	}
	if !InScope(t, t.Host) || strings.HasPrefix(t.Host, "-") {
		return ErrForaDoEscopo
	}
	return nil
}

// ports devolve as portas de aplicação e a do banco do alvo.
func ports(t target.Target) []int {
	out := append([]int(nil), t.AppPorts...)
	if t.DBPort > 0 {
		out = append(out, t.DBPort)
	}
	return out
}
