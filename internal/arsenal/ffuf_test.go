package arsenal

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/target"
)

var ffufTarget = target.Target{Name: "loja", Host: "127.0.0.1", AppPorts: []int{3000, 8443}, DBPort: 5433}

func TestFfufArgs(t *testing.T) {
	args, err := FfufArgs(ffufTarget, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"-u", "http://127.0.0.1:3000/FUZZ", "-w", "/omni/lista.txt", "-of", "json", "-o", "/dev/stdout",
		"-s", "-t", "10", "-rate", "100", "-maxtime", "540", "-mc", "all", "-fc", "404", "-ac",
		"-H", "User-Agent: Omni-Arsenal/1"}, args)

	args, err = FfufArgs(ffufTarget, Options{"lista": "media", "porta": "8443"})
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:8443/FUZZ", args[1])

	v6, err := FfufArgs(target.Target{Name: "v6", Host: "::1", AppPorts: []int{80}}, nil)
	require.NoError(t, err)
	assert.Equal(t, "http://[::1]:80/FUZZ", v6[1])

	_, err = FfufArgs(target.Target{Name: "x", Host: "-u", AppPorts: []int{80}}, nil)
	assert.Error(t, err)
}

func TestFfufRejectsBadOptions(t *testing.T) {
	for _, tc := range []struct {
		opts  Options
		field string
	}{
		{Options{"lista": "../../etc/passwd"}, "opcoes.ffuf.lista"},
		{Options{"lista": "grande"}, "opcoes.ffuf.lista"},
		{Options{"porta": "5433"}, "opcoes.ffuf.porta"}, // porta do banco, não de aplicação
		{Options{"porta": "3000 -x"}, "opcoes.ffuf.porta"},
		{Options{"metodo": "POST"}, "opcoes.ffuf.metodo"},
	} {
		args, err := FfufArgs(ffufTarget, tc.opts)
		assert.Nil(t, args, tc.opts)
		var oe *OptionError
		require.ErrorAs(t, err, &oe, tc.opts)
		assert.Equal(t, tc.field, oe.Field(), tc.opts)

		files, err := FfufFiles(tc.opts)
		if tc.field == "opcoes.ffuf.lista" {
			assert.ErrorIs(t, err, ErrOpcaoInvalida)
			assert.Nil(t, files)
		}
	}
}

func TestFfufFiles(t *testing.T) {
	small, err := FfufFiles(nil)
	require.NoError(t, err)
	require.Contains(t, small, "/omni/lista.txt")
	assert.Equal(t, 4751, bytes.Count(small["/omni/lista.txt"], []byte("\n")))

	medium, err := FfufFiles(Options{"lista": "media", "porta": "8443"})
	require.NoError(t, err)
	assert.Greater(t, len(medium["/omni/lista.txt"]), len(small["/omni/lista.txt"]))
}

func TestParseFfufJSON(t *testing.T) {
	got, err := ParseFfufJSON(bytes.NewReader(fixture(t, "ffuf.out")))
	require.NoError(t, err)
	require.Len(t, got, 4)
	bySev := map[string]findings.Severity{}
	for _, f := range got {
		assert.Equal(t, "ffuf", f.Tool)
		assert.Equal(t, 3000, f.Port)
		bySev[f.Title] = f.Severity
	}
	assert.Equal(t, findings.Medium, bySev["Caminho encontrado: /.env"])
	assert.Equal(t, findings.Medium, bySev["Caminho encontrado: /.git/config"])
	assert.Equal(t, findings.Info, bySev["Caminho encontrado: /render/https://www.google.com"])
	assert.Contains(t, got[0].Evidence, "HTTP 200")
}

func TestParseFfufCutOutputKeepsSilentLines(t *testing.T) {
	// Cortada antes do JSON final, a saída só tem as palavras do modo -s.
	out := fixture(t, "ffuf.out")
	cut := out[:bytes.Index(out, []byte(`{"commandline"`))+40]
	got, err := ParseFfufJSON(bytes.NewReader(cut))
	require.NoError(t, err)
	require.Len(t, got, 4)
	assert.Equal(t, "Caminho encontrado: /.env", got[0].Title)
	assert.Equal(t, findings.Medium, got[0].Severity)

	_, err = FfufReportFrom(cut)
	assert.Error(t, err)
}

func TestFfufReport(t *testing.T) {
	raw, err := FfufReportFrom(fixture(t, "ffuf.out"))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "User-Agent", "a configuração da ferramenta não entra no relatório")
	var rep FfufReport
	require.NoError(t, json.Unmarshal(raw, &rep))
	assert.Equal(t, "http://127.0.0.1:3000", rep.BaseURL)
	require.Len(t, rep.Results, 4)
	assert.Equal(t, FfufResult{Path: "/.env", Status: 200, Size: 165, Words: 1, DurationMS: 0}, rep.Results[0])
	last := rep.Results[3]
	assert.Equal(t, "/render/https://www.google.com", last.Path)
	assert.Equal(t, 307, last.Status)
	assert.Equal(t, "/render/https:/www.google.com", last.Redirect)
}

func TestSensitivePath(t *testing.T) {
	for _, p := range []string{"/.git/HEAD", "/.env", "/backup", "/db.sql", "/index.php.bak", "/admin/login", "/phpinfo.php", "/server-status"} {
		assert.True(t, sensitivePath(p), p)
	}
	for _, p := range []string{"/produtos", "/environment", "/static/app.js", "/"} {
		assert.False(t, sensitivePath(p), p)
	}
	assert.True(t, strings.HasPrefix(FfufImage, "secsi/ffuf:"))
}
