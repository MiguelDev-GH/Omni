package httpapi

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlaws(t *testing.T) {
	h := New(newFake())
	for path, body := range map[string]string{
		"/.env":        "APP_ENV=producao\nDB_HOST=127.0.0.1\nDB_USER=admin\nDB_PASSWORD=FALSO-senha-da-cobaia\nAWS_ACCESS_KEY_ID=FALSO-AKIAEXEMPLO\nAWS_SECRET_ACCESS_KEY=FALSO-segredo-da-cobaia\n",
		"/.git/config": "[core]\n\trepositoryformatversion = 0\n\tfilemode = true\n\tbare = false\n[remote \"origin\"]\n\turl = https://github.com/exemplo/loja-cobaia.git\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n",
		"/.git/HEAD":   "ref: refs/heads/main\n",
	} {
		rec := do(t, h, "GET", path, "")
		assert.Equal(t, 200, rec.Code, path)
		assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"), path)
		assert.Equal(t, body, rec.Body.String(), path)
	}
}

func TestEnvHasNoRealSecrets(t *testing.T) {
	body := do(t, New(newFake()), "GET", "/.env", "").Body.String()
	assert.Contains(t, body, "DB_PASSWORD=")
	assert.NotContains(t, body, "loja-cobaia")
	for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
		k, v, _ := strings.Cut(line, "=")
		if strings.Contains(k, "PASSWORD") || strings.Contains(k, "SECRET") || strings.Contains(k, "KEY") {
			assert.True(t, strings.HasPrefix(v, "FALSO-"), line)
		}
	}
}
