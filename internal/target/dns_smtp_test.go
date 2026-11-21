package target

import (
	"errors"
	"testing"
)

func TestValidateDNSAndSMTPPorts(t *testing.T) {
	base := Target{Name: "loja", Host: "127.0.0.1", AppPorts: []int{3000}, DBPort: 5433, Domain: "loja.local"}
	ok := base
	ok.DNSPort, ok.SMTPPort = 1053, 1025
	if err := ok.Validate(); err != nil {
		t.Fatalf("alvo válido recusado: %v", err)
	}

	cases := []struct {
		name  string
		edit  func(*Target)
		field string
	}{
		{"dns fora do intervalo", func(t *Target) { t.DNSPort = 70000 }, "porta_dns"},
		{"dns igual à app", func(t *Target) { t.DNSPort = 3000 }, "porta_dns"},
		{"dns igual ao banco", func(t *Target) { t.DNSPort = 5433 }, "porta_dns"},
		{"dns sem domínio", func(t *Target) { t.DNSPort = 53; t.Domain = "" }, "dominio"},
		{"smtp negativa", func(t *Target) { t.SMTPPort = -1 }, "porta_smtp"},
		{"smtp igual à app", func(t *Target) { t.SMTPPort = 3000 }, "porta_smtp"},
		{"smtp igual ao dns", func(t *Target) { t.DNSPort = 1053; t.SMTPPort = 1053 }, "porta_smtp"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tg := base
			c.edit(&tg)
			var ve *ValidationError
			if err := tg.Validate(); !errors.As(err, &ve) || ve.Field != c.field {
				t.Fatalf("erro = %v, quer campo %s", err, c.field)
			}
		})
	}

	// SMTP sem domínio é válido: a sonda só não testa o RCPT.
	smtpOnly := base
	smtpOnly.Domain, smtpOnly.SMTPPort = "", 25
	if err := smtpOnly.Validate(); err != nil {
		t.Fatalf("SMTP sem domínio recusado: %v", err)
	}
}
