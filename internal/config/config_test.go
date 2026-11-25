package config

import (
	"net"
	"testing"
	"time"

	"github.com/MiguelDev-GH/omni/internal/alerts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

const tok = "0123456789abcdef0123456789abcdef" // 32 caracteres

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(map[string]string{"OMNI_TOKEN": tok}))
	require.NoError(t, err)
	assert.Equal(t, Config{Addr: "127.0.0.1:0", DBPath: "omni.db", Token: tok, PollInterval: 30 * time.Second, CheckRetention: 168 * time.Hour,
		AllowedOrigins: []string{"http://127.0.0.1:0", "http://localhost:0", "http://[::1]:0"},
		Interface:      "any"}, c)
}

func TestBindUsesRealPort(t *testing.T) {
	c, err := Load(env(map[string]string{"OMNI_TOKEN": tok, "OMNI_ORIGENS": "http://localhost:9999"}))
	require.NoError(t, err)
	c = c.Bind(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 41234})
	assert.Equal(t, "127.0.0.1:41234", c.Addr)
	assert.Equal(t, []string{"http://127.0.0.1:41234", "http://localhost:41234", "http://[::1]:41234",
		"http://localhost:9999"}, c.AllowedOrigins)
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
		{"sem-porta", "1", false},
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

func TestLoadPollInterval(t *testing.T) {
	ok := map[string]time.Duration{"5s": 5 * time.Second, "1s": time.Second}
	for v, want := range ok {
		c, err := Load(env(map[string]string{"OMNI_TOKEN": tok, "OMNI_INTERVALO_POLLER": v}))
		require.NoError(t, err, v)
		assert.Equal(t, want, c.PollInterval, v)
	}
	for _, v := range []string{"500ms", "0", "-1s", "abc"} {
		_, err := Load(env(map[string]string{"OMNI_TOKEN": tok, "OMNI_INTERVALO_POLLER": v}))
		assert.ErrorIs(t, err, ErrIntervaloInvalido, v)
		assert.ErrorContains(t, err, v)
	}
}

func TestLoadOrigins(t *testing.T) {
	load := func(m map[string]string) (Config, error) {
		m["OMNI_TOKEN"] = tok
		return Load(env(m))
	}

	c, err := load(map[string]string{"OMNI_ADDR": "127.0.0.1:9000"})
	require.NoError(t, err)
	assert.Equal(t, []string{"http://127.0.0.1:9000", "http://localhost:9000", "http://[::1]:9000"}, c.AllowedOrigins)

	c, err = load(map[string]string{"OMNI_ORIGENS": " http://localhost:9999 ,https://omni.exemplo.com"})
	require.NoError(t, err)
	assert.Equal(t, []string{"http://localhost:9999", "https://omni.exemplo.com"}, c.AllowedOrigins[3:])

	c, err = load(map[string]string{"OMNI_ADDR": "127.0.0.1:8080", "OMNI_ORIGENS": "http://localhost:8080"})
	require.NoError(t, err)
	assert.Len(t, c.AllowedOrigins, 3)

	c, err = load(map[string]string{"OMNI_ORIGENS": "HTTP://LocalHost:9999,https://omni.exemplo.com:443,http://x.com:80,http://x.com,http://localhost:9999"})
	require.NoError(t, err)
	assert.Equal(t, []string{"http://localhost:9999", "https://omni.exemplo.com", "http://x.com"}, c.AllowedOrigins[3:])

	_, err = load(map[string]string{"OMNI_ORIGENS": "http://x.com,"})
	assert.ErrorContains(t, err, `""`)

	for _, bad := range []string{"localhost:9999", "http://localhost:9999/painel", "ftp://x.com", "http://u:s@x.com", "http://", "http://x.com?", "http://x.com#"} {
		_, err := load(map[string]string{"OMNI_ORIGENS": bad})
		assert.ErrorIs(t, err, ErrOrigemInvalida, bad)
		assert.ErrorContains(t, err, bad)
	}
}

func TestLoadInterface(t *testing.T) {
	c, err := Load(env(map[string]string{"OMNI_TOKEN": tok, "OMNI_INTERFACE": "eth0"}))
	require.NoError(t, err)
	assert.Equal(t, "eth0", c.Interface)

	for _, bad := range []string{"eth0; rm -rf /", "-i", "nome-muito-longo-demais", "a b"} {
		_, err := Load(env(map[string]string{"OMNI_TOKEN": tok, "OMNI_INTERFACE": bad}))
		assert.ErrorIs(t, err, ErrInterfaceInvalida, bad)
	}
}

func TestLoadMail(t *testing.T) {
	c, err := Load(env(map[string]string{"OMNI_TOKEN": tok}))
	require.NoError(t, err)
	assert.False(t, c.Mail.Enabled())

	c, err = Load(env(map[string]string{
		"OMNI_TOKEN": tok, "OMNI_SMTP_HOST": "127.0.0.1", "OMNI_SMTP_PORTA": "1025",
		"OMNI_SMTP_REMETENTE": "omni@exemplo.com", "OMNI_ALERTA_EMAILS": "ana@exemplo.com",
	}))
	require.NoError(t, err)
	assert.True(t, c.Mail.Enabled())
	assert.Equal(t, 1025, c.Mail.Port)
	assert.Equal(t, []string{"ana@exemplo.com"}, c.Mail.To)

	_, err = Load(env(map[string]string{"OMNI_TOKEN": tok, "OMNI_ALERTA_EMAILS": "ana@exemplo.com"}))
	assert.ErrorIs(t, err, alerts.ErrSMTPSemHost)
}

func TestLoadCheckRetention(t *testing.T) {
	c, err := Load(env(map[string]string{"OMNI_TOKEN": tok}))
	require.NoError(t, err)
	assert.Equal(t, 168*time.Hour, c.CheckRetention)

	c, err = Load(env(map[string]string{"OMNI_TOKEN": tok, "OMNI_RETENCAO_VERIFICACOES": "2h"}))
	require.NoError(t, err)
	assert.Equal(t, 2*time.Hour, c.CheckRetention)

	for _, v := range []string{"59m", "7d", "abc"} {
		_, err := Load(env(map[string]string{"OMNI_TOKEN": tok, "OMNI_RETENCAO_VERIFICACOES": v}))
		assert.ErrorIs(t, err, ErrRetencaoInvalida, v)
	}
}

func TestLoadWatchedFiles(t *testing.T) {
	c, err := Load(env(map[string]string{"OMNI_TOKEN": tok}))
	require.NoError(t, err)
	assert.Empty(t, c.WatchedFiles)

	c, err = Load(env(map[string]string{"OMNI_TOKEN": tok, "OMNI_ARQUIVOS_MONITORADOS": "/etc/caddy/Caddyfile, /etc/hosts"}))
	require.NoError(t, err)
	assert.Equal(t, []string{"/etc/caddy/Caddyfile", "/etc/hosts"}, c.WatchedFiles)
}
