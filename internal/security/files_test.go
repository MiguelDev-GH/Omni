package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MiguelDev-GH/omni/internal/alerts"
)

func newTestFiles(t *testing.T, paths []string, state string) (*Files, *alerts.Engine, *[]alerts.Alert) {
	t.Helper()
	e := alerts.NewEngine(alerts.DefaultRules())
	var got []alerts.Alert
	e.OnAlert(func(a alerts.Alert) { got = append(got, a) })
	return NewFiles(e, paths, state), e, &got
}

func write(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func TestFileChangeIsYellowThenRecovers(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "Caddyfile")
	write(t, cfg, ":8443 {\n\treverse_proxy 127.0.0.1:3000\n}\n", 0o644)
	f, e, got := newTestFiles(t, []string{cfg}, "")

	f.Scan(t0)
	if len(*got) != 0 {
		t.Fatalf("primeira leitura gerou alerta: %+v", *got)
	}
	if lvl, _, _ := level(e, alerts.RuleConfigChange, cfg); lvl != alerts.Green {
		t.Fatalf("antes da mudança: %q", lvl)
	}

	write(t, cfg, ":8443 {\n\treverse_proxy 10.6.6.6:3000\n}\n", 0o644)
	f.Scan(t0.Add(10 * time.Second))
	if len(*got) != 1 || (*got)[0].Level != alerts.Yellow {
		t.Fatalf("depois da mudança: %+v", *got)
	}
	if d := (*got)[0].Detail; !strings.Contains(d, "conteúdo alterado (sha256 ") || strings.Contains(d, "10.6.6.6") {
		t.Errorf("detalhe = %q (não pode mostrar o conteúdo)", d)
	}

	f.Scan(t0.Add(10*time.Second + changedFor - time.Second))
	if len(*got) != 1 {
		t.Fatalf("voltou ao verde cedo demais: %+v", *got)
	}
	f.Scan(t0.Add(10*time.Second + changedFor))
	if len(*got) != 2 || (*got)[1].Level != alerts.Green {
		t.Fatalf("depois de %v: %+v", changedFor, *got)
	}
}

func TestFilePermissionChange(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "app.env")
	write(t, cfg, "SENHA=x\n", 0o600)
	f, _, got := newTestFiles(t, []string{cfg}, "")
	f.Scan(t0)
	if err := os.Chmod(cfg, 0o666); err != nil {
		t.Fatal(err)
	}
	f.Scan(t0.Add(10 * time.Second))
	if len(*got) != 1 || !strings.Contains((*got)[0].Detail, "permissões -rw------- → -rw-rw-rw-") {
		t.Fatalf("alertas = %+v", *got)
	}
}

func TestFileRemovedIsRedUntilBack(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "compose.yaml")
	write(t, cfg, "services: {}\n", 0o644)
	f, e, got := newTestFiles(t, []string{cfg}, "")
	f.Scan(t0)
	if err := os.Remove(cfg); err != nil {
		t.Fatal(err)
	}
	f.Scan(t0.Add(10 * time.Second))
	f.Scan(t0.Add(time.Hour))
	if lvl, _, _ := level(e, alerts.RuleConfigChange, cfg); lvl != alerts.Red {
		t.Fatalf("arquivo removido: %q", lvl)
	}
	if (*got)[0].Detail != "arquivo removido ou ilegível" {
		t.Errorf("detalhe = %q", (*got)[0].Detail)
	}
	write(t, cfg, "services: {}\n", 0o644)
	f.Scan(t0.Add(time.Hour + 10*time.Second))
	if lvl, _, _ := level(e, alerts.RuleConfigChange, cfg); lvl != alerts.Yellow {
		t.Fatalf("arquivo de volta: %q", lvl)
	}
	if !strings.HasPrefix((*got)[len(*got)-1].Detail, "arquivo criado") {
		t.Errorf("detalhe = %q", (*got)[len(*got)-1].Detail)
	}
}

func TestChangeWhileOmniWasOffIsDetected(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "Caddyfile")
	state := filepath.Join(dir, "integridade.json")
	write(t, cfg, "versao 1\n", 0o644)

	first, _, _ := newTestFiles(t, []string{cfg}, state)
	first.Scan(t0)
	info, err := os.Stat(state)
	if err != nil {
		t.Fatalf("linha de base não foi salva: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("permissões da linha de base = %v, quer 0600", info.Mode().Perm())
	}

	write(t, cfg, "versao 2\n", 0o644) // "com o Omni desligado"
	second, _, got := newTestFiles(t, []string{cfg}, state)
	second.Scan(t0.Add(time.Hour))
	if len(*got) != 1 || (*got)[0].Level != alerts.Yellow {
		t.Fatalf("alteração offline não detectada: %+v", *got)
	}
}

func TestMissingFromStartIsRed(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "nao-existe.conf")
	f, e, _ := newTestFiles(t, []string{cfg}, "")
	f.Scan(t0)
	if lvl, _, _ := level(e, alerts.RuleConfigChange, cfg); lvl != alerts.Red {
		t.Fatalf("arquivo ausente: %q", lvl)
	}
}

func TestCorruptStateIsIgnored(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(dir, "integridade.json")
	write(t, state, "{ isso não é json", 0o600)
	f, _, _ := newTestFiles(t, nil, state)
	if len(f.baseline) != 0 {
		t.Fatal("linha de base corrompida foi aceita")
	}
}

func TestParseWatchList(t *testing.T) {
	got, err := ParseWatchList(" /etc/caddy/Caddyfile, ,/etc/caddy/Caddyfile,rel/x.conf")
	if err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	want := []string{"/etc/caddy/Caddyfile", filepath.Join(wd, "rel/x.conf")}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("ParseWatchList = %v, quer %v", got, want)
	}
	if got, _ := ParseWatchList(""); len(got) != 0 {
		t.Errorf("lista vazia = %v", got)
	}
}
