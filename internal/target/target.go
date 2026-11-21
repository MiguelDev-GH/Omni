// Package target define o alvo monitorado pelo Omni e sua validação.
package target

import (
	"net"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	maxNameLen     = 64
	maxHostnameLen = 253
)

var hostnameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)

type Target struct {
	ID       int64  `json:"id"`
	Name     string `json:"nome"`
	Host     string `json:"host"`
	AppPorts []int  `json:"portas_app"`
	DBPort   int    `json:"porta_banco,omitempty"`
	Domain   string `json:"dominio,omitempty"`
	// DNSPort é a porta de um servidor DNS no host; a sonda pergunta por Domain.
	DNSPort int `json:"porta_dns,omitempty"`
	// SMTPPort é a porta de um servidor SMTP no host.
	SMTPPort int `json:"porta_smtp,omitempty"`
}

// ValidationError indica o primeiro campo inválido de um Target.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }

// Validate confere os campos na ordem nome, host, portas_app, porta_banco,
// dominio, porta_dns e porta_smtp.
func (t Target) Validate() error {
	if n := utf8.RuneCountInString(strings.TrimSpace(t.Name)); n < 1 || n > maxNameLen {
		return &ValidationError{"nome", "nome deve ter de 1 a 64 caracteres"}
	}
	if net.ParseIP(t.Host) == nil && !isHostname(t.Host) {
		return &ValidationError{"host", "host deve ser um IP ou um nome de host em minúsculas"}
	}
	if len(t.AppPorts) == 0 {
		return &ValidationError{"portas_app", "informe ao menos uma porta"}
	}
	seen := make(map[int]bool, len(t.AppPorts))
	for _, p := range t.AppPorts {
		if !validPort(p) {
			return &ValidationError{"portas_app", "porta fora do intervalo 1-65535"}
		}
		if seen[p] {
			return &ValidationError{"portas_app", "porta repetida"}
		}
		seen[p] = true
	}
	if t.DBPort != 0 && !validPort(t.DBPort) {
		return &ValidationError{"porta_banco", "porta fora do intervalo 1-65535"}
	}
	if t.Domain != "" && !isHostname(t.Domain) {
		return &ValidationError{"dominio", "domínio inválido"}
	}
	// As portas de DNS e SMTP não podem repetir outra porta do alvo: o
	// estado de cada porta é guardado por (alvo, porta).
	used := func(p int) bool { return seen[p] || p == t.DBPort }
	if t.DNSPort != 0 {
		if !validPort(t.DNSPort) {
			return &ValidationError{"porta_dns", "porta fora do intervalo 1-65535"}
		}
		if used(t.DNSPort) {
			return &ValidationError{"porta_dns", "porta já usada neste alvo"}
		}
		if t.Domain == "" {
			return &ValidationError{"dominio", "informe o domínio que a sonda de DNS vai resolver"}
		}
	}
	if t.SMTPPort != 0 {
		if !validPort(t.SMTPPort) {
			return &ValidationError{"porta_smtp", "porta fora do intervalo 1-65535"}
		}
		if used(t.SMTPPort) || t.SMTPPort == t.DNSPort {
			return &ValidationError{"porta_smtp", "porta já usada neste alvo"}
		}
	}
	return nil
}

func isHostname(s string) bool {
	return len(s) <= maxHostnameLen && hostnameRe.MatchString(s)
}

func validPort(p int) bool { return p >= 1 && p <= 65535 }
