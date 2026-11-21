// Comando carga gera tráfego contra a loja cobaia até Ctrl+C ou -duracao.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MiguelDev-GH/omni/subjects/loja/internal/load"
)

func main() {
	o := load.Options{}
	flag.StringVar(&o.BaseURL, "url", "http://127.0.0.1:3000", "endereço da loja")
	flag.IntVar(&o.Rate, "taxa", 20, "requisições por segundo")
	flag.DurationVar(&o.Duration, "duracao", 0, "quanto tempo rodar (0 = até Ctrl+C)")
	flag.IntVar(&o.Workers, "workers", 4, "requisições em paralelo")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	o.Progress = func(st load.Stats) { fmt.Println("parcial:", st) }

	fmt.Printf("carga em %s: %d req/s\n", o.BaseURL, o.Rate)
	st := load.Run(ctx, &http.Client{Timeout: 5 * time.Second}, load.DefaultMix, o, uint64(time.Now().UnixNano()))
	fmt.Println("final:", st)
	if st.Total > 0 && st.Failures == st.Total {
		os.Exit(1)
	}
}
