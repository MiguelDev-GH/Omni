package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/MiguelDev-GH/omni/internal/capture"
	"github.com/MiguelDev-GH/omni/internal/sniffer"
	"github.com/MiguelDev-GH/omni/internal/ws"
)

// Captures grava e serve as capturas .pcap; é o capture.Recorder.
type Captures interface {
	Start(ctx context.Context, d time.Duration, f ws.InspectorFilter) (int64, error)
	List(ctx context.Context) ([]capture.Capture, error)
	Open(ctx context.Context, id int64) (*os.File, capture.Capture, error)
	Packets(ctx context.Context, id int64, page int) ([]sniffer.PacketSummary, capture.Capture, error)
	Delete(ctx context.Context, id int64) error
}

type captureRequest struct {
	Duration int                `json:"duracao_s"`
	Filter   ws.InspectorFilter `json:"filtro"`
}

// StartCaptureHandler responde POST /api/capturas {"duracao_s", "filtro"}
// com 202 e o id da captura, que grava em segundo plano. O filtro é o do
// Inspetor ({"ip", "porta", "protocolo"}); vazio grava tudo o que o sniffer vê.
func StartCaptureHandler(c Captures) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req captureRequest
		if status, msg, ok := decodeJSON(w, r, &req); !ok {
			writeError(w, status, msg)
			return
		}
		id, err := c.Start(r.Context(), time.Duration(req.Duration)*time.Second, req.Filter)
		switch {
		case err == nil:
			writeJSON(w, http.StatusAccepted, map[string]int64{"id": id})
		case errors.Is(err, capture.ErrDuracaoInvalida):
			writeField(w, http.StatusBadRequest, err.Error(), "duracao_s")
		case errors.Is(err, capture.ErrFiltroInvalido):
			writeField(w, http.StatusBadRequest, err.Error(), "filtro")
		case errors.Is(err, capture.ErrEmAndamento), errors.Is(err, capture.ErrDiretorioCheio):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, capture.ErrSnifferInativo):
			writeError(w, http.StatusServiceUnavailable, err.Error())
		default:
			captureError(w, err)
		}
	})
}

// ListCapturesHandler responde GET /api/capturas, das mais novas para as mais antigas.
func ListCapturesHandler(c Captures) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		list, err := c.List(r.Context())
		if err != nil {
			captureError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}

// CaptureFileHandler responde GET /api/capturas/{id}/arquivo com o .pcap.
func CaptureFileHandler(c Captures) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		f, capt, err := c.Open(r.Context(), id)
		if err != nil {
			captureError(w, err)
			return
		}
		defer func() { _ = f.Close() }()
		w.Header().Set("Content-Type", "application/vnd.tcpdump.pcap")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="omni-captura-%d.pcap"`, capt.ID))
		w.Header().Set("Cache-Control", "no-store")
		if info, err := f.Stat(); err == nil {
			w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
		}
		if _, err := io.Copy(w, f); err != nil {
			slog.Warn("capturas: enviar arquivo", "id", id, "erro", err)
		}
	})
}

type capturePage struct {
	Packets []sniffer.PacketSummary `json:"pacotes"`
	Page    int                     `json:"pagina"`
	PerPage int                     `json:"por_pagina"`
	Total   int                     `json:"total"`
}

// CapturePacketsHandler responde GET /api/capturas/{id}/pacotes?pagina= com
// os resumos de até 500 pacotes, como no Inspetor ao vivo. A página começa em 0.
func CapturePacketsHandler(c Captures) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		page := 0
		if v := r.URL.Query().Get("pagina"); v != "" {
			p, err := strconv.Atoi(v)
			if err != nil || p < 0 {
				writeError(w, http.StatusBadRequest, "pagina inválida")
				return
			}
			page = p
		}
		list, capt, err := c.Packets(r.Context(), id, page)
		if err != nil {
			captureError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, capturePage{Packets: list, Page: page, PerPage: capture.PageSize, Total: capt.Packets})
	})
}

// DeleteCaptureHandler responde DELETE /api/capturas/{id} com 204.
func DeleteCaptureHandler(c Captures) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		if err := c.Delete(r.Context(), id); err != nil {
			captureError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func captureError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, capture.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, capture.ErrAindaGravando):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, capture.ErrArquivoIndisponivel):
		writeError(w, http.StatusGone, err.Error())
	default:
		storeError(w, err)
	}
}
