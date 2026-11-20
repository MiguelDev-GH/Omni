// Comando omni: monitoramento e segurança do servidor onde roda.
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/MiguelDev-GH/omni/internal/config"
)

func main() {
	if err := start(); err != nil {
		fmt.Fprintln(os.Stderr, "omni:", err)
		os.Exit(1)
	}
}

func start() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	return run(ctx, cfg, ln)
}
