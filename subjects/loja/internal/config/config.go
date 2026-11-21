// Package config lê a configuração da loja cobaia das variáveis de ambiente.
package config

import (
	"errors"
	"net"
)

// ErrNonLoopback impede que a loja, que expõe falhas de propósito, saia da máquina.
var ErrNonLoopback = errors.New("LOJA_ADDR precisa ser um endereço de loopback")

type Config struct {
	Addr        string
	DatabaseURL string
	OmniURL     string
	OmniToken   string
}

// Load monta a Config a partir de getenv, com os padrões da cobaia.
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		Addr:        withDefault(getenv("LOJA_ADDR"), "127.0.0.1:3000"),
		DatabaseURL: withDefault(getenv("LOJA_BANCO"), "postgres://loja:loja-cobaia@127.0.0.1:5433/loja?sslmode=disable"),
		OmniURL:     withDefault(getenv("OMNI_URL"), "http://127.0.0.1:8080"),
		OmniToken:   getenv("OMNI_TOKEN"),
	}
	if !isLoopback(c.Addr) {
		return Config{}, ErrNonLoopback
	}
	return c, nil
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

func withDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
