package arsenal

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/target"
)

func gitleaksTarget(t *testing.T) target.Target {
	t.Helper()
	return target.Target{Name: "loja", Host: "127.0.0.1", AppPorts: []int{3000}, Repo: t.TempDir()}
}

func TestGitleaksArgs(t *testing.T) {
	tg := gitleaksTarget(t)
	args, err := GitleaksArgs(tg, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"dir", "/repo", "--report-format", "json", "--report-path", "-",
		"--redact", "--exit-code", "0", "--no-banner", "--log-level", "warn"}, args)
	assert.NotContains(t, strings.Join(args, " "), tg.Repo, "o caminho do host só entra pela montagem")

	require.NoError(t, os.Mkdir(filepath.Join(tg.Repo, ".git"), 0o700))
	args, err = GitleaksArgs(tg, nil)
	require.NoError(t, err)
	assert.Equal(t, "git", args[0], "com histórico, varre os commits")

	tool, ok := Lookup("gitleaks")
	require.True(t, ok)
	assert.Equal(t, "repositorio", tool.Requires)
	assert.True(t, tool.MountsRepo)
}

func TestGitleaksArgsRejects(t *testing.T) {
	tg := gitleaksTarget(t)
	noRepo := tg
	noRepo.Repo = ""
	_, err := GitleaksArgs(noRepo, nil)
	var ve *target.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "repositorio", ve.Field)

	for _, repo := range []string{"/", "/etc", "relativo/x", "/tmp/../etc"} {
		bad := tg
		bad.Repo = repo
		_, err := GitleaksArgs(bad, nil)
		require.ErrorAs(t, err, &ve, repo)
		assert.Equal(t, "repositorio", ve.Field, repo)
	}

	_, err = GitleaksArgs(tg, Options{"regras": "x"})
	var oe *OptionError
	require.ErrorAs(t, err, &oe)
	assert.Equal(t, "opcoes.gitleaks.regras", oe.Field())
}

func TestGitleaksReport(t *testing.T) {
	raw, err := GitleaksReportFrom(fixture(t, "gitleaks.json"))
	require.NoError(t, err)
	var rep GitleaksReport
	require.NoError(t, json.Unmarshal(raw, &rep))
	require.Len(t, rep.Leaks, 2)
	assert.Equal(t, Leak{
		Rule: "github-pat", Description: rep.Leaks[0].Description, File: "app.py", Line: 1,
		Commit: "2a84462ef4bcfd4e166ef2689dc0b9fb238bd26b", Author: "Dev Cobaia", Date: "2026-11-23T14:14:38Z", Entropy: 5.121928,
	}, rep.Leaks[0])
	assert.Equal(t, "config/aws.env", rep.Leaks[1].File)

	// Sem histórico do git, o caminho vem com /repo e sem commit.
	raw, err = GitleaksReportFrom([]byte(`[{"RuleID":"github-pat","StartLine":3,"File":"/repo/src/app.py","Commit":""}]`))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &rep))
	assert.Equal(t, "src/app.py", rep.Leaks[0].File)

	raw, err = GitleaksReportFrom([]byte("[]\n"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"vazamentos":[]}`, string(raw))
}

// TestGitleaksDropsSecrets confere que o valor do segredo e o trecho casado,
// que o fixture traz de propósito, não sobram em nenhum campo do relatório
// nem dos achados.
func TestGitleaksDropsSecrets(t *testing.T) {
	out := fixture(t, "gitleaks.json")
	forbidden := []string{"omni-fixture-segredo", "GITHUB_TOKEN", "AWS_ACCESS_KEY_ID", "dev@loja.test", "configuração inicial"}

	raw, err := GitleaksReportFrom(out)
	require.NoError(t, err)
	got, err := ParseGitleaksJSON(bytes.NewReader(out))
	require.NoError(t, err)
	list, err := json.Marshal(got)
	require.NoError(t, err)
	for _, s := range forbidden {
		assert.NotContains(t, string(raw), s)
		assert.NotContains(t, string(list), s)
	}
	assert.NotContains(t, string(raw), `"segredo"`)
	assert.NotContains(t, string(raw), `"trecho"`)
}

func TestParseGitleaksJSON(t *testing.T) {
	got, err := ParseGitleaksJSON(bytes.NewReader(fixture(t, "gitleaks.json")))
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "github-pat em app.py:1", got[0].Title)
	assert.Equal(t, "commit 2a84462ef4bcfd4e166ef2689dc0b9fb238bd26b; data 2026-11-23T14:14:38Z", got[0].Evidence)
	assert.Equal(t, findings.High, got[0].Severity)
	assert.Equal(t, "gitleaks", got[0].Tool)
	assert.Equal(t, "aws-access-token em config/aws.env:1", got[1].Title)

	got, err = ParseGitleaksJSON(strings.NewReader(`[{"RuleID":"x","StartLine":2,"File":"/repo/a.env"}]`))
	require.NoError(t, err)
	assert.Equal(t, "fora do histórico do git", got[0].Evidence)
}

func TestParseGitleaksTruncated(t *testing.T) {
	out := fixture(t, "gitleaks.json")
	_, err := ParseGitleaksJSON(bytes.NewReader(out[:len(out)/2]))
	assert.Error(t, err)
	_, err = GitleaksReportFrom(out[:len(out)/2])
	assert.Error(t, err)
	_, err = ParseGitleaksJSON(strings.NewReader(""))
	assert.Error(t, err)
}
