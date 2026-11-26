// Comando carga gera tráfego contra a loja cobaia até Ctrl+C ou -duracao.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
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
	attack := flag.String("ataque", "", "simula um ataque em vez do tráfego normal: forca-bruta, spraying ou ddos")
	dns := load.DNSOptions{}
	flag.StringVar(&dns.Addr, "dns", "", "servidor DNS para consultar junto com a carga, como 127.0.0.1:1053 (vazio desliga)")
	flag.StringVar(&dns.Name, "dns-nome", "loja.local", "nome consultado no DNS")
	flag.IntVar(&dns.Rate, "dns-taxa", 5, "consultas DNS por segundo")
	mail := load.MailOptions{}
	flag.StringVar(&mail.Addr, "smtp", "", "servidor SMTP para enviar e-mails de pedido, como 127.0.0.1:1025 (vazio desliga)")
	flag.IntVar(&mail.PerMinute, "emails-por-minuto", 30, "e-mails enviados por minuto")
	flag.Parse()

	mix := load.DefaultMix
	if *attack != "" {
		m, ok := load.Attacks[*attack]
		if !ok {
			fmt.Fprintf(os.Stderr, "ataque desconhecido %q; use forca-bruta, spraying ou ddos\n", *attack)
			os.Exit(2)
		}
		mix = m
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	o.Progress = func(st load.Stats) { fmt.Println("parcial:", st) }

	if *attack != "" {
		fmt.Printf("ataque simulado %q em %s: %d req/s\n", *attack, o.BaseURL, o.Rate)
	} else {
		fmt.Printf("carga em %s: %d req/s\n", o.BaseURL, o.Rate)
	}
	// DNS e e-mail rodam ao lado da carga HTTP e param junto com ela.
	extraCtx, stopExtra := context.WithCancel(ctx)
	var wg sync.WaitGroup
	if dns.Addr != "" {
		fmt.Printf("consultas DNS em %s: %d/s para %s\n", dns.Addr, dns.Rate, dns.Name)
		wg.Add(1)
		go func() {
			defer wg.Done()
			sent, answered := load.RunDNS(extraCtx, dns)
			fmt.Printf("dns: %d consultas, %d respondidas\n", sent, answered)
		}()
	}
	if mail.Addr != "" {
		fmt.Printf("e-mails em %s: %d por minuto\n", mail.Addr, mail.PerMinute)
		wg.Add(1)
		go func() {
			defer wg.Done()
			sent, failed := load.RunMail(extraCtx, mail)
			fmt.Printf("smtp: %d e-mails aceitos, %d falharam\n", sent, failed)
		}()
	}

	st := load.Run(ctx, &http.Client{Timeout: 5 * time.Second}, mix, o, uint64(time.Now().UnixNano()))
	stopExtra()
	wg.Wait()
	fmt.Println("final:", st)
	// Num ataque, 401 em tudo é o esperado; só falha de conexão encerra com erro.
	if st.Total > 0 && st.Failures == st.Total {
		os.Exit(1)
	}
}
