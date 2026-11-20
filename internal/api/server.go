// Package api expõe a API REST do Omni.
package api

import (
	"context"
	"net/http"

	"github.com/MiguelDev-GH/omni/internal/target"
)

type TargetStore interface {
	CreateTarget(ctx context.Context, t target.Target) (target.Target, error)
	ListTargets(ctx context.Context) ([]target.Target, error)
	GetTarget(ctx context.Context, id int64) (target.Target, error)
	UpdateTarget(ctx context.Context, t target.Target) error
	DeleteTarget(ctx context.Context, id int64) error
}

// Server roteia /api/saude sem autenticação e todo o resto atrás do token.
type Server struct {
	outer   *http.ServeMux
	private *http.ServeMux
	targets TargetStore
}

func NewServer(ts TargetStore, token string) *Server {
	s := &Server{outer: http.NewServeMux(), private: http.NewServeMux(), targets: ts}

	s.outer.HandleFunc("GET /api/saude", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.outer.Handle("/", requireToken(token, s.private))

	s.private.HandleFunc("GET /api/alvos", s.listTargets)
	s.private.HandleFunc("POST /api/alvos", s.createTarget)
	s.private.HandleFunc("GET /api/alvos/{id}", s.getTarget)
	s.private.HandleFunc("PUT /api/alvos/{id}", s.updateTarget)
	s.private.HandleFunc("DELETE /api/alvos/{id}", s.deleteTarget)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.outer.ServeHTTP(w, r) }

// Handle registra uma rota extra, já atrás da autenticação.
func (s *Server) Handle(pattern string, h http.Handler) { s.private.Handle(pattern, h) }
