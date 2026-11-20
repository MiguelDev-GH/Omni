package ws

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Tickets guarda tickets de uso único que autorizam abrir um WebSocket.
// O navegador não envia cabeçalhos no upgrade, então o token da API é trocado
// por um ticket curto que vai na query string.
type Tickets struct {
	mu      sync.Mutex
	ttl     time.Duration
	expires map[string]time.Time
	now     func() time.Time
}

func NewTickets(ttl time.Duration) *Tickets {
	return &Tickets{ttl: ttl, expires: make(map[string]time.Time), now: time.Now}
}

// Issue gera um ticket aleatório de 32 bytes em hexadecimal.
func (t *Tickets) Issue() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	ticket := hex.EncodeToString(b)

	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	for k, exp := range t.expires {
		if !now.Before(exp) {
			delete(t.expires, k)
		}
	}
	t.expires[ticket] = now.Add(t.ttl)
	return ticket, nil
}

// Consume apaga o ticket e informa se ele existia e ainda era válido.
func (t *Tickets) Consume(ticket string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	exp, ok := t.expires[ticket]
	if !ok {
		return false
	}
	delete(t.expires, ticket)
	return t.now().Before(exp)
}
