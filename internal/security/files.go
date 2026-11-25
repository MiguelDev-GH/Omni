package security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MiguelDev-GH/omni/internal/alerts"
)

const (
	// FilesEvery é o intervalo entre verificações dos arquivos.
	FilesEvery = 10 * time.Second
	// changedFor é por quanto tempo uma alteração fica no amarelo antes de
	// o arquivo voltar ao verde, se não mudar de novo.
	changedFor = 5 * time.Minute
	// maxHashBytes limita quanto de cada arquivo é lido. Configurações são
	// pequenas; um arquivo enorme na lista não pode travar o Omni.
	maxHashBytes = 10 << 20
)

// Valores da regra de arquivo alterado.
const (
	fileUnchanged = 0
	fileChanged   = 1
	fileMissing   = 2
)

// fileState é a "impressão digital" de um arquivo.
type fileState struct {
	Exists bool   `json:"existe"`
	SHA256 string `json:"sha256,omitempty"`
	Size   int64  `json:"tamanho"`
	Mode   string `json:"permissoes,omitempty"`
	Owner  string `json:"dono,omitempty"`
}

// Files monitora a integridade de arquivos de configuração: guarda o
// SHA-256, o tamanho e as permissões de cada um e alerta quando algo muda.
// A linha de base é salva em disco, então uma alteração feita com o Omni
// desligado também é detectada quando ele volta.
type Files struct {
	engine    *alerts.Engine
	paths     []string
	statePath string

	baseline  map[string]fileState
	changedAt map[string]time.Time
	detail    map[string]string
}

// NewFiles cria o monitor para paths. statePath é onde a linha de base é
// salva (vazio para não salvar).
func NewFiles(e *alerts.Engine, paths []string, statePath string) *Files {
	f := &Files{
		engine: e, paths: paths, statePath: statePath,
		baseline:  map[string]fileState{},
		changedAt: map[string]time.Time{},
		detail:    map[string]string{},
	}
	f.load()
	return f
}

// Paths devolve os arquivos monitorados.
func (f *Files) Paths() []string { return f.paths }

func (f *Files) load() {
	if f.statePath == "" {
		return
	}
	b, err := os.ReadFile(f.statePath)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err == nil {
		err = json.Unmarshal(b, &f.baseline)
	}
	if err != nil {
		slog.Error("segurança: ler linha de base dos arquivos", "erro", err)
		f.baseline = map[string]fileState{}
	}
}

func (f *Files) save() {
	if f.statePath == "" {
		return
	}
	b, err := json.MarshalIndent(f.baseline, "", "  ")
	if err == nil {
		// 0600: a lista de arquivos e hashes só interessa ao dono do Omni.
		err = os.WriteFile(f.statePath, b, 0o600)
	}
	if err != nil {
		slog.Error("segurança: salvar linha de base dos arquivos", "erro", err)
	}
}

// Run verifica os arquivos a cada FilesEvery até ctx ser cancelado.
func (f *Files) Run(ctx context.Context) {
	f.Scan(time.Now().UTC())
	tk := time.NewTicker(FilesEvery)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tk.C:
			f.Scan(now.UTC())
		}
	}
}

// Scan verifica todos os arquivos uma vez e entrega uma amostra por arquivo.
func (f *Files) Scan(now time.Time) {
	dirty := false
	for _, p := range f.paths {
		cur := inspect(p)
		old, known := f.baseline[p]
		switch {
		case !known:
			// Primeira vez: o estado atual vira a referência.
			f.baseline[p] = cur
			dirty = true
			if !cur.Exists {
				f.detail[p] = "arquivo não encontrado"
			}
		case cur != old:
			f.detail[p] = describe(old, cur)
			f.changedAt[p] = now
			f.baseline[p] = cur
			dirty = true
			slog.Warn("segurança: arquivo de configuração alterado", "arquivo", p, "mudanca", f.detail[p])
		}

		value := float64(fileUnchanged)
		switch {
		case !cur.Exists:
			value = fileMissing
		case !f.changedAt[p].IsZero() && now.Sub(f.changedAt[p]) < changedFor:
			value = fileChanged
		}
		f.engine.Observe(alerts.Sample{Rule: alerts.RuleConfigChange, Service: alerts.ServiceSecurity,
			Subject: p, Value: value, Detail: f.detail[p], At: now})
	}
	if dirty {
		f.save()
	}
}

// inspect calcula a impressão digital de path. Arquivo ilegível conta como
// ausente: para quem monitora, o efeito é o mesmo.
func inspect(path string) fileState {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return fileState{}
	}
	fh, err := os.Open(path)
	if err != nil {
		return fileState{}
	}
	defer func() { _ = fh.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(fh, maxHashBytes)); err != nil {
		return fileState{}
	}
	return fileState{
		Exists: true,
		SHA256: hex.EncodeToString(h.Sum(nil)),
		Size:   info.Size(),
		Mode:   info.Mode().Perm().String(),
		Owner:  owner(info),
	}
}

func short(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

// describe diz o que mudou, sem mostrar o conteúdo: um arquivo de
// configuração costuma ter senhas, e o alerta vai por e-mail.
func describe(old, cur fileState) string {
	switch {
	case old.Exists && !cur.Exists:
		return "arquivo removido ou ilegível"
	case !old.Exists && cur.Exists:
		return fmt.Sprintf("arquivo criado (%d bytes, sha256 %s)", cur.Size, short(cur.SHA256))
	}
	var parts []string
	if old.SHA256 != cur.SHA256 {
		parts = append(parts, fmt.Sprintf("conteúdo alterado (sha256 %s → %s, %d → %d bytes)",
			short(old.SHA256), short(cur.SHA256), old.Size, cur.Size))
	}
	if old.Mode != cur.Mode {
		parts = append(parts, fmt.Sprintf("permissões %s → %s", old.Mode, cur.Mode))
	}
	if old.Owner != cur.Owner {
		parts = append(parts, fmt.Sprintf("dono %s → %s", old.Owner, cur.Owner))
	}
	return strings.Join(parts, "; ")
}

// ParseWatchList lê OMNI_ARQUIVOS_MONITORADOS: caminhos separados por
// vírgula, convertidos em absolutos e sem repetição.
func ParseWatchList(raw string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		abs, err := filepath.Abs(item)
		if err != nil {
			return nil, fmt.Errorf("OMNI_ARQUIVOS_MONITORADOS: caminho inválido %q: %w", item, err)
		}
		if !seen[abs] {
			seen[abs] = true
			out = append(out, abs)
		}
	}
	return out, nil
}
