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

var amassTarget = target.Target{Name: "nmap", Host: "45.33.32.156", AppPorts: []int{80}, Domain: "nmap.org"}

func TestAmassArgs(t *testing.T) {
	args, err := AmassArgs(amassTarget, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"enum", "-passive", "-d", "nmap.org", "-silent", "-nocolor", "-json", "/dev/stdout", "-timeout", "8"}, args)
	assert.NotContains(t, strings.Join(args, " "), amassTarget.Host, "o modo passivo não manda nada ao host do alvo")

	tool, ok := Lookup("amass")
	require.True(t, ok)
	assert.Equal(t, "dominio", tool.Requires)
}

func TestAmassArgsRejects(t *testing.T) {
	noDomain := amassTarget
	noDomain.Domain = ""
	_, err := AmassArgs(noDomain, nil)
	var ve *target.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "dominio", ve.Field)

	_, err = AmassArgs(amassTarget, Options{"ativo": "1"})
	var oe *OptionError
	require.ErrorAs(t, err, &oe)
	assert.Equal(t, "opcoes.amass.ativo", oe.Field())
}

func TestAmassReport(t *testing.T) {
	raw, err := AmassReportFrom(amassTarget, fixture(t, "amass.json"))
	require.NoError(t, err)
	var rep AmassReport
	require.NoError(t, json.Unmarshal(raw, &rep))
	assert.Equal(t, "nmap.org", rep.Domain)
	names := []string{}
	for _, s := range rep.Subdomains {
		names = append(names, s.Name)
	}
	assert.Equal(t, []string{"echo.nmap.org", "es.nmap.org", "issues.nmap.org", "scanme.nmap.org", "support.nmap.org", "www.nmap.org"}, names)

	assert.Equal(t, Subdomain{Name: "echo.nmap.org", Addresses: []string{}, Sources: []string{"Maltiverse"}}, rep.Subdomains[0])
	assert.Equal(t, []string{"Maltiverse", "HackerTarget", "CertSpotter"}, rep.Subdomains[2].Sources)
	scanme := rep.Subdomains[3]
	assert.Equal(t, []string{"45.33.32.156", "2600:3c01::f03c:91ff:fe18:bb2f"}, scanme.Addresses)
	assert.True(t, scanme.ThisHost, "o endereço é o host do alvo")
	assert.False(t, rep.Subdomains[5].ThisHost)
}

func TestAmassReportMergesAndLoopback(t *testing.T) {
	out := `{"name":"api.loja.test","domain":"loja.test","addresses":null,"sources":["crtsh"]}
{"name":"API.loja.test.","domain":"loja.test","addresses":[{"ip":"127.0.0.1"}],"sources":["crtsh","DNSDumpster"]}
`
	raw, err := AmassReportFrom(target.Target{Host: "10.0.0.1", Domain: "loja.test"}, []byte(out))
	require.NoError(t, err)
	var rep AmassReport
	require.NoError(t, json.Unmarshal(raw, &rep))
	assert.Equal(t, []Subdomain{{Name: "api.loja.test", Addresses: []string{"127.0.0.1"}, Sources: []string{"crtsh", "DNSDumpster"}, ThisHost: true}},
		rep.Subdomains)

	raw, err = AmassReportFrom(target.Target{Domain: "vazio.test"}, nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"dominio":"vazio.test","subdominios":[]}`, string(raw))
}

func TestParseAmassJSON(t *testing.T) {
	got, err := ParseAmassJSON(bytes.NewReader(fixture(t, "amass.json")))
	require.NoError(t, err)
	require.Len(t, got, 6)
	for _, f := range got {
		assert.Equal(t, "amass", f.Tool)
		assert.Equal(t, findings.Info, f.Severity, "sem endereço não vira alerta: o modo passivo não resolve nomes")
		assert.Equal(t, 0, f.Port)
	}
	assert.Equal(t, "Subdomínio encontrado: echo.nmap.org", got[0].Title)
	assert.Equal(t, "fontes: Maltiverse", got[0].Evidence)
	assert.Equal(t, "fontes: HackerTarget; endereços: 45.33.32.156, 2600:3c01::f03c:91ff:fe18:bb2f", got[3].Evidence)

	_, err = ParseAmassJSON(strings.NewReader("{quebrado\n"))
	assert.Error(t, err)
}
