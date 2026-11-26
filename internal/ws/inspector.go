package ws

import (
	"context"
	"encoding/json"
	"net/netip"
	"sync/atomic"
	"time"
)

const (
	// EventPackets leva um lote do Inspetor aos clientes que o assinaram.
	EventPackets = "PACOTES"
	// CommandInspector é o comando do cliente que liga, desliga ou filtra o Inspetor.
	CommandInspector = "INSPETOR"

	inspectorQueue    = 4096
	inspectorEvery    = 250 * time.Millisecond
	inspectorMaxBatch = 500
	maxProtocolLen    = 32
)

// InspectorFilter restringe os pacotes que um cliente recebe; campos vazios
// ou zero não filtram.
type InspectorFilter struct {
	IP       string `json:"ip"`
	Port     int    `json:"porta"`
	Protocol string `json:"protocolo"`
}

// Inspectable é um pacote resumido que sabe se passa num filtro.
type Inspectable interface {
	MatchesFilter(InspectorFilter) bool
}

// PacketBatch é o conteúdo do evento PACOTES. Dropped soma os pacotes que não
// couberam na fila do Inspetor e os que passaram do limite do lote.
type PacketBatch struct {
	Packets []Inspectable `json:"pacotes"`
	Dropped uint64        `json:"descartados"`
}

// inspectorCommand é o campo "dados" do comando INSPETOR.
type inspectorCommand struct {
	Active bool            `json:"ativo"`
	Filter InspectorFilter `json:"filtro"`
}

// Inspector junta os pacotes do tap e os envia em lotes aos clientes do hub
// que assinaram o Inspetor, já filtrados por cliente. Tap nunca bloqueia.
type Inspector struct {
	hub      *Hub
	in       chan Inspectable
	dropped  atomic.Uint64
	every    time.Duration
	maxBatch int
	// onActive é protegida por hub.mu.
	onActive func(bool)
}

// NewInspector liga o Inspetor ao hub, que passa a aceitar o comando INSPETOR.
func NewInspector(h *Hub) *Inspector {
	i := &Inspector{hub: h, in: make(chan Inspectable, inspectorQueue), every: inspectorEvery, maxBatch: inspectorMaxBatch}
	h.mu.Lock()
	h.onInspect = i.activeChanged
	h.mu.Unlock()
	return i
}

// OnActive registra fn, chamada com true quando o primeiro cliente assina e
// com false quando o último sai. É chamada na hora com o estado atual. fn roda
// com o hub travado e não pode chamar o hub.
func (i *Inspector) OnActive(fn func(bool)) {
	i.hub.mu.Lock()
	defer i.hub.mu.Unlock()
	i.onActive = fn
	fn(i.hub.inspecting.Load() > 0)
}

// activeChanged roda com hub.mu travado.
func (i *Inspector) activeChanged(active bool) {
	if i.onActive != nil {
		i.onActive(active)
	}
}

// Tap enfileira p sem bloquear; com a fila cheia, ou sem nenhum cliente
// assinando, o pacote é descartado.
func (i *Inspector) Tap(p Inspectable) {
	if i.hub.inspecting.Load() == 0 {
		return
	}
	select {
	case i.in <- p:
	default:
		i.dropped.Add(1)
	}
}

// Run envia um lote a cada intervalo até ctx acabar.
func (i *Inspector) Run(ctx context.Context) {
	t := time.NewTicker(i.every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			i.flush()
		}
	}
}

// flush esvazia a fila e entrega a cada cliente ativo os pacotes do seu
// filtro, no máximo maxBatch, ficando com os mais recentes.
func (i *Inspector) flush() {
	var pkts []Inspectable
drain:
	for len(pkts) < inspectorQueue {
		select {
		case p := <-i.in:
			pkts = append(pkts, p)
		default:
			break drain
		}
	}
	dropped := i.dropped.Swap(0)
	for _, sub := range i.hub.inspectors() {
		batch := PacketBatch{Packets: []Inspectable{}, Dropped: dropped}
		for _, p := range pkts {
			if p.MatchesFilter(sub.filter) {
				batch.Packets = append(batch.Packets, p)
			}
		}
		if extra := len(batch.Packets) - i.maxBatch; extra > 0 {
			batch.Packets = batch.Packets[extra:]
			batch.Dropped += uint64(extra)
		}
		if len(batch.Packets) == 0 && batch.Dropped == 0 {
			continue
		}
		i.hub.sendTo(sub.client, Event{Type: EventPackets, Data: batch})
	}
}

// parseInspectorCommand lê uma mensagem do cliente. ok é falso para outros
// tipos de mensagem e para filtros inválidos, que são ignorados.
func parseInspectorCommand(msg []byte) (cmd inspectorCommand, ok bool) {
	var env struct {
		Type string          `json:"tipo"`
		Data json.RawMessage `json:"dados"`
	}
	if json.Unmarshal(msg, &env) != nil || env.Type != CommandInspector {
		return cmd, false
	}
	if json.Unmarshal(env.Data, &cmd) != nil {
		return cmd, false
	}
	f, ok := NormalizeFilter(cmd.Filter)
	cmd.Filter = f
	return cmd, ok
}

// NormalizeFilter confere o filtro do Inspetor e devolve o IP na forma
// canônica. ok é false com IP que não é endereço, porta fora de 0 a 65535 ou
// protocolo longo demais.
func NormalizeFilter(f InspectorFilter) (InspectorFilter, bool) {
	if f.IP != "" {
		addr, err := netip.ParseAddr(f.IP)
		if err != nil {
			return f, false
		}
		f.IP = addr.Unmap().String()
	}
	if f.Port < 0 || f.Port > 65535 || len(f.Protocol) > maxProtocolLen {
		return f, false
	}
	return f, true
}
