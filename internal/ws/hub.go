// Package ws entrega eventos do Omni aos navegadores por WebSocket.
package ws

import (
	"encoding/json"
	"log/slog"
	"sync"
)

// Event é a mensagem enviada a cada cliente conectado.
type Event struct {
	Type string `json:"tipo"`
	Data any    `json:"dados"`
}

// Publisher recebe eventos para distribuir; Publish nunca bloqueia.
type Publisher interface {
	Publish(Event)
}

type client struct {
	send chan []byte
	done chan struct{}
	once sync.Once
}

func (c *client) stop() { c.once.Do(func() { close(c.done) }) }

// Hub distribui eventos a todos os clientes inscritos. Um cliente lento perde
// as mensagens mais antigas em vez de atrasar os outros.
type Hub struct {
	mu      sync.Mutex
	clients map[*client]struct{}
	closed  bool
	buffer  int
}

func NewHub(clientBuffer int) *Hub {
	if clientBuffer < 1 {
		clientBuffer = 1
	}
	return &Hub{clients: make(map[*client]struct{}), buffer: clientBuffer}
}

// Publish serializa o evento uma vez e o entrega a cada cliente sem bloquear.
func (h *Hub) Publish(ev Event) {
	msg, err := json.Marshal(ev)
	if err != nil {
		slog.Error("serializar evento", "tipo", ev.Type, "erro", err)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		deliver(c, msg)
	}
}

// deliver envia msg sem bloquear; com o canal cheio, descarta a mais antiga.
func deliver(c *client, msg []byte) {
	for {
		select {
		case c.send <- msg:
			return
		default:
		}
		select {
		case <-c.send:
		default:
		}
	}
}

// Close desconecta todos os clientes e recusa novos. Pode ser chamado mais de uma vez.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for c := range h.clients {
		c.stop()
		delete(h.clients, c)
	}
}

func (h *Hub) subscribe() (*client, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, false
	}
	c := &client{send: make(chan []byte, h.buffer), done: make(chan struct{})}
	h.clients[c] = struct{}{}
	return c, true
}

func (h *Hub) unsubscribe(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	c.stop()
}
