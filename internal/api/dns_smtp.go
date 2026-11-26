package api

import (
	"context"
	"net/http"
	"time"

	"github.com/MiguelDev-GH/omni/internal/mailqueue"
	"github.com/MiguelDev-GH/omni/internal/sniffer/l7"
)

type DNSStore interface {
	DNSSince(ctx context.Context, since time.Time, limit int) ([]l7.DNSWindow, error)
}

type SMTPStore interface {
	SMTPSince(ctx context.Context, since time.Time, limit int) ([]l7.SMTPWindow, error)
}

// DNSHandler responde GET /api/dns?desde=<RFC3339> com as janelas de 10 s das
// consultas DNS vistas no tráfego; sem desde, devolve os últimos 15 minutos.
func DNSHandler(ds DNSStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, ok := parseSince(w, r, l7Window)
		if !ok {
			return
		}
		list, err := ds.DNSSince(r.Context(), since, l7Limit)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}

// SMTPHandler responde GET /api/smtp?desde=<RFC3339> com as janelas de 10 s
// das transações SMTP vistas no tráfego; sem desde, os últimos 15 minutos.
func SMTPHandler(ss SMTPStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, ok := parseSince(w, r, l7Window)
		if !ok {
			return
		}
		list, err := ss.SMTPSince(r.Context(), since, l7Limit)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}

type QueueStore interface {
	QueueSince(ctx context.Context, since time.Time, limit int) ([]mailqueue.Snapshot, error)
}

// queueLimit cobre 24 h de contagens a cada 30 s.
const queueLimit = 2880

type queueResponse struct {
	Configured bool                 `json:"configurado"`
	Counts     []mailqueue.Snapshot `json:"contagens"`
}

// SMTPQueueHandler responde GET /api/smtp/fila?desde=<RFC3339> com as
// contagens da fila do SMTP; sem desde, os últimos 15 minutos. configured diz
// se OMNI_SMTP_FILA_DIRS está definido.
func SMTPQueueHandler(qs QueueStore, configured bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, ok := parseSince(w, r, l7Window)
		if !ok {
			return
		}
		list, err := qs.QueueSince(r.Context(), since, queueLimit)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, queueResponse{Configured: configured, Counts: list})
	})
}
