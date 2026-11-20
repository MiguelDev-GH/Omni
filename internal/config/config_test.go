package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

const tok = "0123456789abcdef0123456789abcdef" // 32 caracteres

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(map[string]string{"OMNI_TOKEN": tok}))
	require.NoError(t, err)
	assert.Equal(t, Config{Addr: "127.0.0.1:8080", DBPath: "omni.db", Token: tok}, c)
}

func TestLoadReadsOverrides(t *testing.T) {
	c, err := Load(env(map[string]string{"OMNI_TOKEN": tok, "OMNI_ADDR": "127.0.0.1:9000", "OMNI_DB": "/tmp/x.db"}))
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:9000", c.Addr)
	assert.Equal(t, "/tmp/x.db", c.DBPath)
}

func TestLoadRejectsMissingToken(t *testing.T) {
	_, err := Load(env(nil))
	assert.ErrorIs(t, err, ErrTokenAusente)
}

func TestLoadRejectsShortToken(t *testing.T) {
	_, err := Load(env(map[string]string{"OMNI_TOKEN": tok[:31]}))
	assert.ErrorIs(t, err, ErrTokenCurto)
}

func TestLoadAddrRules(t *testing.T) {
	cases := []struct {
		addr, remoto string
		ok           bool
	}{
		{"127.0.0.1:9000", "", true},
		{"[::1]:9000", "", true},
		{"localhost:9000", "", true},
		{"0.0.0.0:8080", "", false},
		{":8080", "", false},
		{"192.168.0.10:8080", "", false},
		{"sem-porta", "", false},
		{"0.0.0.0:8080", "1", true},
	}
	for _, c := range cases {
		cfg, err := Load(env(map[string]string{"OMNI_TOKEN": tok, "OMNI_ADDR": c.addr, "OMNI_PERMITIR_REMOTO": c.remoto}))
		if c.ok {
			assert.NoError(t, err, c.addr)
			assert.Equal(t, c.remoto == "1", cfg.AllowRemote, c.addr)
		} else {
			assert.ErrorIs(t, err, ErrEnderecoRemoto, c.addr)
		}
	}
}
