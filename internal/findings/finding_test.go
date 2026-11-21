package findings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFingerprintAndSort(t *testing.T) {
	a := New("nmap", "Porta 22/tcp aberta: ssh", "", Info, 22)
	b := New("nmap", "Porta 22/tcp aberta: ssh", "outra evidência", Info, 22)
	assert.Equal(t, a.Fingerprint, b.Fingerprint)
	assert.Len(t, a.Fingerprint, 64)
	assert.NotEqual(t, a.Fingerprint, New("nmap", "Porta 22/tcp aberta: ssh", "", Info, 2222).Fingerprint)

	list := []Finding{a, New("nuclei", "x", "", Critical, 80), New("nuclei", "y", "", Medium, 80)}
	Sort(list)
	assert.Equal(t, []Severity{Critical, Medium, Info}, []Severity{list[0].Severity, list[1].Severity, list[2].Severity})
}

func TestFromTool(t *testing.T) {
	for in, want := range map[string]Severity{"critical": Critical, "HIGH": High, "medium": Medium, "low": Low, "info": Info, "OK": Info, "unknown": Info} {
		assert.Equal(t, want, FromTool(in), in)
	}
	assert.Equal(t, 4, Critical.Rank())
	assert.Equal(t, -1, Severity("x").Rank())
}
