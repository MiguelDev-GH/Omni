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

func TestNmapArgs(t *testing.T) {
	args, err := NmapArgs(target.Target{Name: "api", Host: "127.0.0.1", AppPorts: []int{3000, 8080}, DBPort: 5432}, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"-sV", "-Pn", "-oX", "-", "-p", "3000,8080,5432", "--", "127.0.0.1"}, args)
}

func TestNmapArgsRejectsMaliciousHosts(t *testing.T) {
	for _, host := range []string{"-oX", "127.0.0.1; rm -rf /", "$(id)", "127.0.0.1 -iL /etc/passwd", ""} {
		args, err := NmapArgs(target.Target{Name: "x", Host: host, AppPorts: []int{80}}, nil)
		assert.Error(t, err, host)
		assert.Nil(t, args, host)
	}
	_, err := NmapArgs(target.Target{Name: "x", Host: "127.0.0.1", AppPorts: []int{70000}}, nil)
	assert.Error(t, err)
}

func TestParseNmapXML(t *testing.T) {
	f, err := os.Open("testdata/nmap.xml")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	got, err := ParseNmapXML(f)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "Porta 38080/tcp aberta: http (SimpleHTTPServer 0.6)", got[0].Title)
	assert.Equal(t, "Python 3.14.10", got[0].Evidence)
	assert.Equal(t, 38080, got[0].Port)
	assert.Equal(t, "Porta 55432/tcp aberta: postgresql (PostgreSQL DB 9.6.0 or later)", got[1].Title)
	for _, f := range got {
		assert.Equal(t, findings.Info, f.Severity)
		assert.Equal(t, "nmap", f.Tool)
		assert.NotEmpty(t, f.Fingerprint)
	}

	_, err = ParseNmapXML(strings.NewReader("<nmaprun><host"))
	assert.Error(t, err)
}

func TestLookup(t *testing.T) {
	tool, ok := Lookup("nmap")
	require.True(t, ok)
	assert.Equal(t, NmapImage, tool.Image)
	_, ok = Lookup("rm")
	assert.False(t, ok)
}
