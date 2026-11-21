package findings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiff(t *testing.T) {
	ssh := New("nmap", "Porta 22/tcp aberta: ssh", "", Info, 22)
	web := New("nmap", "Porta 80/tcp aberta: http", "", Info, 80)
	db := New("nmap", "Porta 5432/tcp aberta: postgresql", "", Info, 5432)

	novos, resolvidos := Diff([]Finding{ssh, web}, []Finding{web, db})
	assert.Equal(t, []Finding{db}, novos, "porta nova aparece em novos")
	assert.Equal(t, []Finding{ssh}, resolvidos, "porta fechada aparece em resolvidos")

	novos, resolvidos = Diff(nil, []Finding{ssh, web})
	assert.Equal(t, []Finding{ssh, web}, novos, "primeiro scan: tudo é novo")
	assert.Empty(t, resolvidos)
}
