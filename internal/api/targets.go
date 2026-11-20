package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/MiguelDev-GH/omni/internal/store"
	"github.com/MiguelDev-GH/omni/internal/target"
)

func (s *Server) listTargets(w http.ResponseWriter, r *http.Request) {
	list, err := s.targets.ListTargets(r.Context())
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) createTarget(w http.ResponseWriter, r *http.Request) {
	t, ok := readTarget(w, r)
	if !ok {
		return
	}
	created, err := s.targets.CreateTarget(r.Context(), t)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) getTarget(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	t, err := s.targets.GetTarget(r.Context(), id)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) updateTarget(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	t, ok := readTarget(w, r)
	if !ok {
		return
	}
	t.ID = id
	if err := s.targets.UpdateTarget(r.Context(), t); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) deleteTarget(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.targets.DeleteTarget(r.Context(), id); err != nil {
		storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// readTarget decodifica e valida o corpo; o id do corpo é sempre descartado.
func readTarget(w http.ResponseWriter, r *http.Request) (target.Target, bool) {
	var t target.Target
	if status, msg, ok := decodeJSON(w, r, &t); !ok {
		writeError(w, status, msg)
		return target.Target{}, false
	}
	t.ID = 0
	t.Name = strings.TrimSpace(t.Name)
	if err := t.Validate(); err != nil {
		var ve *target.ValidationError
		if errors.As(err, &ve) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"erro": ve.Message, "campo": ve.Field})
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return target.Target{}, false
	}
	return t, true
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id inválido")
		return 0, false
	}
	return id, true
}

func storeError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "alvo não encontrado")
		return
	}
	slog.Error("erro no armazenamento", "erro", err)
	writeError(w, http.StatusInternalServerError, "erro interno")
}
