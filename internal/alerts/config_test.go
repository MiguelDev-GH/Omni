package alerts

import (
	"errors"
	"os"
	"reflect"
	"testing"
)

func envFrom(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func noFile(string) ([]byte, error) { return nil, os.ErrNotExist }

func TestMailConfigDisabledByDefault(t *testing.T) {
	c, err := MailConfigFromEnv(envFrom(nil), noFile)
	if err != nil || c.Enabled() {
		t.Fatalf("c = %+v, err = %v", c, err)
	}
}

func TestMailConfigFull(t *testing.T) {
	c, err := MailConfigFromEnv(envFrom(map[string]string{
		"OMNI_SMTP_HOST":      "smtp.gmail.com",
		"OMNI_SMTP_PORTA":     "465",
		"OMNI_SMTP_USUARIO":   "omni@gmail.com",
		"OMNI_SMTP_SENHA":     "abcd efgh",
		"OMNI_SMTP_REMETENTE": "Omni <omni@gmail.com>",
		"OMNI_ALERTA_EMAILS":  " ana@exemplo.com, Bia <bia@exemplo.com>,",
	}), noFile)
	if err != nil {
		t.Fatal(err)
	}
	want := MailConfig{Host: "smtp.gmail.com", Port: 465, User: "omni@gmail.com", Password: "abcd efgh",
		From: `"Omni" <omni@gmail.com>`, To: []string{"ana@exemplo.com", "bia@exemplo.com"}}
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("c = %+v\nquer %+v", c, want)
	}
	if !c.Enabled() {
		t.Error("deveria estar ativo")
	}
}

func TestMailConfigDefaultsAndPasswordFile(t *testing.T) {
	read := func(p string) ([]byte, error) {
		if p != "/run/secrets/smtp" {
			t.Errorf("arquivo lido: %s", p)
		}
		return []byte("segredo\n"), nil
	}
	c, err := MailConfigFromEnv(envFrom(map[string]string{
		"OMNI_SMTP_HOST":          "127.0.0.1",
		"OMNI_SMTP_USUARIO":       "alertas@exemplo.com",
		"OMNI_SMTP_SENHA_ARQUIVO": "/run/secrets/smtp",
		"OMNI_ALERTA_EMAILS":      "ana@exemplo.com",
	}), read)
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 587 || c.Password != "segredo" || c.From != "<alertas@exemplo.com>" {
		t.Fatalf("c = %+v", c)
	}
}

func TestMailConfigErrors(t *testing.T) {
	base := map[string]string{
		"OMNI_SMTP_HOST": "127.0.0.1", "OMNI_SMTP_REMETENTE": "omni@exemplo.com", "OMNI_ALERTA_EMAILS": "ana@exemplo.com",
	}
	with := func(k, v string) map[string]string {
		m := map[string]string{}
		for a, b := range base {
			m[a] = b
		}
		m[k] = v
		return m
	}
	cases := []struct {
		name string
		env  map[string]string
		want error
	}{
		{"destino sem host", map[string]string{"OMNI_ALERTA_EMAILS": "a@b.c"}, ErrSMTPSemHost},
		{"host sem destino", with("OMNI_ALERTA_EMAILS", ""), ErrSMTPSemDestino},
		{"só vírgulas", with("OMNI_ALERTA_EMAILS", " , "), ErrSMTPSemDestino},
		{"sem remetente", with("OMNI_SMTP_REMETENTE", ""), ErrSMTPSemRemetente},
		{"porta texto", with("OMNI_SMTP_PORTA", "smtp"), ErrSMTPPortaInvalida},
		{"porta zero", with("OMNI_SMTP_PORTA", "0"), ErrSMTPPortaInvalida},
		{"senha dupla", func() map[string]string {
			m := with("OMNI_SMTP_SENHA", "a")
			m["OMNI_SMTP_SENHA_ARQUIVO"] = "/x"
			return m
		}(), ErrSMTPSenhaDupla},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := MailConfigFromEnv(envFrom(c.env), noFile); !errors.Is(err, c.want) {
				t.Fatalf("err = %v, quer %v", err, c.want)
			}
		})
	}
	for name, env := range map[string]map[string]string{
		"remetente com CRLF": with("OMNI_SMTP_REMETENTE", "omni@exemplo.com\r\nBcc: x@y.z"),
		"destino inválido":   with("OMNI_ALERTA_EMAILS", "não é e-mail"),
		"arquivo ausente":    with("OMNI_SMTP_SENHA_ARQUIVO", "/nao/existe"),
	} {
		if _, err := MailConfigFromEnv(envFrom(env), noFile); err == nil {
			t.Errorf("%s: deveria falhar", name)
		}
	}
}
