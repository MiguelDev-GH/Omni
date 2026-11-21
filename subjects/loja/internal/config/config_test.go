package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	c, err := Load(func(string) string { return "" })
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:3000", c.Addr)
	assert.Equal(t, "postgres://loja:loja-cobaia@127.0.0.1:5433/loja?sslmode=disable", c.DatabaseURL)
	assert.Equal(t, "http://127.0.0.1:8080", c.OmniURL)
	assert.Empty(t, c.OmniToken)
}

func TestLoadReadsEnv(t *testing.T) {
	c, err := Load(env(map[string]string{
		"LOJA_ADDR":  "127.0.0.1:3001",
		"LOJA_BANCO": "postgres://x@127.0.0.1:1/y",
		"OMNI_URL":   "http://127.0.0.1:9090",
		"OMNI_TOKEN": "segredo",
	}))
	require.NoError(t, err)
	assert.Equal(t, Config{Addr: "127.0.0.1:3001", DatabaseURL: "postgres://x@127.0.0.1:1/y", OmniURL: "http://127.0.0.1:9090", OmniToken: "segredo"}, c)
}

func TestLoadRejectsNonLoopback(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:3000", ":3000", "192.168.0.10:3000", "exemplo.com:3000", "3000"} {
		_, err := Load(env(map[string]string{"LOJA_ADDR": addr}))
		assert.ErrorIs(t, err, ErrNonLoopback, addr)
	}
	for _, addr := range []string{"127.0.0.1:3001", "[::1]:3000", "localhost:3000"} {
		_, err := Load(env(map[string]string{"LOJA_ADDR": addr}))
		assert.NoError(t, err, addr)
	}
}
