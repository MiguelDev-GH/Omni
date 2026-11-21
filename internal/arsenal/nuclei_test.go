package arsenal

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/target"
)

func TestNucleiArgs(t *testing.T) {
	args, err := NucleiArgs(target.Target{Name: "api", Host: "127.0.0.1", AppPorts: []int{3000, 8080}, DBPort: 5432})
	require.NoError(t, err)
	assert.Equal(t, []string{"-jsonl", "-silent", "-nc", "-t", "http/", "-tags", "exposure,misconfig,tech,cve",
		"-rl", "100", "-c", "10", "-bs", "10", "-timeout", "5", "-retries", "1", "-H", "User-Agent: Omni-Arsenal/1",
		"-u", "http://127.0.0.1:3000", "-u", "http://127.0.0.1:8080"}, args)

	v6, err := NucleiArgs(target.Target{Name: "v6", Host: "::1", AppPorts: []int{80}})
	require.NoError(t, err)
	assert.Equal(t, "http://[::1]:80", v6[len(v6)-1])

	_, err = NucleiArgs(target.Target{Name: "x", Host: "-u", AppPorts: []int{80}})
	assert.Error(t, err)
}

func TestParseNucleiJSONL(t *testing.T) {
	f, err := os.Open("testdata/nuclei.jsonl")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	got, err := ParseNucleiJSONL(f)
	require.NoError(t, err)
	require.Len(t, got, 13, "a tabela de templates e a linha cortada são ignoradas")

	bySev := map[findings.Severity]int{}
	for _, f := range got {
		bySev[f.Severity]++
		assert.Equal(t, "nuclei", f.Tool)
		assert.Equal(t, 38082, f.Port)
		assert.NotContains(t, f.Evidence, "User-Agent", "requisição e resposta não são guardadas")
	}
	assert.Equal(t, map[findings.Severity]int{findings.Info: 12, findings.Medium: 1}, bySev)

	var git findings.Finding
	for _, f := range got {
		if f.Severity == findings.Medium {
			git = f
		}
	}
	assert.Equal(t, "Git Configuration - Detect", git.Title)
	assert.Equal(t, "http://127.0.0.1:38082/.git/config", git.Evidence)
	assert.Contains(t, titles(got), "HTTP Missing Security Headers: x-frame-options")
}

func TestParseNucleiSeverities(t *testing.T) {
	in := `{"template-id":"a","info":{"name":"A","severity":"critical"},"port":"80"}
lixo
{"template-id":"b","info":{"name":"B","severity":"high"},"port":"80"}
{"template-id":"c","info":{"name":"C","severity":"low"},"port":"80"}
{"template-id":"d","info":{"name":"D","severity":"unknown"},"port":"80"}`
	got, err := ParseNucleiJSONL(strings.NewReader(in))
	require.NoError(t, err)
	require.Len(t, got, 4)
	assert.Equal(t, []findings.Severity{findings.Critical, findings.High, findings.Low, findings.Info},
		[]findings.Severity{got[0].Severity, got[1].Severity, got[2].Severity, got[3].Severity})
}

func titles(list []findings.Finding) []string {
	out := make([]string, len(list))
	for i, f := range list {
		out[i] = f.Title
	}
	return out
}
