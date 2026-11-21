package l7

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeSQL(t *testing.T) {
	cases := map[string]string{
		"SELECT * FROM u WHERE id = 42 AND nome = 'ana'":            "SELECT * FROM u WHERE id = ? AND nome = ?",
		"select  *\n\tfrom t1 where x=1.5e3":                        "select * from t1 where x=?",
		"SELECT 'it''s', E'a\\'b' FROM t":                           "SELECT ?, ? FROM t",
		"SELECT * FROM t WHERE id IN (1, 2, 3)":                     "SELECT * FROM t WHERE id IN (?)",
		"INSERT INTO t (a, b) VALUES (1, 'x'), (2, 'y')":            "INSERT INTO t (a, b) VALUES (?)",
		"SELECT $1, $2 FROM t":                                      "SELECT $1, $2 FROM t",
		"SELECT $$segredo$$, $f$x$f$":                               "SELECT ?, ?",
		`SELECT "Coluna1" FROM "t2" -- senha 123` + "\nWHERE a = 1": `SELECT "Coluna1" FROM "t2" WHERE a = ?`,
		"ALTER USER app PASSWORD 'segredo' /* nota */":              "ALTER USER app PASSWORD ?",
		"SELECT col2 FROM tab3":                                     "SELECT col2 FROM tab3",
	}
	for in, want := range cases {
		assert.Equal(t, want, NormalizeSQL(in), in)
	}
	assert.Len(t, NormalizeSQL("SELECT "+strings.Repeat("a, ", 1000)), maxQueryLen)
	assert.Equal(t, "SELECT ?", NormalizeSQL("SELECT 'sem fim"))
}

func TestQueryKind(t *testing.T) {
	assert.Equal(t, "SELECT", QueryKind("select 1"))
	assert.Equal(t, "WITH", QueryKind("WITH x AS (SELECT 1) SELECT * FROM x"))
	assert.Equal(t, "SELECT", QueryKind("(SELECT 1)"))
	assert.Equal(t, "OUTRO", QueryKind("LISTEN canal"))
	assert.Equal(t, "OUTRO", QueryKind(""))
}
