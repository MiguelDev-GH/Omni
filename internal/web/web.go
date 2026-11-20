// Package web serve a interface React embutida no binário.
package web

import (
	"bytes"
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed all:static
var static embed.FS

const csp = "default-src 'self'; connect-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"

// Assets devolve os arquivos compilados do frontend (ou só o placeholder).
func Assets() fs.FS {
	sub, err := fs.Sub(static, "static")
	if err != nil {
		panic(err)
	}
	return sub
}

// Handler serve os arquivos de fsys; caminhos sem arquivo e sem extensão
// recebem o index.html, para que as rotas do React funcionem ao recarregar.
func Handler(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			h.Set("Allow", "GET, HEAD")
			http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
			return
		}
		p := r.URL.Path
		if p == "/api" || strings.HasPrefix(p, "/api/") || p == "/ws" {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+p), "/")
		if name != "" && isFile(fsys, name) {
			if strings.HasPrefix(name, "assets/") {
				h.Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				h.Set("Cache-Control", "no-cache")
			}
			serveFile(w, r, fsys, name)
			return
		}
		if name != "" && strings.Contains(path.Base(name), ".") {
			http.NotFound(w, r)
			return
		}
		h.Set("Cache-Control", "no-cache")
		if isFile(fsys, "index.html") {
			serveFile(w, r, fsys, "index.html")
			return
		}
		serveFile(w, r, fsys, "placeholder.html")
	})
}

func isFile(fsys fs.FS, name string) bool {
	st, err := fs.Stat(fsys, name)
	return err == nil && st.Mode().IsRegular()
}

func serveFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
