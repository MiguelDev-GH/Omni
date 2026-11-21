// Comando cadastrar cria os alvos da loja cobaia no Omni, sem duplicar.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/MiguelDev-GH/omni/subjects/loja/internal/config"
	"github.com/MiguelDev-GH/omni/subjects/loja/internal/register"
)

func main() {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		fail(err)
	}
	if cfg.OmniToken == "" {
		fail(fmt.Errorf("defina OMNI_TOKEN"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	created, existing, err := register.Register(ctx, &http.Client{}, cfg.OmniURL, cfg.OmniToken, register.Targets)
	for _, n := range created {
		fmt.Println("criado:", n)
	}
	for _, n := range existing {
		fmt.Println("já existia:", n)
	}
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "erro:", err)
	os.Exit(1)
}
