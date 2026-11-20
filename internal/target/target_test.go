package target

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Target)
		field  string // vazio = sem erro
	}{
		{"válido", func(*Target) {}, ""},
		{"IPv6", func(t *Target) { t.Host = "::1" }, ""},
		{"hostname", func(t *Target) { t.Host = "api.local" }, ""},
		{"domínio válido", func(t *Target) { t.Domain = "app.exemplo.com.br" }, ""},
		{"banco válido", func(t *Target) { t.DBPort = 5432 }, ""},
		{"nome com acentos, 64 caracteres", func(t *Target) { t.Name = strings.Repeat("ç", 64) }, ""},
		{"host com shell", func(t *Target) { t.Host = "127.0.0.1; rm -rf /" }, "host"},
		{"host como opção", func(t *Target) { t.Host = "-oX" }, "host"},
		{"host maiúsculo", func(t *Target) { t.Host = "API.local" }, "host"},
		{"host vazio", func(t *Target) { t.Host = "" }, "host"},
		{"rótulo com 64 caracteres", func(t *Target) { t.Host = strings.Repeat("a", 64) + ".com" }, "host"},
		{"hostname com 254 caracteres", func(t *Target) { t.Host = strings.Repeat("a.", 126) + "ab" }, "host"},
		{"nome vazio", func(t *Target) { t.Name = "  " }, "nome"},
		{"nome longo", func(t *Target) { t.Name = strings.Repeat("a", 65) }, "nome"},
		{"sem portas", func(t *Target) { t.AppPorts = nil }, "portas_app"},
		{"porta 0", func(t *Target) { t.AppPorts = []int{0} }, "portas_app"},
		{"porta 70000", func(t *Target) { t.AppPorts = []int{70000} }, "portas_app"},
		{"porta repetida", func(t *Target) { t.AppPorts = []int{3000, 3000} }, "portas_app"},
		{"porta do banco inválida", func(t *Target) { t.DBPort = 70000 }, "porta_banco"},
		{"porta do banco negativa", func(t *Target) { t.DBPort = -1 }, "porta_banco"},
		{"domínio com espaço", func(t *Target) { t.Domain = "a b.com" }, "dominio"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tg := Target{Name: "api", Host: "127.0.0.1", AppPorts: []int{3000}}
			c.mutate(&tg)
			err := tg.Validate()
			if c.field == "" {
				assert.NoError(t, err)
				return
			}
			var ve *ValidationError
			require.ErrorAs(t, err, &ve)
			assert.Equal(t, c.field, ve.Field)
		})
	}
}
