package main

import "github.com/MiguelDev-GH/omni/internal/ws"

// alertTap repassa cada evento ao próximo Publisher e uma cópia a feed, sem
// bloquear: é assim que o observador de alertas enxerga as métricas.
type alertTap struct {
	next ws.Publisher
	feed func(eventType string, data any)
}

func (t alertTap) Publish(ev ws.Event) {
	t.next.Publish(ev)
	t.feed(ev.Type, ev.Data)
}
