# Sprint 2: Poller e WebSocket, Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** O `omni` testa periodicamente as portas de cada alvo, guarda o histórico no SQLite e avisa em tempo real, por WebSocket, quando um serviço cai ou volta, além de publicar CPU, memória e disco a cada 5 s.

**Architecture:** Três pacotes novos com dependência em uma só direção: `ws` (hub, tickets e handler; não depende de nada do projeto), `poller` (verificações e agendador; depende de `ws` e `target`) e `sysmetrics` (amostragem com gopsutil; depende de `ws`). O `store` passa a importar `poller` para gravar `poller.Result`; o `poller` nunca importa o `store`, e enxerga o banco por uma interface própria. `cmd/omni/run.go` liga tudo e desliga na ordem certa: HTTP e conexões WebSocket primeiro, depois as goroutines do poller e das métricas, por último o banco.

**Tech Stack:** Go 1.24, `github.com/gorilla/websocket` v1.5.3, `github.com/shirou/gopsutil/v4` v4.26.8 (a v4.26.9 já exige Go 1.26), `modernc.org/sqlite`, `testify`, golangci-lint v2.

**Spec:** `docs/superpowers/plans/2026-11-18-implementacao-basica.md` (seção "Sprint 2" e "Global Constraints"), `docs/SPRINTS.md` (seção "Sprint 2") e `README.md` (seções "Comunicação em tempo real" e "Segurança da própria plataforma"). Convenções: `.claude/skills/convencoes-omni/SKILL.md`. Este documento expande a Sprint 2 com o código real da Sprint 1 à vista e decide o que o plano básico deixou em aberto; onde os dois divergirem, vale este.

## Global Constraints

- Linha `go 1.24.0` no `go.mod` continua igual. Dependências novas fixadas: `go get github.com/gorilla/websocket@v1.5.3 github.com/shirou/gopsutil/v4@v4.26.8`. Se o `go get` tentar subir a linha `go`, rebaixe a dependência transitiva que causou isso, em vez de subir o Go.
- Identificadores em inglês; erros da API, nomes de eventos e campos JSON em português. Erros da API têm a forma `{"erro": "<mensagem>"}`, como na Sprint 1.
- Todo evento do WebSocket tem o envelope `{"tipo": "<NOME>", "dados": {...}}`. Nomes desta sprint: `STATUS_ALVOS`, `ALERTA_QUEDA`, `SERVICO_RESTAURADO`, `METRICAS_SISTEMA`.
- Horários em JSON são strings RFC 3339 em UTC; no SQLite, inteiros em milissegundos Unix (`*_ms`), para que a ordenação e o filtro por período sejam numéricos.
- Toda requisição feita pelo Poller usa `User-Agent: Omni-Poller/1`, para ser filtrada das métricas do Sniffer na Sprint 5.
- Ticket do WebSocket: 32 bytes de `crypto/rand` em hex (64 caracteres), validade de 30 s, removido no primeiro uso.
- Nenhuma goroutine iniciada por `run` sobrevive ao retorno de `run`, e o banco só é fechado depois que todas terminaram.
- Todos os testes Go usam `testify`. Testes com tempo usam intervalos curtos (≤ 100 ms) e `require.Eventually`, nunca `time.Sleep` fixo para esperar resultado.
- Branch `feat/poller-e-websocket`; commits e PR seguem a skill `convencoes-omni`. PR com labels `funcionalidade` e `motor`. Antes de cada commit, rode a verificação do backend da skill sobre o estado final do código e relate o resultado real.

## Review Focus

1. **Desligar o `omni` (Ctrl+C) com uma aba do navegador conectada ao WebSocket:** `run` retorna em até 2 s. `http.Server.Shutdown` não espera nem fecha conexões sequestradas (hijacked), então o hub precisa fechá-las via `RegisterOnShutdown`. Teste `TestRunShutsDownWithOpenWebSocket` na Task 7.
2. **Alvo apagado enquanto uma rodada do Poller está em andamento:** a gravação do resultado falha por chave estrangeira; o erro vai para o log, as outras portas da rodada são gravadas e publicadas, e a próxima rodada já não verifica o alvo. Teste `TestRunOnceContinuesAfterInsertError` na Task 5.
3. **Aplicação que responde com redirecionamento (301/302) para outro host:** o Poller registra o próprio 30x como "no ar" e nunca segue o redirecionamento, para não fazer requisições a hosts que não foram cadastrados. Teste `TestCheckHTTPDoesNotFollowRedirects` na Task 3.
4. **Serviço que já está fora do ar quando o `omni` inicia, e alvo com IPv6 (`::1`):** a primeira rodada publica `ALERTA_QUEDA` (não há "estado anterior no ar" para esperar), e a URL verificada é `http://[::1]:<porta>/`. Testes `TestRunOnceAlertsWhenFirstSeenDown` na Task 5 e `TestCheckHTTPIPv6` na Task 3.
5. **Acesso pelo túnel SSH numa porta local diferente** (`ssh -L 9999:127.0.0.1:8080`): o navegador manda `Origin: http://localhost:9999`, que só é aceito se estiver em `OMNI_ORIGENS`; origem com caminho ou sem esquema em `OMNI_ORIGENS` impede o `omni` de subir. Testes `TestLoadOrigins` na Task 1 e `TestRejectsForeignOrigin` na Task 2.

## Pré-requisitos

- Sprint 1 mesclada na `main` (PR #5) e Go e golangci-lint v2 instalados. Sem eles, a verificação é relatada como "não verificado", nunca como "passou".
- Branch criado a partir da `main` atualizada: `git switch main && git pull --ff-only && git switch -c feat/poller-e-websocket`.

## Estrutura de arquivos

| Arquivo | Responsabilidade |
|---|---|
| `internal/config/config.go` | Ganha `PollInterval` e `AllowedOrigins` |
| `internal/ws/hub.go` | `Event`, `Publisher`, `Hub`: assinantes com buffer próprio, descarte do mais antigo |
| `internal/ws/ticket.go` | `Tickets`: emissão e consumo de tickets de uso único |
| `internal/ws/handler.go` | `GET /ws` (origem, ticket, upgrade, laços de leitura e escrita) e `POST /api/ws/ticket` |
| `internal/api/server.go` | Ganha `HandlePublic`, para rotas fora da autenticação por token |
| `internal/api/history.go` | `GET /api/alvos/{id}/historico` |
| `internal/poller/checks.go` | `Result`, `CheckHTTP`, `CheckTCP` |
| `internal/poller/scheduler.go` | `Scheduler`: rodadas, estado por porta, eventos de transição |
| `internal/store/checks.go` | `InsertCheckResult`, `CheckHistory` |
| `internal/store/migrations/002_checks.sql` | Tabela `check_results` |
| `internal/sysmetrics/sysmetrics.go` | `Sample` e `Run` (evento a cada 5 s) |
| `cmd/omni/run.go` | Liga hub, tickets, poller e métricas; desliga na ordem certa |

---

### Task 1: Configuração do Poller e das origens

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces:
  - `Config` ganha `PollInterval time.Duration` e `AllowedOrigins []string`
  - `var ErrIntervaloInvalido, ErrOrigemInvalida error`

Variáveis novas:

| Variável | Padrão | Regra |
|---|---|---|
| `OMNI_INTERVALO_POLLER` | `30s` | `time.ParseDuration`, mínimo `1s`; fora disso → `ErrIntervaloInvalido` com o valor na mensagem |
| `OMNI_DEV` | vazio | `"1"` acrescenta `http://localhost:5173` e `http://127.0.0.1:5173` (servidor do Vite) |
| `OMNI_ORIGENS` | vazio | lista separada por vírgula, espaços em volta ignorados; cada item precisa ter esquema `http` ou `https`, host, e nada além de porta (sem caminho, query, fragmento ou usuário); senão → `ErrOrigemInvalida` com o item na mensagem |

`AllowedOrigins`, nesta ordem e sem repetição: `http://127.0.0.1:<P>`, `http://localhost:<P>`, `http://[::1]:<P>` (com `<P>` = porta de `Addr`), depois as de `OMNI_DEV`, depois as de `OMNI_ORIGENS`. Ordem das checagens em `Load`: token ausente → token curto → endereço → intervalo → origens.

- [ ] **Step 1: Atualizar e escrever os testes que falham**

`TestLoadDefaults` passa a esperar:

```go
Config{Addr: "127.0.0.1:8080", DBPath: "omni.db", Token: tok, PollInterval: 30 * time.Second,
	AllowedOrigins: []string{"http://127.0.0.1:8080", "http://localhost:8080", "http://[::1]:8080"}}
```

- `TestLoadPollInterval`: `"5s"` → `5*time.Second`; `"1s"` → ok; `"500ms"`, `"0"`, `"-1s"` e `"abc"` → `assert.ErrorIs(err, ErrIntervaloInvalido)`.
- `TestLoadOrigins`:
  - `OMNI_ADDR=127.0.0.1:9000` → as três origens usam a porta `9000`.
  - `OMNI_DEV=1` → `AllowedOrigins[3:]` = `{"http://localhost:5173", "http://127.0.0.1:5173"}`.
  - `OMNI_ORIGENS=" http://localhost:9999 ,https://omni.exemplo.com"` → as duas no fim da lista, sem espaços.
  - `OMNI_ORIGENS="http://localhost:8080"` (repetida) → a lista continua com 3 itens.
  - `"localhost:9999"`, `"http://localhost:9999/painel"`, `"ftp://x.com"`, `"http://u:s@x.com"` e `"http://"` → `ErrOrigemInvalida`.

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/config/`
Expected: FAIL (`unknown field PollInterval`)

- [ ] **Step 3: Implementar em `internal/config/config.go`**

Validação de origem com `url.Parse`: exige `Scheme` em `{"http","https"}`, `Host != ""`, `User == nil`, e `Path`, `RawQuery`, `Fragment` e `Opaque` vazios. Porta de `Addr` via `net.SplitHostPort`. Com `OMNI_PERMITIR_REMOTO=1` a checagem de loopback é pulada, então um `Addr` sem porta só aparece aqui: devolva `fmt.Errorf("%w: %s", ErrEnderecoRemoto, addr)` nesse caso, e acrescente `{"sem-porta", "1", false}` à tabela de `TestLoadAddrRules`.

- [ ] **Step 4: Rodar e ver passar**

Run: `go test ./internal/config/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config
git commit -m "feat(poller): adiciona intervalo do poller e origens permitidas à config"
```

### Task 2: Hub WebSocket com ticket de uso único

**Files:**
- Create: `internal/ws/hub.go`, `internal/ws/ticket.go`, `internal/ws/handler.go`
- Modify: `internal/api/server.go`
- Test: `internal/ws/hub_test.go`, `internal/ws/ticket_test.go`, `internal/ws/handler_test.go`, `internal/api/server_test.go`

**Interfaces:**
- Produces:
  - `type Event struct { Type string \`json:"tipo"\`; Data any \`json:"dados"\` }`
  - `type Publisher interface { Publish(Event) }`
  - `func NewHub(clientBuffer int) *Hub`; `(*Hub) Publish(Event)` (nunca bloqueia); `(*Hub) Close()` (fecha todos os clientes e recusa novos; idempotente)
  - `func NewTickets(ttl time.Duration) *Tickets`; `(*Tickets) Issue() (string, error)`; `(*Tickets) Consume(ticket string) bool`
  - `func Handler(h *Hub, t *Tickets, allowedOrigins []string) http.Handler`: serve `GET /ws?ticket=...`
  - `func TicketHandler(t *Tickets) http.Handler`: responde 200 `{"ticket":"<64 hex>"}`
  - `func (s *api.Server) HandlePublic(pattern string, h http.Handler)`: registra no mux externo, sem token

Decisões:
- **Sem goroutine central.** O `Hub` guarda um `map[*client]struct{}` sob `sync.Mutex`. `Publish` serializa o evento uma vez (`json.Marshal`; erro vai para `slog.Error` e o evento é descartado) e, para cada cliente, tenta enviar os bytes no canal dele sem bloquear; com o canal cheio, retira o mais antigo e envia o novo (laço `select` com `default`, já que outro escritor pode competir). Isso substitui o `Run(ctx)` do plano básico.
- Funções internas usadas pelos testes do pacote: `(*Hub) subscribe() (*client, bool)` (false depois de `Close`), `(*Hub) unsubscribe(*client)`, e `client.send chan []byte`, `client.done chan struct{}` (fechado por `Close`/`unsubscribe`, uma vez, com `sync.Once`).
- **Ordem no handler:** método diferente de GET → 405; `Origin` presente e fora de `allowedOrigins` (comparação exata) → 403 `{"erro":"origem não permitida"}`; ticket ausente ou `Consume` falso → 401 `{"erro":"ticket inválido"}`; hub fechado → 503 `{"erro":"servidor desligando"}`; só então o upgrade. A origem é checada antes do ticket para que uma página de outro site nunca gaste um ticket válido. `Origin` ausente é aceito (clientes fora do navegador; o ticket continua obrigatório). O `websocket.Upgrader` usa `CheckOrigin: func(*http.Request) bool { return true }`, porque a checagem já foi feita.
- **Laços por conexão:** escrita lê `client.send` e escreve com `SetWriteDeadline(10 s)`; manda ping a cada 30 s; sai em erro de escrita ou em `client.done`, enviando close frame. Leitura: `SetReadLimit(512)`, deadline de leitura de 60 s renovado a cada pong, descarta mensagens recebidas (comandos do Inspetor chegam na Sprint 4); em erro, chama `unsubscribe`. A conexão é fechada quando os dois laços terminam.
- **Tickets:** `map[string]time.Time` (expiração) sob mutex; `Issue` remove os vencidos antes de inserir; `Consume` apaga o ticket e devolve `true` só se ele existia e não venceu. Tempo vem de um campo `now func() time.Time` (padrão `time.Now`) para os testes.

- [ ] **Step 1: Escrever os testes que falham**

`hub_test.go`:
- `TestPublishDropsOldestWhenFull`: `NewHub(4)`; `c, _ := h.subscribe()`; publica eventos com `Data: i` para `i` de 1 a 10; lê 4 mensagens de `c.send` sem bloquear; os `dados` são `7, 8, 9, 10`.
- `TestPublishWithoutClientsDoesNotBlock`: `NewHub(1)`; 1000 `Publish` terminam (teste roda com `-race`).
- `TestCloseRejectsNewClients`: depois de `Close`, `subscribe` devolve `false`; `Close` duas vezes não entra em pânico.

`ticket_test.go`:
- `TestTicketIsSingleUse`: `Issue` devolve 64 caracteres hex; primeiro `Consume` → `true`, segundo → `false`.
- `TestTicketExpires`: `now` controlado; `Issue`, avança 31 s, `Consume` → `false`.
- `TestUnknownTicket`: `Consume("abc")` → `false`.

`handler_test.go` (servidor `httptest.NewServer(Handler(hub, tickets, []string{"http://127.0.0.1:8080"}))`, cliente `websocket.DefaultDialer`):
- `TestRejectsForeignOrigin`: ticket válido e `Origin: https://evil.example` → 403; o ticket continua válido (`Consume` → `true`).
- `TestRejectsMissingOrExpiredTicket`: sem ticket → 401; ticket inexistente → 401; com `NewTickets(10*time.Millisecond)`, ticket usado depois de 50 ms → 401.
- `TestDeliversEvents`: conecta com ticket e `Origin: http://127.0.0.1:8080`; `hub.Publish(Event{Type: "TESTE", Data: map[string]int{"n": 1}})`; o cliente lê exatamente `{"tipo":"TESTE","dados":{"n":1}}`.
- `TestSlowClientDoesNotBlockOthers`: `NewHub(4)`; cliente A conecta e nunca lê; cliente B lê; publica 100 eventos com `Data: i`; B recebe o evento com `dados == 100` em até 1 s.
- `TestHubCloseDisconnectsClients`: conecta; `hub.Close()`; o próximo `ReadMessage` do cliente devolve erro em até 1 s.
- `TestTicketHandler`: `POST` → 200, `Content-Type: application/json` e `ticket` com 64 caracteres que `Consume` aceita.

`internal/api/server_test.go`:
- `TestHandlePublicSkipsAuth`: `srv.HandlePublic("GET /ws", handler que responde 200)`; sem token → 200.

- [ ] **Step 2: Rodar e ver falhar**

Run: `go get github.com/gorilla/websocket@v1.5.3 && go test ./internal/ws/ ./internal/api/`
Expected: FAIL (`undefined: NewHub`, `srv.HandlePublic undefined`)

- [ ] **Step 3: Implementar `hub.go` e `ticket.go`**

- [ ] **Step 4: Implementar `handler.go` e `HandlePublic`**

Os erros JSON do pacote `ws` usam um `writeError` local com o mesmo formato de `api` (o pacote `ws` não importa `api`).

- [ ] **Step 5: Rodar e ver passar**

Run: `go test -race ./internal/ws/ ./internal/api/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/ws internal/api go.mod go.sum
git commit -m "feat(ws): adiciona hub websocket com ticket de uso único"
```

### Task 3: Verificações do Poller

**Files:**
- Create: `internal/poller/checks.go`
- Test: `internal/poller/checks_test.go`

**Interfaces:**
- Produces:
  - `type Result struct` com os campos e tags JSON abaixo
  - `func NewHTTPClient(timeout time.Duration) *http.Client`
  - `func CheckHTTP(ctx context.Context, c *http.Client, targetID int64, host string, port int) Result`
  - `func CheckTCP(ctx context.Context, targetID int64, host string, port int, timeout time.Duration) Result`

```go
type Result struct {
	TargetID   int64     `json:"alvo_id"`
	Port       int       `json:"porta"`
	Kind       string    `json:"tipo"` // "http" ou "tcp"
	Up         bool      `json:"no_ar"`
	StatusCode int       `json:"status_http,omitempty"`
	LatencyMS  float64   `json:"latencia_ms"`
	Err        string    `json:"erro,omitempty"`
	At         time.Time `json:"hora"`
}
```

Decisões:
- Endereço sempre com `net.JoinHostPort(host, strconv.Itoa(port))`; URL `http://` + endereço + `/`.
- `NewHTTPClient`: `Timeout` dado, `CheckRedirect` devolvendo `http.ErrUseLastResponse`.
- `Up` em `CheckHTTP` = recebeu resposta, com qualquer status. O corpo é fechado sem ler. `LatencyMS` = tempo até os cabeçalhos, `float64(d.Microseconds())/1000`. `At` = início da verificação, em UTC.
- Em falha, `Err` é `err.Error()`, `StatusCode` 0 e `LatencyMS` o tempo até a falha.

- [ ] **Step 1: Escrever os testes que falham**

Helper `hostPort(t, url string) (string, int)` que extrai host e porta de um `httptest.Server`.

- `TestCheckHTTPUp`: servidor que responde 200 e grava o `User-Agent` recebido → `Up`, `StatusCode == 200`, `Kind == "http"`, `LatencyMS >= 0`, `User-Agent == "Omni-Poller/1"`, `At` em UTC.
- `TestCheckHTTPServerError`: servidor que responde 500 → `Up` e `StatusCode == 500`.
- `TestCheckHTTPDown`: servidor criado e fechado → `!Up` e `Err != ""`.
- `TestCheckHTTPTimeout`: servidor que dorme 1 s; cliente com timeout de 50 ms → `!Up` e a chamada retorna em menos de 500 ms.
- `TestCheckHTTPDoesNotFollowRedirects`: servidor que responde 302 com `Location` para um segundo servidor que conta acessos → `Up`, `StatusCode == 302` e o segundo servidor recebeu 0 acessos.
- `TestCheckHTTPIPv6`: `net.Listen("tcp", "[::1]:0")` (se falhar, `t.Skip("sem IPv6")`) servindo 200; `CheckHTTP(…, "::1", porta)` → `Up`.
- `TestCheckTCP`: listener aberto → `Up`, `Kind == "tcp"`; listener fechado → `!Up` e `Err != ""`.

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/poller/`
Expected: FAIL (`undefined: CheckHTTP`)

- [ ] **Step 3: Implementar em `internal/poller/checks.go`**

`CheckTCP` usa `net.Dialer{Timeout: timeout}.DialContext` e fecha a conexão logo após abrir.

- [ ] **Step 4: Rodar e ver passar**

Run: `go test -race ./internal/poller/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/poller
git commit -m "feat(poller): adiciona verificações http e tcp"
```

### Task 4: Histórico de verificações no banco e na API

**Files:**
- Create: `internal/store/checks.go`, `internal/store/migrations/002_checks.sql`, `internal/api/history.go`
- Test: `internal/store/checks_test.go`, `internal/api/history_test.go`
- Modify: `internal/store/store_test.go` (`user_version` esperado passa a 2)

**Interfaces:**
- Consumes: `poller.Result` (Task 3); `store.ErrNotFound`, `GetTarget` (Sprint 1)
- Produces:
  - `func (s *Store) InsertCheckResult(ctx context.Context, r poller.Result) error`
  - `func (s *Store) CheckHistory(ctx context.Context, targetID int64, since time.Time, limit int) ([]poller.Result, error)`: resultados com `At >= since`, os `limit` mais recentes, devolvidos em ordem cronológica crescente; sem resultados, slice vazio não nulo
  - `type HistoryStore interface { GetTarget(ctx context.Context, id int64) (target.Target, error); CheckHistory(ctx context.Context, targetID int64, since time.Time, limit int) ([]poller.Result, error) }` em `internal/api`
  - `func HistoryHandler(hs HistoryStore) http.Handler` em `internal/api`, registrado em `run` com `srv.Handle("GET /api/alvos/{id}/historico", …)`

```sql
CREATE TABLE check_results (
  id INTEGER PRIMARY KEY,
  target_id INTEGER NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  port INTEGER NOT NULL,
  kind TEXT NOT NULL,
  up INTEGER NOT NULL,
  status_code INTEGER NOT NULL DEFAULT 0,
  latency_ms REAL NOT NULL,
  error TEXT NOT NULL DEFAULT '',
  checked_at_ms INTEGER NOT NULL
);
CREATE INDEX check_results_target_time ON check_results (target_id, checked_at_ms);
```

`GET /api/alvos/{id}/historico?desde=<RFC3339>`:

| Situação | Status | Corpo |
|---|---|---|
| Sucesso | 200 | lista de `Result` (vazia = `[]`), no máximo 5000 itens |
| `desde` ausente | — | usa as últimas 24 h |
| `desde` não é RFC 3339 | 400 | `{"erro":"desde inválido"}` |
| `{id}` não é inteiro positivo | 400 | `{"erro":"id inválido"}` |
| Alvo inexistente | 404 | `{"erro":"alvo não encontrado"}` |
| Outro erro do store | 500 | `{"erro":"erro interno"}` |

Reaproveite `pathID`, `writeJSON`, `writeError` e a lógica de `storeError` de `internal/api` (extraia `storeError` para função livre se precisar dela fora de `*Server`).

- [ ] **Step 1: Escrever os testes que falham**

Store (helper `openTest` da Sprint 1; um alvo criado antes de cada teste):
- `TestInsertAndReadCheckResult`: grava um `Result` com todos os campos preenchidos e `At` com milissegundos; `CheckHistory` devolve 1 item igual, com `At` igual ao gravado truncado em milissegundos e em UTC.
- `TestCheckHistoryFiltersBySince`: resultados em `t0`, `t0+1m`, `t0+2m`; `since = t0+1m` → 2 itens, em ordem crescente.
- `TestCheckHistoryLimitKeepsNewest`: 5 resultados, `limit 2` → os dois mais recentes, em ordem crescente.
- `TestCheckHistoryEmpty`: `NotNil` e `Len 0`.
- `TestDeleteTargetCascadesHistory`: grava resultado, apaga o alvo, cria outro alvo; `CheckHistory` do ID antigo → vazio.
- `TestInsertCheckResultUnknownTarget`: `TargetID: 999` → erro (chave estrangeira).

API (`newTestServer` da Sprint 1 com `srv.Handle("GET /api/alvos/{id}/historico", HistoryHandler(st))`):
- `TestHistoryReturnsResults`: alvo com 2 resultados recentes → 200 e 2 itens com `alvo_id`, `porta`, `no_ar`, `hora`.
- `TestHistoryEmptyIsArray`: alvo sem resultados → corpo exatamente `[]`.
- `TestHistoryErrors`: `desde=ontem` → 400 `{"erro":"desde inválido"}`; alvo 999 → 404; id `abc` → 400; sem token → 401.

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/store/ ./internal/api/`
Expected: FAIL (`undefined: InsertCheckResult`)

- [ ] **Step 3: Criar a migração e implementar `internal/store/checks.go`**

`CheckHistory`: `SELECT … WHERE target_id = ? AND checked_at_ms >= ? ORDER BY checked_at_ms DESC, id DESC LIMIT ?`, depois inverte a slice. Converte com `time.UnixMilli(ms).UTC()`.

- [ ] **Step 4: Implementar `internal/api/history.go`**

- [ ] **Step 5: Rodar e ver passar**

Run: `go test -race ./internal/store/ ./internal/api/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/store internal/api
git commit -m "feat(db): adiciona histórico de verificações e rota de consulta"
```

### Task 5: Agendador e eventos de transição

**Files:**
- Create: `internal/poller/scheduler.go`
- Test: `internal/poller/scheduler_test.go`

**Interfaces:**
- Consumes: `ws.Event`, `ws.Publisher` (Task 2); `CheckHTTP`, `CheckTCP`, `NewHTTPClient`, `Result` (Task 3); `InsertCheckResult` (Task 4, via interface)
- Produces:
  - `type ResultStore interface { ListTargets(ctx context.Context) ([]target.Target, error); InsertCheckResult(ctx context.Context, r Result) error }`
  - `func NewScheduler(st ResultStore, pub ws.Publisher, interval time.Duration) *Scheduler`
  - `(*Scheduler) RunOnce(ctx context.Context) error`: uma rodada; erro só se `ListTargets` falhar
  - `(*Scheduler) Run(ctx context.Context)`: rodada imediata e depois a cada `interval`, até `ctx` ser cancelado; erro de `RunOnce` vai para `slog.Error` e não interrompe
  - Constante `CheckTimeout = 5 * time.Second` (timeout do cliente HTTP e do TCP)

Eventos (o `Data` é `map[string]any` ou struct com estas tags):

| Evento | Quando | `dados` |
|---|---|---|
| `STATUS_ALVOS` | fim de toda rodada com ao menos uma porta | `{"resultados": [Result, ...]}` em ordem de alvo e porta |
| `ALERTA_QUEDA` | porta passou de no ar (ou desconhecida) para fora do ar | `{"alvo_id", "alvo" (nome), "porta", "erro", "hora"}` |
| `SERVICO_RESTAURADO` | porta passou de fora do ar para no ar | `{"alvo_id", "alvo", "porta", "hora"}` |

Decisões:
- Cada alvo gera uma verificação `CheckHTTP` por porta de `AppPorts` e uma `CheckTCP` em `DBPort` quando diferente de 0.
- As verificações de uma rodada correm em paralelo, no máximo 16 ao mesmo tempo (semáforo com canal); a rodada espera todas antes de gravar e publicar.
- Estado anterior em `map[portKey]bool` (`portKey{TargetID int64; Port int}`), usado só pela goroutine que chama `RunOnce`. Primeira observação fora do ar gera `ALERTA_QUEDA`; primeira observação no ar não gera nada. Chaves de alvos ou portas que sumiram da lista são apagadas no fim da rodada, para que um alvo recriado comece sem estado.
- `InsertCheckResult` com erro: `slog.Error` com `alvo_id` e `porta`, e a rodada segue (o resultado ainda entra no `STATUS_ALVOS` e nas transições).

- [ ] **Step 1: Escrever os testes que falham**

Fakes no teste: `fakeStore` (alvos fixos, resultados gravados, erro opcional por `TargetID`, protegido por mutex) e `fakePub` (guarda os eventos, mutex). Servidor `httptest` cuja porta é usada em `AppPorts`; para "derrubar", feche o servidor; para "voltar", abra um novo `net.Listen` na mesma porta e sirva com `http.Serve` (se a porta não puder ser reaberta, `t.Skip`).

- `TestRunOnceEmitsDownOnlyOnTransition`: rodada 1 com o servidor no ar → nenhum `ALERTA_QUEDA`; servidor fechado, rodada 2 → exatamente um `ALERTA_QUEDA` com `alvo_id`, `alvo` e `porta` certos; rodada 3 ainda fechado → nenhum novo; servidor de volta, rodada 4 → exatamente um `SERVICO_RESTAURADO`.
- `TestRunOnceAlertsWhenFirstSeenDown`: alvo apontando para porta fechada; rodada 1 → um `ALERTA_QUEDA`.
- `TestRunOncePublishesStatusAndStores`: alvo com 2 portas de app e `DBPort` (listener TCP aberto) → `fakeStore` recebe 3 resultados, e um `STATUS_ALVOS` com 3 resultados, os de `Kind` `"http"` antes do `"tcp"`.
- `TestRunOnceContinuesAfterInsertError`: dois alvos; `fakeStore` falha a gravação do primeiro → o segundo é gravado e o `STATUS_ALVOS` traz os dois; `RunOnce` devolve `nil`.
- `TestRunOnceForgetsRemovedTargets`: rodada 1 com alvo fora do ar (um alerta); remove o alvo do fake; rodada 2 sem alvos → nenhum `STATUS_ALVOS`; recoloca o alvo, rodada 3 → um novo `ALERTA_QUEDA`.
- `TestRunStopsOnCancel`: `interval` 20 ms; `Run` numa goroutine; depois de ao menos 2 `STATUS_ALVOS`, cancela; `Run` retorna em até 1 s.

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/poller/`
Expected: FAIL (`undefined: NewScheduler`)

- [ ] **Step 3: Implementar `internal/poller/scheduler.go`**

- [ ] **Step 4: Rodar e ver passar**

Run: `go test -race ./internal/poller/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/poller
git commit -m "feat(poller): adiciona agendador com alertas de queda e retorno"
```

### Task 6: Métricas do sistema

**Files:**
- Create: `internal/sysmetrics/sysmetrics.go`
- Test: `internal/sysmetrics/sysmetrics_test.go`

**Interfaces:**
- Consumes: `ws.Event`, `ws.Publisher` (Task 2)
- Produces:
  - `type Snapshot struct { CPUPercent float64 \`json:"cpu"\`; MemPercent float64 \`json:"memoria"\`; DiskPercent float64 \`json:"disco"\`; At time.Time \`json:"hora"\` }`
  - `func Sample(ctx context.Context, diskPath string) (Snapshot, error)`
  - `func Run(ctx context.Context, pub ws.Publisher, diskPath string, interval time.Duration)`: publica `METRICAS_SISTEMA` com o `Snapshot` em `dados` a cada `interval`, até `ctx` ser cancelado; erro de `Sample` vai para `slog.Error` e o ciclo segue
  - Constante `DefaultInterval = 5 * time.Second`

`Sample` usa `cpu.PercentWithContext(ctx, 0, false)` (percentual desde a chamada anterior; a primeira chamada pode devolver 0), `mem.VirtualMemoryWithContext(ctx).UsedPercent` e `disk.UsageWithContext(ctx, diskPath).UsedPercent`. Em `run`, `diskPath` é `filepath.Dir(cfg.DBPath)`: o disco que enche primeiro é o que o Omni usa para gravar.

- [ ] **Step 1: Escrever os testes que falham**

- `TestSampleInRange`: `Sample(ctx, t.TempDir())` → sem erro; os três percentuais entre 0 e 100; `At` não zero.
- `TestSampleBadDiskPath`: caminho inexistente → erro.
- `TestRunPublishesUntilCancel`: `interval` 20 ms e um `fakePub` próprio do pacote (mesmo formato do da Task 5); recebe ao menos 2 eventos `METRICAS_SISTEMA` cujo `Data` é `Snapshot`; cancela; `Run` retorna em até 1 s.

- [ ] **Step 2: Rodar e ver falhar**

Run: `go get github.com/shirou/gopsutil/v4@v4.26.8 && go test ./internal/sysmetrics/`
Expected: FAIL (`undefined: Sample`). Confira que `go.mod` continua com `go 1.24.0`.

- [ ] **Step 3: Implementar `internal/sysmetrics/sysmetrics.go`**

- [ ] **Step 4: Rodar e ver passar**

Run: `go test -race ./internal/sysmetrics/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/sysmetrics go.mod go.sum
git commit -m "feat(poller): adiciona coleta de métricas do sistema"
```

### Task 7: Ligação no binário, documentação e PR

**Files:**
- Modify: `cmd/omni/run.go`, `README.md`, `docs/SPRINTS.md`
- Test: `cmd/omni/run_test.go`

**Interfaces:**
- Consumes: tudo das Tasks 1 a 6
- Produces: `run(ctx, cfg, ln)` com a mesma assinatura da Sprint 1

Ligação em `run`, depois de `store.Open`:
1. `hub := ws.NewHub(256)`, `tickets := ws.NewTickets(30 * time.Second)`.
2. `srv := api.NewServer(st, cfg.Token)`; `srv.Handle("POST /api/ws/ticket", ws.TicketHandler(tickets))`; `srv.Handle("GET /api/alvos/{id}/historico", api.HistoryHandler(st))`; `srv.HandlePublic("GET /ws", ws.Handler(hub, tickets, cfg.AllowedOrigins))`.
3. `httpSrv.RegisterOnShutdown(hub.Close)`.
4. Contexto filho `bgCtx, stopBg := context.WithCancel(ctx)` e um `sync.WaitGroup` com duas goroutines: `poller.NewScheduler(st, hub, cfg.PollInterval).Run(bgCtx)` e `sysmetrics.Run(bgCtx, hub, filepath.Dir(cfg.DBPath), sysmetrics.DefaultInterval)`.
5. Ordem de saída (no cancelamento **e** quando `Serve` falha): `Shutdown` do HTTP (que dispara `hub.Close`) → `stopBg()` → `wg.Wait()` → `st.Close()` (já no `defer`). Um `defer` com `stopBg(); wg.Wait()` registrado depois do `defer st.Close()` garante essa ordem nos dois caminhos.

- [ ] **Step 1: Escrever os testes que falham**

Helper `startRun` da Sprint 1 passa a aceitar a `config.Config` inteira (os testes existentes mandam `DBPath` e `Token`; os novos também `PollInterval: 50 * time.Millisecond` e `AllowedOrigins`). Helper `dialWS(t, base) *websocket.Conn`: `POST /api/ws/ticket` com o token e conexão em `ws://…/ws?ticket=…` sem `Origin`.

- `TestRunWiresHubAndPoller`: cria via API um alvo apontando para a porta de um `httptest.Server`; conecta no WebSocket; recebe um evento `STATUS_ALVOS` com `no_ar: true` em até 2 s.
- `TestRunServesHistory`: mesmo cenário; `GET /api/alvos/{id}/historico` com o token devolve ao menos 1 item em até 2 s (`require.Eventually`).
- `TestRunShutsDownWithOpenWebSocket`: conecta no WebSocket e não lê; cancela o contexto; `run` retorna `nil` em até 2 s e o `ReadMessage` do cliente devolve erro.
- `TestWebSocketRequiresTicket`: `GET /ws` sem ticket → 401; `POST /api/ws/ticket` sem token → 401.

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./cmd/omni/`
Expected: FAIL (rota `/api/ws/ticket` responde 404)

- [ ] **Step 3: Implementar a ligação em `cmd/omni/run.go`**

- [ ] **Step 4: Rodar e ver passar**

Run: `go test -race ./cmd/omni/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/omni
git commit -m "feat(poller): liga poller, websocket e métricas no binário"
```

- [ ] **Step 6: Verificação completa da sprint**

Run: `gofmt -l . && go vet ./... && golangci-lint run ./... && go build ./... && go test -race ./...`
Expected: nenhuma saída do `gofmt`, `0 issues` no linter, todos os pacotes `ok`. `grep '^go ' go.mod` → `go 1.24.0`.

Teste manual (precisa de `websocat`; sem ele, use o console do navegador com `new WebSocket(...)`):

```bash
export OMNI_TOKEN=$(openssl rand -hex 16) OMNI_DB=$(mktemp -d)/omni.db OMNI_INTERVALO_POLLER=2s
go run ./cmd/omni &
python3 -m http.server 3000 &                                   # aplicação de mentira
curl -s -H "Authorization: Bearer $OMNI_TOKEN" -d '{"nome":"api","host":"127.0.0.1","portas_app":[3000]}' 127.0.0.1:8080/api/alvos
T=$(curl -s -X POST -H "Authorization: Bearer $OMNI_TOKEN" 127.0.0.1:8080/api/ws/ticket | jq -r .ticket)
websocat "ws://127.0.0.1:8080/ws?ticket=$T"                     # STATUS_ALVOS a cada 2 s, METRICAS_SISTEMA a cada 5 s
# em outro terminal: kill %2 → um ALERTA_QUEDA; subir de novo → um SERVICO_RESTAURADO
curl -s -H "Authorization: Bearer $OMNI_TOKEN" 127.0.0.1:8080/api/alvos/1/historico | jq length
kill %1                                                         # sai em até 2 s, com o websocat ainda conectado
```

- [ ] **Step 7: Atualizar a documentação**

- `docs/SPRINTS.md`: Sprint 1 passa a `✅ Concluída` e Sprint 2 a `🚧 Em andamento` na tabela de resumo.
- `README.md`: linha de status vira `> **Status:** em implementação. A Sprint 1 (núcleo do backend) está concluída e a Sprint 2 (Poller e WebSocket) está em andamento; veja [docs/SPRINTS.md](docs/SPRINTS.md).`
- `README.md`, seção "Comunicação em tempo real": troque os exemplos de mensagem pelos formatos reais desta sprint (envelope `tipo`/`dados`), e acrescente que a conexão abre em `GET /ws?ticket=<ticket>`, com o ticket obtido em `POST /api/ws/ticket`.
- `README.md`, seção "Segurança da própria plataforma": acrescente que acesso por túnel numa porta local diferente de 8080 exige a origem em `OMNI_ORIGENS` (ex.: `OMNI_ORIGENS=http://localhost:9999`).

```bash
git add README.md docs/SPRINTS.md
git commit -m "docs: marca a sprint 2 como em andamento e documenta o websocket"
```

- [ ] **Step 8: Abrir o PR**

`git push -u origin feat/poller-e-websocket` e PR com título `feat(poller): adiciona poller, websocket e métricas do sistema`, base `main`, labels `funcionalidade` e `motor`, descrição com o que mudou, por quê e a saída real da verificação do Step 6. Depois de mesclado, a Sprint 2 vira `✅ Concluída` em `docs/SPRINTS.md` (no PR da Sprint 3 ou num PR `docs:` próprio).
