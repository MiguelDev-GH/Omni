package arsenal

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/target"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return b
}

func TestToolsRejectUnknownOptions(t *testing.T) {
	tg := target.Target{Name: "api", Host: "127.0.0.1", AppPorts: []int{443}}
	for _, tool := range Tools {
		_, err := tool.Args(tg, Options{})
		require.NoError(t, err, tool.Name)
		args, err := tool.Args(tg, Options{"x": "1"})
		assert.ErrorIs(t, err, ErrOpcaoInvalida, tool.Name)
		assert.Nil(t, args, tool.Name)
		var oe *OptionError
		require.ErrorAs(t, err, &oe)
		assert.Equal(t, "opcoes."+tool.Name+".x", oe.Field())
		assert.NotNil(t, tool.Report, tool.Name)
	}
}

func TestNmapReport(t *testing.T) {
	raw, err := NmapReportFrom(fixture(t, "nmap.xml"))
	require.NoError(t, err)
	var rep NmapReport
	require.NoError(t, json.Unmarshal(raw, &rep))
	require.Len(t, rep.Hosts, 1)
	h := rep.Hosts[0]
	assert.Equal(t, "127.0.0.1", h.Address)
	assert.Equal(t, []NmapPort{
		{Port: 38080, Protocol: "tcp", State: "open", Service: "http", Product: "SimpleHTTPServer", Version: "0.6", Extra: "Python 3.14.10"},
		{Port: 39999, Protocol: "tcp", State: "closed"},
		{Port: 55432, Protocol: "tcp", State: "open", Service: "postgresql", Product: "PostgreSQL DB", Version: "9.6.0 or later"},
	}, h.Ports)
	assert.Contains(t, string(raw), `"estado":"closed"`)

	full := fixture(t, "nmap.xml")
	_, err = NmapReportFrom(full[:len(full)/2])
	assert.Error(t, err)
}

func TestNucleiReport(t *testing.T) {
	raw, err := NucleiReportFrom(fixture(t, "nuclei.jsonl"))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"request"`)
	assert.NotContains(t, string(raw), `"response"`)
	var rep NucleiReport
	require.NoError(t, json.Unmarshal(raw, &rep))
	require.NotEmpty(t, rep.Results)
	var git NucleiResult
	for _, r := range rep.Results {
		if r.Template == "git-config" {
			git = r
		}
	}
	assert.Equal(t, "Git Configuration - Detect", git.Name)
	assert.Equal(t, findings.Medium, git.Severity)
	assert.Equal(t, []string{"config", "git", "exposure", "vuln"}, git.Tags)
	assert.Equal(t, "http://127.0.0.1:38082/.git/config", git.URL)
	assert.NotEmpty(t, git.Description)
	assert.Equal(t, []string{}, git.References)

	// O fixture real não traz referências; os templates as declaram como
	// texto ou lista.
	line := `{"template-id":"x","info":{"name":"X","severity":"high","reference":"https://a.example"},"request":"GET / HTTP/1.1"}` + "\n" +
		`{"template-id":"y","info":{"name":"Y","severity":"low","reference":["https://b.example","https://c.example"]},"response":"segredo"}`
	raw, err = NucleiReportFrom([]byte(line))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "segredo")
	assert.NotContains(t, string(raw), "GET / HTTP")
	require.NoError(t, json.Unmarshal(raw, &rep))
	assert.Equal(t, []string{"https://a.example"}, rep.Results[0].References)
	assert.Equal(t, []string{"https://b.example", "https://c.example"}, rep.Results[1].References)

	// JSONL cortado no meio: as linhas completas continuam no relatório.
	full := fixture(t, "nuclei.jsonl")
	raw, err = NucleiReportFrom(full[:len(full)/2])
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &rep))
	assert.NotEmpty(t, rep.Results)
}

func TestTestsslReport(t *testing.T) {
	now := time.Date(2026, 11, 21, 16, 15, 0, 0, time.UTC)
	rep, err := testsslReport(fixture(t, "testssl.out"), now)
	require.NoError(t, err)
	assert.Equal(t, "T", rep.Grade)
	assert.Equal(t, []TLSProtocol{
		{"SSLv2", false}, {"SSLv3", false}, {"TLS 1.0", false},
		{"TLS 1.1", false}, {"TLS 1.2", true}, {"TLS 1.3", true},
	}, rep.Protocols)
	assert.Equal(t, "localhost", rep.Certificate.Subject)
	assert.Equal(t, "localhost", rep.Certificate.Issuer)
	assert.False(t, rep.Certificate.Trusted)
	require.NotNil(t, rep.Certificate.NotAfter)
	assert.Equal(t, time.Date(2026, 11, 23, 16, 15, 0, 0, time.UTC), *rep.Certificate.NotAfter)
	assert.Equal(t, 2, rep.Certificate.DaysLeft)
	require.NotEmpty(t, rep.Vulns)
	assert.Equal(t, TLSVuln{ID: "heartbleed", Name: "Heartbleed", CVE: "CVE-2014-0160"}, rep.Vulns[0])

	full := fixture(t, "testssl.out")
	_, err = testsslReport(full[:len(full)/2], now)
	assert.Error(t, err)
	_, err = TestsslReportFrom(nil)
	assert.Error(t, err)
}
