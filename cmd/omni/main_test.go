package main

import (
	"os"
	"testing"

	"github.com/MiguelDev-GH/omni/internal/sniffer"
)

// TestMain impede que os testes abram uma captura de verdade, cujo resultado
// dependeria das permissões da máquina.
func TestMain(m *testing.M) {
	openCapture = func(string, string) (sniffer.Source, error) { return nil, sniffer.ErrSemPermissao }
	os.Exit(m.Run())
}
