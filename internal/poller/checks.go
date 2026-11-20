// Package poller verifica periodicamente se os alvos monitorados estão no ar.
package poller

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"time"
)

const userAgent = "Omni-Poller/1"

// Result é o resultado de uma verificação de uma porta de um alvo.
type Result struct {
	TargetID   int64     `json:"alvo_id"`
	Port       int       `json:"porta"`
	Kind       string    `json:"tipo"` // "http" ou "tcp"
	Up         bool      `json:"no_ar"`
	StatusCode int       `json:"status_http,omitempty"`
	LatencyMS  float64   `json:"latencia_ms"`
	Err        string    `json:"erro,omitempty"`
	At         time.Time `json:"hora"`
}

// NewHTTPClient cria o cliente das verificações HTTP: com o timeout dado e
// sem seguir redirecionamentos.
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
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
