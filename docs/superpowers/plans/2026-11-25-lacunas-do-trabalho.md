# Lacunas do trabalho final: plano de implementação

> **Para agentes:** SUB-SKILL OBRIGATÓRIA: use superpowers:subagent-driven-development (recomendado) ou superpowers:executing-plans para implementar este plano tarefa por tarefa. Os passos usam checkbox (`- [ ]`).

**Objetivo:** cobrir as métricas do enunciado (seção 3 do PDF "Plataforma de Monitoramento para DevOps") que a `main` ainda não tem e atualizar o README.

**Arquitetura:** cada parte segue o caminho que o Omni já usa: coletor (sniffer, poller ou runner periódico) → evento WebSocket → tabela SQLite com retenção `OMNI_RETENCAO_VERIFICACOES` → rota `GET /api/...` → página React. Métricas novas que pedem alerta entram em `alerts.DefaultRules()` e no `alerts.Watcher`.

**Tech stack:** Go 1.24, gopacket/reassembly, gopsutil v4, modernc SQLite, pgx v5 (só na Parte 4), React + TypeScript, Recharts, Zustand, Vitest.

**Spec:** enunciado em PDF (fora do repositório), mais a análise de lacunas feita em 25/11/2026. Os requisitos que este plano cobre:

| Parte | Requisito do enunciado |
|---|---|
| 1 | Dashboards por serviço: DNS e SMTP sem tela própria |
| 2 | Web: requisições por segundo, contagem por código de erro, conexões ativas. Banco: conexões abertas e pools |
| 3 | Banco/servidor: I/O e latência de disco |
| 4 | Banco: transações que falharam e rollbacks, tamanho do banco e crescimento (GB/dia) |
| 5 | DNS: consultas por segundo. SMTP: taxa de entrega, fila, latência de envio, taxa de erros (rejeições, timeouts), volume por minuto e hora |
| 6 | README atualizado |

O documento ABNT fica com o autor e está fora deste plano.

## Restrições globais

- Base: `origin/main`. Cada parte vira um branch e um PR próprios, seguindo a skill `convencoes-omni` (branch `feat/<descricao>`, commits em português no presente, labels no PR).
- **Execute as partes em ordem.** As migrações têm número fixo (011 a 017) e o `store` só aplica versões maiores que a atual: mesclar a 013 antes da 012 faria a 012 ser pulada. Mudou a ordem? Renumere para o próximo número livre antes do PR.
- Toda tabela nova é apagada pela retenção `cfg.CheckRetention`, como as existentes.
- Nomes de campos JSON em português, como no resto da API (`hora`, `porta`, `consultas`...).
- Nunca guardar conteúdo sensível: nomes consultados no DNS, corpo e endereços de e-mail no SMTP, credenciais AUTH e o DSN do PostgreSQL não vão para o banco, para o log nem para o WebSocket.
- Verificação antes de cada commit: `gofmt -l .`, `go vet ./...`, `go test -race ./...` no backend; `npm run lint`, `npx tsc -b`, `npm test` em `web/`.
- Frontend: os limites coloridos dos gráficos repetem os das regras de alerta, como já faz `SystemPage.tsx`.

## Foco da revisão

1. **Sem alvo com porta DNS/SMTP, ou sem dados:** as páginas novas mostram estado vazio com link para Alvos, nunca quebram nem mostram `NaN`. Teste nas Tarefas 1.2 e 5.5.
2. **Contadores de disco que voltam para trás** (reboot, troca de dispositivo) ou intervalo zero: a taxa sai 0, nunca negativa nem infinita. Teste na Tarefa 3.1.
3. **SMTP com STARTTLS ou AUTH:** depois do `220` ao `STARTTLS` a conexão é ignorada, e as linhas depois de `AUTH` nunca são lidas. Teste na Tarefa 5.2.
4. **DNS sem resposta:** consulta pendente há mais de 5 s conta como timeout (falha), e o mapa de pendentes tem limite. Teste na Tarefa 5.1.
5. **`OMNI_PG_DSN` errado ou banco fora do ar:** o Omni sobe normalmente, registra o erro sem mostrar o DSN e tenta de novo no ciclo seguinte. Teste na Tarefa 4.2.

---

## Parte 1: tela de DNS e SMTP

Branch `feat/painel-dns-smtp`. Hoje o resultado das sondas vai ao WebSocket com `degradado`, `detalhe` e `starttls`, mas `check_results` não guarda esses campos, então o histórico não serve para uma tela própria.

### Tarefa 1.1: guardar degradado, detalhe e STARTTLS no histórico

**Arquivos:**
- Criar: `internal/store/migrations/011_check_details.sql`
- Modificar: `internal/store/checks.go` (`InsertCheckResult`, `CheckHistory`, `ChecksBetween`)
- Teste: `internal/store/checks_test.go`

**Interfaces:**
- Produz: `GET /api/alvos/{id}/historico` passa a devolver `degradado`, `detalhe` e `starttls` em cada `poller.Result`, sem mudar a rota.

- [ ] **Passo 1: teste que falha**

```go
func TestCheckHistoryKeepsProbeDetails(t *testing.T) {
	// grava um Result dns com Degraded=true, Detail="NXDOMAIN"
	// e um smtp com StartTLS=ptr(false), Detail="aceita destinatário postmaster"
	// CheckHistory devolve os dois com os mesmos Degraded, Detail e *StartTLS;
	// um Result http sai com StartTLS == nil
}
```

- [ ] **Passo 2:** `go test ./internal/store -run TestCheckHistoryKeepsProbeDetails` falha (campos vazios).
- [ ] **Passo 3:** migração `011_check_details.sql`:

```sql
ALTER TABLE check_results ADD COLUMN degraded INTEGER NOT NULL DEFAULT 0;
ALTER TABLE check_results ADD COLUMN detail TEXT NOT NULL DEFAULT '';
-- -1 = não se aplica (fora do SMTP), 0 = sem STARTTLS, 1 = com STARTTLS
ALTER TABLE check_results ADD COLUMN starttls INTEGER NOT NULL DEFAULT -1;
```

Grave e leia as três colunas nas três funções; `starttls` -1 vira `nil`.
- [ ] **Passo 4:** o teste passa, e `go test -race ./internal/store ./internal/api` continua passando.
- [ ] **Passo 5:** commit `feat(db): guarda degradado, detalhe e STARTTLS das sondas`.

### Tarefa 1.2: páginas DNS e SMTP

**Arquivos:**
- Criar: `web/src/pages/ProbeServicePage.tsx`, `web/src/pages/ProbeServicePage.test.tsx`, `web/src/lib/probes.ts`, `web/src/lib/probes.test.ts`
- Modificar: `web/src/router.tsx` (rotas `/dns` e `/smtp`), `web/src/components/Sidebar.tsx` (itens "DNS" e "SMTP" em Monitoramento, depois de "Consultas no banco"), `web/src/api/targets.ts` (tipo do resultado com `degradado?`, `detalhe?`, `starttls?`)

**Interfaces:**
- Consome: `GET /api/alvos/{id}/historico` (Tarefa 1.1) e os `STATUS_ALVOS` ao vivo já guardados no store.
- Produz:
  - `summarizeProbes(results: CheckResult[]): { disponibilidade: number | null; latenciaMedia: number | null; latenciaP95: number | null; taxaFalhas: number | null; ultimo?: CheckResult }`, em `lib/probes.ts`
  - `<ProbeServicePage kind="dns" | "smtp" />`. A Parte 5 acrescenta cartões a ela.

- [ ] **Passo 1: testes que falham.** Em `probes.test.ts`:
  - lista vazia devolve tudo `null`;
  - 3 resultados `no_ar` e 1 fora do ar: `disponibilidade` 75 e `taxaFalhas` 25. Degradado conta como no ar para a disponibilidade e como falha para a taxa, como em `alerts.Watcher.failureRate`;
  - a latência ignora resultados fora do ar.

  Em `ProbeServicePage.test.tsx`:
  - `"shows availability, latency and the last detail for the selected target's DNS port"`;
  - `"SMTP shows the STARTTLS state"`, com o texto "Sem STARTTLS" em tom de aviso;
  - `"without a DNS port on the target, shows an empty state linking to /alvos"`.
- [ ] **Passo 2:** `npm test -- probes ProbeServicePage` falha.
- [ ] **Passo 3:** implemente. A página usa o alvo do seletor global e filtra o histórico pela porta `porta_dns` ou `porta_smtp`. O layout segue `SystemPage`: uma linha de `Stat` (Disponibilidade, Latência média, p95, Taxa de falhas, mais STARTTLS no SMTP), um `CountChart` de latência (`label="Latência (ms)"`) e uma tabela com as últimas 20 verificações (hora, no ar, latência, detalhe). Os limites de cor repetem `dns_latencia` (200/1000 ms) e `smtp_latencia` (2000/5000 ms).
- [ ] **Passo 4:** `npm test`, `npm run lint` e `npx tsc -b` passam.
- [ ] **Passo 5:** commit `feat(frontend): adiciona telas de DNS e SMTP`.

---

## Parte 2: métricas de Web, códigos de erro e conexões

Branch `feat/conexoes-e-rps`.

### Tarefa 2.1: requisições e consultas por segundo

**Arquivos:**
- Modificar: `web/src/lib/l7.ts`, `web/src/lib/l7.test.ts`, `web/src/pages/OverviewPage.tsx`, `web/src/pages/RequestsPage.tsx`, `web/src/pages/QueriesPage.tsx` e os testes das três páginas.

**Interfaces:**
- Produz: `perSecond(count: number, stepMs = L7_STEP_MS): number` e `lastRate<T extends Windowed>(windows: T[], pick: (w: T) => number): number | null`. `lastRate` devolve `null` sem janelas ou quando a última janela tem mais de 2 × `L7_STEP_MS` (dado parado).

- [ ] **Passo 1:** testes que falham: `perSecond(42)` vale `4.2`; `lastRate([])` é `null`; `lastRate` de uma janela de 30 s atrás é `null`. Nas páginas: `"shows requests per second from the last window"` (Requisições), `"shows queries per second"` (Consultas) e, na Geral, `Stat` "Requisições por segundo" e "Consultas por segundo".
- [ ] **Passo 2:** `npm test` falha nesses casos.
- [ ] **Passo 3:** implemente. Na Geral, a linha de `Stat` passa a ter 5 itens: `grid-cols-2 sm:grid-cols-3 lg:grid-cols-5`.
- [ ] **Passo 4:** `npm test`, `npm run lint` e `npx tsc -b` passam.
- [ ] **Passo 5:** commit `feat(frontend): mostra requisições e consultas por segundo`.

### Tarefa 2.2: contagem por código de erro HTTP

**Arquivos:**
- Criar: `internal/store/migrations/012_http_codes.sql`
- Modificar: `internal/sniffer/l7/httpstats.go`, `internal/sniffer/l7/httpstats_test.go`, `internal/store/http.go`, `internal/store/http_test.go`, `web/src/api/requests.ts`, `web/src/pages/RequestsPage.tsx` e o teste dela.

**Interfaces:**
- Produz: o campo `HTTPWindow.ByCode map[string]uint64` com tag `json:"codigos"`, que só conta status ≥ 400, com a chave sendo o código em texto (`"404"`). Ele sai no `METRICAS_HTTP` e em `GET /api/requisicoes`.

- [ ] **Passo 1:** testes que falham:
  - `TestHTTPStatsCountsErrorCodes`: as entradas 200, 404, 404 e 503 dão `ByCode == {"404": 2, "503": 1}`, e o `Flush` zera o mapa;
  - `TestHTTPWindowRoundTripsCodes` no store;
  - no front, `"lists error codes summed over the loaded windows"`, com uma tabela "Códigos de erro (15 min)" ordenada pela contagem.
- [ ] **Passo 2:** os testes falham.
- [ ] **Passo 3:** a migração é `ALTER TABLE http_windows ADD COLUMN by_code TEXT NOT NULL DEFAULT '{}';`. Grave e leia como JSON, igual a `by_status`.
- [ ] **Passo 4:** `go test -race ./...` e `npm test` passam.
- [ ] **Passo 5:** commit `feat(sniffer): conta respostas HTTP por código de erro`.

### Tarefa 2.3: coletor de conexões por porta

**Arquivos:**
- Criar: `internal/connstats/connstats.go`, `internal/connstats/connstats_test.go`, `internal/store/migrations/013_connections.sql`, `internal/store/connections.go`, `internal/store/connections_test.go`, `internal/api/connections.go`, `internal/api/connections_test.go`
- Modificar: `cmd/omni/run.go` (goroutine e rota)

**Interfaces:**
- Produz:

```go
package connstats

const EventConnections = "METRICAS_CONEXOES"
const DefaultInterval = 5 * time.Second

type PortRole struct { Port int; Kind string } // Kind: "app", "banco" ou "smtp"

type ClientCount struct {
	Process string `json:"processo"` // nome do processo cliente; "" sem permissão
	Conns   int    `json:"conexoes"`
}

type PortConns struct {
	Port        int            `json:"porta"`
	Kind        string         `json:"tipo"`
	Established int            `json:"estabelecidas"`
	States      map[string]int `json:"estados"`  // "ESTABLISHED", "TIME_WAIT", ...
	Clients     []ClientCount  `json:"clientes"` // o "pool" de cada processo cliente, maior primeiro, no máximo 10
}

type Snapshot struct {
	At    time.Time   `json:"hora"`
	Ports []PortConns `json:"portas"`
}

// Summarize é a parte pura: conta as conexões do lado servidor (Laddr.Port ==
// porta) por estado e agrupa as do lado cliente (Raddr.Port == porta,
// ESTABLISHED) por nome de processo.
func Summarize(conns []gnet.ConnectionStat, roles []PortRole, names func(pid int32) string) []PortConns

type Sink interface {
	AddConnSnapshot(ctx context.Context, s Snapshot) error
	PruneConnections(ctx context.Context, before time.Time) error
}

func Run(ctx context.Context, pub ws.Publisher, sink Sink, targets func(context.Context) ([]target.Target, error), interval, retention time.Duration)
```

  Mais `store.ConnectionsSince(ctx, since time.Time, limit int) ([]connstats.Snapshot, error)` e `GET /api/conexoes?desde=<RFC3339>` (padrão 15 min, no máximo 24 h, como `/api/sistema`).

- [ ] **Passo 1:** testes que falham:
  - `TestSummarizeCountsServerSideByState`: 3 ESTABLISHED e 1 TIME_WAIT com `Laddr.Port` 3000 dão `Established == 3` e `States["TIME_WAIT"] == 1`;
  - `TestSummarizeGroupsClientPoolsByProcess`: 5 conexões com `Raddr.Port` 5432 dos PIDs 10 (`loja`) e 11 (`psql`) dão `Clients == [{loja 4} {psql 1}]`;
  - `TestSummarizeUnknownPIDGroupsAsEmptyName`: PID 0 vira `Process == ""`;
  - `TestConnectionsSinceRoundTrip` no store;
  - `TestConnectionsHandlerDefaultsTo15Minutes` na API.
- [ ] **Passo 2:** os testes falham.
- [ ] **Passo 3:** implemente. `Run` chama `gnet.ConnectionsWithContext(ctx, "tcp")` (gopsutil `net`) a cada intervalo. Os nomes de processo vêm de `process.NewProcessWithContext(...).Name()` com cache por PID, limpo a cada ciclo. Os papéis saem dos alvos: `AppPorts` → "app", `DBPort` → "banco", `SMTPPort` → "smtp". A migração cria `conn_samples(time_ms INTEGER, port INTEGER, kind TEXT, established INTEGER, states TEXT, clients TEXT, PRIMARY KEY (time_ms, port))`.
- [ ] **Passo 4:** `go test -race ./...` passa, e `./omni` com a loja no ar publica `METRICAS_CONEXOES` (confira no DevTools, aba WS).
- [ ] **Passo 5:** commit `feat(motor): mede conexões abertas e pools por porta`.

### Tarefa 2.4: conexões na interface

**Arquivos:**
- Criar: `web/src/api/connections.ts`
- Modificar: `web/src/store/app.ts` (fatia `connections`), `web/src/realtime/events.ts` (caso `METRICAS_CONEXOES`), `web/src/pages/RequestsPage.tsx`, `web/src/pages/QueriesPage.tsx`, `web/src/pages/OverviewPage.tsx` e os testes delas.

**Interfaces:**
- Consome: `GET /api/conexoes` e `METRICAS_CONEXOES` (Tarefa 2.3).
- Produz: `connectionSeries(snaps: ConnSnapshot[], kind: "app" | "banco"): CountPoint[]`, que soma `estabelecidas` das portas do tipo.

- [ ] **Passo 1:** testes que falham:
  - `"Requests page shows active connections chart for app ports"`;
  - `"Queries page lists the client pools of the database port"`, com uma tabela Processo / Conexões em que nome vazio aparece como "(sem permissão para ver)";
  - a Geral ganha o `Stat` "Conexões ativas".
- [ ] **Passo 2:** os testes falham.
- [ ] **Passo 3:** implemente.
- [ ] **Passo 4:** `npm test`, `npm run lint` e `npx tsc -b` passam.
- [ ] **Passo 5:** commit `feat(frontend): mostra conexões ativas e pools do banco`.

---

## Parte 3: I/O e latência de disco

Branch `feat/io-de-disco`.

### Tarefa 3.1: amostrador com I/O de disco

**Arquivos:**
- Modificar: `internal/sysmetrics/sysmetrics.go`, `internal/sysmetrics/sysmetrics_test.go`, `internal/store/system.go`, `internal/store/system_test.go`, `cmd/omni/run.go`
- Criar: `internal/store/migrations/014_system_io.sql`

**Interfaces:**
- Produz:

```go
type Snapshot struct {
	CPUPercent, MemPercent, DiskPercent float64 // como hoje
	IO            bool    `json:"io"` // false quando o dispositivo do disco não foi encontrado
	ReadBps       float64 `json:"leitura_bps"`
	WriteBps      float64 `json:"escrita_bps"`
	IOPS          float64 `json:"iops"`
	DiskLatencyMS float64 `json:"latencia_disco_ms"` // tempo médio por operação no intervalo
	DiskBusy      float64 `json:"disco_ocupado"`     // % do intervalo com I/O em andamento
	At            time.Time `json:"hora"`
}

// deviceFor devolve o nome do dispositivo (como "sda1" ou "nvme0n1p2") da
// partição de mountpoint mais longo que contém path; "" quando nenhuma contém.
func deviceFor(path string, parts []disk.PartitionStat) string

// ioRates calcula as taxas entre duas leituras. dt <= 0 ou contador menor
// que o anterior devolve zeros.
func ioRates(prev, cur disk.IOCountersStat, dt time.Duration) (readBps, writeBps, iops, latencyMS, busy float64)

type Sampler struct { /* diskPath, device, prev, prevAt */ }
func NewSampler(diskPath string) *Sampler
func (s *Sampler) Sample(ctx context.Context) (Snapshot, error)
```

  `Run` passa a criar um `Sampler`, mantendo a assinatura atual.

- [ ] **Passo 1:** testes que falham:
  - `TestDeviceForPicksLongestMountpoint`: com `/` → `/dev/sda1` e `/var` → `/dev/sdb1`, o caminho `/var/lib/omni` dá `sdb1`;
  - `TestIORates`: Δ de 10 s com `ReadBytes` +1 MiB, `ReadCount` +50, `WriteCount` +50, `ReadTime+WriteTime` +500 ms e `IoTime` +2000 ms dão 104857,6 B/s de leitura, 10 IOPS, latência de 5 ms e 20% ocupado;
  - `TestIORatesCounterReset`: contador menor que o anterior dá zeros;
  - `TestIORatesZeroOps`: sem operações, a latência é 0, não NaN;
  - `TestSystemSinceRoundTripsIO` no store.
- [ ] **Passo 2:** os testes falham.
- [ ] **Passo 3:** implemente. A migração `014_system_io.sql` acrescenta as colunas `io INTEGER NOT NULL DEFAULT 0`, `read_bps REAL NOT NULL DEFAULT 0`, `write_bps REAL NOT NULL DEFAULT 0`, `iops REAL NOT NULL DEFAULT 0`, `disk_latency_ms REAL NOT NULL DEFAULT 0` e `disk_busy REAL NOT NULL DEFAULT 0` a `system_metrics`. As médias por minuto de `SystemSince` e `SystemBetween` incluem as colunas novas, e `io` usa `MAX`. A primeira leitura sai com `IO: false`, porque ainda não há leitura anterior.
- [ ] **Passo 4:** `go test -race ./...` passa.
- [ ] **Passo 5:** commit `feat(motor): coleta I/O e latência do disco`.

### Tarefa 3.2: alerta e gráficos de disco

**Arquivos:**
- Modificar: `internal/alerts/rules.go`, `internal/alerts/watch.go`, `internal/alerts/watch_test.go`, `web/src/store/app.ts` (tipo `SystemMetrics`), `web/src/pages/SystemPage.tsx` e o teste dela.

**Interfaces:**
- Produz: a regra `RuleDiskLatency = "sistema_latencia_disco"`, no serviço Sistema, com nome "Latência do disco", unidade "ms", Warn 20, Crit 100 e For 1 min.

- [ ] **Passo 1:** testes que falham:
  - `TestWatcherDiskLatency`: `METRICAS_SISTEMA` com `io: true` e `latencia_disco_ms: 150` por 1 min dá vermelho, e com `io: false` não gera amostra;
  - no front, `"shows disk throughput and latency when io is available"` e `"hides the I/O cards when io is false"`.
- [ ] **Passo 2:** os testes falham.
- [ ] **Passo 3:** implemente. A `SystemPage` ganha os `Stat` Leitura, Escrita, IOPS e Latência do disco, mais dois gráficos: um com leitura/escrita em bytes/s (reaproveite `BandwidthChart` ou `CountChart`) e um com a latência, com limites 20/100.
- [ ] **Passo 4:** os testes de Go e do front passam.
- [ ] **Passo 5:** commit `feat(alertas): alerta de latência do disco`.

---

## Parte 4: transações, tamanho e crescimento do banco

Branch `feat/banco-transacoes-e-tamanho`. Os rollbacks pedidos pelo enunciado já existem: são `DBWindow.ByKind["ROLLBACK"]`, porque `QueryKind` reconhece `ROLLBACK`. Falta mostrar esse número e contar as transações que falharam.

### Tarefa 4.1: transações que falharam (passivo)

**Arquivos:**
- Criar: `internal/store/migrations/015_db_tx.sql`
- Modificar: `internal/sniffer/l7/postgres.go`, `internal/sniffer/l7/postgres_test.go`, `internal/sniffer/l7/dbstats.go`, `internal/sniffer/l7/dbstats_test.go`, `internal/sniffer/engine.go`, `internal/store/db_queries.go` e o teste dele, `web/src/api/queries.ts`, `web/src/pages/QueriesPage.tsx` e o teste dela.

**Interfaces:**
- Produz:
  - `func (a *PostgresAssembler) OnTxFailed(fn func(at time.Time))`, chamado quando o `ReadyForQuery` (`'Z'`) traz o status `'E'` (transação abortada) e o status anterior da conexão não era `'E'`;
  - `func (s *DBStats) AddTxFailure()`;
  - o campo `DBWindow.TxFailed uint64` com tag `json:"transacoes_falhas"`.

- [ ] **Passo 1:** testes que falham:
  - `TestPostgresTxFailedOncePerAbortedBlock`: a sequência `BEGIN`, um erro (`Z` com `E`), outra consulta ignorada (`Z` com `E`) e o `ROLLBACK` (`Z` com `I`) chama `OnTxFailed` uma única vez;
  - `TestDBStatsCountsTxFailures`;
  - `TestDBWindowRoundTripsTxFailed` no store;
  - no front, `"shows rollbacks and failed transactions per window"`, com os `Stat` "Rollbacks (15 min)" e "Transações que falharam (15 min)".
- [ ] **Passo 2:** os testes falham.
- [ ] **Passo 3:** implemente. `serverMessage` passa a ler o primeiro byte do corpo do `'Z'`, e `pgConn` ganha o campo `txStatus byte`. A migração é `ALTER TABLE db_windows ADD COLUMN tx_failed INTEGER NOT NULL DEFAULT 0;`. Em `engine.go`, ligue `pgAsm.OnTxFailed(func(time.Time) { dbStats.AddTxFailure() })`.
- [ ] **Passo 4:** `go test -race ./...` e `npm test` passam.
- [ ] **Passo 5:** commit `feat(sniffer): conta transações do PostgreSQL que falharam`.

### Tarefa 4.2: tamanho e crescimento do banco (opcional, com DSN)

O tamanho só existe dentro do PostgreSQL. A pasta de dados (`/var/lib/postgresql`) pertence ao usuário `postgres` com permissão 700, então o Omni não consegue medi-la de fora. Por isso esta tarefa pede uma conexão opcional, só de leitura, com um papel de monitoramento. Sem a conexão, a tela diz como ligar.

**Arquivos:**
- Criar: `internal/dbsize/dbsize.go`, `internal/dbsize/dbsize_test.go`, `internal/store/migrations/016_db_sizes.sql`, `internal/store/db_sizes.go`, `internal/store/db_sizes_test.go`, `internal/api/db_size.go`, `internal/api/db_size_test.go`
- Modificar: `internal/config/config.go` e o teste dele, `cmd/omni/run.go`, `go.mod` (`github.com/jackc/pgx/v5 v5.7.6`, a mesma versão da loja), `web/src/api/queries.ts`, `web/src/pages/QueriesPage.tsx` e o teste dela.

**Interfaces:**
- Produz:

```go
// config
PGDSN string // de OMNI_PG_DSN ou do arquivo em OMNI_PG_DSN_ARQUIVO (preferível); vazio desliga

package dbsize
const DefaultInterval = 5 * time.Minute
type Sample struct { Name string `json:"banco"`; Bytes int64 `json:"bytes"`; At time.Time `json:"hora"` }
type Querier func(ctx context.Context) ([]Sample, error) // pgx: SELECT datname, pg_database_size(datname) FROM pg_database WHERE NOT datistemplate
type Sink interface {
	AddDBSizes(ctx context.Context, list []Sample) error
	PruneDBSizes(ctx context.Context, before time.Time) error
}
func Run(ctx context.Context, q Querier, sink Sink, interval, retention time.Duration)

// Growth devolve o crescimento em GB/dia por regressão linear das amostras
// (bytes × tempo). Devolve ok=false com menos de 2 amostras ou quando elas
// cobrem menos de 1 h.
func Growth(samples []Sample) (gbPerDay float64, ok bool)

// store
func (s *Store) DBSizesSince(ctx context.Context, since time.Time) ([]dbsize.Sample, error)
```

  A rota `GET /api/banco/tamanho` responde `{"configurado": bool, "bancos": [{"banco", "bytes", "crescimento_gb_dia": number|null, "serie": [{"hora", "bytes"}]}]}` com as últimas 24 h.

- [ ] **Passo 1:** testes que falham:
  - `TestGrowthLinear`: 1 GiB às 0 h e 1,5 GiB às 12 h dão cerca de 1,0737 GB/dia (GB = 10⁹ bytes, tolerância 1e-3);
  - `TestGrowthTooShort`: amostras com 30 min de distância dão `ok == false`;
  - `TestRunKeepsGoingWhenQuerierFails`: um `Querier` que falha não derruba o `Run`, e o erro registrado não contém o DSN;
  - `TestLoadReadsPGDSNFromFile` em config;
  - `TestDBSizeHandlerNotConfigured`: devolve `configurado: false`;
  - no front, `"shows database size and growth"` e `"explains how to enable size monitoring when not configured"`.
- [ ] **Passo 2:** os testes falham.
- [ ] **Passo 3:** implemente. Use `pgx.Connect` com `connect_timeout` de 5 s e uma conexão por ciclo, fechada no fim. Com o DSN vazio, `run.go` não sobe a goroutine. A migração cria `db_sizes(time_ms INTEGER, name TEXT, bytes INTEGER, PRIMARY KEY (time_ms, name))`. A mensagem da tela sem configuração diz: "Defina `OMNI_PG_DSN_ARQUIVO` com uma conexão de um papel com `pg_monitor`."
- [ ] **Passo 4:** `go test -race ./...` e `npm test` passam. Teste também manualmente com a loja: `CREATE ROLE omni LOGIN PASSWORD '...' IN ROLE pg_monitor;`.
- [ ] **Passo 5:** commit `feat(motor): mede tamanho e crescimento do PostgreSQL`.

---

## Parte 5: DNS e SMTP pelo tráfego real

Branch `feat/dns-smtp-passivo`. O sniffer passa a capturar a porta DNS (UDP) e a porta SMTP (TCP) dos alvos.

### Tarefa 5.1: decodificador e janelas do DNS

**Arquivos:**
- Criar: `internal/sniffer/l7/dns.go`, `internal/sniffer/l7/dns_test.go`, `internal/sniffer/l7/dnsstats.go`, `internal/sniffer/l7/dnsstats_test.go`

**Interfaces:**
- Produz:

```go
type DNSExchange struct {
	RCode   string        // "NOERROR", "NXDOMAIN", "SERVFAIL"...; "TIMEOUT" sem resposta
	Failed  bool          // RCode diferente de NOERROR, ou timeout
	Latency time.Duration
	At      time.Time
}
// O nome consultado nunca sai daqui.

type DNSTracker struct{ /* pendentes por (ip e porta do cliente, ID) */ }
func NewDNSTracker(dnsPorts []int, emit func(DNSExchange)) *DNSTracker
func (t *DNSTracker) Add(p gopacket.Packet)
func (t *DNSTracker) Flush(now time.Time) // emite TIMEOUT para pendentes com mais de dnsTimeout

const dnsTimeout = 5 * time.Second
const maxDNSPending = 10_000

type DNSWindow struct {
	Start    time.Time         `json:"inicio"`
	Queries  uint64            `json:"consultas"`
	Failures uint64            `json:"falhas"`
	ByRCode  map[string]uint64 `json:"rcodes"`
	P50MS    float64           `json:"p50_ms"`
	P95MS    float64           `json:"p95_ms"`
}
type DNSStats struct{}
func NewDNSStats() *DNSStats
func (s *DNSStats) Add(ex DNSExchange)
func (s *DNSStats) Flush(now time.Time) DNSWindow
```

- [ ] **Passo 1:** testes que falham. Monte os pacotes com `internal/sniffer/testutil` e `layers.DNS` serializado:
  - `TestDNSTrackerPairsQueryAndResponse`: resposta 3 ms depois dá `Latency == 3ms` e `RCode "NOERROR"`;
  - `TestDNSTrackerNXDOMAINIsFailure`;
  - `TestDNSTrackerTimeoutOnFlush`: pendente há 6 s dá `RCode "TIMEOUT"` e `Failed`;
  - `TestDNSTrackerIgnoresOtherPortsAndGarbage`: um payload aleatório na porta DNS não causa panic nem emissão;
  - `TestDNSTrackerBoundsPending`: 10 001 consultas sem resposta deixam no máximo 10 000 pendentes, e a mais antiga sai como TIMEOUT;
  - `TestDNSStatsWindow`.
- [ ] **Passo 2:** os testes falham.
- [ ] **Passo 3:** implemente. Decodifique o payload UDP com `(&layers.DNS{}).DecodeFromBytes`, sem depender do gopacket reconhecer a porta, porque 1053 não é a 53. DNS sobre TCP fica de fora; anote isso no README (Parte 6). Para os quantis, reaproveite o `group` de `stats.go`.
- [ ] **Passo 4:** `go test -race ./internal/sniffer/...` passa.
- [ ] **Passo 5:** commit `feat(sniffer): decodifica consultas DNS do tráfego`.

### Tarefa 5.2: decodificador e janelas do SMTP

**Arquivos:**
- Criar: `internal/sniffer/l7/smtp.go`, `internal/sniffer/l7/smtp_test.go`, `internal/sniffer/l7/smtpstats.go`, `internal/sniffer/l7/smtpstats_test.go`

**Interfaces:**
- Produz:

```go
type SMTPExchange struct {
	Outcome    string        // "entregue", "rejeitado" ou "abortado" (conexão caiu no meio)
	Code       int           // resposta final (250, 550...); 0 quando abortado
	Recipients int           // RCPT TO aceitos
	Latency    time.Duration // do fim do DATA ("\r\n.\r\n") até a resposta; 0 se não chegou ao DATA
	At         time.Time
}

func NewSMTPAssembler(smtpPorts []int, emit func(SMTPExchange)) *SMTPAssembler // usa newStreamAssembler, como o HTTP
func (a *SMTPAssembler) Add(p gopacket.Packet)
func (a *SMTPAssembler) Flush(now time.Time)

type SMTPWindow struct {
	Start     time.Time         `json:"inicio"`
	Attempts  uint64            `json:"tentativas"` // transações iniciadas com MAIL FROM
	Delivered uint64            `json:"entregues"`
	Rejected  uint64            `json:"rejeitados"`
	Aborted   uint64            `json:"abortados"`
	ByCode    map[string]uint64 `json:"codigos"` // só 4xx e 5xx
	P50MS     float64           `json:"p50_ms"`
	P95MS     float64           `json:"p95_ms"`
}
type SMTPStats struct{}
func NewSMTPStats() *SMTPStats
func (s *SMTPStats) Add(ex SMTPExchange)
func (s *SMTPStats) Flush(now time.Time) SMTPWindow
```

  Uma transação é rejeitada quando `MAIL FROM`, todos os `RCPT TO` ou o fim do `DATA` recebem 4xx ou 5xx. Uma transação aberta quando o fluxo fecha ou fica ocioso conta como abortada; é o "timeout" do enunciado.

- [ ] **Passo 1:** testes que falham:
  - `TestSMTPDelivered`: `EHLO`, `MAIL`, 2 `RCPT`, `DATA`, `354`, o corpo, `.` e a resposta `250` 40 ms depois dão `Outcome "entregue"`, `Recipients 2` e `Latency 40ms`;
  - `TestSMTPRejectedAtRcpt`: o único `RCPT` recebe 550, e o resultado é `"rejeitado"` com `Code 550`;
  - `TestSMTPPipelinedCommands`: `MAIL`, `RCPT` e `DATA` num só segmento;
  - `TestSMTPBodyTerminatorSplitAcrossSegments`: o `\r\n.\r\n` vem dividido em dois pacotes;
  - `TestSMTPStopsAfterStartTLS`: depois de `STARTTLS` e da resposta `220` não sai nenhuma emissão, e o parser ignora o resto;
  - `TestSMTPNeverReadsAuth`: depois de `AUTH LOGIN`, as linhas em base64 não chegam ao parser de comandos até a resposta 235 ou 535 (asserção: um comando falso `MAIL FROM` dentro do AUTH não abre transação);
  - `TestSMTPAbortedOnClose`;
  - `TestSMTPStatsWindow`.
- [ ] **Passo 2:** os testes falham.
- [ ] **Passo 3:** implemente. O parser lê linhas de comando do cliente e respostas do servidor (código de 3 dígitos; as linhas `250-` continuam a resposta). Durante o DATA, ele só procura o terminador e nunca guarda o corpo. Endereços de `MAIL FROM` e `RCPT TO` não são guardados.
- [ ] **Passo 4:** `go test -race ./internal/sniffer/...` passa.
- [ ] **Passo 5:** commit `feat(sniffer): decodifica transações SMTP do tráfego`.

### Tarefa 5.3: ligar DNS e SMTP ao motor, banco e API

**Arquivos:**
- Criar: `internal/store/migrations/017_dns_smtp_windows.sql`, `internal/store/dns_smtp.go`, `internal/store/dns_smtp_test.go`, `internal/api/dns_smtp.go`, `internal/api/dns_smtp_test.go`
- Modificar: `internal/sniffer/source.go` (`BPFForTargets`), `internal/sniffer/source_test.go`, `internal/sniffer/classify.go`, `internal/sniffer/aggregate.go` e testes, `internal/sniffer/engine.go`, `internal/sniffer/engine_test.go`, `internal/store/traffic.go` (`snifferTables`), `cmd/omni/run.go`

**Interfaces:**
- Consome: as Tarefas 5.1 e 5.2.
- Produz:
  - `EventDNS = "METRICAS_DNS"` e `EventSMTP = "METRICAS_SMTP"`, a cada 10 s, como o HTTP;
  - `TrafficStore` ganha `InsertDNSWindow(ctx, l7.DNSWindow) error` e `InsertSMTPWindow(ctx, l7.SMTPWindow) error`;
  - `store.DNSSince(ctx, since, limit) ([]l7.DNSWindow, error)` e `store.SMTPSince(ctx, since, limit) ([]l7.SMTPWindow, error)`;
  - as rotas `GET /api/dns?desde=` e `GET /api/smtp?desde=` (padrão 15 min, máximo 24 h);
  - `type PortRoles struct{ App, DNS, SMTP []int }`; `NewAggregator(roles PortRoles)` e `Classify(src, dst uint16, isUDP bool, roles PortRoles) string`. As portas DNS e SMTP dos alvos são classificadas como "DNS" e "SMTP".

- [ ] **Passo 1:** testes que falham:
  - `TestBPFIncludesDNSUDPAndSMTP`: um alvo com app 3000, DNS 1053 e SMTP 1025 dá exatamente `tcp port 1025 or tcp port 3000 or udp port 1053`, com as TCP em ordem crescente e depois as UDP;
  - `TestClassifyCustomDNSPort`;
  - `TestEnginePublishesDNSAndSMTPWindows`, com a fonte falsa de `fakesource_test.go`;
  - testes de ida e volta no store;
  - `TestDNSHandler` e `TestSMTPHandler`.
- [ ] **Passo 2:** os testes falham.
- [ ] **Passo 3:** implemente. As tabelas são `dns_windows(start_ms INTEGER PRIMARY KEY, queries, failures, by_rcode TEXT, p50_ms, p95_ms)` e `smtp_windows(start_ms INTEGER PRIMARY KEY, attempts, delivered, rejected, aborted, by_code TEXT, p50_ms, p95_ms)`. As duas entram em `snifferTables`, para a retenção valer. A mesma migração cria `smtp_queue(time_ms INTEGER PRIMARY KEY, messages INTEGER NOT NULL)`, usada na Tarefa 5.4. Janelas vazias não são gravadas, como no `emitHTTP`.
- [ ] **Passo 4:** `go test -race ./...` passa.
- [ ] **Passo 5:** commit `feat(sniffer): publica métricas de DNS e SMTP do tráfego`.

### Tarefa 5.4: fila do SMTP e alertas

**Arquivos:**
- Criar: `internal/mailqueue/mailqueue.go`, `internal/mailqueue/mailqueue_test.go`
- Modificar: `internal/config/config.go` e o teste dele, `internal/store/dns_smtp.go` e o teste dele (`AddQueueSnapshot` e `PruneQueue`, sobre a tabela `smtp_queue` criada na Tarefa 5.3), `internal/alerts/rules.go`, `internal/alerts/watch.go`, `internal/alerts/watch_test.go`, `cmd/omni/run.go`

**Interfaces:**
- Produz:

```go
// config
SMTPQueueDirs []string // OMNI_SMTP_FILA_DIRS, separados por vírgula; vazio desliga

package mailqueue
const EventQueue = "FILA_SMTP"
const DefaultInterval = 30 * time.Second
type Snapshot struct { At time.Time `json:"hora"`; Messages int `json:"mensagens"`; Err string `json:"erro,omitempty"` }
// Count conta os arquivos regulares sob os diretórios, sem abrir nenhum e
// sem seguir links simbólicos.
func Count(dirs []string) (int, error)
type Sink interface {
	AddQueueSnapshot(ctx context.Context, s Snapshot) error
	PruneQueue(ctx context.Context, before time.Time) error
}
func Run(ctx context.Context, pub ws.Publisher, sink Sink, dirs []string, interval, retention time.Duration)
```

  Para o Postfix, os diretórios são `/var/spool/postfix/active` e `/var/spool/postfix/deferred`. O usuário do Omni precisa de leitura neles (ACL ou grupo `postdrop`); documente isso no README.

  Regras novas em `DefaultRules()`:

  | ID | Serviço | Nome | Unidade | Amarelo | Vermelho | Por |
  |---|---|---|---|---|---|---|
  | `dns_falhas_trafego` | dns | Falhas de resolução no tráfego | % | 5 | 20 | 30 s |
  | `smtp_rejeicoes` | smtp | Taxa de rejeição de e-mails | % | 10 | 30 | 30 s |
  | `smtp_fila` | smtp | Tamanho da fila de e-mails | msgs | 50 | 200 | 5 min |

  As taxas só contam com pelo menos 20 consultas DNS ou 10 tentativas SMTP na janela, como `minHTTPRequests`. A taxa de rejeição é `(rejeitados + abortados) / tentativas`.

- [ ] **Passo 1:** testes que falham:
  - `TestCountIgnoresSymlinksAndDirs`;
  - `TestCountMissingDirReturnsError`;
  - `TestWatcherDNSFailureRate`: 30 consultas com 9 falhas por 30 s dão vermelho (30%);
  - `TestWatcherSMTPRejectionRate`: 10 tentativas com 2 rejeitadas dão amarelo; 5 tentativas não geram amostra;
  - `TestWatcherSMTPQueue`.
- [ ] **Passo 2:** os testes falham.
- [ ] **Passo 3:** implemente. `Watcher.Feed` aceita `METRICAS_DNS`, `METRICAS_SMTP` e `FILA_SMTP`.
- [ ] **Passo 4:** `go test -race ./...` passa.
- [ ] **Passo 5:** commit `feat(alertas): alerta de falhas DNS, rejeições e fila do SMTP`.

### Tarefa 5.5: DNS e SMTP passivos nas telas e carga de teste

**Arquivos:**
- Criar: `web/src/api/dnsSmtp.ts`, `subjects/loja/internal/load/dnsmail.go`, `subjects/loja/internal/load/dnsmail_test.go`
- Modificar: `web/src/store/app.ts`, `web/src/realtime/events.ts`, `web/src/pages/ProbeServicePage.tsx` e o teste dela (da Tarefa 1.2), `subjects/loja/cmd/carga/main.go`, `subjects/loja/README.md`

**Interfaces:**
- Consome: `GET /api/dns`, `GET /api/smtp`, `METRICAS_DNS`, `METRICAS_SMTP` e `FILA_SMTP`.
- Produz: na carga, as opções `-dns 127.0.0.1:1053`, `-dns-nome loja.local`, `-dns-taxa 5` (consultas/s), `-smtp 127.0.0.1:1025` e `-emails-por-minuto 30`. Elas rodam junto da carga HTTP e ficam desligadas quando o endereço é vazio.

- [ ] **Passo 1:** testes que falham:
  - página DNS: `"shows queries per second, failure rate and rcodes from traffic"`;
  - página SMTP: `"shows delivery rate, send latency, volume per minute and per hour, and queue length"`. O volume por hora soma as janelas da última hora com `?desde=`. Sem `FILA_SMTP` recebido, a fila mostra "não configurada";
  - carga: `TestMailLoadSendsAtRate` contra um servidor SMTP falso em `net.Listen`, e `TestDNSLoadQueries` contra um servidor UDP falso.
- [ ] **Passo 2:** os testes falham.
- [ ] **Passo 3:** implemente. Na página, separe "Sonda ativa" (Parte 1) de "Tráfego real" (esta tarefa). A taxa de entrega é `entregues / tentativas`, com "—" quando não há tentativas. A carga envia e-mails com `net/smtp` e consultas com um pacote DNS mínimo (pergunta A).
- [ ] **Passo 4:** `npm test`, `npm run lint` e `npx tsc -b` passam, e `go test ./...` em `subjects/loja` também. Teste manual: com `exemplos/dns-smtp` no ar e `make carga ARGS="-dns 127.0.0.1:1053 -smtp 127.0.0.1:1025"`, as duas páginas mostram números em até 20 s.
- [ ] **Passo 5:** commits `feat(frontend): mostra DNS e SMTP do tráfego real` e `feat(subjects): carga de DNS e e-mail na loja`. São dois commits porque as labels são diferentes (`visual` e `cobaia`).

---

## Parte 6: README

Branch `docs/readme-lacunas`. Só documentação; os testes são dispensados, e o PR diz isso.

### Tarefa 6.1: atualizar o README

**Arquivos:**
- Modificar: `README.md`, `docs/TESTES-NAVEGACAO.md` (roteiros das telas novas)

- [ ] **Passo 1:** corrija a linha de **Status** e a tabela do Arsenal: ffuf, Nikto, SQLMap, Amass, Sistema e PDF já estão na `main`. Confira cada item com `ls internal/arsenal` e `web/src/router.tsx` antes de marcar como pronto.
- [ ] **Passo 2:** em "Comunicação em tempo real", documente `METRICAS_CONEXOES`, `METRICAS_DNS`, `METRICAS_SMTP` e `FILA_SMTP` com um exemplo JSON cada, mais os campos novos de `METRICAS_HTTP` (`codigos`), `METRICAS_BANCO` (`transacoes_falhas`) e `METRICAS_SISTEMA` (I/O).
- [ ] **Passo 3:** na tabela de regras de alerta, inclua `sistema_latencia_disco`, `dns_falhas_trafego`, `smtp_rejeicoes` e `smtp_fila`.
- [ ] **Passo 4:** acrescente as variáveis `OMNI_PG_DSN` / `OMNI_PG_DSN_ARQUIVO` (com o papel `pg_monitor`) e `OMNI_SMTP_FILA_DIRS` (com a permissão de leitura da fila do Postfix).
- [ ] **Passo 5:** substitua o trecho "Ainda não coberto" de DNS/SMTP pelo que passou a existir. Registre o limite que sobra: DNS sobre TCP e SMTP com STARTTLS não são decodificados.
- [ ] **Passo 6:** em "O que falta implementar", marque os itens concluídos ("DNS: consultas por segundo", "SMTP: volume de envios e fila", "Conexões por estado").
- [ ] **Passo 7:** acrescente uma tabela "Cobertura do enunciado": cada métrica da seção 3 do PDF, onde aparece na interface e de onde vem o dado (sonda, sniffer, `/proc` ou PostgreSQL).
- [ ] **Passo 8:** commit `docs: atualiza o README com as métricas novas`.
