package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/MiguelDev-GH/omni/internal/arsenal"
	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/orchestrator"
	"github.com/MiguelDev-GH/omni/internal/store"
	"github.com/MiguelDev-GH/omni/internal/target"
)

// Scanner dispara varreduras; é o orquestrador.
type Scanner interface {
	Start(ctx context.Context, targetID int64, tools []string, opts map[string]arsenal.Options) (int64, error)
}

type ScanStore interface {
	GetScan(ctx context.Context, id int64) (store.Scan, error)
	ListScans(ctx context.Context, targetID int64) ([]store.Scan, error)
	ScanFindings(ctx context.Context, scanID int64) ([]findings.Finding, error)
	Report(ctx context.Context, scanID int64, tool string) (json.RawMessage, bool, error)
}

type scanRequest struct {
	TargetID int64                      `json:"alvo_id"`
	Tools    []string                   `json:"ferramentas"`
	Options  map[string]arsenal.Options `json:"opcoes"`
}

// StartScanHandler responde POST /api/varreduras {"alvo_id", "ferramentas",
// "opcoes"} com 202 e o id da varredura, que roda em segundo plano. As
// opções vêm por ferramenta: {"<ferramenta>": {"<chave>": "<valor>"}}.
func StartScanHandler(sc Scanner) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req scanRequest
		if status, msg, ok := decodeJSON(w, r, &req); !ok {
			writeError(w, status, msg)
			return
		}
		if req.TargetID <= 0 {
			writeField(w, http.StatusBadRequest, "informe o alvo", "alvo_id")
			return
		}
		id, err := sc.Start(r.Context(), req.TargetID, req.Tools, req.Options)
		var (
			ve *target.ValidationError
			oe *arsenal.OptionError
		)
		switch {
		case err == nil:
			writeJSON(w, http.StatusAccepted, map[string]int64{"id": id})
		case errors.As(err, &oe):
			writeField(w, http.StatusBadRequest, err.Error(), oe.Field())
		case errors.Is(err, orchestrator.ErrSemFerramentas), errors.Is(err, orchestrator.ErrFerramentaDesconhecida):
			writeField(w, http.StatusBadRequest, err.Error(), "ferramentas")
		case errors.As(err, &ve):
			writeField(w, http.StatusBadRequest, "alvo inválido: "+ve.Error(), "alvo_id")
		case errors.Is(err, arsenal.ErrForaDoEscopo):
			writeField(w, http.StatusBadRequest, err.Error(), "alvo_id")
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "alvo não encontrado")
		case errors.Is(err, orchestrator.ErrDockerIndisponivel):
			slog.Warn("varredura recusada", "erro", err)
			writeError(w, http.StatusServiceUnavailable, "Docker indisponível")
		case errors.Is(err, orchestrator.ErrVarreduraEmAndamento):
			writeError(w, http.StatusConflict, err.Error())
		default:
			storeError(w, err)
		}
	})
}

// ListScansHandler responde GET /api/varreduras?alvo_id= com as varreduras
// mais recentes primeiro; sem alvo_id, de todos os alvos.
func ListScansHandler(st ScanStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var targetID int64
		if v := r.URL.Query().Get("alvo_id"); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil || id <= 0 {
				writeError(w, http.StatusBadRequest, "alvo_id inválido")
				return
			}
			targetID = id
		}
		list, err := st.ListScans(r.Context(), targetID)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}

// GetScanHandler responde GET /api/varreduras/{id}.
func GetScanHandler(st ScanStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		sc, err := st.GetScan(r.Context(), id)
		if err != nil {
			scanError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, sc)
	})
}

// ScanFindingsHandler responde GET /api/varreduras/{id}/achados, dos mais graves aos menos graves.
func ScanFindingsHandler(st ScanStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		if _, err := st.GetScan(r.Context(), id); err != nil {
			scanError(w, err)
			return
		}
		list, err := st.ScanFindings(r.Context(), id)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}

// ScanReportHandler responde GET /api/varreduras/{id}/relatorio/{ferramenta}
// com o relatório estruturado da ferramenta, ou 404 quando não há.
func ScanReportHandler(st ScanStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		if _, err := st.GetScan(r.Context(), id); err != nil {
			scanError(w, err)
			return
		}
		report, ok, err := st.Report(r.Context(), id, r.PathValue("ferramenta"))
		if err != nil {
			storeError(w, err)
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, "relatório indisponível")
			return
		}
		writeJSON(w, http.StatusOK, report)
	})
}

// ToolsHandler responde GET /api/ferramentas com as ferramentas do Arsenal.
func ToolsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, arsenal.Tools)
	})
}

func scanError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrScanNotFound) {
		writeError(w, http.StatusNotFound, "varredura não encontrada")
		return
	}
	storeError(w, err)
}

func writeField(w http.ResponseWriter, status int, msg, field string) {
	writeJSON(w, status, map[string]string{"erro": msg, "campo": field})
}

// CompareScanHandler responde GET /api/varreduras/{id}/comparacao com os
// achados novos e resolvidos em relação à varredura concluída anterior do
// mesmo alvo, ferramenta por ferramenta.
func CompareScanHandler(st orchestrator.Store, scans ScanStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		sc, err := scans.GetScan(r.Context(), id)
		if err != nil {
			scanError(w, err)
			return
		}
		cmp, err := orchestrator.Compare(r.Context(), st, sc)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, cmp)
	})
}
