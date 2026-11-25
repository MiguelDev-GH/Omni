package api

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/MiguelDev-GH/omni/internal/dbsize"
)

const dbSizeWindow = 24 * time.Hour

type DBSizeStore interface {
	DBSizesSince(ctx context.Context, since time.Time) ([]dbsize.Sample, error)
}

type dbSizePoint struct {
	At    time.Time `json:"hora"`
	Bytes int64     `json:"bytes"`
}

type dbSizeInfo struct {
	Name  string `json:"banco"`
	Bytes int64  `json:"bytes"` // medição mais recente
	// Growth fica null enquanto as medições cobrem menos de 1 h.
	Growth *float64      `json:"crescimento_gb_dia"`
	Series []dbSizePoint `json:"serie"`
}

type dbSizeResponse struct {
	Configured bool         `json:"configurado"`
	Databases  []dbSizeInfo `json:"bancos"`
}

// DBSizeHandler responde GET /api/banco/tamanho com o tamanho atual de cada
// banco, a série das últimas 24 h e o crescimento estimado em GB/dia, do
// maior banco para o menor. configured diz se OMNI_PG_DSN está definido.
func DBSizeHandler(ds DBSizeStore, configured bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := dbSizeResponse{Configured: configured, Databases: []dbSizeInfo{}}
		samples, err := ds.DBSizesSince(r.Context(), time.Now().Add(-dbSizeWindow))
		if err != nil {
			storeError(w, err)
			return
		}
		byName := map[string][]dbsize.Sample{}
		var names []string
		for _, s := range samples {
			if _, ok := byName[s.Name]; !ok {
				names = append(names, s.Name)
			}
			byName[s.Name] = append(byName[s.Name], s)
		}
		for _, name := range names {
			list := byName[name]
			info := dbSizeInfo{Name: name, Bytes: list[len(list)-1].Bytes, Series: []dbSizePoint{}}
			for _, s := range list {
				info.Series = append(info.Series, dbSizePoint{At: s.At, Bytes: s.Bytes})
			}
			if g, ok := dbsize.Growth(list); ok {
				info.Growth = &g
			}
			resp.Databases = append(resp.Databases, info)
		}
		sort.SliceStable(resp.Databases, func(i, j int) bool { return resp.Databases[i].Bytes > resp.Databases[j].Bytes })
		writeJSON(w, http.StatusOK, resp)
	})
}
