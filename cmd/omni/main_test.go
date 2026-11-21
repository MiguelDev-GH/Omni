package main

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/MiguelDev-GH/omni/internal/orchestrator"
	"github.com/MiguelDev-GH/omni/internal/sniffer"
)

// noDocker é um executor sem Docker, para os testes não dependerem da máquina.
type noDocker struct{}

func (noDocker) Run(context.Context, orchestrator.Spec, io.Writer) ([]byte, error) {
	return nil, orchestrator.ErrDockerIndisponivel
}
func (noDocker) Available(context.Context) error { return orchestrator.ErrDockerIndisponivel }

// TestMain impede que os testes abram uma captura de verdade ou usem o
// Docker, cujo resultado dependeria das permissões e dos serviços da máquina.
func TestMain(m *testing.M) {
	openCapture = func(string, string) (sniffer.Source, error) { return nil, sniffer.ErrSemPermissao }
	newRunner = func() (orchestrator.Runner, error) { return noDocker{}, nil }
	os.Exit(m.Run())
}
