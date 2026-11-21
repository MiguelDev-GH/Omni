package alerts

import (
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
)

const defaultSMTPPort = 587

var (
	ErrSMTPSemHost       = errors.New("OMNI_ALERTA_EMAILS definido sem OMNI_SMTP_HOST")
	ErrSMTPSemDestino    = errors.New("OMNI_SMTP_HOST definido sem OMNI_ALERTA_EMAILS")
	ErrSMTPSemRemetente  = errors.New("defina OMNI_SMTP_REMETENTE com o endereço que envia os alertas")
	ErrSMTPPortaInvalida = errors.New("OMNI_SMTP_PORTA inválida; use um número entre 1 e 65535")
	ErrSMTPSenhaDupla    = errors.New("use OMNI_SMTP_SENHA ou OMNI_SMTP_SENHA_ARQUIVO, não os dois")
)

// MailConfigFromEnv lê a configuração de e-mail das variáveis OMNI_SMTP_* e
// OMNI_ALERTA_EMAILS. Sem OMNI_SMTP_HOST o envio fica desligado. readFile
// lê OMNI_SMTP_SENHA_ARQUIVO (normalmente os.ReadFile).
func MailConfigFromEnv(getenv func(string) string, readFile func(string) ([]byte, error)) (MailConfig, error) {
	host := strings.TrimSpace(getenv("OMNI_SMTP_HOST"))
	rawTo := strings.TrimSpace(getenv("OMNI_ALERTA_EMAILS"))
	if host == "" {
		if rawTo != "" {
			return MailConfig{}, ErrSMTPSemHost
		}
		return MailConfig{}, nil
	}
	if rawTo == "" {
		return MailConfig{}, ErrSMTPSemDestino
	}

	c := MailConfig{Host: host, Port: defaultSMTPPort, User: getenv("OMNI_SMTP_USUARIO")}
	if p := strings.TrimSpace(getenv("OMNI_SMTP_PORTA")); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return MailConfig{}, fmt.Errorf("%w: %q", ErrSMTPPortaInvalida, p)
		}
		c.Port = n
	}

	// A senha pode vir de um arquivo: variáveis de ambiente aparecem em
	// /proc/<pid>/environ e são herdadas por todo processo filho.
	pass, passFile := getenv("OMNI_SMTP_SENHA"), getenv("OMNI_SMTP_SENHA_ARQUIVO")
	switch {
	case pass != "" && passFile != "":
		return MailConfig{}, ErrSMTPSenhaDupla
	case passFile != "":
		b, err := readFile(passFile)
		if err != nil {
			return MailConfig{}, fmt.Errorf("ler OMNI_SMTP_SENHA_ARQUIVO: %w", err)
		}
		pass = strings.TrimRight(string(b), "\r\n")
	}
	c.Password = pass

	from := strings.TrimSpace(getenv("OMNI_SMTP_REMETENTE"))
	if from == "" && strings.Contains(c.User, "@") {
		from = c.User
	}
	if from == "" {
		return MailConfig{}, ErrSMTPSemRemetente
	}
	addr, err := mail.ParseAddress(from)
	if err != nil {
		return MailConfig{}, fmt.Errorf("OMNI_SMTP_REMETENTE inválido: %w", err)
	}
	c.From = addr.String()

	for _, item := range strings.Split(rawTo, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		a, err := mail.ParseAddress(item)
		if err != nil {
			return MailConfig{}, fmt.Errorf("endereço inválido em OMNI_ALERTA_EMAILS: %q", item)
		}
		c.To = append(c.To, a.Address)
	}
	if len(c.To) == 0 {
		return MailConfig{}, ErrSMTPSemDestino
	}
	return c, nil
}
