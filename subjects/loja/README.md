# Loja cobaia

API de loja (produtos e pedidos) feita para ser observada e varrida pelo Omni. Cada rota e cada falha existe para exercitar um motor do Omni.

| Serviço | Endereço | Onde roda |
|---|---|---|
| API da loja | `127.0.0.1:3000` (HTTP/1.1) | no host (`./bin/loja` ou `make rodar`) |
| PostgreSQL 17 | `127.0.0.1:5433` (sem SSL) | Docker, `network_mode: host` |
| Caddy (TLS interno) | `https://127.0.0.1:8443` → app | Docker, `network_mode: host` |

Tudo usa a rede do host para que o tráfego proxy → app e app → banco passe pela interface `lo`, onde o sniffer do Omni captura.

## Como rodar

Pré-requisitos: Go, Docker com Compose v2 e o Omni rodando (`OMNI_TOKEN=<token> ./omni` na raiz do repositório). Para o sniffer, o binário do Omni precisa das capabilities de captura (`sudo setcap cap_net_raw,cap_net_admin=eip ./omni`).

No Windows, rode tudo dentro do WSL2 com o Docker Engine instalado na própria distribuição: com o Docker Desktop, o `network_mode: host` deixa o banco e o Caddy fora do alcance da loja e do sniffer (veja [Windows](../../README.md#windows)).

```bash
OMNI_TOKEN=<token> make demo
```

O `demo` sobe o PostgreSQL e o Caddy, compila e inicia a loja, cadastra os alvos no Omni e gera carga até Ctrl+C. Ctrl+C para a loja e a carga; os containers continuam até `make derrubar`.

| Alvo do Makefile | O que faz |
|---|---|
| `subir` / `derrubar` / `derrubar-tudo` | Sobe os containers; para mantendo os dados; para e apaga os volumes |
| `rodar` | Sobe só a API (`go run ./cmd/loja`) |
| `carga` | Gera tráfego; flags em `ARGS`, por exemplo `ARGS="-taxa 50 -duracao 1m"` |
| `cadastrar` | Cria os alvos `loja` (portas 3000 e banco 5433) e `loja-tls` (porta 8443) no Omni, sem duplicar |
| `app` | Sobe containers, loja e carga, sem falar com o Omni (não pede credenciais); `PORTA=3001` troca a porta da loja e o Caddy acompanha |
| `demo` | Tudo acima, em sequência, incluindo o `cadastrar` |
| `teste` | `go vet`, `golangci-lint` e `go test -race` |

Variáveis: `LOJA_ADDR` (padrão `127.0.0.1:3000`, só loopback), `LOJA_BANCO` (padrão `postgres://loja:loja-cobaia@127.0.0.1:5433/loja?sslmode=disable`), `OMNI_URL` (padrão `http://127.0.0.1:8080`), `OMNI_TOKEN`. Os testes de integração com o banco rodam com `LOJA_TESTE_BANCO=<url>` e apagam as tabelas do banco apontado.

## O que cada parte exercita

| Parte da cobaia | Motor do Omni | O que deve aparecer |
|---|---|---|
| `GET /` sempre 200, mesmo sem banco | Poller | `STATUS_ALVOS` com `no_ar: true`; parar a loja gera `ALERTA_QUEDA`, subir de novo gera `SERVICO_RESTAURADO` |
| Porta 5433 do banco | Poller (TCP) e Sniffer | Verificação TCP da porta do banco; protocolo `PostgreSQL` em `METRICAS_TRAFEGO` |
| `GET /produtos/{id}`, `GET /pedidos/{id}` | Sniffer HTTP | Rotas normalizadas `/produtos/:id` e `/pedidos/:id` |
| IDs inexistentes, `/rota-inexistente` | Sniffer HTTP | Classe `4xx` |
| `GET /erro` | Sniffer HTTP e PostgreSQL | Classe `5xx` e erros de consulta |
| `GET /lento?ms=` (0–2000) | Sniffer HTTP e PostgreSQL | p95 alto em `/lento` e `SELECT pg_sleep(?)` entre as consultas lentas |
| `GET /produtos/busca?q=` (protocolo simples) | Sniffer PostgreSQL | Consulta simples com o literal trocado por `?` |
| Demais consultas (protocolo estendido) | Sniffer PostgreSQL | `SELECT`, `INSERT` e `UPDATE` pareados |
| Caddy em 8443 → loja em 3000 | Sniffer e testssl | HTTP em texto puro no `lo`; testssl acusa a cadeia do certificado |
| `/.env`, `/.git/config`, `Server`/`X-Powered-By`, sem cabeçalhos de segurança | Nuclei | Achados de `.env`, `git config` e cabeçalhos ausentes |
| Portas cadastradas | Nmap | 3000 e 5433 no alvo `loja`; 8443 no alvo `loja-tls` |
| `User-Agent: Loja-Carga/1` | Sniffer HTTP | A carga é contada (não começa com `Omni-`) |
| `POST /entrar` (Basic; `ana`/`ana123`, `bia`/`bia123`, `carlos`/`carlos123`, `admin`/`admin`), sem limite de tentativas | Detector de brute-force | `make carga ARGS="-ataque forca-bruta -taxa 2"` e `-ataque spraying` deixam o cartão Segurança vermelho |
| `carga -ataque ddos` (muitas origens via `X-Forwarded-For`) | Detector de pico de tráfego | "Pico de tráfego (possível DDoS)" vermelho depois de alguns minutos de carga normal |
| `Caddyfile` e `compose.yaml` | Integridade de arquivos | Com os dois em `OMNI_ARQUIVOS_MONITORADOS`, editar um deles gera alerta amarelo |

O alvo `loja-tls` existe porque o testssl do Omni testa a primeira porta de aplicação do alvo. O Poller vê a porta 8443 no ar com status 400, a resposta do Caddy para HTTP sem TLS.

## Roteiro de teste do Omni

Com o Omni no ar e `make demo` rodando, confira na interface (`http://127.0.0.1:8080`):

- [ ] Geral: requisições, status 2xx/4xx/5xx e protocolos `HTTP` e `PostgreSQL` subindo.
- [ ] Requisições na API: `/produtos/:id`, `/pedidos/:id` e `/lento` com p95 alto.
- [ ] Consultas no banco: `SELECT pg_sleep(?)` entre as mais lentas, erros > 0 e a busca com `?` no lugar do literal.
- [ ] Ctrl+C no `make demo`: `ALERTA_QUEDA`; `make demo` de novo: `SERVICO_RESTAURADO`.
- [ ] Arsenal no alvo `loja`: Nmap com 3000 e 5433; Nuclei com `.env`, `git config` e cabeçalhos ausentes.
- [ ] Arsenal no alvo `loja-tls`: testssl com a cadeia do certificado não confiável.
- [ ] Relatórios: repetir a mesma varredura mostra zero achados novos.

Um item que não bater é um achado sobre o Omni, não sobre a cobaia: não ajuste a loja para escondê-lo.
