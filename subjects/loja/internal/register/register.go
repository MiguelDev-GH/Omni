// Package register cadastra a loja cobaia como alvo na API do Omni.
package register

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ErrBadToken indica que o Omni recusou o token.
var ErrBadToken = errors.New("token do Omni recusado (401)")

// Target tem os mesmos campos JSON do alvo do Omni.
type Target struct {
	ID       int64  `json:"id,omitempty"`
	Name     string `json:"nome"`
	Host     string `json:"host"`
	AppPorts []int  `json:"portas_app"`
	DBPort   int    `json:"porta_banco,omitempty"`
}

// Targets são os alvos da cobaia. O loja-tls existe porque o testssl do Omni
// testa a primeira porta de aplicação, e assim ele mira o Caddy.
var Targets = []Target{
	{Name: "loja", Host: "127.0.0.1", AppPorts: []int{3000}, DBPort: 5433},
	{Name: "loja-tls", Host: "127.0.0.1", AppPorts: []int{8443}},
}

// Register cria no Omni os alvos de want que ainda não existem, comparando
// pelo nome, e devolve os nomes criados e os que já existiam.
func Register(ctx context.Context, c *http.Client, baseURL, token string, want []Target) (created, existing []string, err error) {
	var have []Target
	if err := call(ctx, c, http.MethodGet, baseURL+"/api/alvos", token, nil, http.StatusOK, &have); err != nil {
		return nil, nil, err
	}
	names := make(map[string]bool, len(have))
	for _, t := range have {
		names[t.Name] = true
	}
	for _, t := range want {
		if names[t.Name] {
			existing = append(existing, t.Name)
			continue
		}
		if err := call(ctx, c, http.MethodPost, baseURL+"/api/alvos", token, t, http.StatusCreated, nil); err != nil {
			return created, existing, fmt.Errorf("cadastrar %s: %w", t.Name, err)
		}
		created = append(created, t.Name)
	}
	return created, existing, nil
}

func call(ctx context.Context, c *http.Client, method, url, token string, in any, want int, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return ErrBadToken
	case resp.StatusCode != want:
		var e struct {
			Erro string `json:"erro"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 4<<10)).Decode(&e)
		return fmt.Errorf("%s %s: status %d: %s", method, url, resp.StatusCode, e.Erro)
	case out != nil:
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
