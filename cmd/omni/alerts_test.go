package main

import (
	"testing"

	"github.com/MiguelDev-GH/omni/internal/ws"
)

type recordPub struct{ got []ws.Event }

func (r *recordPub) Publish(ev ws.Event) { r.got = append(r.got, ev) }

func TestAlertTapForwardsAndFeeds(t *testing.T) {
	next := &recordPub{}
	var fed []string
	tap := alertTap{next: next, feed: func(typ string, _ any) { fed = append(fed, typ) }}

	tap.Publish(ws.Event{Type: "METRICAS_SISTEMA", Data: map[string]float64{"cpu": 1}})

	if len(next.got) != 1 || next.got[0].Type != "METRICAS_SISTEMA" {
		t.Fatalf("hub recebeu %+v", next.got)
	}
	if len(fed) != 1 || fed[0] != "METRICAS_SISTEMA" {
		t.Fatalf("observador recebeu %v", fed)
	}
}
