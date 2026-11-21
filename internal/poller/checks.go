// Package poller verifica periodicamente se os alvos monitorados estão no ar.
package poller

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/MiguelDev-GH/omni/internal/probe"
)

const userAgent = "Omni-Poller/1"

// Result é o resultado de uma verificação de uma porta de um alvo.
type Result struct {
	TargetID   int64     `json:"alvo_id"`
	Port       int       `json:"porta"`
	Kind       string    `json:"tipo"` // "http", "tcp", "dns" ou "smtp"
	Up         bool      `json:"no_ar"`
	StatusCode int       `json:"status_http,omitempty"`
	LatencyMS  float64   `json:"latencia_ms"`
	Err        string    `json:"erro,omitempty"`
	At         time.Time `json:"hora"`
	// Degraded marca um servidor que respondeu com erro: RCODE diferente
	// de NOERROR no DNS ou comando recusado no SMTP. No HTTP, o erro fica
	// em StatusCode.
	Degraded bool `json:"degradado,omitempty"`
	// Detail resume a resposta das sondas DNS e SMTP.
	Detail string `json:"detalhe,omitempty"`
	// StartTLS diz se o servidor SMTP oferece STARTTLS; nil fora do SMTP.
	StartTLS *bool `json:"starttls,omitempty"`
}

// Tipos de verificação.
const (
	KindHTTP = "http"
	KindTCP  = "tcp"
	KindDNS  = "dns"
	KindSMTP = "smtp"
)

// NewHTTPClient cria o cliente das verificações HTTP: com o timeout dado e
// sem seguir redirecionamentos nem usar o proxy do ambiente (um proxy
// responderia 502 por um alvo fora do ar, que seria dado como no ar).
func NewHTTPClient(timeout time.Duration) *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil
	return &http.Client{
		Timeout:   timeout,
		Transport: t,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func latencyMS(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}

// CheckHTTP faz um GET em http://host:porta/. O alvo está no ar se houver
// resposta, com qualquer status.
func CheckHTTP(ctx context.Context, c *http.Client, targetID int64, host string, port int) Result {
	start := time.Now()
	res := Result{TargetID: targetID, Port: port, Kind: "http", At: start.UTC()}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/", nil)
	if err != nil {
		res.Err = err.Error()
		res.LatencyMS = latencyMS(time.Since(start))
		return res
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.Do(req)
	res.LatencyMS = latencyMS(time.Since(start))
	if err != nil {
		res.Err = err.Error()
		return res
	}
	_ = resp.Body.Close()
	res.Up = true
	res.StatusCode = resp.StatusCode
	return res
}

// CheckTCP tenta abrir uma conexão TCP com host:porta e a fecha em seguida.
func CheckTCP(ctx context.Context, targetID int64, host string, port int, timeout time.Duration) Result {
	start := time.Now()
	res := Result{TargetID: targetID, Port: port, Kind: "tcp", At: start.UTC()}

	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	res.LatencyMS = latencyMS(time.Since(start))
	if err != nil {
		res.Err = err.Error()
		return res
	}
	_ = conn.Close()
	res.Up = true
	return res
}

// CheckDNS pergunta ao servidor DNS em host:porta pelo registro A de domain.
// O servidor está no ar se responder; um RCODE de erro ou uma resposta sem
// endereços marca Degraded.
func CheckDNS(ctx context.Context, targetID int64, host string, port int, domain string) Result {
	at := time.Now().UTC()
	r := probe.QueryDNS(ctx, host, port, domain, CheckTimeout)
	return Result{
		TargetID: targetID, Port: port, Kind: KindDNS, At: at,
		Up: r.Up, Degraded: r.Up && !r.OK(), LatencyMS: r.LatencyMS, Err: r.Err, Detail: r.Detail(),
	}
}

// CheckSMTP conversa com o servidor SMTP em host:porta sem enviar e-mail.
// Com domain, testa também o destinatário postmaster@domain.
func CheckSMTP(ctx context.Context, targetID int64, host string, port int, domain string) Result {
	at := time.Now().UTC()
	r := probe.ProbeSMTP(ctx, host, port, domain, CheckTimeout)
	res := Result{
		TargetID: targetID, Port: port, Kind: KindSMTP, At: at,
		Up: r.Up, Degraded: r.Up && !r.OK(), LatencyMS: r.LatencyMS, Err: r.Err, Detail: r.Detail(),
	}
	// O suporte a STARTTLS só é conhecido depois de um EHLO aceito.
	if r.Up && r.Stage != "ehlo" {
		tls := r.StartTLS
		res.StartTLS = &tls
	}
	return res
}
