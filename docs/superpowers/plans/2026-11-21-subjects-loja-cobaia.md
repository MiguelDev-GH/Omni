# Aplicação cobaia em `subjects/`, Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ter em `subjects/loja/` uma aplicação completa (API de loja em Go + PostgreSQL + reverse proxy TLS), um gerador de carga e um comando que cadastra a loja no Omni, para que cada motor do Omni (Poller, Sniffer HTTP e PostgreSQL, Arsenal com Nmap, Nuclei e testssl) tenha algo real para observar com um único `make demo`.

**Architecture:** Um módulo Go próprio (`subjects/loja/go.mod`), separado do módulo do Omni, com três binários: `loja` (API HTTP/1.1 em `127.0.0.1:3000`, falando com o PostgreSQL via TCP em `127.0.0.1:5433` pelo `pgx`), `carga` (cliente que gera uma mistura ponderada de requisições, com sucessos, 404, 500, latência alta e consultas com erro) e `cadastrar` (cria os alvos `loja` e `loja-tls` pela API do Omni, sem duplicar). O PostgreSQL e o Caddy rodam no Docker Compose com `network_mode: host`, para que todo o tráfego (proxy → app e app → banco) passe pela interface `lo`, como o README do Omni exige. O Caddy termina TLS com certificado interno (autoassinado) em `127.0.0.1:8443` e repassa em texto puro para a app. A app expõe de propósito, só no loopback, falhas falsas (`/.env`, `/.git/config`, cabeçalhos de versão) para o Nuclei encontrar.

**Tech Stack:** Go 1.24 (`net/http` padrão, `github.com/jackc/pgx/v5@v5.7.6`, `testify` igual ao do Omni), PostgreSQL `postgres:17.6-alpine`, Caddy `caddy:2.10.2-alpine`, Docker Compose v2.

**Spec:** pedido do dono do projeto ("crie uma aplicação completa feita em uma pasta /subjects, onde vai ser a cobaia de teste para o omni"), mais os requisitos do alvo descritos no `README.md` do Omni (seções "Fluxo de funcionamento", "Comunicação em tempo real", "Requisitos de implantação" e "Segurança da própria plataforma") e as decisões das Sprints 5 a 8 em `docs/SPRINTS.md`. Convenções: `.claude/skills/convencoes-omni/SKILL.md`. Cada rota e cada falha da cobaia existe para exercitar um comportamento já implementado no Omni; a tabela "O que cada parte exercita" abaixo é o critério de aceite.

## O que cada parte exercita

| Parte da cobaia | Motor do Omni | O que deve aparecer |
|---|---|---|
| `GET /` sempre 200 | Poller | `STATUS_ALVOS` com `no_ar: true`; parar a app gera `ALERTA_QUEDA`, subir de novo gera `SERVICO_RESTAURADO` |
| Porta 5433 do banco | Poller (TCP) e Sniffer | Verificação TCP da porta do banco; protocolo `PostgreSQL` em `METRICAS_TRAFEGO` |
| `GET /produtos/{id}`, `GET /pedidos/{id}` | Sniffer HTTP | Rotas normalizadas `/produtos/:id` e `/pedidos/:id` em `METRICAS_HTTP` |
| IDs inexistentes, `/rota-inexistente` | Sniffer HTTP | Classe `4xx` |
| `GET /erro` | Sniffer HTTP e PostgreSQL | Classe `5xx` e `erros` > 0 em `METRICAS_BANCO` |
| `GET /lento?ms=` | Sniffer HTTP e PostgreSQL | p95 alto na rota `/lento` e `SELECT pg_sleep(?)` no topo de `GET /api/consultas/lentas` |
| `GET /produtos/busca?q=` (protocolo simples) | Sniffer PostgreSQL | Consulta simples com literal normalizado para `?`; a query string não aparece em lugar nenhum |
| Demais consultas (protocolo estendido do `pgx`) | Sniffer PostgreSQL | Parse/Bind/Execute pareados; tipos `SELECT`, `INSERT`, `UPDATE` |
| Caddy TLS em 8443 → app em 3000 | Sniffer e testssl | HTTP em texto puro visto no `lo` atrás do proxy; testssl acusa certificado não confiável |
| `/.env`, `/.git/config`, `Server`/`X-Powered-By`, sem cabeçalhos de segurança | Nuclei | Achados de `exposure`, `misconfig` e `tech` |
| Portas 3000, 5433, 8443 | Nmap | Três portas abertas com serviço e versão |
| `User-Agent: Loja-Carga/1` | Sniffer HTTP | Tráfego da carga contado (não começa com `Omni-`) |

## Global Constraints

- Tudo fica em `subjects/`; a cobaia é um módulo Go separado: `module github.com/MiguelDev-GH/omni/subjects/loja`, linha `go 1.24.0`. Nenhum arquivo fora de `subjects/` importa a cobaia, e o `go.mod` raiz não muda.
- Dependências Go da cobaia: só `github.com/jackc/pgx/v5 v5.7.6` e `github.com/stretchr/testify` (mesma versão do `go.mod` raiz). Roteamento com o `http.ServeMux` padrão (padrões `GET /produtos/{id}`).
- Imagens com tag exata: `postgres:17.6-alpine` e `caddy:2.10.2-alpine`.
- Endereços fixos: app `127.0.0.1:3000`, PostgreSQL `127.0.0.1:5433` (sem SSL), Caddy `127.0.0.1:8443`. A app recusa endereço fora do loopback, sem exceção: as falhas expostas são de propósito e não podem sair da máquina.
- Credenciais de teste, só da cobaia: usuário `loja`, senha `loja-cobaia`, banco `loja`; URL padrão `postgres://loja:loja-cobaia@127.0.0.1:5433/loja?sslmode=disable`. O conteúdo do `/.env` falso usa valores que começam com `FALSO-` e nunca reaproveita as credenciais reais da cobaia.
- Variáveis de ambiente: `LOJA_ADDR` (padrão `127.0.0.1:3000`), `LOJA_BANCO` (padrão acima), `OMNI_URL` (padrão `http://127.0.0.1:8080`), `OMNI_TOKEN` (obrigatória no `cadastrar`), `LOJA_TESTE_BANCO` (quando definida, liga os testes de integração com o banco).
- Identificadores em inglês; JSON, mensagens e textos em português com acentuação, como no Omni. Erros da API da cobaia no formato `{"erro": "<mensagem>"}`.
- Testes com `testing` + `testify`. Testes que precisam do PostgreSQL chamam `t.Skip` sem `LOJA_TESTE_BANCO`; nenhum teste depende de rede externa.
- Branch `feat/subjects-loja` a partir da `main`. Commits com escopo `subjects`, por exemplo `feat(subjects): adiciona gerador de carga`. PR `feat(subjects): adiciona aplicação cobaia para testar o Omni` com labels `funcionalidade` e `cobaia` (label nova, criada na Task 1). Antes de cada commit, rode `make check` na raiz e `make teste` em `subjects/loja` e relate o resultado real.

## Review Focus

1. **Banco fora do ar quando a app sobe, ou caindo no meio:** a app sobe mesmo assim (o Poller precisa ver `GET /` respondendo), `GET /` continua 200, `GET /saude` devolve 503 `{"erro":"banco indisponível"}` e as rotas de dados devolvem 503 em vez de travar ou entrar em pânico; quando o banco volta, as rotas voltam sem reiniciar a app. Testes `TestRootIgnoresDB` e `TestDataRoutesReturn503WhenDBDown` (Task 3) e `TestMigrateRetriesUntilDB` (Task 5).
2. **IDs e parâmetros malformados** (`/produtos/abc`, `/produtos/-1`, `/produtos/99999999999999999999`, `?ms=-5`, `?ms=999999`, `?limite=0`, corpo JSON inválido ou `quantidade: 0` no `POST /pedidos`): 400 com mensagem, nunca 500, e `ms` limitado a 0–2000. Testes `TestBadIDs`, `TestSlowClampsMS` e `TestCreateOrderValidation` (Task 3).
3. **Rodar `cadastrar` duas vezes, ou com token errado:** a segunda execução não cria alvos repetidos (procura pelo nome); token errado termina com código 1 e a mensagem "token do Omni recusado (401)". Testes `TestRegisterIsIdempotent` e `TestRegisterReportsBadToken` (Task 7).
4. **Pedidos concorrentes para o último item do estoque:** exatamente um recebe 201 e os outros 409 `{"erro":"estoque insuficiente"}`; o estoque nunca fica negativo. Teste `TestConcurrentOrdersNeverOversell` (Task 2, integração).
5. **Endereço fora do loopback em `LOJA_ADDR`** (`0.0.0.0:3000`, `:3000`, `192.168.0.10:3000`): a app recusa subir, com código 1 e "LOJA_ADDR precisa ser um endereço de loopback". Teste `TestLoadRejectsNonLoopback` (Task 1).

## Pré-requisitos

- Go, golangci-lint v2, Docker com Compose v2 e o Omni compilável na `main`. Ferramenta ausente é relatada como "não verificado", nunca como "passou".
- Branch: `git switch main && git pull --ff-only && git switch -c feat/subjects-loja`.
- Fora do escopo (o Omni ainda não tem os motores): MySQL, DNS, SMTP, HTTP/2 e rotas vulneráveis a SQL injection. Quando a Fase 4 trouxer esses motores, a cobaia ganha as partes correspondentes.

## Estrutura de arquivos

| Arquivo | Responsabilidade |
|---|---|
| `subjects/README.md` | O que é a pasta, aviso de que as falhas são de propósito, como rodar a demo |
| `subjects/loja/go.mod`, `go.sum` | Módulo da cobaia |
| `subjects/loja/Makefile` | `subir`, `rodar`, `carga`, `cadastrar`, `demo`, `derrubar`, `teste` |
| `subjects/loja/compose.yaml`, `Caddyfile` | PostgreSQL e Caddy em `network_mode: host` |
| `subjects/loja/internal/config/config.go` | `Load(getenv)`: endereços, URL do banco, recusa de não loopback |
| `subjects/loja/internal/store/store.go` | Tipos `Product`, `Order`, interface `Store`, erros sentinela |
| `subjects/loja/internal/store/postgres.go` | Implementação com `pgxpool`, migração e dados iniciais |
| `subjects/loja/internal/httpapi/api.go` | `New(store.Store) http.Handler`: rotas da loja, erros, cabeçalhos |
| `subjects/loja/internal/httpapi/flaws.go` | Falhas expostas de propósito para o Nuclei |
| `subjects/loja/internal/load/load.go` | Mistura ponderada de requisições e execução da carga |
| `subjects/loja/internal/register/register.go` | Cadastro idempotente dos alvos no Omni |
| `subjects/loja/cmd/loja`, `cmd/carga`, `cmd/cadastrar` | `main` de cada binário |
| `Makefile` (raiz), `.github/workflows/ci.yml`, `.github/labels.yml` | Verificação da cobaia no `make check` e no CI; label `cobaia` |
| `README.md` (raiz) | Seção curta "Aplicação cobaia" apontando para `subjects/` |

---

### Task 1: Módulo, configuração e verificação no CI

**Files:**
- Create: `subjects/loja/go.mod`, `subjects/loja/Makefile`, `subjects/loja/internal/config/config.go`, `subjects/README.md` (esqueleto: título, uma frase e o aviso de falhas de propósito)
- Test: `subjects/loja/internal/config/config_test.go`
- Modify: `Makefile` (alvo `check`), `.github/workflows/ci.yml` (job novo), `.github/labels.yml`

**Interfaces:**
- Produces: `type Config struct { Addr, DatabaseURL, OmniURL, OmniToken string }`; `func Load(getenv func(string) string) (Config, error)`; `var ErrNonLoopback = errors.New("LOJA_ADDR precisa ser um endereço de loopback")`. `OmniToken` vazio não é erro aqui (só o `cadastrar` exige).

- [ ] **Step 1: Write the failing tests**

```go
func TestLoadDefaults(t *testing.T) {
	c, err := Load(func(string) string { return "" })
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:3000", c.Addr)
	assert.Equal(t, "postgres://loja:loja-cobaia@127.0.0.1:5433/loja?sslmode=disable", c.DatabaseURL)
	assert.Equal(t, "http://127.0.0.1:8080", c.OmniURL)
}

func TestLoadRejectsNonLoopback(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:3000", ":3000", "192.168.0.10:3000", "exemplo.com:3000"} {
		_, err := Load(env(map[string]string{"LOJA_ADDR": addr}))
		assert.ErrorIs(t, err, ErrNonLoopback, addr)
	}
	for _, addr := range []string{"127.0.0.1:3001", "[::1]:3000", "localhost:3000"} {
		_, err := Load(env(map[string]string{"LOJA_ADDR": addr}))
		assert.NoError(t, err, addr)
	}
}
```

(`env` é um helper do teste que devolve uma função `getenv` a partir do mapa.)

- [ ] **Step 2: Run to verify it fails**

Run: `cd subjects/loja && go test ./internal/config/`
Expected: FAIL (pacote sem `Load`).

- [ ] **Step 3: Implement `Load` in `internal/config/config.go`**

`net.SplitHostPort`; host `localhost` ou `net.ParseIP(host).IsLoopback()`; host vazio é recusado.

- [ ] **Step 4: Run to verify it passes**

Run: `cd subjects/loja && go test -race ./...`
Expected: PASS.

- [ ] **Step 5: Wire verification**

- `subjects/loja/Makefile`: alvo `teste` = `go vet ./... && golangci-lint run ./... && go test -race ./...` (os outros alvos entram nas tasks seguintes).
- `Makefile` raiz, no fim do `check`: `cd subjects/loja && $(MAKE) teste`.
- `ci.yml`: job `subjects` igual ao job `go` (sem libpcap), com `working-directory: subjects/loja`, `go-version-file: subjects/loja/go.mod` e o `golangci-lint-action` com `working-directory: subjects/loja`.
- `labels.yml`: `- name: cobaia`, `color: "D4C5F9"`, `description: "Aplicação de teste em subjects/, usada como alvo do Omni"`. Atualize a tabela de labels e os escopos sugeridos (`subjects`) em `.claude/skills/convencoes-omni/SKILL.md`.

Run: `make check` na raiz. Expected: tudo passa, incluindo a linha nova.

- [ ] **Step 6: Commit**

```bash
git add subjects .github Makefile .claude/skills/convencoes-omni/SKILL.md
git commit -m "feat(subjects): cria módulo da aplicação cobaia com configuração"
```

---

### Task 2: Banco da loja (PostgreSQL)

**Files:**
- Create: `subjects/loja/internal/store/store.go`, `subjects/loja/internal/store/postgres.go`, `subjects/loja/compose.yaml` (só o serviço `postgres` por enquanto)
- Test: `subjects/loja/internal/store/postgres_test.go`
- Modify: `subjects/loja/Makefile` (`subir`, `derrubar`)

**Interfaces:**
- Produces:
  - `type Product struct { ID int64 \`json:"id"\`; Name string \`json:"nome"\`; PriceCents int \`json:"preco_centavos"\`; Stock int \`json:"estoque"\` }`
  - `type Order struct { ID, ProductID int64; Quantity int; CreatedAt time.Time }` com tags `id`, `produto_id`, `quantidade`, `criado_em`
  - `var ErrNotFound, ErrOutOfStock, ErrUnavailable error` (mensagens "não encontrado", "estoque insuficiente", "banco indisponível")
  - `type Store interface { Ping(ctx) error; ListProducts(ctx, limit, offset int) ([]Product, error); GetProduct(ctx, id int64) (Product, error); SearchProducts(ctx, q string) ([]Product, error); CreateOrder(ctx, productID int64, qty int) (Order, error); GetOrder(ctx, id int64) (Order, error); Sleep(ctx, d time.Duration) error; Broken(ctx) error }`
  - `func Open(ctx context.Context, url string) (*Postgres, error)` — cria o pool sem conectar (`pgxpool.NewWithConfig`), nunca falha por banco fora do ar.
  - `func (p *Postgres) Migrate(ctx context.Context) error` — cria `produtos` e `pedidos` (`IF NOT EXISTS`) e insere 50 produtos (`Produto 01`…`Produto 50`, preço `1000 + i*137` centavos, estoque 100) só se a tabela estiver vazia.
  - `func (p *Postgres) Close()`
- Comportamentos que o protocolo exige: `SearchProducts` usa `pgx.QueryExecModeSimpleProtocol` (`... WHERE nome ILIKE '%' || $1 || '%' LIMIT 20`), para gerar consulta simples com literal; todas as outras usam o modo padrão (estendido). `Sleep` roda `SELECT pg_sleep($1)`. `Broken` roda `SELECT * FROM tabela_inexistente` e devolve o erro do PostgreSQL. `CreateOrder` numa transação: `UPDATE produtos SET estoque = estoque - $2 WHERE id = $1 AND estoque >= $2`; zero linhas → distingue `ErrNotFound` de `ErrOutOfStock` com um `SELECT`; depois `INSERT ... RETURNING`. Erro de conexão (qualquer `*pgconn.ConnectError` ou falha de dial) vira `ErrUnavailable` com `errors.Join`/`%w`.

- [ ] **Step 1: `compose.yaml` com o PostgreSQL**

Serviço `postgres`: imagem `postgres:17.6-alpine`, `network_mode: host`, `command: ["postgres", "-p", "5433", "-c", "listen_addresses=127.0.0.1", "-c", "ssl=off"]`, ambiente `POSTGRES_USER=loja`, `POSTGRES_PASSWORD=loja-cobaia`, `POSTGRES_DB=loja`, volume nomeado `loja-dados`, healthcheck `pg_isready -h 127.0.0.1 -p 5433 -U loja`. Makefile: `subir` = `docker compose up -d --wait`, `derrubar` = `docker compose down` (o volume fica; `derrubar-tudo` usa `down -v`).

Run: `cd subjects/loja && make subir && pg_isready -h 127.0.0.1 -p 5433` (ou `docker compose exec postgres pg_isready -h 127.0.0.1 -p 5433`). Expected: `accepting connections`.

- [ ] **Step 2: Write the failing integration tests** (todos começam com `url := os.Getenv("LOJA_TESTE_BANCO"); if url == "" { t.Skip("defina LOJA_TESTE_BANCO") }` e usam um banco limpo: `DROP TABLE IF EXISTS pedidos, produtos` antes de `Migrate`)

- `TestMigrateSeedsOnce`: `Migrate` duas vezes → `ListProducts(ctx, 100, 0)` tem 50 itens; o primeiro é `Product{ID: 1, Name: "Produto 01", PriceCents: 1137, Stock: 100}`.
- `TestGetProductNotFound`: `GetProduct(ctx, 999)` → `ErrorIs ErrNotFound`.
- `TestSearchProducts`: `SearchProducts(ctx, "o 0")` → 9 itens (Produto 01–09); `SearchProducts(ctx, "' OR 1=1 --")` → 0 itens, sem erro.
- `TestCreateOrder`: pedido de 3 do produto 1 → `Order.Quantity == 3`, estoque do produto 1 vira 97; `GetOrder` devolve o mesmo pedido; produto 999 → `ErrNotFound`; 101 unidades → `ErrOutOfStock`.
- `TestConcurrentOrdersNeverOversell`: estoque do produto 2 ajustado para 1; 20 goroutines pedem 1 unidade → exatamente 1 sucesso, 19 `ErrOutOfStock`, estoque final 0.
- `TestBrokenReturnsPGError`: `Broken` → erro cujo `*pgconn.PgError` tem `Code == "42P01"`.
- `TestUnavailable` (não precisa de banco, roda sempre): `Open(ctx, "postgres://loja:x@127.0.0.1:1/loja?sslmode=disable")` não falha; `Ping` → `ErrorIs ErrUnavailable`.

- [ ] **Step 3: Run to verify they fail**

Run: `cd subjects/loja && LOJA_TESTE_BANCO=postgres://loja:loja-cobaia@127.0.0.1:5433/loja?sslmode=disable go test ./internal/store/`
Expected: FAIL (tipos indefinidos).

- [ ] **Step 4: Implement `store.go` and `postgres.go`**

`go get github.com/jackc/pgx/v5@v5.7.6`. Pool com `MaxConns = 10`, `ConnConfig.ConnectTimeout = 2 * time.Second`.

- [ ] **Step 5: Run to verify they pass**

Run: o comando do Step 3, e depois `make teste` sem a variável (os de integração aparecem como SKIP).
Expected: PASS nos dois.

- [ ] **Step 6: Commit**

```bash
git add subjects/loja
git commit -m "feat(subjects): adiciona banco PostgreSQL da loja cobaia"
```

---

### Task 3: API HTTP da loja

**Files:**
- Create: `subjects/loja/internal/httpapi/api.go`
- Test: `subjects/loja/internal/httpapi/api_test.go` (com um `fakeStore` que implementa `store.Store` em memória e um campo `down bool` que faz todo método devolver `store.ErrUnavailable`)

**Interfaces:**
- Consumes: `store.Store`, `store.Product`, `store.Order`, erros sentinela (Task 2).
- Produces: `func New(s store.Store) http.Handler`.

Rotas e respostas (todas com `Content-Type: application/json; charset=utf-8`, `Server: Loja/0.1` e `X-Powered-By: Go`; nenhum cabeçalho de segurança, de propósito):

| Rota | Sucesso | Erros |
|---|---|---|
| `GET /{$}` | 200 `{"servico":"loja","status":"ok"}`, sem tocar no banco | — |
| `GET /saude` | 200 `{"banco":"ok"}` | 503 banco indisponível |
| `GET /produtos?limite=&pagina=` | 200 lista (limite 1–100, padrão 20; página ≥ 1, padrão 1) | 400 parâmetro inválido |
| `GET /produtos/{id}` | 200 produto | 400 id inválido (não inteiro, ≤ 0, overflow), 404 |
| `GET /produtos/busca?q=` | 200 lista | 400 `q` vazio ou com mais de 100 caracteres |
| `POST /pedidos` corpo `{"produto_id","quantidade"}` | 201 pedido | 400 JSON inválido ou quantidade fora de 1–10, 404, 409 `estoque insuficiente` |
| `GET /pedidos/{id}` | 200 pedido | 400, 404 |
| `GET /lento?ms=` | 200 `{"dormiu_ms":N}` (`ms` padrão 500, limitado a 0–2000, valor negativo vira 0) | 400 `ms` não numérico |
| `GET /erro` | — | 500 `{"erro":"falha proposital no banco"}` (chama `Broken`) |
| outra rota | — | 404 `{"erro":"rota não encontrada"}` |

Em qualquer rota de dados, `ErrUnavailable` vira 503 `{"erro":"banco indisponível"}`; outro erro inesperado vira 500 `{"erro":"erro interno"}` e é logado com `log/slog`. Corpo do `POST` limitado a 4 KiB com `http.MaxBytesReader`. `/produtos/busca` tem de ser registrado de forma que não seja capturado por `/produtos/{id}` (o `ServeMux` prioriza o padrão literal).

- [ ] **Step 1: Write the failing tests**

Tabela de casos com `httptest`, um teste por grupo:
- `TestRoot`: 200 e corpo exato; cabeçalhos `Server: Loja/0.1` e `X-Powered-By: Go`.
- `TestRootIgnoresDB`: com `down: true`, `GET /` ainda 200.
- `TestDataRoutesReturn503WhenDBDown`: com `down: true`, `/saude`, `/produtos`, `/produtos/1`, `/produtos/busca?q=a`, `POST /pedidos`, `/pedidos/1`, `/lento?ms=0`, `/erro` → 503 `{"erro":"banco indisponível"}`.
- `TestBadIDs`: `/produtos/abc`, `/produtos/-1`, `/produtos/0`, `/produtos/99999999999999999999`, `/pedidos/x` → 400.
- `TestProductsPagination`: `?limite=5&pagina=2` chama `ListProducts(ctx, 5, 5)`; `?limite=0`, `?limite=101`, `?pagina=0` → 400.
- `TestSearchRouteNotShadowed`: `GET /produtos/busca?q=01` chama `SearchProducts` com `"01"` (não `GetProduct`).
- `TestCreateOrderValidation`: corpo `{`, `{"produto_id":1,"quantidade":0}`, `quantidade: 11`, corpo de 5 KiB → 400; produto inexistente → 404; `ErrOutOfStock` → 409; válido → 201 com `Location: /pedidos/<id>`.
- `TestSlowClampsMS`: `?ms=999999` chama `Sleep` com `2*time.Second`; `?ms=-5` com `0`; `?ms=abc` → 400; sem `ms` → `500*time.Millisecond`.
- `TestErrorRoute`: `/erro` → 500 com a mensagem acima.
- `TestUnknownRoute`: `/rota-inexistente` → 404 JSON.

- [ ] **Step 2: Run to verify they fail**

Run: `cd subjects/loja && go test ./internal/httpapi/`
Expected: FAIL (`New` indefinido).

- [ ] **Step 3: Implement `New` in `api.go`**

Um middleware que define `Server` e `X-Powered-By`; um helper `writeJSON(w, status, v)` e um `writeError(w, err)` que mapeia os erros sentinela para os status da tabela.

- [ ] **Step 4: Run to verify they pass**

Run: `cd subjects/loja && make teste`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add subjects/loja/internal/httpapi
git commit -m "feat(subjects): adiciona API HTTP da loja cobaia"
```

---

### Task 4: Falhas expostas de propósito

**Files:**
- Create: `subjects/loja/internal/httpapi/flaws.go`
- Test: `subjects/loja/internal/httpapi/flaws_test.go`
- Modify: `subjects/loja/internal/httpapi/api.go` (registrar as rotas)

**Interfaces:**
- Consumes: `New` (Task 3).
- Produces: as rotas abaixo dentro do mesmo `http.Handler`, sem tocar no banco.

| Rota | Resposta (`Content-Type: text/plain; charset=utf-8`) |
|---|---|
| `GET /.env` | `APP_ENV=producao\nDB_HOST=127.0.0.1\nDB_USER=admin\nDB_PASSWORD=FALSO-senha-da-cobaia\nAWS_ACCESS_KEY_ID=FALSO-AKIAEXEMPLO\nAWS_SECRET_ACCESS_KEY=FALSO-segredo-da-cobaia\n` |
| `GET /.git/config` | `[core]\n\trepositoryformatversion = 0\n\tfilemode = true\n\tbare = false\n[remote "origin"]\n\turl = https://github.com/exemplo/loja-cobaia.git\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n` |
| `GET /.git/HEAD` | `ref: refs/heads/main\n` |

Um comentário no topo de `flaws.go` explica que as rotas existem para o Nuclei e só ficam acessíveis porque `config.Load` impede endereços fora do loopback.

- [ ] **Step 1: Write the failing test**

`TestFlaws`: as três rotas respondem 200 com o corpo exato da tabela; `TestEnvHasNoRealSecrets`: o corpo de `/.env` não contém `loja-cobaia` (a senha real do banco da cobaia), e toda linha com `PASSWORD`, `SECRET` ou `KEY` tem valor começando com `FALSO-`.

- [ ] **Step 2: Run to verify it fails**

Run: `cd subjects/loja && go test ./internal/httpapi/ -run 'Flaws|EnvHasNo'`
Expected: FAIL (404).

- [ ] **Step 3: Implement in `flaws.go`**

- [ ] **Step 4: Run to verify it passes**

Run: `cd subjects/loja && make teste`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add subjects/loja/internal/httpapi
git commit -m "feat(subjects): expõe falhas falsas para o Nuclei encontrar"
```

---

### Task 5: Binário `loja`, Caddy e subida completa

**Files:**
- Create: `subjects/loja/cmd/loja/main.go`, `subjects/loja/Caddyfile`
- Test: `subjects/loja/internal/store/postgres_test.go` (novo teste), verificação manual com `curl`
- Modify: `subjects/loja/compose.yaml` (serviço `caddy`), `subjects/loja/internal/store/postgres.go`, `subjects/loja/Makefile` (`rodar`)

**Interfaces:**
- Consumes: `config.Load` (Task 1), `store.Open`, `(*Postgres).Migrate` (Task 2), `httpapi.New` (Tasks 3–4).
- Produces: `func MigrateWithRetry(ctx context.Context, m interface{ Migrate(context.Context) error }, every time.Duration) error` em `internal/store/postgres.go` — tenta `Migrate` até dar certo ou o `ctx` acabar, logando cada falha.

Comportamento do `main`: carrega a config (erro → `slog.Error` e `os.Exit(1)`), abre o pool, começa a servir **antes** de o banco estar pronto e roda `MigrateWithRetry(ctx, db, 2*time.Second)` numa goroutine; `http.Server` com `ReadHeaderTimeout: 5s`; `signal.NotifyContext` para `SIGINT`/`SIGTERM` e `Shutdown` com 5 s de prazo. Loga `loja ouvindo em 127.0.0.1:3000`.

Caddyfile:

```
{
	admin off
	auto_https disable_redirects
}

https://localhost:8443, https://127.0.0.1:8443 {
	bind 127.0.0.1
	tls internal
	reverse_proxy 127.0.0.1:3000
}
```

Serviço `caddy` no compose: `caddy:2.10.2-alpine`, `network_mode: host`, Caddyfile montado em `/etc/caddy/Caddyfile:ro`, volume `loja-caddy` em `/data`. `rodar` = `go run ./cmd/loja`.

- [ ] **Step 1: Write the failing test**

`TestMigrateRetriesUntilDB` (sem banco): um `fakeMigrator` que falha nas duas primeiras chamadas e acerta na terceira → `MigrateWithRetry(ctx, f, time.Millisecond)` devolve `nil` com 3 chamadas; com `ctx` cancelado e um migrador que sempre falha → devolve `context.Canceled`.

- [ ] **Step 2: Run to verify it fails**

Run: `cd subjects/loja && go test ./internal/store/ -run MigrateRetries`
Expected: FAIL.

- [ ] **Step 3: Implement `MigrateWithRetry`, `cmd/loja/main.go`, Caddyfile e serviço `caddy`**

- [ ] **Step 4: Verify**

Run: `cd subjects/loja && make teste`. Expected: PASS.

Run, com `make derrubar` antes (banco parado): `make rodar` num terminal e `curl -s 127.0.0.1:3000/` noutro. Expected: `{"servico":"loja","status":"ok"}` e `/saude` com 503. Depois `make subir` e, em até 5 s, `curl -s 127.0.0.1:3000/saude` → `{"banco":"ok"}` sem reiniciar a app.

Run: `curl -sk https://127.0.0.1:8443/produtos/1` → JSON do `Produto 01`. `curl -s 127.0.0.1:8443/` → resposta 400 do Caddy (HTTP em porta TLS), sem derrubar nada.

Run: `LOJA_ADDR=0.0.0.0:3000 go run ./cmd/loja; echo $?` → mensagem de loopback e `1`.

- [ ] **Step 5: Commit**

```bash
git add subjects/loja
git commit -m "feat(subjects): adiciona binário da loja e proxy TLS com Caddy"
```

---

### Task 6: Gerador de carga

**Files:**
- Create: `subjects/loja/internal/load/load.go`, `subjects/loja/cmd/carga/main.go`
- Test: `subjects/loja/internal/load/load_test.go`
- Modify: `subjects/loja/Makefile` (`carga`)

**Interfaces:**
- Produces:
  - `type Request struct { Method, Path string; Body []byte }`
  - `type Kind struct { Name string; Weight int; Build func(r *rand.Rand) Request }` (`math/rand/v2`)
  - `var DefaultMix []Kind` — nomes e pesos exatos: `listar` 30 (`GET /produtos?limite=20&pagina=<1–3>`), `produto` 25 (`GET /produtos/<1–60>`, de propósito 51–60 dão 404), `busca` 10 (`GET /produtos/busca?q=<"Produto 0","o 1","o 2","inexistente">`), `pedido` 10 (`POST /pedidos` com `produto_id` 1–50 e `quantidade` 1–3), `ver_pedido` 5 (`GET /pedidos/<1–200>`), `lento` 5 (`GET /lento?ms=<200–1500>`), `erro` 5 (`GET /erro`), `inexistente` 5 (`GET /rota-inexistente`), `raiz` 5 (`GET /`).
  - `func Pick(mix []Kind, r *rand.Rand) Kind` — sorteio ponderado.
  - `type Options struct { BaseURL string; Rate int; Duration time.Duration; Workers int }` (padrões no `main`: `http://127.0.0.1:3000`, 20 req/s, 0 = até Ctrl+C, 4 workers).
  - `type Stats struct { Total int; ByStatus map[int]int; Failures int }`
  - `func Run(ctx context.Context, c *http.Client, mix []Kind, o Options, seed uint64) Stats` — respeita a taxa com `time.Ticker`, envia `User-Agent: Loja-Carga/1` e `Content-Type: application/json` quando há corpo, lê e descarta o corpo para reaproveitar a conexão.
- `cmd/carga`: flags `-url`, `-taxa`, `-duracao`, `-workers`; imprime um resumo a cada 10 s e no fim (`total=…, 2xx=…, 4xx=…, 5xx=…, falhas=…`). Falha de conexão conta em `Failures` e não para a carga (a app pode cair durante o teste de queda).

- [ ] **Step 1: Write the failing tests**

- `TestPickRespectsWeights`: 100 000 sorteios com `rand.New(rand.NewPCG(1, 2))` → a frequência de cada `Kind` fica a até 1 ponto percentual do peso / soma dos pesos.
- `TestDefaultMixPaths`: para cada `Kind` de `DefaultMix`, 200 chamadas a `Build` geram só caminhos com o prefixo da tabela; `pedido` gera JSON válido com `quantidade` entre 1 e 3.
- `TestRunHitsServer`: `httptest.Server` que registra método, caminho e `User-Agent`; `Run` com `Rate: 200, Duration: 300*time.Millisecond` → `Stats.Total` entre 30 e 70, todo `User-Agent` é `Loja-Carga/1`, `ByStatus` soma `Total - Failures`.
- `TestRunCountsConnectionFailures`: `BaseURL` apontando para uma porta fechada → `Failures == Total` e `Total > 0`, sem pânico.

- [ ] **Step 2: Run to verify they fail**

Run: `cd subjects/loja && go test ./internal/load/`
Expected: FAIL.

- [ ] **Step 3: Implement `load.go` and `cmd/carga/main.go`**

- [ ] **Step 4: Run to verify they pass**

Run: `cd subjects/loja && make teste`, depois `make carga ARGS="-duracao 10s"` com a loja rodando.
Expected: PASS; resumo com 2xx, 4xx e 5xx todos maiores que zero.

- [ ] **Step 5: Commit**

```bash
git add subjects/loja
git commit -m "feat(subjects): adiciona gerador de carga para a loja cobaia"
```

---

### Task 7: Cadastro dos alvos no Omni

**Files:**
- Create: `subjects/loja/internal/register/register.go`, `subjects/loja/cmd/cadastrar/main.go`
- Test: `subjects/loja/internal/register/register_test.go`
- Modify: `subjects/loja/Makefile` (`cadastrar`, `demo`)

**Interfaces:**
- Consumes: `config.Load` (Task 1) para `OmniURL` e `OmniToken`.
- Produces:
  - `type Target struct { ID int64 \`json:"id,omitempty"\`; Name string \`json:"nome"\`; Host string \`json:"host"\`; AppPorts []int \`json:"portas_app"\`; DBPort int \`json:"porta_banco,omitempty"\` }` (mesmos campos JSON de `internal/target` do Omni, sem importar o módulo do Omni)
  - `var Targets = []Target{{Name: "loja", Host: "127.0.0.1", AppPorts: []int{3000}, DBPort: 5433}, {Name: "loja-tls", Host: "127.0.0.1", AppPorts: []int{8443}}}` — o segundo alvo existe porque o testssl usa a primeira porta de aplicação, e assim ele mira o Caddy.
  - `var ErrBadToken = errors.New("token do Omni recusado (401)")`
  - `func Register(ctx context.Context, c *http.Client, baseURL, token string, want []Target) (created, existing []string, err error)` — `GET /api/alvos`, compara por `nome`, faz `POST /api/alvos` só dos que faltam, sempre com `Authorization: Bearer <token>`. 401 em qualquer chamada → `ErrBadToken`; outro status inesperado → erro com o status e o `erro` do corpo, quando houver.
- `cmd/cadastrar`: `OMNI_TOKEN` vazio → "defina OMNI_TOKEN" e código 1; imprime `criado: loja` / `já existia: loja-tls`.
- `Makefile`: `cadastrar` = `go run ./cmd/cadastrar`; `demo` = `subir`, depois a loja em segundo plano (`go run ./cmd/loja & `), `cadastrar` e `carga` em primeiro plano, com `trap` que mata a loja no Ctrl+C.

- [ ] **Step 1: Write the failing tests** (`httptest.Server` simulando `GET`/`POST /api/alvos` com uma lista em memória e checagem do token)

- `TestRegisterCreatesMissing`: servidor vazio → `created == ["loja","loja-tls"]`, e os corpos recebidos têm exatamente os campos de `Targets`.
- `TestRegisterIsIdempotent`: duas chamadas seguidas → a segunda devolve `created` vazio e `existing == ["loja","loja-tls"]`; o servidor tem 2 alvos.
- `TestRegisterReportsBadToken`: token errado → `ErrorIs ErrBadToken`, nenhum `POST` recebido.
- `TestRegisterSurfacesValidationError`: servidor responde 400 `{"erro":"portas_app: porta repetida"}` no `POST` → o erro contém `400` e `porta repetida`.

- [ ] **Step 2: Run to verify they fail**

Run: `cd subjects/loja && go test ./internal/register/`
Expected: FAIL.

- [ ] **Step 3: Implement `register.go` and `cmd/cadastrar/main.go`**

- [ ] **Step 4: Run to verify they pass**

Run: `cd subjects/loja && make teste`
Expected: PASS.

Run, com o Omni rodando (`OMNI_TOKEN=<token> ./omni` na raiz): `OMNI_TOKEN=<token> make cadastrar` duas vezes.
Expected: primeira `criado: loja`, `criado: loja-tls`; segunda `já existia: …` para os dois.

- [ ] **Step 5: Commit**

```bash
git add subjects/loja
git commit -m "feat(subjects): cadastra a loja cobaia como alvo do Omni"
```

---

### Task 8: Roteiro de ponta a ponta e documentação

**Files:**
- Modify: `subjects/README.md` (versão completa), `README.md` (seção "Aplicação cobaia", logo antes de "Requisitos de implantação")
- Create: `subjects/loja/README.md`

**Interfaces:**
- Consumes: todos os alvos do `Makefile` da cobaia.

- [ ] **Step 1: Escrever `subjects/loja/README.md`**

Pré-requisitos, `make demo`, cada alvo do Makefile, a tabela "O que cada parte exercita" deste plano (copiada) e o roteiro do Step 2 como checklist. `subjects/README.md`: aviso em destaque de que a loja tem falhas de propósito e só pode rodar no loopback, e link para a loja. README raiz: três linhas e o link.

- [ ] **Step 2: Rodar o roteiro de ponta a ponta e anotar o resultado real**

Com `sudo setcap cap_net_raw,cap_net_admin=eip ./omni` feito depois do `make build`, `OMNI_TOKEN=<token> ./omni` num terminal e `OMNI_TOKEN=<token> make demo` noutro, confira na interface do Omni (`http://127.0.0.1:8080`):

1. Geral: requisições, status 2xx/4xx/5xx e protocolos `HTTP` e `PostgreSQL` subindo.
2. Requisições na API: rotas `/produtos/:id`, `/pedidos/:id` e `/lento` com p95 alto.
3. Consultas no banco: `SELECT pg_sleep(?)` entre as mais lentas, erros > 0, e a consulta da busca com `?` no lugar do literal.
4. Parar a loja (Ctrl+C no `make demo`): `ALERTA_QUEDA` em até um intervalo do Poller; `make demo` de novo: `SERVICO_RESTAURADO`.
5. Arsenal no alvo `loja`: Nmap com 3000 e 5433; Nuclei com achados de `.env`/`.git` e cabeçalhos ausentes. No alvo `loja-tls`: testssl com certificado não confiável.
6. Relatórios: segunda varredura igual mostra zero achados novos.

Item que não bater vira anotação no PR (é um achado sobre o Omni, não da cobaia) e, se for defeito do Omni, uma issue separada; não ajuste a cobaia para esconder o problema.

- [ ] **Step 3: Verificação final**

Run: `make check` na raiz e `make teste` em `subjects/loja` (com e sem `LOJA_TESTE_BANCO`).
Expected: tudo passa; relate a saída real.

- [ ] **Step 4: Commit e PR**

```bash
git add README.md subjects
git commit -m "docs(subjects): documenta a loja cobaia e o roteiro de teste do Omni"
```

PR `feat(subjects): adiciona aplicação cobaia para testar o Omni`, base `main`, labels `funcionalidade` e `cobaia`, com o resultado do roteiro do Step 2 item a item.
