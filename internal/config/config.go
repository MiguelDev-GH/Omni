// Package config lê a configuração do Omni a partir de variáveis de ambiente.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

const (
	minTokenLen         = 32
	defaultPollInterval = "30s"
	minPollInterval     = time.Second
)

var (
	ErrTokenAusente   = errors.New("OMNI_TOKEN não definido")
	ErrTokenCurto     = fmt.Errorf("OMNI_TOKEN precisa ter no mínimo %d caracteres", minTokenLen)
	ErrEnderecoRemoto = errors.New("endereço fora do loopback; defina OMNI_PERMITIR_REMOTO=1 para permitir")

	ErrIntervaloInvalido = errors.New("OMNI_INTERVALO_POLLER inválido; use uma duração de no mínimo 1s, como 30s")
	ErrOrigemInvalida    = errors.New("origem inválida em OMNI_ORIGENS; use esquema http ou https, host e porta opcional")
)

type Config struct {
	Addr        string
	DBPath      string
	Token       string
	AllowRemote bool

	PollInterval   time.Duration
	AllowedOrigins []string
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
	interval, err := parsePollInterval(withDefault(getenv("OMNI_INTERVALO_POLLER"), defaultPollInterval))
	if err != nil {
		return Config{}, err
	}
	c.PollInterval = interval
	origins, err := buildOrigins(c.Addr, getenv("OMNI_DEV") == "1", getenv("OMNI_ORIGENS"))
	if err != nil {
		return Config{}, err
	}
	c.AllowedOrigins = origins
	return c, nil
}

func parsePollInterval(v string) (time.Duration, error) {
	d, err := time.ParseDuration(v)
	if err != nil || d < minPollInterval {
		return 0, fmt.Errorf("%w: %s", ErrIntervaloInvalido, v)
	}
	return d, nil
}

// buildOrigins devolve as origens aceitas, sem repetição: loopback na porta
// de addr, depois as do servidor de desenvolvimento e por fim as extras.
func buildOrigins(addr string, dev bool, extra string) ([]string, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrEnderecoRemoto, addr)
	}
	origins := []string{
		"http://127.0.0.1:" + port,
		"http://localhost:" + port,
		"http://[::1]:" + port,
	}
	if dev {
		origins = append(origins, "http://localhost:5173", "http://127.0.0.1:5173")
	}
	if strings.TrimSpace(extra) != "" {
		for _, item := range strings.Split(extra, ",") {
			item = strings.TrimSpace(item)
			origin, ok := normalizeOrigin(item)
			if !ok {
				return nil, fmt.Errorf("%w: %q", ErrOrigemInvalida, item)
			}
			origins = append(origins, origin)
		}
	}
	seen := make(map[string]bool, len(origins))
	unique := origins[:0]
	for _, o := range origins {
		if !seen[o] {
			seen[o] = true
			unique = append(unique, o)
		}
	}
	return unique, nil
}

// normalizeOrigin valida s e a devolve na forma que os navegadores enviam:
// esquema e host em minúsculas, sem a porta padrão do esquema.
func normalizeOrigin(s string) (string, bool) {
	if strings.HasSuffix(s, "#") {
		return "", false
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", false
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil ||
		u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return "", false
	}
	host := strings.ToLower(u.Host)
	host = strings.TrimSuffix(host, map[string]string{"http": ":80", "https": ":443"}[u.Scheme])
	return u.Scheme + "://" + host, true
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
