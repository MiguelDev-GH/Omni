package arsenal

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/MiguelDev-GH/omni/internal/target"
)

func TestInScope(t *testing.T) {
	tg := target.Target{Name: "app", Host: "127.0.0.1", AppPorts: []int{3000}, Domain: "app.com"}
	for host, want := range map[string]bool{
		"127.0.0.1": true, "127.0.0.53": true, "::1": true, "localhost": true,
		"app.com": true, "api.app.com": true, "API.App.com.": true,
		"evil.com": false, "app.com.evil.com": false, "evilapp.com": false, "10.0.0.1": false, "": false,
	} {
		assert.Equal(t, want, InScope(tg, host), host)
	}
	lan := target.Target{Name: "lan", Host: "10.0.0.5", AppPorts: []int{80}}
	assert.True(t, InScope(lan, "10.0.0.5"))
	assert.False(t, InScope(lan, "10.0.0.6"))
}
