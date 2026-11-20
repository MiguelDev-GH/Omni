# Sprint 1: Núcleo do Backend, Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Entregar o binário `omni` que sobe em `127.0.0.1:8080`, responde `GET /api/saude` e oferece o CRUD autenticado de alvos persistido no SQLite, com CI rodando as verificações do Go.

**Architecture:** Módulo Go único na raiz. Quatro pacotes com dependência em uma só direção: `config` e `target` não dependem de nada do projeto; `store` depende de `target`; `api` depende de `target` e de uma interface própria (`TargetStore`), satisfeita por `*store.Store`; `cmd/omni` liga tudo. Sem frontend nesta sprint.

**Tech Stack:** Go 1.24, `net/http` (`ServeMux` com padrões do Go 1.22+), `modernc.org/sqlite` (sem cgo), `github.com/stretchr/testify`, `log/slog`, golangci-lint v2.5.0, GitHub Actions.

**Spec:** `docs/superpowers/plans/2026-11-18-implementacao-basica.md` (seção "Sprint 1: Núcleo do backend" e "Global Constraints") e `README.md` (seção "Segurança da própria plataforma"). Convenções: `.claude/skills/convencoes-omni/SKILL.md`. Este documento expande a Sprint 1 e decide o que o plano básico deixou em aberto; onde os dois divergirem, vale este.

## Global Constraints

- Módulo `github.com/MiguelDev-GH/omni`, linha `go 1.24` no `go.mod`. Se uma dependência exigir versão maior, use a versão mais nova da dependência que ainda aceite 1.24 (`go get pacote@vX.Y.Z`); só suba a linha `go` se não houver nenhuma, e registre isso na descrição do PR.
- Estrutura: `cmd/omni/` (binário), `internal/<pacote>/` (backend).
- Identificadores em inglês; mensagens de erro da API e textos para o usuário em português. Toda resposta de erro da API tem a forma `{"erro": "<mensagem>"}`, mais `"campo"` quando o erro é de validação.
- SQLite via `modernc.org/sqlite` (driver `"sqlite"`), modo WAL, `busy_timeout` 5000 ms, `foreign_keys` ligado, `db.SetMaxOpenConns(1)`. Migrações em `internal/store/migrations/NNN_nome.sql`, controladas por `PRAGMA user_version`.
- Endereço padrão `127.0.0.1:8080`. Endereço fora de loopback é recusado, a não ser com `OMNI_PERMITIR_REMOTO=1`.
- `OMNI_TOKEN` obrigatório, com no mínimo 32 caracteres. Toda rota `/api/*` exige `Authorization: Bearer <token>`, exceto `GET /api/saude`. Comparação com `crypto/subtle.ConstantTimeCompare`.
- Todos os testes Go usam `testify` (`assert`/`require`).
- Branch `feat/nucleo-backend`; commits e PR seguem a skill `convencoes-omni` (cabeçalho em português, presente do indicativo, minúsculo, sem ponto final, até 72 caracteres). PR com labels `funcionalidade` e `motor`.
- Antes de cada commit, rode a verificação do backend da skill `convencoes-omni` sobre o estado final do código e relate o resultado real.

## Review Focus

1. **`PUT /api/alvos/{id}` com um `id` diferente no corpo:** o `id` do caminho vale e o do corpo é ignorado; nunca se altera outro alvo. Teste `TestUpdateTargetUsesPathID` na Tarefa 4.
2. **Nenhum alvo cadastrado:** `GET /api/alvos` responde `[]`, nunca `null`, para o frontend da Sprint 3 não quebrar. Teste `TestListEmptyReturnsArray` na Tarefa 4.
3. **Corpo vazio, JSON quebrado ou dois objetos colados (`{...}{...}`):** 400 `{"erro":"json inválido"}`, nunca 500 nem gravação parcial. Teste `TestCreateTargetRejectsMalformedJSON` na Tarefa 4.
4. **Host IPv6 (`::1`), hostname com mais de 253 caracteres ou rótulo com mais de 63, nome com acentos:** IPv6 é aceito; hostnames longos demais dão erro no campo `host`; o limite de 64 do nome conta caracteres, não bytes (64 × `"ç"` é válido). Casos na tabela de `TestValidate`, Tarefa 2.
5. **Banco em caminho sem permissão ou inexistente:** `run` devolve erro com o caminho na mensagem, `main` escreve em stderr e sai com código 1, sem panic. Teste `TestRunFailsOnBadDBPath` na Tarefa 5.

## Pré-requisitos

- Go 1.24 ou mais novo e golangci-lint v2 instalados (no Arch: `sudo pacman -S go golangci-lint`). Confira com `go version` e `golangci-lint --version`. Sem eles, a verificação é relatada como "não verificado", nunca como "passou".
- Branch criado a partir da `main` atualizada: `git switch -c feat/nucleo-backend`.

## Estrutura de arquivos

| Arquivo | Responsabilidade |
|---|---|
| `go.mod`, `go.sum`, `.gitignore` | Módulo e arquivos ignorados |
| `internal/config/config.go` | Lê e valida as variáveis de ambiente |
| `internal/target/target.go` | Tipo `Target` e sua validação |
| `internal/store/store.go` | Abre o SQLite, aplica PRAGMAs e migrações |
| `internal/store/targets.go` | CRUD de alvos |
| `internal/store/migrations/001_targets.sql` | Tabela `targets` |
| `internal/api/server.go` | `Server`, roteamento, `Handle` |
| `internal/api/auth.go` | Middleware de token |
| `internal/api/targets.go` | Handlers de `/api/alvos` |
| `internal/api/respond.go` | Escrita de JSON e de erros, leitura do corpo |
| `cmd/omni/run.go` | `run`: liga store, API e `http.Server`, desliga com graça |
| `cmd/omni/main.go` | Sinais, config, listener, código de saída |
| `.github/workflows/ci.yml` | Verificação do Go no CI |

---

### Task 1: Módulo Go e configuração

**Files:**
- Create: `go.mod`, `.gitignore`, `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces:
  - `type Config struct { Addr string; DBPath string; Token string; AllowRemote bool }`
  - `func Load(getenv func(string) string) (Config, error)`
  - `var ErrTokenAusente, ErrTokenCurto, ErrEnderecoRemoto error`

- [ ] **Step 1: Criar o módulo e o `.gitignore`**

Run: `go mod init github.com/MiguelDev-GH/omni && go mod edit -go=1.24 && go get github.com/stretchr/testify@latest`
`.gitignore`: `/omni`, `*.db`, `*.db-*`, `web/node_modules/`, `web/dist/`, `*.pcap`.

- [ ] **Step 2: Escrever os testes que falham**

```go
func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

const tok = "0123456789abcdef0123456789abcdef" // 32 caracteres

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(map[string]string{"OMNI_TOKEN": tok}))
	require.NoError(t, err)
	assert.Equal(t, Config{Addr: "127.0.0.1:8080", DBPath: "omni.db", Token: tok}, c)
}

func TestLoadReadsOverrides(t *testing.T) {
	c, err := Load(env(map[string]string{"OMNI_TOKEN": tok, "OMNI_ADDR": "127.0.0.1:9000", "OMNI_DB": "/tmp/x.db"}))
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:9000", c.Addr)
	assert.Equal(t, "/tmp/x.db", c.DBPath)
}

func TestLoadRejectsMissingToken(t *testing.T) {
	_, err := Load(env(nil))
	assert.ErrorIs(t, err, ErrTokenAusente)
}

func TestLoadRejectsShortToken(t *testing.T) {
	_, err := Load(env(map[string]string{"OMNI_TOKEN": tok[:31]}))
	assert.ErrorIs(t, err, ErrTokenCurto)
}

func TestLoadAddrRules(t *testing.T) {
	cases := []struct{ addr, remoto string; ok bool }{
		{"127.0.0.1:9000", "", true},
		{"[::1]:9000", "", true},
		{"localhost:9000", "", true},
		{"0.0.0.0:8080", "", false},
		{":8080", "", false},
		{"192.168.0.10:8080", "", false},
		{"sem-porta", "", false},
		{"0.0.0.0:8080", "1", true},
	}
	for _, c := range cases {
		cfg, err := Load(env(map[string]string{"OMNI_TOKEN": tok, "OMNI_ADDR": c.addr, "OMNI_PERMITIR_REMOTO": c.remoto}))
		if c.ok {
			assert.NoError(t, err, c.addr)
			assert.Equal(t, c.remoto == "1", cfg.AllowRemote, c.addr)
		} else {
			assert.ErrorIs(t, err, ErrEnderecoRemoto, c.addr)
		}
	}
}
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `go test ./internal/config/`
Expected: FAIL (`undefined: Load`)

- [ ] **Step 4: Implementar `Load` em `internal/config/config.go`**

Variáveis: `OMNI_ADDR` (padrão `127.0.0.1:8080`), `OMNI_DB` (padrão `omni.db`), `OMNI_TOKEN`, `OMNI_PERMITIR_REMOTO` (só `"1"` liga). Ordem das checagens: token ausente → token curto (`len < 32`) → endereço. Loopback = `net.SplitHostPort` sem erro e host `localhost` ou `net.ParseIP(host).IsLoopback()`; host vazio (`:8080`) e endereço sem porta não são loopback. Mensagens dos erros em português (ex.: `"OMNI_TOKEN precisa ter no mínimo 32 caracteres"`); o erro de endereço inclui o valor recebido (`fmt.Errorf("%w: %s", ErrEnderecoRemoto, addr)`).

- [ ] **Step 5: Rodar e ver passar**

Run: `go test ./internal/config/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum .gitignore internal/config
git commit -m "feat(api): adiciona carregamento de configuração por variáveis de ambiente"
```

### Task 2: Alvo e validação

**Files:**
- Create: `internal/target/target.go`
- Test: `internal/target/target_test.go`

**Interfaces:**
- Produces:
  - `type Target struct { ID int64 \`json:"id"\`; Name string \`json:"nome"\`; Host string \`json:"host"\`; AppPorts []int \`json:"portas_app"\`; DBPort int \`json:"porta_banco,omitempty"\`; Domain string \`json:"dominio,omitempty"\` }`
  - `func (t Target) Validate() error`: devolve `*ValidationError` no primeiro campo inválido, na ordem `nome`, `host`, `portas_app`, `porta_banco`, `dominio`
  - `type ValidationError struct { Field string; Message string }` com `Error() string` = `Field + ": " + Message`

- [ ] **Step 1: Escrever o teste de tabela que falha**

`TestValidate`, cada caso aplicado sobre o alvo válido `Target{Name: "api", Host: "127.0.0.1", AppPorts: []int{3000}}`:

| Caso | Mudança | Campo do erro |
|---|---|---|
| válido | — | nenhum |
| IPv6 | `Host:"::1"` | nenhum |
| hostname | `Host:"api.local"` | nenhum |
| domínio válido | `Domain:"app.exemplo.com.br"` | nenhum |
| banco válido | `DBPort:5432` | nenhum |
| nome com acentos, 64 caracteres | `Name: strings.Repeat("ç", 64)` | nenhum |
| host com shell | `Host:"127.0.0.1; rm -rf /"` | `host` |
| host como opção | `Host:"-oX"` | `host` |
| host maiúsculo | `Host:"API.local"` | `host` |
| host vazio | `Host:""` | `host` |
| rótulo com 64 caracteres | `Host: strings.Repeat("a", 64) + ".com"` | `host` |
| hostname com 254 caracteres | `Host: strings.Repeat("a.", 126) + "ab"` | `host` |
| nome vazio | `Name:"  "` | `nome` |
| nome longo | `Name: strings.Repeat("a", 65)` | `nome` |
| sem portas | `AppPorts:nil` | `portas_app` |
| porta 0 | `AppPorts:[]int{0}` | `portas_app` |
| porta 70000 | `AppPorts:[]int{70000}` | `portas_app` |
| porta repetida | `AppPorts:[]int{3000, 3000}` | `portas_app` |
| porta do banco inválida | `DBPort:70000` | `porta_banco` |
| porta do banco negativa | `DBPort:-1` | `porta_banco` |
| domínio com espaço | `Domain:"a b.com"` | `dominio` |

Assertion para os casos com erro: `var ve *ValidationError; require.ErrorAs(t, err, &ve); assert.Equal(t, campo, ve.Field)`. Para os válidos: `assert.NoError(t, err)`.

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/target/`
Expected: FAIL (`undefined: Target`)

- [ ] **Step 3: Implementar `Validate` em `internal/target/target.go`**

- Nome: `utf8.RuneCountInString(strings.TrimSpace(Name))` entre 1 e 64. `Validate` não altera o alvo.
- Host: IP literal (`net.ParseIP(h) != nil`) ou hostname com no máximo 253 caracteres que case com o regex abaixo (compilado uma vez em `var` do pacote). O mesmo regex e limite valem para `Domain` quando não vazio.

```go
var hostnameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)
```

- `AppPorts`: pelo menos uma, cada uma entre 1 e 65535, sem repetição. `DBPort`: 0 significa "sem banco", senão entre 1 e 65535.
- Mensagens em português: `"nome deve ter de 1 a 64 caracteres"`, `"host deve ser um IP ou um nome de host em minúsculas"`, `"informe ao menos uma porta"`, `"porta fora do intervalo 1-65535"`, `"porta repetida"`, `"domínio inválido"`.

- [ ] **Step 4: Rodar e ver passar**

Run: `go test ./internal/target/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/target
git commit -m "feat(api): adiciona modelo de alvo com validação estrita"
```

### Task 3: Armazenamento SQLite

**Files:**
- Create: `internal/store/store.go`, `internal/store/targets.go`, `internal/store/migrations/001_targets.sql`
- Test: `internal/store/store_test.go`, `internal/store/targets_test.go`

**Interfaces:**
- Consumes: `target.Target` (Task 2)
- Produces:
  - `type Store struct { db *sql.DB }`
  - `func Open(ctx context.Context, path string) (*Store, error)`: aplica PRAGMAs e migrações pendentes; erro envolve o caminho (`fmt.Errorf("abrir banco %s: %w", path, err)`)
  - `func (s *Store) Close() error`
  - `func (s *Store) CreateTarget(ctx context.Context, t target.Target) (target.Target, error)`: ignora `t.ID` e devolve o alvo com o `ID` gerado
  - `func (s *Store) ListTargets(ctx context.Context) ([]target.Target, error)`: ordenado por `ID`; sem alvos devolve slice vazio não nulo (`[]target.Target{}`)
  - `func (s *Store) GetTarget(ctx context.Context, id int64) (target.Target, error)`
  - `func (s *Store) UpdateTarget(ctx context.Context, t target.Target) error`: atualiza o alvo de `t.ID`
  - `func (s *Store) DeleteTarget(ctx context.Context, id int64) error`
  - `var ErrNotFound = errors.New("alvo não encontrado")`: devolvido por Get, Update e Delete de ID inexistente

Esquema `001_targets.sql`:

```sql
CREATE TABLE targets (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  host TEXT NOT NULL,
  app_ports TEXT NOT NULL,
  db_port INTEGER NOT NULL DEFAULT 0,
  domain TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

`app_ports` guardado como JSON (`json.Marshal([]int)`).

- [ ] **Step 1: Escrever os testes que falham**

Helper `openTest(t) *Store`: `Open(ctx, filepath.Join(t.TempDir(), "omni.db"))` com `t.Cleanup(func() { _ = s.Close() })`. Alvo de exemplo: `target.Target{Name: "api", Host: "127.0.0.1", AppPorts: []int{3000, 3001}, DBPort: 5432, Domain: "app.com"}`.

- `TestOpenEnablesWAL`: `PRAGMA journal_mode` retorna `"wal"`.
- `TestOpenIsIdempotent`: abre, cria um alvo, fecha, abre o mesmo arquivo de novo; `ListTargets` tem 1 item e `PRAGMA user_version` = 1.
- `TestOpenFailsOnMissingDir`: `Open` em `filepath.Join(t.TempDir(), "nao-existe", "omni.db")` devolve erro cuja mensagem contém o caminho.
- `TestListTargetsEmpty`: banco novo → `NotNil` e `Len 0`.
- `TestTargetCRUDRoundTrip`: cria (`ID > 0`) → `GetTarget` igual ao criado, incluindo `AppPorts`, `DBPort` e `Domain` → muda `Name` e `UpdateTarget` → `GetTarget` reflete → `DeleteTarget` → `GetTarget` dá `ErrNotFound`.
- `TestListTargetsOrderedByID`: cria três; `ListTargets` devolve os IDs em ordem crescente.
- `TestMissingTargetErrors`: `GetTarget`, `UpdateTarget` (alvo com `ID: 999`) e `DeleteTarget` com ID 999 devolvem `ErrNotFound` (`assert.ErrorIs`).

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/store/`
Expected: FAIL (`undefined: Open`)

- [ ] **Step 3: Implementar `Open` em `internal/store/store.go`**

Run antes: `go get modernc.org/sqlite`.
Import em branco `_ "modernc.org/sqlite"`; `sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)")`, depois `db.SetMaxOpenConns(1)` e `db.PingContext(ctx)` (é o ping que detecta caminho inválido). Migrações embutidas com `//go:embed migrations/*.sql`; o número `N` vem do prefixo do nome (`001_targets.sql` → 1, via `strconv.Atoi` do trecho antes do primeiro `_`; nome fora do padrão é erro). Aplique em ordem crescente só as com `N > user_version`, cada uma numa transação que executa o SQL e `PRAGMA user_version = N`. Em qualquer erro depois do `sql.Open`, feche o `db` antes de retornar.

- [ ] **Step 4: Implementar o CRUD em `internal/store/targets.go`**

Get, Update e Delete: `sql.ErrNoRows` (Get) ou `RowsAffected() == 0` (Update e Delete) viram `ErrNotFound`.

- [ ] **Step 5: Rodar e ver passar**

Run: `go test -race ./internal/store/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/store go.mod go.sum
git commit -m "feat(db): adiciona armazenamento SQLite com migrações e CRUD de alvos"
```

### Task 4: API REST autenticada

**Files:**
- Create: `internal/api/server.go`, `internal/api/auth.go`, `internal/api/targets.go`, `internal/api/respond.go`
- Test: `internal/api/server_test.go`

**Interfaces:**
- Consumes: os cinco métodos de CRUD e `store.ErrNotFound` (Task 3); `target.Target`, `Validate` e `*target.ValidationError` (Task 2)
- Produces:
  - `type TargetStore interface` com os cinco métodos de CRUD, mesmas assinaturas da Task 3
  - `func NewServer(ts TargetStore, token string) *Server`; `*Server` implementa `http.Handler`
  - `func (s *Server) Handle(pattern string, h http.Handler)`: registra rotas extras já atrás da autenticação (usado nas sprints seguintes)

Rotas (padrões do `http.ServeMux` do Go 1.22+):

| Método e rota | Sucesso | Erros |
|---|---|---|
| `GET /api/saude` (sem auth) | 200 `{"status":"ok"}` | — |
| `GET /api/alvos` | 200 lista (vazia = `[]`) | 401 |
| `POST /api/alvos` | 201 alvo com `id` | 400, 401, 413 |
| `GET /api/alvos/{id}` | 200 alvo | 400, 401, 404 |
| `PUT /api/alvos/{id}` | 200 alvo atualizado | 400, 401, 404, 413 |
| `DELETE /api/alvos/{id}` | 204 sem corpo | 400, 401, 404 |

Corpos de erro (exatos):

| Situação | Status | Corpo |
|---|---|---|
| Sem token, esquema diferente de `Bearer ` ou token errado | 401 | `{"erro":"não autorizado"}`, com header `WWW-Authenticate: Bearer` |
| JSON inválido, vazio, com campo desconhecido ou com dados depois do objeto | 400 | `{"erro":"json inválido"}` |
| Corpo acima de 1 MiB | 413 | `{"erro":"corpo muito grande"}` |
| `Validate` falhou | 400 | `{"erro":"<Message>","campo":"<Field>"}` |
| `{id}` não é inteiro positivo | 400 | `{"erro":"id inválido"}` |
| `ErrNotFound` | 404 | `{"erro":"alvo não encontrado"}` |
| Qualquer outro erro do store | 500 | `{"erro":"erro interno"}` (o erro real vai para `slog.Error`, nunca para o cliente) |

Decisões:
- A autenticação envolve tudo, exceto `GET /api/saude`; por isso uma rota `/api/*` inexistente sem token responde 401, e com token responde o 404 padrão do `ServeMux`.
- `POST` e `PUT` aplicam `strings.TrimSpace` em `Name` antes do `Validate`. `POST` ignora `id` do corpo; `PUT` usa o `{id}` do caminho e ignora o do corpo.
- Toda resposta com corpo usa `Content-Type: application/json`.

- [ ] **Step 1: Escrever os testes que falham**

Helper `newTestServer(t) *Server` com store real (`store.Open` em `t.TempDir()`) e o token de 32 caracteres; helper `do(t, srv, method, path, body string, auth bool) *httptest.ResponseRecorder`.

- `TestHealthNeedsNoToken`: 200, corpo `{"status":"ok"}` e `Content-Type: application/json`.
- `TestAlvosRequireToken`: sem header → 401 e `{"erro":"não autorizado"}`; `Bearer errado` → 401; `Basic <token>` → 401.
- `TestUnknownAPIRouteNeedsToken`: `GET /api/nada` sem token → 401; com token → 404.
- `TestListEmptyReturnsArray`: corpo exatamente `[]` (após `strings.TrimSpace`).
- `TestCreateTarget`: POST `{"nome":" api ","host":"127.0.0.1","portas_app":[3000]}` → 201, `id > 0` e `nome == "api"`; GET lista → 1 item.
- `TestCreateTargetRejectsShellHost`: host `"127.0.0.1; rm -rf /"` → 400 e `campo == "host"`; lista continua vazia.
- `TestCreateTargetRejectsUnknownField`: corpo válido com `"extra":1` → 400 `{"erro":"json inválido"}`.
- `TestCreateTargetRejectsMalformedJSON`: corpos `""`, `"{"` e dois objetos válidos colados → 400 `{"erro":"json inválido"}` cada; lista continua vazia.
- `TestCreateTargetRejectsHugeBody`: corpo de 2 MiB → 413.
- `TestGetTarget`: cria; GET `/api/alvos/{id}` → 200 com o mesmo alvo.
- `TestTargetNotFound`: GET `/api/alvos/999` → 404 `{"erro":"alvo não encontrado"}`; GET `/api/alvos/abc` → 400; GET `/api/alvos/0` → 400.
- `TestUpdateTargetUsesPathID`: cria A e B; PUT `/api/alvos/{A}` com corpo `{"id": B, "nome": "novo", ...}` → 200 com `id == A`; GET B continua com o nome original.
- `TestUpdateTargetValidates`: PUT com porta 70000 → 400 `campo == "portas_app"`; PUT em `/api/alvos/999` com corpo válido → 404.
- `TestDeleteTarget`: DELETE → 204 e corpo vazio; GET seguinte → 404; DELETE de novo → 404.
- `TestHandleIsBehindAuth`: `srv.Handle("GET /api/extra", handler que responde 200)`; sem token → 401; com token → 200.

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/api/`
Expected: FAIL (`undefined: NewServer`)

- [ ] **Step 3: Implementar `respond.go`**

`func writeJSON(w http.ResponseWriter, status int, v any)`, `func writeError(w http.ResponseWriter, status int, msg string)` e `func decodeJSON(w http.ResponseWriter, r *http.Request, v any) (status int, msg string, ok bool)`. `decodeJSON` usa `http.MaxBytesReader(w, r.Body, 1<<20)`, `DisallowUnknownFields`, e depois do `Decode` exige `dec.Decode(&struct{}{}) == io.EOF` para recusar dados extras; `*http.MaxBytesError` (via `errors.As`) vira 413, o resto vira 400 `"json inválido"`.

- [ ] **Step 4: Implementar `auth.go` e `server.go`**

`Server` guarda dois `http.ServeMux`: o externo (`/api/saude` e `/` → middleware de auth → mux interno) e o interno (rotas de alvos e as de `Handle`). Middleware: header precisa começar com `"Bearer "`; o resto é comparado ao token com `subtle.ConstantTimeCompare`.

- [ ] **Step 5: Implementar os handlers em `targets.go`**

`{id}` lido com `r.PathValue("id")` e `strconv.ParseInt(…, 10, 64)`; valores `<= 0` são inválidos. Erros do store mapeados conforme a tabela de corpos de erro.

- [ ] **Step 6: Rodar e ver passar**

Run: `go test -race ./internal/api/`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/api
git commit -m "feat(api): adiciona API REST autenticada de alvos"
```

### Task 5: Binário, desligamento gracioso e CI

**Files:**
- Create: `cmd/omni/main.go`, `cmd/omni/run.go`, `.github/workflows/ci.yml`
- Test: `cmd/omni/run_test.go`

**Interfaces:**
- Consumes: `config.Config` e `config.Load` (Task 1), `store.Open` (Task 3), `api.NewServer` (Task 4)
- Produces: `func run(ctx context.Context, cfg config.Config, ln net.Listener) error`: abre o store em `cfg.DBPath`, serve a API em `ln` até `ctx` ser cancelado e retorna `nil` depois de `Shutdown` (timeout de 10 s); erro do store ou do `Serve` (exceto `http.ErrServerClosed`) é devolvido. Sempre fecha o store e o `ln`.

- [ ] **Step 1: Escrever os testes que falham**

- `TestRunServesAndShutsDown`: `ln, _ := net.Listen("tcp", "127.0.0.1:0")`; `run` numa goroutine com `DBPath` em `t.TempDir()` e o token de 32 caracteres; `GET http://<ln.Addr()>/api/saude` → 200 (tente por até 2 s); cancela o contexto; `run` retorna `nil` em até 2 s.
- `TestRunServesAuthenticatedTargets`: mesmo setup; `GET /api/alvos` com o token → 200 `[]`; sem token → 401.
- `TestRunFailsOnBadDBPath`: `DBPath` = `filepath.Join(t.TempDir(), "nao-existe", "omni.db")`; `run` retorna erro em até 2 s, com o caminho na mensagem, e o `ln` fica fechado (`net.Dial` nele falha).

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./cmd/omni/`
Expected: FAIL (`undefined: run`)

- [ ] **Step 3: Implementar `run` em `cmd/omni/run.go`**

`http.Server{Handler: api.NewServer(st, cfg.Token), ReadHeaderTimeout: 5 * time.Second}`. `Serve` numa goroutine; `select` entre o erro do `Serve` e `ctx.Done()`; no cancelamento, `Shutdown` com contexto de 10 s. Ao começar a servir, `slog.Info("omni no ar", "endereco", ln.Addr().String())`.

- [ ] **Step 4: Implementar `main` em `cmd/omni/main.go`**

`signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` → `config.Load(os.Getenv)` → `net.Listen("tcp", cfg.Addr)` → `run`. Qualquer erro: `fmt.Fprintln(os.Stderr, "omni:", err)` e `os.Exit(1)`.

- [ ] **Step 5: Rodar e ver passar**

Run: `go test -race ./cmd/omni/`
Expected: PASS

- [ ] **Step 6: Commit do binário**

```bash
git add cmd/omni
git commit -m "feat(api): adiciona binário omni com desligamento gracioso"
```

- [ ] **Step 7: Criar o CI em `.github/workflows/ci.yml`**

`name: CI`; dispara em `pull_request` e em `push` na `main`; job `go` em `ubuntu-latest`, passos em ordem:
1. `actions/checkout@v4`
2. `actions/setup-go@v5` com `go-version-file: go.mod`
3. `test -z "$(gofmt -l .)"`
4. `go vet ./...`
5. `golangci/golangci-lint-action@v8` com `version: v2.5.0`
6. `go test -race ./...`

```bash
git add .github/workflows/ci.yml
git commit -m "ci: adiciona verificação do backend Go"
```

- [ ] **Step 8: Verificação completa da sprint**

Run: `gofmt -l . && go vet ./... && golangci-lint run ./... && go build ./... && go test -race ./...`
Expected: nenhuma saída do `gofmt`, `0 issues` no linter, todos os pacotes `ok`.

Teste manual:

```bash
export OMNI_TOKEN=$(openssl rand -hex 16) OMNI_DB=$(mktemp -d)/omni.db
go run ./cmd/omni &
curl -s 127.0.0.1:8080/api/saude                                  # {"status":"ok"}
curl -s -o /dev/null -w '%{http_code}\n' 127.0.0.1:8080/api/alvos # 401
curl -s -H "Authorization: Bearer $OMNI_TOKEN" -d '{"nome":"api","host":"127.0.0.1","portas_app":[3000]}' 127.0.0.1:8080/api/alvos
kill %1                                                           # sai sem erro
OMNI_ADDR=0.0.0.0:8080 go run ./cmd/omni; echo $?                 # mensagem em stderr e 1
```

- [ ] **Step 9: Atualizar a documentação**

- `README.md`: troque a linha de status por `> **Status:** em implementação. A Sprint 1 (núcleo do backend) está em andamento; veja [docs/SPRINTS.md](docs/SPRINTS.md).`
- `docs/SPRINTS.md`: Sprint 1 passa a `🚧 Em andamento` na tabela de resumo.

```bash
git add README.md docs/SPRINTS.md
git commit -m "docs: marca a sprint 1 como em andamento"
```

- [ ] **Step 10: Abrir o PR**

`git push -u origin feat/nucleo-backend` e PR com título `feat(api): adiciona núcleo do backend`, base `main`, labels `funcionalidade` e `motor`, descrição com o que mudou, por quê e a saída real da verificação do Step 8. Depois de mesclado, a Sprint 1 vira `✅ Concluída` em `docs/SPRINTS.md` (num PR `docs:` próprio ou no PR da Sprint 2).
