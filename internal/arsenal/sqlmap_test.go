package arsenal

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MiguelDev-GH/omni/internal/findings"
)

func TestSqlmapArgs(t *testing.T) {
	args, err := SqlmapArgs(ffufTarget, Options{"rota": "/busca", "parametro": "q"})
	require.NoError(t, err)
	assert.Equal(t, []string{"-u", "http://127.0.0.1:3000/busca?q=1", "-p", "q", "--batch", "--level", "1", "--risk", "1",
		"--threads", "1", "--flush-session", "--disable-coloring", "--user-agent", "Omni-Arsenal/1"}, args)

	args, err = SqlmapArgs(ffufTarget, Options{"rota": "/produtos/:id/avaliacoes", "parametro": "ordem", "porta": "8443"})
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:8443/produtos/1/avaliacoes?ordem=1", args[1])
}

func TestSqlmapRejectsBadOptions(t *testing.T) {
	for _, tc := range []struct {
		opts  Options
		field string
	}{
		{Options{"rota": "/a", "parametro": "id --os-shell"}, "opcoes.sqlmap.parametro"},
		{Options{"rota": "/a", "parametro": "1id"}, "opcoes.sqlmap.parametro"},
		{Options{"rota": "/a"}, "opcoes.sqlmap.parametro"},
		{Options{"rota": "http://evil.com/x", "parametro": "id"}, "opcoes.sqlmap.rota"},
		{Options{"rota": "/a/../b", "parametro": "id"}, "opcoes.sqlmap.rota"},
		{Options{"rota": "//evil.com", "parametro": "id"}, "opcoes.sqlmap.rota"},
		{Options{"rota": "/a?x=1", "parametro": "id"}, "opcoes.sqlmap.rota"},
		{Options{"rota": "/a b", "parametro": "id"}, "opcoes.sqlmap.rota"},
		{Options{"parametro": "id"}, "opcoes.sqlmap.rota"},
		{Options{"rota": "/a", "parametro": "id", "porta": "5433"}, "opcoes.sqlmap.porta"},
		{Options{"rota": "/a", "parametro": "id", "dump": "1"}, "opcoes.sqlmap.dump"},
		{Options{"rota": "/a", "parametro": "id", "nivel": "6"}, "opcoes.sqlmap.nivel"},
		{Options{"rota": "/a", "parametro": "id", "risco": "0"}, "opcoes.sqlmap.risco"},
		{Options{"rota": "/a", "parametro": "id", "threads": "11"}, "opcoes.sqlmap.threads"},
		{Options{"rota": "/a", "parametro": "id", "tecnica": "Z"}, "opcoes.sqlmap.tecnica"},
		{Options{"rota": "/a", "parametro": "id", "sgbd": "Mongo"}, "opcoes.sqlmap.sgbd"},
		{Options{"rota": "/a", "parametro": "id", "tamper": "rm -rf"}, "opcoes.sqlmap.tamper"},
		{Options{"rota": "/a", "parametro": "id", "acao": "apagar"}, "opcoes.sqlmap.acao"},
		{Options{"rota": "/a", "parametro": "id", "acao": "ler-arquivo"}, "opcoes.sqlmap.arquivo"},
		{Options{"rota": "/a", "parametro": "id", "acao": "ler-arquivo", "arquivo": "passwd"}, "opcoes.sqlmap.arquivo"},
		{Options{"rota": "/a", "parametro": "id", "arquivo": "/etc/passwd"}, "opcoes.sqlmap.arquivo"},
	} {
		args, err := SqlmapArgs(ffufTarget, tc.opts)
		assert.Nil(t, args, tc.opts)
		var oe *OptionError
		require.ErrorAs(t, err, &oe, tc.opts)
		assert.Equal(t, tc.field, oe.Field(), tc.opts)
	}
}

func TestSqlmapDefaultIsDetectionOnly(t *testing.T) {
	args, err := SqlmapArgs(ffufTarget, Options{"rota": "/busca", "parametro": "q"})
	require.NoError(t, err)
	for _, a := range args {
		for _, bad := range []string{"--dump", "--os-", "--file-", "--sql-shell", "--eval", "--proxy", "--passwords", "--dbs", "--tables", "--users", "--fingerprint", "--banner", "--columns"} {
			assert.False(t, strings.HasPrefix(a, bad), "argumento proibido no padrão: %s", a)
		}
	}
	assert.NotContains(t, args, "--technique")
	assert.NotContains(t, args, "--dbms")
	assert.NotContains(t, args, "--tamper")
}

func TestSqlmapTuning(t *testing.T) {
	args, err := SqlmapArgs(ffufTarget, Options{
		"rota": "/busca", "parametro": "q", "nivel": "5", "risco": "3", "threads": "10",
		"tecnica": "T", "sgbd": "PostgreSQL", "tamper": "space2comment",
	})
	require.NoError(t, err)
	joined := strings.Join(args, " ")
	assert.Contains(t, joined, "--level 5")
	assert.Contains(t, joined, "--risk 3")
	assert.Contains(t, joined, "--threads 10")
	assert.Contains(t, joined, "--technique T")
	assert.Contains(t, joined, "--dbms PostgreSQL")
	assert.Contains(t, joined, "--tamper space2comment")
}

func TestSqlmapActions(t *testing.T) {
	base := Options{"rota": "/busca", "parametro": "q"}
	for acao, want := range map[string]string{
		"dump":     "--dump",
		"dump-all": "--dump-all",
		"bancos":   "--dbs",
		"os-shell": "--os-shell",
	} {
		o := Options{"rota": base["rota"], "parametro": base["parametro"], "acao": acao}
		args, err := SqlmapArgs(ffufTarget, o)
		require.NoError(t, err, acao)
		assert.Contains(t, args, want, acao)
	}

	args, err := SqlmapArgs(ffufTarget, Options{"rota": "/busca", "parametro": "q", "acao": "ler-arquivo", "arquivo": "/etc/passwd"})
	require.NoError(t, err)
	assert.Contains(t, args, "--file-read=/etc/passwd")
}

func TestSqlmapInjectable(t *testing.T) {
	out := fixture(t, "sqlmap-injetavel.txt")
	raw, err := SqlmapReportFrom(out)
	require.NoError(t, err)
	var rep SqlmapReport
	require.NoError(t, json.Unmarshal(raw, &rep))
	assert.True(t, rep.Injectable)
	assert.Equal(t, "q", rep.Parameter)
	assert.Equal(t, "PostgreSQL", rep.DBMS)
	assert.Equal(t, "http://127.0.0.1:3000/busca?q=1", rep.URL)
	require.Len(t, rep.Techniques, 2)
	assert.Equal(t, SqlmapTechnique{Type: "boolean-based blind", Title: "AND boolean-based blind - WHERE or HAVING clause", Payload: "q=1 AND 5871=5871"}, rep.Techniques[0])
	assert.Equal(t, "time-based blind", rep.Techniques[1].Type)

	list, err := ParseSqlmap(bytes.NewReader(out))
	require.NoError(t, err)
	require.Len(t, list, 2)
	for _, f := range list {
		assert.Equal(t, "sqlmap", f.Tool)
		assert.Equal(t, findings.Critical, f.Severity)
		assert.Contains(t, f.Title, "parâmetro q")
		assert.Contains(t, f.Evidence, "PostgreSQL")
	}
}

func TestSqlmapClean(t *testing.T) {
	out := fixture(t, "sqlmap-limpo.txt")
	raw, err := SqlmapReportFrom(out)
	require.NoError(t, err)
	var rep SqlmapReport
	require.NoError(t, json.Unmarshal(raw, &rep))
	assert.False(t, rep.Injectable)
	assert.Equal(t, "ordem", rep.Parameter)
	assert.Empty(t, rep.DBMS)
	assert.NotNil(t, rep.Techniques)
	assert.Empty(t, rep.Techniques)

	list, err := ParseSqlmap(bytes.NewReader(out))
	require.NoError(t, err)
	assert.Empty(t, list)
}
