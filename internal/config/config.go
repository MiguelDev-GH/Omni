// Package config lê a configuração do Omni a partir de variáveis de ambiente.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/MiguelDev-GH/omni/internal/alerts"
	"github.com/MiguelDev-GH/omni/internal/security"
)

const (
	minTokenLen         = 32
	defaultPollInterval = "30s"
	minPollInterval     = time.Second
	defaultInterface    = "any"
	defaultRetention    = "168h"
	minRetention        = time.Hour
)

// interfaceRe aceita nomes de interface do Linux (até 15 caracteres), sem hífen no início.
var interfaceRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:@-]{0,14}$`)

var (
	ErrTokenAusente   = errors.New("OMNI_TOKEN não definido")
	ErrTokenCurto     = fmt.Errorf("OMNI_TOKEN precisa ter no mínimo %d caracteres", minTokenLen)
	ErrEnderecoRemoto = errors.New("endereço fora do loopback; defina OMNI_PERMITIR_REMOTO=1 para permitir")

	ErrIntervaloInvalido = errors.New("OMNI_INTERVALO_POLLER inválido; use uma duração de no mínimo 1s, como 30s")
	ErrOrigemInvalida    = errors.New("origem inválida em OMNI_ORIGENS; use esquema http ou https, host e porta opcional")
	ErrRetencaoInvalida  = errors.New("OMNI_RETENCAO_VERIFICACOES inválida; use uma duração de no mínimo 1h, como 168h")
	ErrInterfaceInvalida = errors.New("OMNI_INTERFACE inválida; use o nome de uma interface, como eth0, ou any")
	ErrPGDSNDuplo        = errors.New("use OMNI_PG_DSN ou OMNI_PG_DSN_ARQUIVO, não os dois")
)

type Config struct {
	Addr        string
	DBPath      string
	Token       string
	AllowRemote bool

	PollInterval time.Duration
	// CheckRetention é por quanto tempo os resultados das verificações ficam no banco.
	CheckRetention time.Duration
	AllowedOrigins []string

	// Interface é onde o sniffer captura; "any" cobre todas, inclusive o loopback.
	Interface string

	// Mail configura o envio de alertas por e-mail; desligado sem OMNI_SMTP_HOST.
	Mail alerts.MailConfig

	// WatchedFiles são os arquivos de configuração cuja integridade é
	// monitorada (OMNI_ARQUIVOS_MONITORADOS, separados por vírgula).
	WatchedFiles []string

	// PGDSN é a conexão opcional com o PostgreSQL usada para medir o tamanho
	// dos bancos (OMNI_PG_DSN ou, preferível, o arquivo em
	// OMNI_PG_DSN_ARQUIVO). Vazio desliga a medição.
	PGDSN string

	// extraOrigins guarda as origens de OMNI_ORIGENS já validadas, para Bind
	// refazer AllowedOrigins com a porta real.
	extraOrigins []string
}

// Load monta a Config usando getenv (normalmente os.Getenv).
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		// A porta 0 faz o sistema escolher uma porta alta livre a cada execução.
		Addr:        withDefault(getenv("OMNI_ADDR"), "127.0.0.1:0"),
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
	retention, err := parseRetention(withDefault(getenv("OMNI_RETENCAO_VERIFICACOES"), defaultRetention))
	if err != nil {
		return Config{}, err
	}
	c.CheckRetention = retention
	extra, err := parseOrigins(getenv("OMNI_ORIGENS"))
	if err != nil {
		return Config{}, err
	}
	c.extraOrigins = extra
	origins, err := buildOrigins(c.Addr, extra)
	if err != nil {
		return Config{}, err
	}
	c.AllowedOrigins = origins
	c.Interface = withDefault(getenv("OMNI_INTERFACE"), defaultInterface)
	if !interfaceRe.MatchString(c.Interface) {
		return Config{}, fmt.Errorf("%w: %q", ErrInterfaceInvalida, c.Interface)
	}
	mail, err := alerts.MailConfigFromEnv(getenv, os.ReadFile)
	if err != nil {
		return Config{}, err
	}
	c.Mail = mail
	c.WatchedFiles, err = security.ParseWatchList(getenv("OMNI_ARQUIVOS_MONITORADOS"))
	if err != nil {
		return Config{}, err
	}
	c.PGDSN, err = pgDSN(getenv, os.ReadFile)
	if err != nil {
		return Config{}, err
	}
	return c, nil
}

// pgDSN lê OMNI_PG_DSN ou o arquivo em OMNI_PG_DSN_ARQUIVO. O arquivo é
// preferível: variáveis de ambiente ficam visíveis em /proc/<pid>/environ.
// O erro de leitura nunca traz o conteúdo do arquivo.
func pgDSN(getenv func(string) string, readFile func(string) ([]byte, error)) (string, error) {
	dsn, file := getenv("OMNI_PG_DSN"), getenv("OMNI_PG_DSN_ARQUIVO")
	switch {
	case dsn != "" && file != "":
		return "", ErrPGDSNDuplo
	case file != "":
		b, err := readFile(file)
		if err != nil {
			return "", fmt.Errorf("ler OMNI_PG_DSN_ARQUIVO: %w", err)
		}
		return strings.TrimSpace(string(b)), nil
	default:
		return strings.TrimSpace(dsn), nil
	}
}

func parsePollInterval(v string) (time.Duration, error) {
	d, err := time.ParseDuration(v)
	if err != nil || d < minPollInterval {
		return 0, fmt.Errorf("%w: %s", ErrIntervaloInvalido, v)
	}
	return d, nil
}

func parseRetention(v string) (time.Duration, error) {
	d, err := time.ParseDuration(v)
	if err != nil || d < minRetention {
		return 0, fmt.Errorf("%w: %s", ErrRetencaoInvalida, v)
	}
	return d, nil
}

// Bind ajusta a Config ao endereço em que o servidor de fato escuta. Com a
// porta 0 é o sistema que a escolhe, e as origens aceitas dependem dela.
func (c Config) Bind(addr net.Addr) Config {
	c.Addr = addr.String()
	// O host já passou por Load e a porta veio do sistema; não há erro possível.
	c.AllowedOrigins, _ = buildOrigins(c.Addr, c.extraOrigins)
	return c
}

// parseOrigins valida a lista de OMNI_ORIGENS, separada por vírgulas.
func parseOrigins(extra string) ([]string, error) {
	if strings.TrimSpace(extra) == "" {
		return nil, nil
	}
	var origins []string
	for _, item := range strings.Split(extra, ",") {
		item = strings.TrimSpace(item)
		origin, ok := normalizeOrigin(item)
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrOrigemInvalida, item)
		}
		origins = append(origins, origin)
	}
	return origins, nil
}

// buildOrigins devolve as origens aceitas, sem repetição: loopback na porta
// de addr e depois as extras.
func buildOrigins(addr string, extra []string) ([]string, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrEnderecoRemoto, addr)
	}
	origins := []string{
		"http://127.0.0.1:" + port,
		"http://localhost:" + port,
		"http://[::1]:" + port,
	}
	origins = append(origins, extra...)
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
