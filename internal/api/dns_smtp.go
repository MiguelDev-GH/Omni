package api

import (
	"context"
	"net/http"
	"time"

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
