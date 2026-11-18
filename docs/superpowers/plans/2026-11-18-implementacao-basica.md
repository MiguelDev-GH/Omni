# Omni: Plano de Implementação Básica

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Construir a versão básica do Omni em 8 sprints, agrupadas em 4 fases, em que cada sprint entrega um binário funcional e testado que serve de base para a seguinte.

**Architecture:** Binário Go único, instalado no servidor monitorado. O núcleo (API REST, Hub WebSocket, SQLite) recebe dados de três motores: o Poller (testes ativos), o Sniffer (captura passiva com gopacket) e o Orquestrador Docker (Arsenal). O frontend React + TypeScript é embutido no binário com `go:embed`.

**Tech Stack:** Go 1.24, `modernc.org/sqlite`, `gorilla/websocket`, `gopacket/gopacket` (+ `reassembly`, `pcapgo`, `pcap`), `gopsutil/v4`, Docker SDK; React + TypeScript (Vite), Tailwind CSS, Zustand, TanStack Table, Recharts, `react-use-websocket`, Vitest + Testing Library.

**Spec:** `README.md` (arquitetura, componentes, requisitos, segurança e pendências por camada). Convenções de trabalho: `.claude/skills/convencoes-omni/SKILL.md`.

## Como usar este plano

- O plano se divide em **fases** e **sprints**. Cada sprint vira **um branch e um PR**, aberto só depois que a sprint anterior foi mesclada na `main`.
- **A Sprint 1 está detalhada passo a passo.** As sprints seguintes trazem tarefas, arquivos, interfaces e testes. Ao iniciar cada uma, rode `/superpowers:writing-plans` sobre a seção da sprint para expandi-la em passos, já com o código real das sprints anteriores à vista.
- Toda tarefa segue o ciclo: escrever o teste → ver falhar → implementar → ver passar → verificar (tabela de verificação da skill `convencoes-omni`) → commitar.

| Fase | Sprint | Entrega | Branch | Labels do PR |
|---|---|---|---|---|
| 1. Fundação | 1 | Núcleo do backend: config, alvos no SQLite, API autenticada | `feat/nucleo-backend` | `funcionalidade`, `motor` |
| 1. Fundação | 2 | WebSocket, Poller e métricas do sistema | `feat/poller-e-websocket` | `funcionalidade`, `motor` |
| 1. Fundação | 3 | Frontend base embutido no binário | `feat/frontend-base` | `funcionalidade`, `visual` |
| 2. Observação passiva | 4 | Sniffer nas camadas 3 e 4 + gráficos de banda e protocolos | `feat/sniffer-camadas-3-4` | `funcionalidade`, `motor`, `visual` |
| 2. Observação passiva | 5 | Decodificação HTTP/1.1 + gráficos de requisições e status | `feat/sniffer-http` | `funcionalidade`, `motor`, `visual` |
| 2. Observação passiva | 6 | Decodificação PostgreSQL + gráfico de consultas | `feat/sniffer-postgres` | `funcionalidade`, `motor`, `visual` |
| 3. Arsenal | 7 | Orquestrador Docker + Nmap | `feat/orquestrador-nmap` | `funcionalidade`, `docker`, `visual` |
| 3. Arsenal | 8 | Nuclei, testssl e comparação entre scans | `feat/arsenal-nuclei-testssl` | `funcionalidade`, `docker`, `visual` |
| 4. Expansão | — | Backlog fora do escopo básico (lista no fim) | — | — |

## Global Constraints

- Módulo Go na raiz do repositório: `module github.com/MiguelDev-GH/omni`, `go 1.24`.
- Estrutura: `cmd/omni/` (binário), `internal/<pacote>/` (backend), `web/` (frontend Vite), `internal/web/static/` (frontend compilado e embutido).
- Identificadores de código em inglês; textos da interface, mensagens de erro da API e eventos do WebSocket em português (ex.: `{"erro": "não autorizado"}`, `{"tipo": "ALERTA_QUEDA"}`).
- SQLite via `modernc.org/sqlite` (sem cgo), modo WAL, `busy_timeout` 5000 ms, `SetMaxOpenConns(1)` (um único escritor). Migrações em `internal/store/migrations/NNN_nome.sql`, controladas por `PRAGMA user_version`.
- Endereço padrão `127.0.0.1:8080`. Endereço fora de loopback é recusado, a não ser com `OMNI_PERMITIR_REMOTO=1`.
- Token de acesso em `OMNI_TOKEN`, obrigatório, com no mínimo 32 caracteres. Toda rota `/api/*` exige `Authorization: Bearer <token>`, exceto `GET /api/saude`. Comparação em tempo constante (`crypto/subtle`).
- O WebSocket autentica com um ticket de uso único (`POST /api/ws/ticket`, válido por 30 s) e recusa origens fora da lista permitida.
- gopacket a partir do fork `github.com/gopacket/gopacket`; remontagem TCP com `reassembly`, nunca `tcpassembly`. Só o arquivo da captura ao vivo usa cgo (`pcap`); o restante do sniffer é testado com arquivos `.pcap` (`pcapgo`).
- Containers do Arsenal: argumentos sempre como `[]string`, nunca via shell; imagem com tag fixa; `NetworkMode: "host"`; limite de 1 CPU e 512 MiB; varredura só contra loopback ou o domínio cadastrado do alvo.
- Dados capturados nunca guardam valores de `Authorization`, `Cookie`, `Set-Cookie` nem corpos de requisição.
- Branches, commits, PRs e labels seguem a skill `convencoes-omni`.

## Review Focus

1. **Alvo com metacaracteres de shell ou opções disfarçadas** (`127.0.0.1; rm -rf /`, `-oX`, porta `70000`): a API responde 400 indicando o campo, e o valor nunca chega aos argumentos de um container. Testes nas Tarefas 1.2, 1.4 e 7.2.
2. **Token ausente ou errado, e WebSocket aberto a partir de outro site:** a API responde 401 e o WebSocket responde 403, sem vazar dados. Testes nas Tarefas 1.4 e 2.1.
3. **Cliente WebSocket lento ou travado:** os outros clientes continuam recebendo os eventos, e o hub não bloqueia. Teste na Tarefa 2.1.
4. **Omni rodando sem permissão de captura (sem `CAP_NET_RAW`):** o binário continua no ar com Poller e API funcionando, e a interface mostra "Sniffer: sem permissão". Teste na Tarefa 4.3.
5. **Docker parado ou não instalado:** pedir uma varredura responde 503 com mensagem clara, e o resto da aplicação segue funcionando. Testes nas Tarefas 7.3 e 7.4.

---

# Fase 1: Fundação

## Sprint 1: Núcleo do backend

Entrega: `omni` sobe em `127.0.0.1:8080`, responde `GET /api/saude` e oferece o CRUD autenticado de alvos persistido no SQLite. CI rodando as verificações do Go.

### Tarefa 1.1: Módulo Go e configuração

**Files:**
- Create: `go.mod`, `.gitignore`
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces:
  - `type Config struct { Addr string; DBPath string; Token string; AllowRemote bool }`
  - `func Load(getenv func(string) string) (Config, error)`
  - `var ErrTokenAusente, ErrTokenCurto, ErrEnderecoRemoto error`

- [ ] **Step 1: Criar o módulo**

Run: `go mod init github.com/MiguelDev-GH/omni && go mod edit -go=1.24`
`.gitignore` com: `/omni`, `*.db`, `*.db-*`, `web/node_modules/`, `web/dist/`, `*.pcap`.

- [ ] **Step 2: Escrever os testes que falham**

```go
func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

const tok = "0123456789abcdef0123456789abcdef" // 32 caracteres

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(map[string]string{"OMNI_TOKEN": tok}))
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:8080", c.Addr)
	assert.Equal(t, "omni.db", c.DBPath)
}

func TestLoadRejectsMissingToken(t *testing.T) {
	_, err := Load(env(nil))
	assert.ErrorIs(t, err, ErrTokenAusente)
}

func TestLoadRejectsShortToken(t *testing.T) {
	_, err := Load(env(map[string]string{"OMNI_TOKEN": "curto"}))
	assert.ErrorIs(t, err, ErrTokenCurto)
}

func TestLoadAddrRules(t *testing.T) {
	cases := []struct{ addr, remoto string; ok bool }{
		{"127.0.0.1:9000", "", true},
		{"[::1]:9000", "", true},
		{"localhost:9000", "", true},
		{"0.0.0.0:8080", "", false},
		{"192.168.0.10:8080", "", false},
		{"0.0.0.0:8080", "1", true},
	}
	for _, c := range cases {
		_, err := Load(env(map[string]string{"OMNI_TOKEN": tok, "OMNI_ADDR": c.addr, "OMNI_PERMITIR_REMOTO": c.remoto}))
		if c.ok { assert.NoError(t, err, c.addr) } else { assert.ErrorIs(t, err, ErrEnderecoRemoto, c.addr) }
	}
}
```

Use `github.com/stretchr/testify` (`assert`/`require`) em todos os testes Go do projeto.

- [ ] **Step 3: Rodar e ver falhar**

Run: `go test ./internal/config/`
Expected: FAIL (`undefined: Load`)

- [ ] **Step 4: Implementar `Load` em `internal/config/config.go`**

Variáveis: `OMNI_ADDR` (padrão `127.0.0.1:8080`), `OMNI_DB` (padrão `omni.db`), `OMNI_TOKEN`, `OMNI_PERMITIR_REMOTO` (`"1"` liga). Loopback = host `localhost` ou IP com `IsLoopback()` (via `net.SplitHostPort` + `net.ParseIP`).

- [ ] **Step 5: Rodar e ver passar**

Run: `go test ./internal/config/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum .gitignore internal/config
git commit -m "feat(api): adiciona carregamento de configuração por variáveis de ambiente"
```

### Tarefa 1.2: Alvo e validação

**Files:**
- Create: `internal/target/target.go`
- Test: `internal/target/target_test.go`

**Interfaces:**
- Produces:
  - `type Target struct { ID int64 \`json:"id"\`; Name string \`json:"nome"\`; Host string \`json:"host"\`; AppPorts []int \`json:"portas_app"\`; DBPort int \`json:"porta_banco,omitempty"\`; Domain string \`json:"dominio,omitempty"\` }`
  - `func (t Target) Validate() error` — retorna `*ValidationError` no primeiro campo inválido
  - `type ValidationError struct { Field string; Message string }` com `Error() string`

- [ ] **Step 1: Escrever o teste de tabela que falha**

`TestValidate` com os casos (campo esperado no erro, ou nenhum erro):

| Caso | Mudança sobre um alvo válido (`Name:"api"`, `Host:"127.0.0.1"`, `AppPorts:[]int{3000}`) | Campo do erro |
|---|---|---|
| válido | — | nenhum |
| domínio válido | `Domain:"app.exemplo.com.br"` | nenhum |
| host com shell | `Host:"127.0.0.1; rm -rf /"` | `host` |
| host como opção | `Host:"-oX"` | `host` |
| host maiúsculo | `Host:"API.local"` | `host` |
| nome vazio | `Name:"  "` | `nome` |
| nome longo | `Name:` 65 caracteres | `nome` |
| sem portas | `AppPorts:nil` | `portas_app` |
| porta 0 | `AppPorts:[]int{0}` | `portas_app` |
| porta 70000 | `AppPorts:[]int{70000}` | `portas_app` |
| porta repetida | `AppPorts:[]int{3000,3000}` | `portas_app` |
| porta do banco inválida | `DBPort:70000` | `porta_banco` |
| domínio com espaço | `Domain:"a b.com"` | `dominio` |

Assertion: `var ve *ValidationError; require.ErrorAs(t, err, &ve); assert.Equal(t, campo, ve.Field)`.

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/target/`
Expected: FAIL (`undefined: Target`)

- [ ] **Step 3: Implementar `Validate`**

Host válido = IP literal (`net.ParseIP`) ou hostname casando com
`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`; o mesmo regex vale para `Domain` quando não vazio. Nome: 1 a 64 caracteres após `strings.TrimSpace`. Portas: 1 a 65535; `DBPort` 0 significa "sem banco". Mensagens em português (ex.: `"porta fora do intervalo 1-65535"`).

- [ ] **Step 4: Rodar e ver passar**

Run: `go test ./internal/target/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/target
git commit -m "feat(api): adiciona modelo de alvo com validação estrita"
```

### Tarefa 1.3: Armazenamento SQLite

**Files:**
- Create: `internal/store/store.go`, `internal/store/targets.go`, `internal/store/migrations/001_targets.sql`
- Test: `internal/store/store_test.go`, `internal/store/targets_test.go`

**Interfaces:**
- Consumes: `target.Target` (Tarefa 1.2)
- Produces:
  - `func Open(ctx context.Context, path string) (*Store, error)` — aplica PRAGMAs e migrações pendentes
  - `func (s *Store) Close() error`
  - `func (s *Store) CreateTarget(ctx, t target.Target) (target.Target, error)` — devolve com `ID`
  - `func (s *Store) ListTargets(ctx) ([]target.Target, error)` — ordenado por `ID`
  - `func (s *Store) GetTarget(ctx, id int64) (target.Target, error)`
  - `func (s *Store) UpdateTarget(ctx, t target.Target) error`
  - `func (s *Store) DeleteTarget(ctx, id int64) error`
  - `var ErrNotFound error` — devolvido por Get, Update e Delete de ID inexistente

Esquema `001_targets.sql`: tabela `targets(id INTEGER PRIMARY KEY, name TEXT NOT NULL, host TEXT NOT NULL, app_ports TEXT NOT NULL, db_port INTEGER NOT NULL DEFAULT 0, domain TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`; `app_ports` guardado como JSON.

- [ ] **Step 1: Escrever os testes que falham**

- `TestOpenEnablesWAL`: abre em `filepath.Join(t.TempDir(), "omni.db")`; `PRAGMA journal_mode` retorna `"wal"`.
- `TestOpenIsIdempotent`: abre, cria um alvo, fecha, abre de novo; `ListTargets` tem 1 item e `PRAGMA user_version` = 1.
- `TestTargetCRUDRoundTrip`: cria → `GetTarget` igual ao criado (incluindo `AppPorts`) → atualiza `Name` → `GetTarget` reflete → `DeleteTarget` → `GetTarget` dá `ErrNotFound`.
- `TestMissingTargetErrors`: `GetTarget`, `UpdateTarget` e `DeleteTarget` com ID 999 retornam `ErrNotFound` (`assert.ErrorIs`).

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/store/`
Expected: FAIL (`undefined: Open`)

- [ ] **Step 3: Implementar `Open` e o repositório de alvos**

DSN `file:<path>?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)`. Migrações embutidas com `//go:embed migrations/*.sql`, aplicadas em ordem numérica dentro de uma transação cada, cada uma seguida de `PRAGMA user_version = N`.

- [ ] **Step 4: Rodar e ver passar**

Run: `go test -race ./internal/store/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store go.mod go.sum
git commit -m "feat(db): adiciona armazenamento SQLite com migrações e CRUD de alvos"
```

### Tarefa 1.4: API REST autenticada

**Files:**
- Create: `internal/api/server.go`, `internal/api/auth.go`, `internal/api/targets.go`, `internal/api/respond.go`
- Test: `internal/api/server_test.go`

**Interfaces:**
- Consumes: métodos do `*store.Store` e `store.ErrNotFound` (Tarefa 1.3), `target.Validate` e `*target.ValidationError` (Tarefa 1.2)
- Produces:
  - `type TargetStore interface { CreateTarget; ListTargets; GetTarget; UpdateTarget; DeleteTarget }` (mesmas assinaturas da Tarefa 1.3)
  - `func NewServer(ts TargetStore, token string) *Server`; `*Server` implementa `http.Handler`
  - `func (s *Server) Handle(pattern string, h http.Handler)` — registra rotas extras **já atrás da autenticação** (usado pelas sprints seguintes)

Rotas (padrões do `http.ServeMux` do Go 1.22+):

| Método e rota | Sucesso | Erros |
|---|---|---|
| `GET /api/saude` (sem auth) | 200 `{"status":"ok"}` | — |
| `GET /api/alvos` | 200 lista (vazia = `[]`) | 401 |
| `POST /api/alvos` | 201 alvo com `id` | 400 `{"erro","campo"}`, 401, 413 |
| `GET /api/alvos/{id}` | 200 | 400 id inválido, 401, 404 |
| `PUT /api/alvos/{id}` | 200 | 400, 401, 404 |
| `DELETE /api/alvos/{id}` | 204 | 401, 404 |

Corpo limitado a 1 MiB (`http.MaxBytesReader`, 413); JSON com `DisallowUnknownFields` (400). Respostas sempre `Content-Type: application/json`.

- [ ] **Step 1: Escrever os testes que falham** (store real em `t.TempDir()`, `httptest.NewRecorder`)

- `TestHealthNeedsNoToken`: 200 e corpo `{"status":"ok"}`.
- `TestAlvosRequireToken`: sem header → 401 `{"erro":"não autorizado"}`; `Bearer errado` → 401.
- `TestCreateTarget`: POST válido → 201 e `id > 0`; GET lista → 1 item.
- `TestCreateTargetRejectsShellHost`: host `"127.0.0.1; rm -rf /"` → 400 e `campo == "host"`.
- `TestCreateTargetRejectsUnknownField`: corpo com `"extra":1` → 400.
- `TestCreateTargetRejectsHugeBody`: corpo de 2 MiB → 413.
- `TestTargetNotFound`: GET `/api/alvos/999` → 404; GET `/api/alvos/abc` → 400.
- `TestDeleteTarget`: DELETE → 204; GET seguinte → 404.

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/api/`
Expected: FAIL (`undefined: NewServer`)

- [ ] **Step 3: Implementar servidor, middleware de autenticação e handlers**

- [ ] **Step 4: Rodar e ver passar**

Run: `go test -race ./internal/api/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api
git commit -m "feat(api): adiciona API REST autenticada de alvos"
```

### Tarefa 1.5: Binário, desligamento gracioso e CI

**Files:**
- Create: `cmd/omni/main.go`, `cmd/omni/run.go`
- Test: `cmd/omni/run_test.go`
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: `config.Load`, `store.Open`, `api.NewServer`
- Produces: `func run(ctx context.Context, cfg config.Config, ln net.Listener) error` — serve até `ctx` ser cancelado e retorna `nil` após desligamento gracioso (timeout de 10 s). `main` faz `signal.NotifyContext(SIGINT, SIGTERM)`, `config.Load(os.Getenv)`, `net.Listen("tcp", cfg.Addr)` e chama `run`; erro → mensagem em stderr e código de saída 1.

- [ ] **Step 1: Escrever o teste que falha**

`TestRunServesAndShutsDown`: `ln, _ := net.Listen("tcp", "127.0.0.1:0")`; roda `run` numa goroutine com `DBPath` em `t.TempDir()`; `GET http://<ln.Addr()>/api/saude` → 200; cancela o contexto; `run` retorna `nil` em até 2 s.

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./cmd/omni/`
Expected: FAIL (`undefined: run`)

- [ ] **Step 3: Implementar `run` e `main`**

`http.Server` com `ReadHeaderTimeout: 5 * time.Second`.

- [ ] **Step 4: Rodar e ver passar**

Run: `go test -race ./cmd/omni/`
Expected: PASS

- [ ] **Step 5: Criar o CI**

`.github/workflows/ci.yml`, disparado em `pull_request` e em `push` na `main`, job `go` em `ubuntu-latest`:
`actions/checkout@v4` → `actions/setup-go@v5` (`go-version-file: go.mod`) → `test -z "$(gofmt -l .)"` → `go vet ./...` → `golangci/golangci-lint-action@v8` (`version: v2.5.0`) → `go test -race ./...`.

- [ ] **Step 6: Verificação completa da sprint**

Run: `gofmt -l . && go vet ./... && golangci-lint run ./... && go build ./... && go test -race ./...`
Expected: nenhuma saída do `gofmt`, nenhum problema do linter, todos os testes PASS.
Teste manual: `OMNI_TOKEN=$(openssl rand -hex 16) go run ./cmd/omni` e `curl -s 127.0.0.1:8080/api/saude` → `{"status":"ok"}`.

- [ ] **Step 7: Commits e PR**

```bash
git add cmd/omni
git commit -m "feat(api): adiciona binário omni com desligamento gracioso"
git add .github/workflows/ci.yml
git commit -m "ci: adiciona verificação do backend Go"
```

PR `feat(api): adiciona núcleo do backend` com labels `funcionalidade` e `motor`.

## Sprint 2: WebSocket, Poller e métricas do sistema

Entrega: o backend testa periodicamente as portas de cada alvo, guarda o histórico, publica mudanças de estado e métricas do sistema em tempo real pelo WebSocket.

### Tarefa 2.1: Hub WebSocket com ticket de uso único

**Files:** Create `internal/ws/hub.go`, `internal/ws/ticket.go`, `internal/ws/handler.go`; Test `internal/ws/hub_test.go`, `internal/ws/handler_test.go`

**Interfaces:**
- Produces:
  - `type Event struct { Type string \`json:"tipo"\`; Data any \`json:"dados"\` }`
  - `type Publisher interface { Publish(Event) }` — usado por Poller, Sniffer e Orquestrador
  - `func NewHub(clientBuffer int) *Hub`; `(*Hub) Run(ctx context.Context)`; `(*Hub) Publish(Event)` (nunca bloqueia)
  - `type Tickets struct{...}`; `func NewTickets(ttl time.Duration) *Tickets`; `(*Tickets) Issue() string`; `(*Tickets) Consume(t string) bool`
  - `func Handler(h *Hub, t *Tickets, allowedOrigins []string) http.Handler` — rota `GET /ws?ticket=...`
  - Rota autenticada `POST /api/ws/ticket` → 200 `{"ticket":"..."}` (registrada via `api.Server.Handle`)

Cada cliente tem um canal com buffer `clientBuffer` (padrão 256). Buffer cheio: descarta a mensagem mais antiga daquele cliente. Ticket: 32 bytes aleatórios em hex, validade de 30 s, removido no primeiro uso. Origens permitidas por padrão: `http://127.0.0.1:8080` e `http://localhost:8080`, mais `http://localhost:5173` em desenvolvimento (`OMNI_DEV=1`).

**Testes:**
- `TestSlowClientDoesNotBlockOthers`: buffer 4; cliente A conecta e nunca lê; cliente B lê; publicar 100 eventos; B recebe o evento 100 em até 1 s.
- `TestRejectsForeignOrigin`: `Origin: https://evil.example` → 403.
- `TestRejectsMissingOrExpiredTicket`: sem ticket → 401; ticket expirado (TTL 10 ms) → 401.
- `TestTicketIsSingleUse`: segundo `Consume` do mesmo ticket → `false`.

### Tarefa 2.2: Verificações do Poller

**Files:** Create `internal/poller/checks.go`; Test `internal/poller/checks_test.go`

**Interfaces:**
- Produces:
  - `type Result struct { TargetID int64; Port int; Kind string /* "http" | "tcp" */; Up bool; StatusCode int; Latency time.Duration; Err string; At time.Time }`
  - `func CheckHTTP(ctx context.Context, c *http.Client, targetID int64, url string) Result` — `Up` = respondeu (qualquer status); `User-Agent: Omni-Poller/1`
  - `func CheckTCP(ctx context.Context, targetID int64, addr string, timeout time.Duration) Result`

**Testes:** `httptest` com 200 → `Up` e `StatusCode` 200; com 500 → `Up` e 500; servidor fechado → `Up == false` e `Err` não vazio; `CheckTCP` em porta aberta e em porta fechada; timeout de 50 ms contra servidor que dorme 1 s → `Up == false` em menos de 500 ms.

### Tarefa 2.3: Agendador, histórico e eventos de transição

**Files:** Create `internal/poller/scheduler.go`, `internal/store/checks.go`, `internal/store/migrations/002_checks.sql`; Test `internal/poller/scheduler_test.go`, `internal/store/checks_test.go`

**Interfaces:**
- Consumes: `ws.Publisher`, `store.ListTargets`, `CheckHTTP`, `CheckTCP`
- Produces:
  - `func (s *Store) InsertCheckResult(ctx, r poller.Result) error` e `func (s *Store) CheckHistory(ctx, targetID int64, since time.Time) ([]poller.Result, error)` — o `store` importa `poller`; o `poller` nunca importa o `store`
  - `type ResultStore interface { ListTargets(ctx) ([]target.Target, error); InsertCheckResult(ctx, poller.Result) error }`, definida em `internal/poller`
  - `type Scheduler struct { Store ResultStore; Pub ws.Publisher; Interval time.Duration; Client *http.Client }`; `(*Scheduler) RunOnce(ctx) error`; `(*Scheduler) Run(ctx)` (padrão 30 s)
  - Eventos: `ALERTA_QUEDA` e `SERVICO_RESTAURADO` com `dados: {alvo_id, porta, hora}`; `STATUS_ALVOS` com o último resultado de cada porta a cada rodada
  - Rota `GET /api/alvos/{id}/historico?desde=<RFC3339>`

Portas de aplicação usam `CheckHTTP` (`http://<host>:<porta>/`); a porta do banco usa `CheckTCP`.

**Testes:** `TestRunOnceEmitsDownOnlyOnTransition`: publisher falso; rodada 1 servidor no ar → nenhum `ALERTA_QUEDA`; servidor fechado, rodada 2 → exatamente um `ALERTA_QUEDA`; rodada 3 ainda fechado → nenhum novo; servidor de volta, rodada 4 → um `SERVICO_RESTAURADO`. `TestCheckHistoryFiltersBySince`.

### Tarefa 2.4: Métricas do sistema

**Files:** Create `internal/sysmetrics/sysmetrics.go`; Test `internal/sysmetrics/sysmetrics_test.go`; Modify `cmd/omni/run.go`

**Interfaces:**
- Produces: `type Snapshot struct { CPUPercent, MemPercent, DiskPercent float64; At time.Time }` (JSON `cpu`, `memoria`, `disco`, `hora`); `func Sample(ctx context.Context, diskPath string) (Snapshot, error)` com `gopsutil/v4`; evento `METRICAS_SISTEMA` a cada 5 s.

**Testes:** `TestSampleInRange`: os três percentuais entre 0 e 100. `TestRunWiresHubAndPoller` em `cmd/omni`: com um alvo apontando para um `httptest.Server` e intervalo de 50 ms, um cliente WebSocket (ticket obtido via API) recebe `STATUS_ALVOS`.

PR `feat(poller): adiciona poller, websocket e métricas do sistema`, labels `funcionalidade` e `motor`.

## Sprint 3: Frontend base embutido

Entrega: abrir `http://127.0.0.1:8080` mostra a interface: tela de token, barra superior com seletor de alvo e estado dos motores, barra lateral com as seções, página de Alvos funcionando e alertas em tempo real.

### Tarefa 3.1: Projeto Vite e CI do frontend

**Files:** Create `web/` (Vite `react-ts`), `web/eslint.config.js`, `web/vitest.config.ts`, `web/src/test/setup.ts`; Modify `.github/workflows/ci.yml`

- Scripts em `web/package.json`: `dev`, `build` (`tsc -b && vite build`), `lint` (`eslint .`), `typecheck` (`tsc -b`), `test` (`vitest run`).
- Tailwind CSS v4 (`@tailwindcss/vite`), tema escuro como padrão.
- `vite.config.ts`: proxy de `/api` e `/ws` (com `ws: true`) para `http://127.0.0.1:8080`.
- CI: job `web` com `actions/setup-node@v4` (Node 22), `npm ci`, `npm run lint`, `npm run typecheck`, `npm test`, `npm run build`.

**Teste:** `App.test.tsx` → renderiza sem erro.

### Tarefa 3.2: Layout, token e estado global

**Files:** Create `web/src/store/app.ts`, `web/src/components/Topbar.tsx`, `web/src/components/Sidebar.tsx`, `web/src/pages/TokenPage.tsx`, `web/src/router.tsx`; Test `web/src/components/Sidebar.test.tsx`, `web/src/pages/TokenPage.test.tsx`

**Interfaces:**
- Produces: `useAppStore` (Zustand) com `token: string | null`, `setToken`, `selectedTargetId: number | null`, `selectTarget`, `engines: { poller: EngineState; sniffer: EngineState; docker: EngineState }`, onde `type EngineState = "ativo" | "inativo" | "sem-permissao" | "indisponivel"`.
- Token salvo em `sessionStorage` (com try/catch), nunca em `localStorage`.
- Barra lateral com as seções do README: **Monitoramento** (Geral, Requisições na API, Consultas no banco, Status HTTP, Sistema), **Tráfego** (Inspetor, Capturas), **Arsenal**, **Relatórios**. Itens de sprints futuras aparecem desabilitados com o texto "em breve".

**Testes:** sidebar mostra os quatro títulos de seção; sem token, a aplicação mostra a `TokenPage`; após salvar o token, mostra o layout.

### Tarefa 3.3: Cliente da API e página de Alvos

**Files:** Create `web/src/api/client.ts`, `web/src/api/targets.ts`, `web/src/pages/TargetsPage.tsx`, `web/src/components/TargetForm.tsx`; Test `web/src/api/client.test.ts`, `web/src/pages/TargetsPage.test.tsx`

**Interfaces:**
- Produces:
  - `class ApiError extends Error { status: number; field?: string }`
  - `function apiFetch<T>(path: string, init?: RequestInit): Promise<T>` — adiciona `Authorization: Bearer`; resposta 401 limpa o token (volta à `TokenPage`); erro com corpo `{erro, campo}` vira `ApiError`
  - `type Target = { id: number; nome: string; host: string; portas_app: number[]; porta_banco?: number; dominio?: string }`
  - `listTargets()`, `createTarget(t)`, `updateTarget(t)`, `deleteTarget(id)`
- Tabela com TanStack Table e formulário de criação e edição; exclusão pede confirmação.

**Testes** (com `fetch` simulado por `vi.fn`): `apiFetch` envia o header de autenticação; 401 limpa o token; formulário mostra o erro do servidor embaixo do campo `host` quando a API responde `{"erro":"host inválido","campo":"host"}`.

### Tarefa 3.4: Conexão em tempo real

**Files:** Create `web/src/realtime/useOmniSocket.ts`, `web/src/components/Toasts.tsx`; Test `web/src/realtime/useOmniSocket.test.ts`

**Interfaces:**
- Consumes: `POST /api/ws/ticket`, eventos `STATUS_ALVOS`, `ALERTA_QUEDA`, `SERVICO_RESTAURADO` e `METRICAS_SISTEMA` (Sprint 2)
- Produces: `useOmniSocket()` — pede um ticket novo a cada (re)conexão, conecta com `react-use-websocket` (reconexão automática com backoff) e despacha eventos para `useAppStore` (`targetStatus`, `system`, `pushToast`).

**Testes:** com um socket simulado, `ALERTA_QUEDA` gera um toast com o texto "Serviço fora do ar"; cada reconexão pede um ticket novo.

### Tarefa 3.5: Frontend embutido no binário

**Files:** Create `internal/web/embed.go`, `internal/web/static/placeholder.html`, `Makefile`; Test `internal/web/embed_test.go`; Modify `cmd/omni/run.go`, `.gitignore`

**Interfaces:**
- Produces: `func Handler() http.Handler` — serve `internal/web/static` (`//go:embed all:static`); caminho sem arquivo correspondente e fora de `/api` e `/ws` devolve `index.html` (rotas do React); sem `index.html` compilado, serve `placeholder.html` com a instrução `make build`.
- `Makefile`: `build` (roda `npm ci && npm run build` em `web/`, copia `web/dist/*` para `internal/web/static/`, roda `go build -o omni ./cmd/omni`) e `check` (verificações do Go e do frontend da skill `convencoes-omni`). `.gitignore` ignora `internal/web/static/*` exceto `placeholder.html`.

**Testes:** `GET /` → 200 com HTML; `GET /alvos` → mesmo HTML (fallback); `GET /api/saude` continua respondendo JSON.

PR `feat(frontend): adiciona interface base embutida no binário`, labels `funcionalidade` e `visual`.

---

# Fase 2: Observação passiva

## Sprint 4: Sniffer nas camadas 3 e 4

Entrega: o Omni captura o tráfego das portas dos alvos e a tela Geral mostra consumo de banda, distribuição de protocolos e top IPs de origem, em tempo real.

### Tarefa 4.1: Fontes de pacotes

**Files:** Create `internal/sniffer/source.go`, `internal/sniffer/source_file.go`, `internal/sniffer/source_live.go` (único arquivo com cgo), `internal/sniffer/testutil/pcapbuild.go`; Test `internal/sniffer/source_test.go`

**Interfaces:**
- Produces:
  - `type Source interface { Packets(ctx context.Context) <-chan gopacket.Packet; Stats() CaptureStats; Close() error }`
  - `type CaptureStats struct { Received, Dropped uint64 }` — `Dropped` vem do pcap (aviso de métricas incompletas)
  - `func OpenFile(path string) (Source, error)` (`pcapgo`)
  - `func OpenLive(iface string, bpf string) (Source, error)` — snaplen 1600; falha de permissão envolve `ErrSemPermissao`
  - `func BPFForTargets(ts []target.Target) string` — ex.: `tcp port 3000 or tcp port 5432`
  - `testutil.WritePCAP(t, path string, pkts ...[]byte)` e construtores `testutil.TCP(src, dst string, sport, dport uint16, flags string, payload []byte) []byte`, `testutil.UDP(...)` com `gopacket.SerializeLayers`; suportam os enlaces Ethernet e `LINUX_SLL`

**Testes:** `OpenFile` lê N pacotes gravados por `WritePCAP`; `BPFForTargets` para dois alvos; pacote com enlace `LINUX_SLL` é decodificado até a camada TCP.

### Tarefa 4.2: Agregador de tráfego

**Files:** Create `internal/sniffer/aggregate.go`, `internal/sniffer/classify.go`; Test `internal/sniffer/aggregate_test.go`

**Interfaces:**
- Produces:
  - `func Classify(srcPort, dstPort uint16, isUDP bool, appPorts []int) string` — `"HTTP"` (portas de aplicação ou 80), `"HTTPS"` (443), `"DNS"` (53), `"PostgreSQL"` (5432), `"MySQL"` (3306), `"SMTP"` (25, 465, 587), senão `"TCP"` ou `"UDP"`
  - `type Window struct { Start time.Time; Bytes, Packets uint64; ByProtocol map[string]uint64; TopSources []SourceCount /* até 10, por bytes */ }`
  - `type Aggregator struct{...}`; `func NewAggregator(appPorts []int) *Aggregator`; `(*Aggregator) Add(p gopacket.Packet)`; `(*Aggregator) Flush(now time.Time) Window` (zera a janela)

**Testes:** tabela de `Classify`; após adicionar pacotes conhecidos, `Flush` devolve bytes, contagem por protocolo e top IPs corretos, e a janela seguinte começa zerada; IPv6 é contado igual a IPv4.

### Tarefa 4.3: Motor do sniffer, persistência e degradação graciosa

**Files:** Create `internal/sniffer/engine.go`, `internal/store/traffic.go`, `internal/store/migrations/003_traffic.sql`; Test `internal/sniffer/engine_test.go`; Modify `cmd/omni/run.go`

**Interfaces:**
- Consumes: `Source`, `Aggregator`, `ws.Publisher`, store
- Produces:
  - `type Engine struct{...}`; `func NewEngine(open func() (Source, error), st TrafficStore, pub ws.Publisher) *Engine`; `(*Engine) Run(ctx) error`; `(*Engine) State() string` (`"ativo"`, `"sem-permissao"`, `"inativo"`)
  - A cada 1 s: grava a janela (`traffic_windows`) e publica `METRICAS_TRAFEGO`; a cada 5 s publica `ESTADO_MOTORES`
  - Rota `GET /api/trafego?desde=<RFC3339>`
  - Variável `OMNI_INTERFACE` (padrão `any`)

**Testes:** com fonte de arquivo, `Run` grava janelas e publica `METRICAS_TRAFEGO`; `TestSnifferPermissionDeniedKeepsRunning`: `open` retorna `ErrSemPermissao` → `State() == "sem-permissao"`, `ESTADO_MOTORES` publicado, e em `cmd/omni` a API continua respondendo `GET /api/saude`.

### Tarefa 4.4: Tela Geral com banda e protocolos

**Files:** Create `web/src/pages/OverviewPage.tsx`, `web/src/charts/BandwidthChart.tsx`, `web/src/charts/ProtocolDonut.tsx`, `web/src/components/EngineBadge.tsx`; Test `web/src/pages/OverviewPage.test.tsx`

- Gráfico de área de banda (MB/s, últimos 15 minutos: histórico de `GET /api/trafego` + eventos ao vivo, janela deslizante de no máximo 900 pontos).
- Gráfico de rosca de protocolos e tabela de top IPs de origem.
- Barra superior: estados vindos de `ESTADO_MOTORES`; com `sem-permissao`, a tela Geral mostra o aviso "Captura desativada: o Omni está sem permissão de captura (CAP_NET_RAW)".

**Testes:** com eventos simulados, os gráficos recebem os pontos; a janela não passa de 900 pontos; o aviso de permissão aparece no estado `sem-permissao`.

PR `feat(sniffer): adiciona captura e métricas das camadas 3 e 4`, labels `funcionalidade`, `motor` e `visual`.

## Sprint 5: Decodificação HTTP/1.1

Entrega: gráficos de requisições por rota e de status HTTP com dados reais da aplicação, mais a tela de detalhe com latência por rota.

### Tarefa 5.1: Remontagem TCP e pareamento requisição-resposta

**Files:** Create `internal/sniffer/l7/reassembly.go`, `internal/sniffer/l7/http.go`; Test `internal/sniffer/l7/http_test.go` (+ fixtures geradas com `testutil`)

**Interfaces:**
- Produces:
  - `type HTTPExchange struct { Method, Path, Route string; Status int; Latency time.Duration; ClientIP string; Internal bool; At time.Time }`
  - `func NewHTTPAssembler(appPorts []int, out chan<- HTTPExchange) *HTTPAssembler`; `(*HTTPAssembler) Add(p gopacket.Packet)`; `(*HTTPAssembler) FlushOlderThan(t time.Time)`
  - Usa `gopacket/reassembly`; limite de 4 MiB em buffer por fluxo; fluxos sem tráfego há 2 min são descartados

**Testes:** requisição e resposta simples → um `HTTPExchange` com status e latência; duas requisições em pipeline na mesma conexão → pareadas em ordem; resposta dividida em três segmentos; segmentos fora de ordem; tráfego TLS na porta 443 → nenhum exchange e nenhum erro.

### Tarefa 5.2: Normalização de rotas, mascaramento e tráfego interno

**Files:** Create `internal/sniffer/l7/route.go`; Test `internal/sniffer/l7/route_test.go`; Modify `internal/sniffer/l7/http.go`

**Interfaces:**
- Produces: `func NormalizeRoute(path string) string`

| Entrada | Saída |
|---|---|
| `/users/123` | `/users/:id` |
| `/users/123/orders/9` | `/users/:id/orders/:id` |
| `/files/550e8400-e29b-41d4-a716-446655440000` | `/files/:id` |
| `/tx/9f86d081884c7d65` (hex com 16+ caracteres) | `/tx/:id` |
| `/busca?q=senha` | `/busca` |
| `/` | `/` |
| `/v1/status` | `/v1/status` |

- `HTTPExchange` não tem campo de cabeçalhos nem de corpo: o assembler só extrai método, caminho, status e `User-Agent`.
- `Internal = true` quando o `User-Agent` começa com `Omni-` (Poller e Arsenal); esses exchanges não entram nas métricas.

**Testes:** tabela acima; `TestNoSecretsLeak`: requisição com `Authorization: Bearer segredo123` e `Cookie: sessao=abc` → a string serializada do exchange e dos eventos publicados não contém `segredo123` nem `abc`; requisição com `User-Agent: Omni-Poller/1` → `Internal == true`.

### Tarefa 5.3: Métricas HTTP, API e telas

**Files:** Create `internal/sniffer/l7/httpstats.go`, `internal/store/http.go`, `internal/store/migrations/004_http.sql`, `web/src/charts/RequestsChart.tsx`, `web/src/charts/StatusBars.tsx`, `web/src/pages/RequestsPage.tsx`; Tests correspondentes

- Agregação por janela de 10 s: contagem por rota, classes de status (`2xx`, `3xx`, `4xx`, `5xx`), latência p50 e p95 por rota.
- Evento `METRICAS_HTTP`; rotas `GET /api/requisicoes?desde=` e `GET /api/requisicoes/rotas?desde=` (ranking por volume e por p95).
- Tela Geral ganha o gráfico de requisições e as barras empilhadas de status; tela "Requisições na API" com o gráfico e a tabela das rotas mais lentas.

**Testes:** p50 e p95 calculados corretamente para latências conhecidas; exchanges `Internal` não contam; tabela da tela ordena por p95.

PR `feat(sniffer): adiciona decodificação HTTP e métricas de requisições`, labels `funcionalidade`, `motor` e `visual`.

## Sprint 6: Decodificação PostgreSQL

Entrega: gráfico de consultas ao banco e tabela das consultas mais lentas.

### Tarefa 6.1: Decodificador do protocolo PostgreSQL

**Files:** Create `internal/sniffer/l7/postgres.go`, `internal/sniffer/l7/sqlnorm.go`; Test `internal/sniffer/l7/postgres_test.go`, `internal/sniffer/l7/sqlnorm_test.go`

**Interfaces:**
- Produces:
  - `type DBQuery struct { Query string /* normalizada */; Kind string /* "SELECT", "INSERT"... */; Latency time.Duration; Error bool; At time.Time }`
  - `func NewPostgresAssembler(dbPort int, out chan<- DBQuery) *PostgresAssembler` (mesma mecânica de remontagem da Tarefa 5.1)
  - `func NormalizeSQL(q string) string` — literais de texto e números viram `?`, espaços colapsados

Mensagens tratadas: do cliente, `StartupMessage` e `SSLRequest` (sem tipo), `Q` (Query), `P` (Parse), `B`/`E` (Bind/Execute); do servidor, `C` (CommandComplete), `E` (ErrorResponse) e `Z` (ReadyForQuery, que fecha o pareamento). `SSLRequest` respondido com `S` marca a conexão como criptografada e ela é ignorada.

**Testes:** consulta simples `Q` → um `DBQuery` com latência; Parse/Bind/Execute → a consulta do `Parse`; `ErrorResponse` → `Error == true`; conexão com SSL aceito → nada emitido; `NormalizeSQL("SELECT * FROM u WHERE id = 42 AND nome = 'ana'")` → `"SELECT * FROM u WHERE id = ? AND nome = ?"`; a senha da mensagem de autenticação nunca aparece em nenhum `DBQuery`.

### Tarefa 6.2: Métricas de banco, API e telas

**Files:** Create `internal/store/db_queries.go`, `internal/store/migrations/005_db.sql`, `web/src/charts/QueriesChart.tsx`, `web/src/pages/QueriesPage.tsx`; Tests correspondentes

- Evento `METRICAS_BANCO`; rotas `GET /api/consultas?desde=` e `GET /api/consultas/lentas?desde=&limite=` (padrão 20).
- Tela Geral ganha o gráfico de consultas; tela "Consultas no banco" com o gráfico e a tabela das mais lentas das últimas 24 h.

**Testes:** ranking das lentas ordena por p95; tela mostra a consulta normalizada (nunca valores literais).

PR `feat(sniffer): adiciona decodificação PostgreSQL e métricas de consultas`, labels `funcionalidade`, `motor` e `visual`.

---

# Fase 3: Arsenal

## Sprint 7: Orquestrador Docker e Nmap

Entrega: a tela Arsenal dispara uma varredura Nmap num container efêmero, mostra a saída ao vivo e lista as portas e serviços encontrados.

### Tarefa 7.1: Varreduras e achados no banco

**Files:** Create `internal/findings/finding.go`, `internal/store/scans.go`, `internal/store/migrations/006_scans.sql`; Tests correspondentes

**Interfaces:**
- Produces:
  - `type Severity string` com `"info"`, `"baixa"`, `"media"`, `"alta"`, `"critica"`
  - `type Finding struct { Tool, Title, Evidence string; Severity Severity; Port int; Fingerprint string }` — `Fingerprint` = sha256 de `Tool|Title|Port`
  - `type Scan struct { ID, TargetID int64; Tools []string; Status string /* "fila", "rodando", "concluida", "falhou" */; StartedAt, FinishedAt time.Time; Error string }`
  - Store: `CreateScan`, `UpdateScanStatus`, `AddFindings(ctx, scanID, []Finding)`, `ListScans(ctx, targetID)`, `ScanFindings(ctx, scanID)`

**Testes:** ida e volta de scan e achados; achados com o mesmo `Fingerprint` no mesmo scan são gravados uma vez.

### Tarefa 7.2: Escopo e argumentos seguros

**Files:** Create `internal/arsenal/scope.go`, `internal/arsenal/nmap.go`; Test `internal/arsenal/scope_test.go`, `internal/arsenal/nmap_test.go`, `internal/arsenal/testdata/nmap.xml`

**Interfaces:**
- Produces:
  - `func InScope(t target.Target, host string) bool` — loopback, o `Host` do alvo, o `Domain` do alvo e seus subdomínios
  - `func NmapArgs(t target.Target) ([]string, error)` → `[]string{"-sV", "-Pn", "-oX", "-", "-p", "<portas do alvo separadas por vírgula>", "--", t.Host}`; chama `t.Validate()` antes e recusa host fora do escopo
  - `func ParseNmapXML(r io.Reader) ([]findings.Finding, error)` — uma `Finding` por porta aberta (`Title` = `"Porta 3000/tcp aberta: http (nginx 1.25)"`, severidade `info`)

**Testes:** `NmapArgs` para um alvo válido devolve exatamente a lista acima; alvo com host `-oX` ou `127.0.0.1; rm -rf /` → erro e nenhum argumento; `InScope("evil.com")` para alvo com domínio `app.com` → `false`, `api.app.com` → `true`; `ParseNmapXML` sobre o fixture devolve as portas abertas esperadas.

### Tarefa 7.3: Orquestrador Docker

**Files:** Create `internal/orchestrator/orchestrator.go`, `internal/orchestrator/docker.go`; Test `internal/orchestrator/orchestrator_test.go` (com `Runner` falso)

**Interfaces:**
- Produces:
  - `type Spec struct { Image string; Args []string }`
  - `type Runner interface { Run(ctx context.Context, s Spec, logs io.Writer) (stdout []byte, err error); Available(ctx context.Context) error }`
  - `func NewDockerRunner() (*DockerRunner, error)` — Docker SDK; `HostConfig{NetworkMode: "host", Resources: {NanoCPUs: 1e9, Memory: 512 << 20}}`; o container sempre é removido no fim, inclusive em erro ou cancelamento; timeout de 10 min
  - `var ErrDockerIndisponivel error`
  - `type Orchestrator struct{...}`; `(*Orchestrator) Start(ctx, targetID int64, tools []string) (scanID int64, err error)` — roda em segundo plano, publica `SCAN_LOG` (linhas em lote a cada 500 ms), `SCAN_CONCLUIDO` (`{scan_id, ferramenta, achados}`) ou `SCAN_FALHOU`
  - Imagem do Nmap fixada na constante `NmapImage` (tag exata definida ao expandir a sprint, preferindo imagem mantida e construída a partir do Alpine)

**Testes:** com `Runner` falso, `Start` grava o scan, salva os achados do parser e publica `SCAN_CONCLUIDO`; `Runner.Run` com erro → status `"falhou"` e `SCAN_FALHOU`; `Available` retornando `ErrDockerIndisponivel` → `Start` retorna o erro sem criar o scan. Teste de integração `DockerRunner` com build tag `integration` (fora do CI padrão).

### Tarefa 7.4: API de varreduras e tela Arsenal

**Files:** Create `internal/api/scans.go`, `web/src/pages/ArsenalPage.tsx`, `web/src/components/ScanConsole.tsx`; Tests correspondentes

- `POST /api/varreduras` `{"alvo_id": 1, "ferramentas": ["nmap"]}` → 202 `{"id": ...}`; ferramenta desconhecida → 400 `campo: "ferramentas"`; Docker indisponível → 503 `{"erro":"Docker indisponível"}`.
- `GET /api/varreduras?alvo_id=`, `GET /api/varreduras/{id}/achados`.
- Tela Arsenal: seletor de alvo, checkboxes das ferramentas (só Nmap habilitado nesta sprint), botão "Iniciar varredura", console escuro com a saída ao vivo e tabela de achados; toast em `SCAN_CONCLUIDO` com link para os achados. Rótulo fixo "Visão interna" nos resultados.

**Testes:** `TestScanDockerUnavailable` → 503 e a API continua respondendo `GET /api/saude`; ferramenta desconhecida → 400; console do frontend acrescenta linhas de `SCAN_LOG` e limita a 5000 linhas.

PR `feat(orquestrador): adiciona orquestrador docker e varredura nmap`, labels `funcionalidade`, `docker` e `visual`.

## Sprint 8: Nuclei, testssl e comparação entre scans

Entrega: mais duas ferramentas no Arsenal e a tela de Relatórios mostrando o que mudou entre a última varredura e a anterior.

### Tarefa 8.1: Parsers do Nuclei e do testssl

**Files:** Create `internal/arsenal/nuclei.go`, `internal/arsenal/testssl.go`, fixtures em `internal/arsenal/testdata/`; Tests correspondentes

**Interfaces:**
- Produces: `NucleiArgs(t) ([]string, error)` (saída JSONL, `-silent`, alvos `http://<host>:<porta>`), `ParseNucleiJSONL(r) ([]findings.Finding, error)`; `TestsslArgs(t) ([]string, error)` (saída JSON em stdout, porta 443 ou a primeira porta de aplicação), `ParseTestsslJSON(r) ([]findings.Finding, error)`; mapeamento de severidade de cada ferramenta para `findings.Severity` (`critical` → `critica`, `high` → `alta`, `medium` → `media`, `low` → `baixa`, demais → `info`).

**Testes:** fixtures reais de cada ferramenta → achados e severidades esperados; linha JSONL inválida no meio do arquivo é ignorada e as demais são lidas.

### Tarefa 8.2: Comparação entre scans

**Files:** Create `internal/findings/diff.go`; Test `internal/findings/diff_test.go`; Modify `internal/orchestrator/orchestrator.go`, `internal/api/scans.go`

**Interfaces:**
- Produces: `func Diff(prev, curr []Finding) (novos, resolvidos []Finding)` por `Fingerprint`; `SCAN_CONCLUIDO` ganha `novos_desde_ultimo`; rota `GET /api/varreduras/{id}/comparacao` → `{novos, resolvidos}` contra o scan concluído anterior da mesma ferramenta e alvo.

**Testes:** porta nova aparece em `novos`; porta fechada aparece em `resolvidos`; primeiro scan do alvo → todos os achados em `novos`.

### Tarefa 8.3: Tela de Relatórios

**Files:** Create `web/src/pages/ReportsPage.tsx`, `web/src/components/FindingsDiff.tsx`; Tests correspondentes

- Lista de scans por alvo (ferramenta, data, status, quantidade por severidade) e, ao abrir um scan, a comparação com o anterior: novos em destaque e resolvidos riscados.
- Checkboxes do Nuclei e do testssl habilitados na tela Arsenal.

**Testes:** comparação renderiza novos e resolvidos; severidade `critica` aparece primeiro.

PR `feat(orquestrador): adiciona nuclei, testssl e comparação entre scans`, labels `funcionalidade`, `docker` e `visual`.

---

# Fase 4: Expansão (fora do escopo básico)

Cada item abaixo vira uma sprint própria, planejada com `/superpowers:writing-plans` quando entrar na fila. Itens do checklist "O que falta implementar, por camada" do README que não aparecem nas sprints 1 a 8:

- **Camada 7:** HTTP/2, MySQL (`COM_QUERY`), `pg_stat_statements`, Poller de DNS e SMTP (STARTTLS, registros MX/TXT), Nikto, SQLMap, ffuf, Amass, Gitleaks, agendamento de varreduras, alertas externos (Telegram, Discord, e-mail), relatório em PDF.
- **Camada 6:** metadados do handshake TLS (SNI, versão, cifra).
- **Camada 4:** RTT por SYN/SYN-ACK, retransmissões, RST e SYN sem resposta, janela zero, conexões por estado via `/proc/net/tcp`, amplificação UDP.
- **Camada 3:** ICMP, fragmentação, GeoIP.
- **Camada 2:** detecção de ARP spoofing, interfaces `docker0`/`veth*`.
- **Camada 1 e sistema:** contadores de `/sys/class/net`, AF_PACKET com ring buffer, Inspetor de Tráfego pacote a pacote com pausa, retenção configurável e rotação dos `.pcap`.
