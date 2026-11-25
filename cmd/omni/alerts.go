package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/MiguelDev-GH/omni/internal/ws"
)

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

// pruneAlerts apaga, a cada hora, os alertas gravados há mais de retention,
// a mesma retenção das verificações.
func pruneAlerts(ctx context.Context, st interface {
	PruneAlerts(ctx context.Context, before time.Time) error
}, retention time.Duration) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		if err := st.PruneAlerts(ctx, time.Now().Add(-retention)); err != nil && ctx.Err() == nil {
			slog.Error("alertas: apagar antigos", "erro", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
