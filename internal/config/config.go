// Package config lê a configuração do Omni a partir de variáveis de ambiente.
package config

import (
	"errors"
	"fmt"
	"net"
)

const minTokenLen = 32

var (
	ErrTokenAusente   = errors.New("OMNI_TOKEN não definido")
	ErrTokenCurto     = fmt.Errorf("OMNI_TOKEN precisa ter no mínimo %d caracteres", minTokenLen)
	ErrEnderecoRemoto = errors.New("endereço fora do loopback; defina OMNI_PERMITIR_REMOTO=1 para permitir")
)

type Config struct {
	Addr        string
	DBPath      string
	Token       string
	AllowRemote bool
}

// Load monta a Config usando getenv (normalmente os.Getenv).
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		Addr:        withDefault(getenv("OMNI_ADDR"), "127.0.0.1:8080"),
		DBPath:      withDefault(getenv("OMNI_DB"), "omni.db"),
		Token:       getenv("OMNI_TOKEN"),
		AllowRemote: getenv("OMNI_PERMITIR_REMOTO") == "1",
	}
	if c.Token == "" {
		return Config{}, ErrTokenAusente
	}
	if len(c.Token) < minTokenLen {
		return Config{}, ErrTokenCurto
	}
	if !c.AllowRemote && !isLoopback(c.Addr) {
		return Config{}, fmt.Errorf("%w: %s", ErrEnderecoRemoto, c.Addr)
	}
	return c, nil
}

func withDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
