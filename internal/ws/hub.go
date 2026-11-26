// Package ws entrega eventos do Omni aos navegadores por WebSocket.
package ws

import (
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
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
	// inspect é o filtro do Inspetor; nil quando o cliente não o assinou.
	// Protegido por Hub.mu.
	inspect *InspectorFilter
}

func (c *client) stop() { c.once.Do(func() { close(c.done) }) }

// Hub distribui eventos a todos os clientes inscritos. Um cliente lento perde
// as mensagens mais antigas em vez de atrasar os outros.
type Hub struct {
	mu      sync.Mutex
	clients map[*client]struct{}
	closed  bool
	buffer  int

	// inspecting conta os clientes com o Inspetor ativo; onInspect, protegida
	// por mu, é avisada quando a contagem sai de zero ou volta a zero.
	inspecting atomic.Int64
	onInspect  func(bool)
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
		h.setInspectLocked(c, nil)
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
	if _, ok := h.clients[c]; ok {
		h.setInspectLocked(c, nil)
	}
	delete(h.clients, c)
	h.mu.Unlock()
	c.stop()
}

// setInspect liga (f != nil) ou desliga o Inspetor do cliente c.
func (h *Hub) setInspect(c *client, f *InspectorFilter) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		h.setInspectLocked(c, f)
	}
}

// setInspectLocked troca o filtro de c e avisa onInspect quando a contagem de
// clientes do Inspetor sai de zero ou volta a zero. O aviso roda com h.mu
// travado, para chegar na mesma ordem das mudanças. Exige h.mu.
func (h *Hub) setInspectLocked(c *client, f *InspectorFilter) {
	was := c.inspect != nil
	c.inspect = f
	var changed bool
	switch {
	case !was && f != nil:
		changed = h.inspecting.Add(1) == 1
	case was && f == nil:
		changed = h.inspecting.Add(-1) == 0
	}
	if changed && h.onInspect != nil {
		h.onInspect(f != nil)
	}
}

type inspectorSub struct {
	client *client
	filter InspectorFilter
}

// inspectors devolve os clientes com o Inspetor ativo e os seus filtros.
func (h *Hub) inspectors() []inspectorSub {
	h.mu.Lock()
	defer h.mu.Unlock()
	var subs []inspectorSub
	for c := range h.clients {
		if c.inspect != nil {
			subs = append(subs, inspectorSub{client: c, filter: *c.inspect})
		}
	}
	return subs
}

// sendTo entrega ev só ao cliente c, sem bloquear.
func (h *Hub) sendTo(c *client, ev Event) {
	msg, err := json.Marshal(ev)
	if err != nil {
		slog.Error("serializar evento", "tipo", ev.Type, "erro", err)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		deliver(c, msg)
	}
}
