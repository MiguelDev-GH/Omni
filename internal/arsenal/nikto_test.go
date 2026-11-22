package arsenal

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/target"
)

func TestNiktoArgs(t *testing.T) {
	args, err := NiktoArgs(ffufTarget, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"-h", "http://127.0.0.1:3000", "-Format", "json", "-o", "/tmp/nikto.json",
		"-nointeractive", "-ask", "no", "-useragent", "Omni-Arsenal/1", "-maxtime", "540s"}, args)

	args, err = NiktoArgs(ffufTarget, Options{"porta": "8443"})
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:8443", args[1])

	v6, err := NiktoArgs(target.Target{Name: "v6", Host: "::1", AppPorts: []int{80}}, nil)
	require.NoError(t, err)
	assert.Equal(t, "http://[::1]:80", v6[1])

	for _, tc := range []struct {
		opts  Options
		field string
	}{
		{Options{"porta": "5433"}, "opcoes.nikto.porta"},
		{Options{"porta": "-Plugins"}, "opcoes.nikto.porta"},
		{Options{"lista": "media"}, "opcoes.nikto.lista"},
	} {
		args, err := NiktoArgs(ffufTarget, tc.opts)
		assert.Nil(t, args)
		var oe *OptionError
		require.ErrorAs(t, err, &oe, tc.opts)
		assert.Equal(t, tc.field, oe.Field())
	}
	tool, ok := Lookup("nikto")
	require.True(t, ok)
	assert.Equal(t, niktoOutput, tool.OutputFile)
}

func TestNiktoReport(t *testing.T) {
	raw, err := NiktoReportFrom(fixture(t, "nikto.out"))
	require.NoError(t, err)
	var rep NiktoReport
	require.NoError(t, json.Unmarshal(raw, &rep))
	assert.Equal(t, "Loja/0.1", rep.Server, "o banner vem do console: o JSON do Nikto 2.6.1 grava null")
	require.Len(t, rep.Items, 10)
	byCat := map[string]int{}
	for _, it := range rep.Items {
		byCat[it.Category]++
	}
	assert.Equal(t, map[string]int{"cabecalhos": 8, "arquivos": 2}, byCat)
	env := rep.Items[7]
	assert.Equal(t, NiktoItem{ID: "007226", Method: "GET", URL: "/.env", Message: ".env file found. The .env file may contain credentials.",
		References: []string{}, Category: "arquivos"}, env)
	for _, it := range rep.Items {
		if it.Message == "Suggested security header missing: content-security-policy." {
			assert.Equal(t, []string{"https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP"}, it.References)
		}
	}
}

func TestParseNiktoJSON(t *testing.T) {
	got, err := ParseNiktoJSON(bytes.NewReader(fixture(t, "nikto.out")))
	require.NoError(t, err)
	require.Len(t, got, 10)
	bySev := map[findings.Severity]int{}
	for _, f := range got {
		assert.Equal(t, "nikto", f.Tool)
		assert.Equal(t, 3000, f.Port)
		bySev[f.Severity]++
	}
	assert.Equal(t, map[findings.Severity]int{findings.Low: 8, findings.Medium: 2}, bySev)
	assert.Equal(t, "/.env: .env file found. The .env file may contain credentials.", got[7].Title)
}

func TestParseNiktoCutOutputUsesConsole(t *testing.T) {
	// Sem o arquivo JSON (Nikto morto no meio), os itens vêm do console.
	out := fixture(t, "nikto.out")
	cut := out[:bytes.Index(out, []byte(OutputFileMarker))]
	got, err := ParseNiktoJSON(bytes.NewReader(cut))
	require.NoError(t, err)
	require.Len(t, got, 10)
	assert.Equal(t, "/.git/HEAD: Git HEAD file found. Full repo details may be present.", got[6].Title)
	assert.Equal(t, findings.Medium, got[6].Severity)
	assert.Equal(t, 0, got[6].Port)

	_, err = NiktoReportFrom(cut)
	assert.Error(t, err)

	// JSON cortado no meio também fica sem relatório, mas com os achados.
	half := out[:len(out)-200]
	got, err = ParseNiktoJSON(bytes.NewReader(half))
	require.NoError(t, err)
	assert.Len(t, got, 10)
	_, err = NiktoReportFrom(half)
	assert.Error(t, err)
}

func TestNiktoCategory(t *testing.T) {
	for msg, want := range map[string]string{
		"Suggested security header missing: referrer-policy.":                            "cabecalhos",
		"Cookie session created without the httponly flag.":                              "cabecalhos",
		"Apache/2.2.15 appears to be outdated (current is at least Apache/2.4.54).":      "versao",
		"Server may leak inodes via ETags, header found with file /, inode: 1.":          "cabecalhos",
		"/backup/: Directory indexing found.":                                            "arquivos",
		".env file found. The .env file may contain credentials.":                        "arquivos",
		"PHP/5.6 appears to be outdated":                                                 "versao",
		"Allowed HTTP Methods: GET, HEAD, POST, OPTIONS.":                                "outros",
		"The anti-clickjacking X-Frame-Options header is not present.":                   "cabecalhos",
		"/admin/: This might be interesting.":                                            "outros",
		"OpenSSL/1.0.2 appears to be outdated (current is at least 3.0.7). End of life.": "versao",
	} {
		assert.Equal(t, want, niktoCategory(msg), msg)
	}
}
