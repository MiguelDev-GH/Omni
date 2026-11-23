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

func TestTestsslArgs(t *testing.T) {
	base := []string{"--quiet", "--color", "0", "--warnings", "off", "--connect-timeout", "10",
		"--openssl-timeout", "10", "--jsonfile", "/dev/stdout"}
	cases := []struct {
		t    target.Target
		want string
	}{
		{target.Target{Name: "a", Host: "127.0.0.1", AppPorts: []int{3000}}, "127.0.0.1:3000"},
		{target.Target{Name: "a", Host: "127.0.0.1", AppPorts: []int{3000, 443}}, "127.0.0.1:443"},
		{target.Target{Name: "a", Host: "127.0.0.1", AppPorts: []int{3000}, Domain: "app.com"}, "app.com:443"},
	}
	for _, c := range cases {
		args, err := TestsslArgs(c.t, nil)
		require.NoError(t, err)
		assert.Equal(t, append(append([]string{}, base...), c.want), args)
	}
	_, err := TestsslArgs(target.Target{Name: "x", Host: "127.0.0.1; id", AppPorts: []int{443}}, nil)
	assert.Error(t, err)
}

func TestParseTestsslJSON(t *testing.T) {
	f, err := os.Open("testdata/testssl.out")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	got, err := ParseTestsslJSON(f)
	require.NoError(t, err)

	bySev := map[findings.Severity]int{}
	for _, f := range got {
		bySev[f.Severity]++
		assert.Equal(t, "testssl", f.Tool)
		assert.Equal(t, 38443, f.Port)
	}
	// 6 LOW, 1 MEDIUM, 3 HIGH, 1 CRITICAL e a nota geral (CRITICAL).
	assert.Equal(t, map[findings.Severity]int{findings.Low: 6, findings.Medium: 1, findings.High: 3, findings.Critical: 2}, bySev)
	assert.Contains(t, titles(got), "Nota TLS: T")
	assert.Contains(t, titles(got), "cert_chain_of_trust: failed (self signed).")
	assert.Contains(t, titles(got), "HSTS: not offered")
	for _, f := range got {
		if strings.HasPrefix(f.Title, "LUCKY13") {
			assert.Equal(t, "CVE-2013-0169 CWE-310", f.Evidence)
		}
		assert.NotContains(t, f.Title, "engine_problem", "avisos da execução ficam de fora")
	}
}

func TestToolsRegistry(t *testing.T) {
	names := []string{}
	for _, tool := range Tools {
		names = append(names, tool.Name)
		assert.Contains(t, tool.Image, ":", "imagem com versão fixa")
		assert.NotContains(t, tool.Image, ":latest")
	}
	assert.Equal(t, []string{"nmap", "nuclei", "nikto", "sqlmap", "ffuf", "amass", "gitleaks", "testssl"}, names)
	nuclei, _ := Lookup("nuclei")
	assert.Equal(t, int64(1536<<20), nuclei.Memory)
}
