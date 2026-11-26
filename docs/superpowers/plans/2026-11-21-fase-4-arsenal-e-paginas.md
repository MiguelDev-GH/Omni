# Fase 4: Arsenal completo e páginas próprias, Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tirar do "em breve" todas as seções do menu (Sistema, Inspetor de pacotes, Capturas, Nikto, SQLMap, ffuf, Amass, Vazamentos e Exportação em PDF) e dar a cada ferramenta do Arsenal, inclusive Nmap, Nuclei e testssl, uma página própria desenhada a partir de como a ferramenta funciona, em vez de só o console embutido e a lista genérica de achados.

**Architecture:** Hoje cada ferramenta reduz a saída a `[]findings.Finding` (título, severidade, evidência, porta), o que basta para a comparação entre scans mas não para uma tela rica. A Sprint 9 cria a base comum: cada `arsenal.Tool` passa a produzir, além dos achados, um **relatório estruturado** em JSON (`Report`), guardado por varredura e ferramenta, e a aceitar **opções** validadas (`Options`). O frontend ganha `/arsenal/:ferramenta`, com um registro que liga cada ferramenta ao seu formulário de opções e à sua visualização. As sprints seguintes só acrescentam ferramentas e páginas sobre essa base. Sistema, Inspetor, Capturas e PDF são sprints independentes do Arsenal.

**Tech Stack:** o mesmo do projeto (Go 1.24, SQLite `modernc.org/sqlite`, cliente mínimo da API do Docker, `gopacket` com `pcapgo`; React + TypeScript, Tailwind, Zustand, Recharts, Vitest). Única dependência nova prevista: `@tanstack/react-virtual` (Sprint 14), em versão exata.

**Spec:** pedido do dono do projeto nesta conversa ("implementar as seções em breve" e "que cada uma dessas seções tivesse seu front end com base em como essa ferramenta funciona, e não só receber o resultado no terminal embutido"), mais o `README.md` (seções Componentes, Arsenal, Interface, Segurança da própria plataforma e O que falta implementar) e as decisões das Sprints 1 a 8 em `docs/SPRINTS.md`. Convenções: `.claude/skills/convencoes-omni/SKILL.md`.

## Como usar este plano

- Cada sprint vira **um branch e um PR**, aberto depois que a anterior entra na `main`. A Sprint 9 vem primeiro porque as sprints 11 a 13 dependem dela; as sprints 10, 14, 15 e 16 não dependem do Arsenal e podem ir em qualquer ordem depois da 9.
- Este plano traz, por tarefa, arquivos, interfaces e testes. Ao iniciar uma sprint, rode `/superpowers:writing-plans` sobre a seção dela para expandi-la em passos, com o código real à vista, como foi feito nas Sprints 1 a 3.
- Toda tarefa segue o ciclo: teste que falha → implementação → teste passa → verificação da skill `convencoes-omni` → commit.
- **Imagens Docker:** quando a tag exata não está fixada aqui, a primeira tarefa da sprint escolhe a tag, roda a ferramenta uma vez contra a cobaia (`subjects/loja`) e salva a saída real em `internal/arsenal/testdata/` como fixture. O parser é escrito contra esse fixture, nunca contra um formato lembrado de cabeça.

| Sprint | Entrega | Branch | Labels do PR |
|---|---|---|---|
| 9 | Base de relatórios por ferramenta + páginas de Nmap, Nuclei e testssl | `feat/arsenal-paginas` | `funcionalidade`, `docker`, `visual` |
| 10 | Página Sistema com histórico de CPU, memória e disco | `feat/pagina-sistema` | `funcionalidade`, `motor`, `visual` |
| 11 | ffuf e Nikto (descoberta de rotas e falhas do servidor web) | `feat/arsenal-ffuf-nikto` | `funcionalidade`, `docker`, `visual` |
| 12 | SQLMap guiado pelas rotas vistas no tráfego | `feat/arsenal-sqlmap` | `funcionalidade`, `docker`, `visual` |
| 13 | Amass (subdomínios) e Gitleaks (vazamentos de segredos) | `feat/arsenal-amass-gitleaks` | `funcionalidade`, `docker`, `motor`, `visual` |
| 14 | Inspetor de pacotes ao vivo | `feat/inspetor-pacotes` | `funcionalidade`, `motor`, `visual` |
| 15 | Capturas `.pcap` sob demanda | `feat/capturas-pcap` | `funcionalidade`, `motor`, `visual` |
| 16 | Exportação de relatório em PDF | `feat/relatorio-pdf` | `funcionalidade`, `visual` |
| 17 | Parar varreduras e rodar várias ao mesmo tempo | `feat/controle-varreduras` | `funcionalidade`, `docker`, `motor`, `visual` |

## Global Constraints

- Tudo o que vale nas Sprints 1 a 8 continua valendo: identificadores em inglês, textos, JSON e eventos em português; argumentos dos containers sempre como `[]string`, nunca via shell; imagem com tag fixa; `NetworkMode: "host"`; 1 CPU e 512 MiB por padrão; varredura só contra loopback, o host do alvo, o domínio do alvo e seus subdomínios (`arsenal.InScope`); `User-Agent: Omni-Arsenal/1` em toda requisição HTTP das ferramentas que permitem definir o cabeçalho.
- Migrações novas continuam a numeração: a próxima é `internal/store/migrations/008_*.sql`. Os números deste plano (008 a 011) supõem a ordem das sprints; se uma sprint entrar antes de outra, ela usa o próximo número livre na `main`. Cada migração só acrescenta; varreduras antigas continuam legíveis.
- Os achados (`findings.Finding`) continuam sendo a fonte da comparação entre scans, dos contadores por severidade e dos toasts. O relatório estruturado só alimenta a página da ferramenta; nunca substitui os achados.
- **Nenhum segredo é gravado:** o relatório do Gitleaks nunca guarda o valor do segredo nem o trecho casado, mesmo que a ferramenta os devolva; o Inspetor e as capturas resumidas nunca mostram cabeçalhos nem corpos HTTP, só método e rota normalizada, como as métricas da Sprint 5.
- Toda opção de ferramenta vinda da interface é validada no backend contra uma lista fechada ou um padrão estrito antes de virar argumento, e erro de validação responde 400 com `campo` apontando a opção (`"opcoes.sqlmap.parametro"`).
- Ferramentas que dependem de um dado opcional do alvo (Amass precisa de `dominio`; Gitleaks precisa de `repositorio`) aparecem desabilitadas na interface com o motivo, e a API responde 400 com `campo: "ferramentas"` se forem pedidas mesmo assim.
- Rotas novas da interface usam os nomes em português já usados no menu: `/sistema`, `/arsenal/:ferramenta`, `/inspetor`, `/capturas`, `/relatorios/exportar`.
- Cada página nova segue o tema da interface: gráficos e listas direto sobre o fundo, sem bordas, componentes `Card`, `Stat`, `PageHeader`, `Notice` e `Select` de `web/src/components/`.
- Branches, commits, PRs e labels seguem a skill `convencoes-omni`. Ao abrir o PR de cada sprint, atualize o status dela em `docs/SPRINTS.md`; ao mesclar, marque como concluída e marque no README os itens de "O que falta implementar" que ela entregou.

## Review Focus

1. **Saída da ferramenta fora do formato esperado** (imagem atualizada, saída cortada por timeout, ferramenta que morre no meio): a varredura termina com os achados que deu para ler, o erro do relatório aparece na saída ao vivo, e a página da ferramenta mostra "Relatório indisponível para esta varredura" com a lista de achados e a saída bruta. Teste de parser com fixture cortado no meio, em cada tarefa de parser, e `TestReportErrorKeepsFindings` (Tarefa 9.2).
2. **Varreduras antigas, de antes da migração 008, não têm relatório:** a página da ferramenta abre normalmente com a tabela de achados genérica. Teste `ToolPage › falls back to findings without a report` (Tarefa 9.4).
3. **Opções maliciosas ou fora do esperado** (`parametro` = `id --os-shell`, `rota` = `http://evil.com/`, `rota` com `..`, `lista` desconhecida, `repositorio` = `/`, `/etc` ou caminho relativo): 400 com o campo, e nada chega aos argumentos. Testes nas Tarefas 11.2, 12.1 e 13.1.
4. **Segredos e dados pessoais nunca persistidos:** o Gitleaks com saída que contém `Secret` e `Match` gera relatório e achados sem esses valores; um pacote HTTP com `Authorization` e corpo aparece no Inspetor só com método e rota. Testes `TestGitleaksDropsSecrets` (Tarefa 13.3) e `TestSummaryNeverHasHeaders` (Tarefa 14.1).
5. **Inspetor com tráfego intenso:** milhares de pacotes por segundo não travam a captura nem os outros clientes do WebSocket; os lotes têm tamanho máximo e o excedente é contado em `descartados`. Teste `TestInspectorDropsWhenFull` (Tarefa 14.2).

---

## Sprint 9: Base de relatórios e páginas de Nmap, Nuclei e testssl

Entrega: cada ferramenta já existente tem a sua página em `/arsenal/<nome>`, com a visualização própria do relatório; a tela `/arsenal` vira a visão geral, que dispara várias ferramentas de uma vez e mostra o último resultado de cada uma.

### Tarefa 9.1: Opções e relatório na definição da ferramenta

**Files:** Modify `internal/arsenal/tools.go`, `internal/arsenal/nmap.go`, `internal/arsenal/nuclei.go`, `internal/arsenal/testssl.go`; Tests correspondentes

**Interfaces:**
- Produces:
  - `type Options map[string]string`
  - Em `Tool`: `Args func(t target.Target, o Options) ([]string, error)` (troca a assinatura atual; as três ferramentas existentes ignoram `o` e recusam qualquer chave com `ErrOpcaoInvalida`)
  - Em `Tool`: `Report func(stdout []byte) (json.RawMessage, error)`; `nil` quando a ferramenta não tem relatório
  - Em `Tool`: `Requires string` com tag `json:"requer,omitempty"` (`""`, `"dominio"` ou `"repositorio"`), exposto em `GET /api/ferramentas`
  - `var ErrOpcaoInvalida = errors.New("opção inválida")`; o erro devolvido inclui a chave: `fmt.Errorf("%w: %s", ErrOpcaoInvalida, chave)`

**Testes:** os testes atuais de `NmapArgs`, `NucleiArgs` e `TestsslArgs` passam com `Options{}`; `NmapArgs(alvo, Options{"x": "1"})` → `ErrOpcaoInvalida`.

### Tarefa 9.2: Relatórios guardados por varredura

**Files:** Create `internal/store/migrations/008_scan_reports.sql`, `internal/store/reports.go`; Modify `internal/store/scans.go`, `internal/orchestrator/orchestrator.go`, `internal/api/scans.go`; Tests correspondentes

**Interfaces:**
- Produces:
  - Migração: `ALTER TABLE scans ADD COLUMN options TEXT NOT NULL DEFAULT '{}'` e `CREATE TABLE scan_reports (scan_id INTEGER NOT NULL REFERENCES scans(id) ON DELETE CASCADE, tool TEXT NOT NULL, report TEXT NOT NULL, PRIMARY KEY (scan_id, tool))`
  - `func (s *Store) SaveReport(ctx context.Context, scanID int64, tool string, report json.RawMessage) error`
  - `func (s *Store) Report(ctx context.Context, scanID int64, tool string) (json.RawMessage, bool, error)`
  - `store.Scan` ganha `Options map[string]map[string]string` com tag `json:"opcoes"` (por ferramenta; o `store` não importa o `arsenal`)
  - `(*Orchestrator) Start(ctx, targetID int64, tools []string, opts map[string]arsenal.Options) (int64, error)`
  - `POST /api/varreduras` aceita `"opcoes": {"<ferramenta>": {"<chave>": "<valor>"}}`; erro de opção → 400 `{"erro": "...", "campo": "opcoes.<ferramenta>.<chave>"}`
  - `GET /api/varreduras/{id}/relatorio/{ferramenta}` → 200 com o JSON guardado, ou 404 `{"erro":"relatório indisponível"}`

**Testes:** ida e volta de `SaveReport`/`Report`; `TestReportErrorKeepsFindings`: `Report` que falha → achados gravados, status `concluida` e uma linha "relatório da ferramenta indisponível: ..." publicada em `SCAN_LOG`; opção inválida → 400 com `campo` e nenhum scan criado; relatório de outra ferramenta → 404.

### Tarefa 9.3: Relatórios de Nmap, Nuclei e testssl

**Files:** Create `internal/arsenal/reports.go` (tipos dos relatórios); Modify `nmap.go`, `nuclei.go`, `testssl.go`; Tests com os fixtures existentes em `internal/arsenal/testdata/`

**Interfaces:**
- Produces (JSON em português, como o resto da API):
  - `NmapReport { Hosts []NmapHost "hosts" }`, `NmapHost { Address "endereco"; Ports []NmapPort "portas" }`, `NmapPort { Port "porta" int; Protocol "protocolo"; State "estado" /* open, closed, filtered */; Service "servico"; Product "produto"; Version "versao"; Extra "extra" }`; inclui portas fechadas e filtradas, que hoje são descartadas
  - `NucleiReport { Results []NucleiResult "resultados" }`, `NucleiResult { Template "template"; Name "nome"; Severity "severidade" findings.Severity; Tags []string "tags"; URL "url"; Matcher "matcher"; Extracted []string "extraidos"; Description "descricao"; References []string "referencias" }`; `request` e `response` continuam sem ser lidos
  - `TestsslReport { Grade "nota"; Protocols []TLSProtocol "protocolos"; Certificate TLSCertificate "certificado"; Vulns []TLSVuln "vulnerabilidades" }`, `TLSProtocol { Name "nome" /* SSLv2..TLS1.3 */; Offered "oferecido" bool }`, `TLSCertificate { Subject "assunto"; Issuer "emissor"; NotAfter "valido_ate" time.Time; DaysLeft "dias_restantes" int; Trusted "confiavel" bool }`, `TLSVuln { ID "id"; Name "nome"; Vulnerable "vulneravel" bool; CVE "cve" }`
  - `func NmapReportFrom(stdout []byte) (json.RawMessage, error)`, `NucleiReportFrom`, `TestsslReportFrom` (ligados em `Tool.Report`)

**Testes:** fixture do Nmap → portas abertas e fechadas com produto e versão; fixture do Nuclei → tags e referências presentes, nenhum campo `request`/`response`; fixture do testssl → nota, os seis protocolos e `dias_restantes` calculado a partir de uma data fixa injetada; fixture cortado no meio → erro, sem pânico.

### Tarefa 9.4: Página por ferramenta no frontend

**Files:** Create `web/src/arsenal/registry.tsx`, `web/src/pages/ToolPage.tsx`, `web/src/pages/ArsenalPage.tsx` (reescrita como visão geral), `web/src/api/reports.ts`; Modify `web/src/router.tsx`, `web/src/components/Sidebar.tsx`, `web/src/components/ScanConsole.tsx`; Tests correspondentes

**Interfaces:**
- Produces:
  - `type ToolView = { title: string; summary: (report: unknown) => string; Report: React.FC<{ report: unknown; findings: Finding[]; scan: Scan }>; Options?: React.FC<{ value: Record<string,string>; onChange: (v: Record<string,string>) => void; target: Target }> }`
  - `export const toolViews: Record<string, ToolView>`
  - `function getReport(scanId: number, tool: string): Promise<unknown | null>` (404 → `null`)
  - Rotas: `/arsenal` (visão geral) e `/arsenal/:ferramenta`; no menu, cada ferramenta aponta para a própria rota, e "Vazamentos" para `/arsenal/gitleaks`
- Comportamento comum de `ToolPage`: formulário de opções da ferramenta (quando houver), botão "Iniciar varredura", última varredura da ferramenta para o alvo selecionado, visualização do relatório, e a saída ao vivo dentro de um `<details>` "Saída da ferramenta", aberto só enquanto a varredura roda. Sem relatório (varredura antiga ou falha no relatório): `Notice` "Relatório indisponível para esta varredura" + `FindingsTable`.
- Visão geral `/arsenal`: as ferramentas lado a lado, cada uma com a data da última varredura, o resumo (`summary`) e o link para a página; seleção múltipla e "Iniciar varredura" como hoje.

**Testes:** rota `/arsenal/nmap` mostra a visualização do Nmap a partir de um relatório falso; `falls back to findings without a report`; ferramenta com `requer: "dominio"` e alvo sem domínio → item desabilitado com "Precisa do domínio do alvo"; os testes atuais de `ArsenalPage` continuam passando, ajustados à nova estrutura.

### Tarefa 9.5: Visualizações de Nmap, Nuclei e testssl

**Files:** Create `web/src/arsenal/NmapView.tsx`, `NucleiView.tsx`, `TestsslView.tsx`; Tests correspondentes

- **Nmap, "mapa de portas":** uma linha por porta com o estado em cor e texto (aberta, fechada, filtrada), serviço, produto e versão; acima, a faixa das portas do alvo, uma marca por porta. Portas que mudaram desde a varredura anterior (de `GET /api/varreduras/{id}/comparacao`) ganham o rótulo "abriu desde a anterior" ou "fechou desde a anterior".
- **Nuclei, "o que os templates acharam":** filtros por severidade e por tag (a partir das tags do relatório), resultados agrupados por template, cada um expansível com descrição, URL casada, valores extraídos e links de referência (abrem em nova aba, `rel="noreferrer"`).
- **testssl, "boletim TLS":** a nota geral em destaque, a linha dos protocolos SSLv2 a TLS 1.3 (oferecido ou não, com SSLv2, SSLv3, TLS 1.0 e TLS 1.1 oferecidos em vermelho), a validade do certificado como barra de dias restantes (amarelo abaixo de 30, vermelho abaixo de 7) com emissor e confiança, e a grade das vulnerabilidades testadas.

**Testes:** cada visualização renderiza o fixture convertido (`web/src/arsenal/__fixtures__/`); estado da porta legível sem depender de cor; protocolo inseguro oferecido aparece como problema; nota ausente mostra "—".

PR `feat(frontend): adiciona página própria para cada ferramenta do arsenal`, labels `funcionalidade`, `docker` e `visual`.

---

## Sprint 10: Página Sistema

Entrega: `/sistema` mostra CPU, memória e disco dos últimos 15 minutos e das últimas 24 horas, com as linhas dos limites de alerta, e o estado dos motores.

### Tarefa 10.1: Histórico das métricas do sistema

**Files:** Create `internal/store/migrations/009_system.sql`, `internal/store/system.go`, `internal/api/system.go`; Modify `internal/sysmetrics/sysmetrics.go`, `cmd/omni/run.go`; Tests correspondentes

**Interfaces:**
- Produces:
  - Tabela `system_metrics (time_ms INTEGER PRIMARY KEY, cpu REAL, memory REAL, disk REAL)`, retenção de 24 h (limpeza junto com a do tráfego)
  - `func (s *Store) AddSystemSample(ctx, sysmetrics.Snapshot) error`, `func (s *Store) SystemSince(ctx, since time.Time) ([]sysmetrics.Snapshot, error)`
  - `sysmetrics.Run` recebe um `Sink interface { AddSystemSample(ctx, Snapshot) error }`; erro de gravação não interrompe o envio pelo WebSocket
  - `GET /api/sistema?desde=<RFC3339>` (padrão: últimos 15 minutos; máximo 24 h, com amostragem de 1 ponto por minuto acima de 1 h)

**Testes:** gravação e leitura por intervalo; `desde` mais antigo que 24 h é limitado a 24 h; janela de 2 h devolve no máximo 120 pontos.

### Tarefa 10.2: Tela Sistema

**Files:** Create `web/src/pages/SystemPage.tsx`, `web/src/api/system.ts`; Modify `web/src/router.tsx`, `web/src/components/Sidebar.tsx`; Test `web/src/pages/SystemPage.test.tsx`

- Três gráficos de área (CPU, memória, disco) direto sobre o fundo, com linhas de referência nos limites de alerta do README (CPU 80% e 95%, memória e disco 85% e 95%), escolha de período 15 min, 1 h e 24 h (`Select`), valores atuais em `Stat` e estado dos motores Poller, Sniffer e Docker.

**Testes:** carrega o histórico e acrescenta os pontos de `METRICAS_SISTEMA`; trocar o período refaz a busca com o `desde` certo; sem dados mostra "Sem medições ainda".

PR `feat(frontend): adiciona página sistema com histórico de recursos`, labels `funcionalidade`, `motor` e `visual`.

---

## Sprint 11: ffuf e Nikto

Entrega: descoberta de rotas escondidas da aplicação (ffuf) e falhas conhecidas do servidor web (Nikto), cada uma com a sua página.

### Tarefa 11.1: Arquivos enviados ao container

**Files:** Modify `internal/orchestrator/orchestrator.go`, `internal/orchestrator/docker.go`; Tests correspondentes (unitário com servidor HTTP falso no lugar do socket do Docker)

**Interfaces:**
- Produces: `Spec.Files map[string][]byte` (caminho absoluto no container → conteúdo), enviados num tar por `PUT /containers/{id}/archive` antes do start; em `Tool`: `Files func(Options) (map[string][]byte, error)`

**Testes:** o tar enviado contém os arquivos com permissão `0444`; caminho relativo ou com `..` → erro antes de criar o container.

### Tarefa 11.2: ffuf

**Files:** Create `internal/arsenal/ffuf.go`, `internal/arsenal/wordlists/` (listas embutidas com `//go:embed`, com o arquivo de licença do SecLists), fixture `testdata/ffuf.json`; Tests

**Interfaces:**
- Produces:
  - Opção `lista`: `"pequena"` (SecLists `Discovery/Web-Content/common.txt`) ou `"media"` (`raft-small-words.txt`); padrão `"pequena"`; outra → `ErrOpcaoInvalida`. Opção `porta`: uma das `AppPorts` do alvo; padrão a primeira.
  - `FfufArgs(t, o)` → `-u http://<host>:<porta>/FUZZ -w /omni/lista.txt -of json -o /dev/stdout -s -t 10 -rate 50 -mc all -fc 404 -ac -H "User-Agent: Omni-Arsenal/1"`
  - `FfufReport { BaseURL "base_url"; Results []FfufResult "resultados" }`, `FfufResult { Path "caminho"; Status "status" int; Size "tamanho" int; Words "palavras" int; Redirect "redirecionamento"; DurationMS "duracao_ms" int }`
  - Achados: um por caminho encontrado, `info`; caminhos sensíveis (`.git`, `.env`, `backup`, `.bak`, `.sql`, `admin`, `phpinfo`, `server-status`) → `media`

**Testes:** argumentos exatos para um alvo; `lista=../../etc/passwd` → 400 com `campo: "opcoes.ffuf.lista"`; porta fora das `AppPorts` → erro; fixture → caminhos, status e severidade de `.env` igual a `media`.

### Tarefa 11.3: Nikto

**Files:** Create `internal/arsenal/nikto.go`, fixture `testdata/nikto.json`; Tests

**Interfaces:**
- Produces:
  - Imagem com tag fixa escolhida na própria tarefa (precisa aceitar `-Format json`); opção `porta` igual à do ffuf
  - `NiktoArgs(t, o)` → `-h http://<host>:<porta> -Format json -o - -nointeractive -useragent Omni-Arsenal/1 -maxtime 600s`
  - `NiktoReport { Server "servidor"; Items []NiktoItem "itens" }`, `NiktoItem { ID "id"; Method "metodo"; URL "url"; Message "mensagem"; References []string "referencias"; Category "categoria" }`; `Category` ∈ `cabecalhos`, `arquivos`, `versao`, `outros`, derivada da mensagem por regras simples documentadas no código
  - Achados: `cabecalhos` → `baixa`; `arquivos` e `versao` → `media`; `outros` → `info`

**Testes:** fixture → itens nas categorias certas; mensagem sem categoria conhecida → `outros`.

### Tarefa 11.4: Páginas do ffuf e do Nikto

**Files:** Create `web/src/arsenal/FfufView.tsx`, `FfufOptions.tsx`, `NiktoView.tsx`; Tests

- **ffuf, "árvore de rotas":** caminhos encontrados em árvore por segmento, com status em cor e texto, tamanho e redirecionamento; filtro por classe de status; marcação "não aparece no tráfego real" para caminhos que não batem com nenhuma rota de `GET /api/requisicoes/rotas`, que são justamente as rotas escondidas. Opções: lista (`Select` com o tamanho de cada uma) e porta.
- **Nikto, "checagens do servidor":** o servidor identificado em destaque e os itens agrupados por categoria, cada um com método, URL e referências.

**Testes:** árvore agrupa `/admin` e `/admin/login`; rota vista no tráfego não recebe a marcação; Nikto agrupa por categoria.

PR `feat(orquestrador): adiciona ffuf e nikto ao arsenal`, labels `funcionalidade`, `docker` e `visual`.

---

## Sprint 12: SQLMap guiado pelo tráfego

Entrega: o usuário escolhe uma rota que o sniffer já viu e o parâmetro a testar; a página mostra se ele é injetável, por qual técnica e qual banco foi identificado.

### Tarefa 12.1: SQLMap com opções travadas

**Files:** Create `internal/arsenal/sqlmap.go`, fixtures `testdata/sqlmap-injetavel.txt` e `testdata/sqlmap-limpo.txt`; Tests

**Interfaces:**
- Produces:
  - Opções obrigatórias: `rota` (caminho que começa com `/`, só `[A-Za-z0-9/_.:-]`, sem `..`, sem `//`; segmentos `:id` viram `1`), `parametro` (`^[A-Za-z_][A-Za-z0-9_]{0,63}$`), `porta` (uma das `AppPorts`)
  - `SqlmapArgs(t, o)` → `-u "http://<host>:<porta><rota>?<parametro>=1" -p <parametro> --batch --level 1 --risk 1 --threads 1 --flush-session --disable-coloring --user-agent Omni-Arsenal/1`; nunca `--dump`, `--os-*`, `--file-*`, `--sql-shell`, `--eval`, `--proxy` (teste que procura esses prefixos)
  - `SqlmapReport { URL "url"; Parameter "parametro"; Injectable "injetavel" bool; DBMS "sgbd"; Techniques []SqlmapTechnique "tecnicas" }`, `SqlmapTechnique { Type "tipo"; Title "titulo"; Payload "payload" }`, lido do texto da saída padrão (bloco `Parameter: ... Type: ... Title: ... Payload: ...` e linha `back-end DBMS: ...`)
  - Achados: injetável → um achado `critica` por técnica; não injetável → nenhum

**Testes:** `parametro = "id --os-shell"` → 400 `campo: "opcoes.sqlmap.parametro"`; `rota = "http://evil.com/x"` e `rota = "/a/../b"` → 400; argumentos nunca contêm os prefixos proibidos; os dois fixtures → injetável com técnicas e SGBD, e limpo sem achados.

### Tarefa 12.2: Página do SQLMap

**Files:** Create `web/src/arsenal/SqlmapOptions.tsx`, `SqlmapView.tsx`; Tests

- Opções: `Select` das rotas vistas nas últimas 24 h (`GET /api/requisicoes/rotas`, só `GET`), campo do parâmetro e porta; sem rotas vistas, `Notice` explicando que o SQLMap precisa de tráfego real na aplicação.
- Visualização: veredito grande ("Injetável" em vermelho ou "Nenhuma injeção encontrada"), SGBD identificado e uma linha por técnica com título e payload em fonte mono, com aviso de que o teste usou nível 1 e risco 1.

**Testes:** sem rotas mostra o aviso e desabilita "Iniciar varredura"; relatório injetável mostra as técnicas.

PR `feat(orquestrador): adiciona sqlmap guiado pelas rotas do tráfego`, labels `funcionalidade`, `docker` e `visual`.

---

## Sprint 13: Amass e Gitleaks

Entrega: descoberta passiva de subdomínios do domínio do alvo e busca de segredos vazados no código-fonte da aplicação, cada uma com a sua página.

### Tarefa 13.1: Repositório do alvo

**Files:** Create `internal/store/migrations/010_target_repo.sql`; Modify `internal/target/target.go`, `internal/store/targets.go`, `web/src/api/targets.ts`, `web/src/components/TargetForm.tsx`; Tests

**Interfaces:**
- Produces: `Target.Repo string` com tag `json:"repositorio,omitempty"`; validação: caminho absoluto, já limpo (`filepath.Clean(p) == p`), diferente de `/`, fora de `/proc`, `/sys`, `/dev`, `/etc`, `/boot`, `/root`, existente e diretório; campo "Código-fonte (opcional)" no formulário de alvo

**Testes:** `/`, `/etc`, `relativo/x`, `/tmp/../etc` e caminho inexistente → erro com `campo: "repositorio"`; um diretório temporário válido passa.

### Tarefa 13.2: Amass

**Files:** Create `internal/arsenal/amass.go`, fixture `testdata/amass.json`; Tests

**Interfaces:**
- Produces:
  - `Requires: "dominio"`; imagem com tag fixa escolhida na tarefa, que precisa ter modo passivo e saída estruturada (se a versão escolhida não tiver, a tarefa troca para o `subfinder` do ProjectDiscovery e atualiza o README e este plano)
  - Argumentos em modo **passivo** só para `t.Domain`: o Amass consulta fontes públicas na internet, mas não envia nada ao alvo
  - `AmassReport { Domain "dominio"; Subdomains []Subdomain "subdominios" }`, `Subdomain { Name "nome"; Addresses []string "enderecos"; Sources []string "fontes"; ThisHost "neste_servidor" bool }` (`neste_servidor` quando algum endereço é loopback ou igual ao `Host` do alvo)
  - Achados: um `info` por subdomínio; subdomínio sem endereço → `baixa` ("subdomínio sem endereço: possível risco de takeover")

**Testes:** alvo sem domínio → erro; argumentos não contêm o host do alvo, só o domínio; fixture → subdomínios e `neste_servidor` corretos.

### Tarefa 13.3: Gitleaks

**Files:** Create `internal/arsenal/gitleaks.go`, fixture `testdata/gitleaks.json` (com `Secret` e `Match` preenchidos de propósito); Modify `internal/orchestrator/docker.go` (montagem somente leitura); Tests

**Interfaces:**
- Produces:
  - `Requires: "repositorio"`; imagem `zricethezav/gitleaks` com tag fixa escolhida na tarefa
  - `Spec.Binds []Bind`, `Bind { Source, Target string }`, sempre montado como somente leitura (`ReadOnly: true` no `Mounts` do Docker); o único bind permitido é o `Repo` validado do alvo em `/repo`
  - Argumentos: varredura de `/repo` com relatório JSON em `/dev/stdout`, `--redact` e código de saída 0 em achados
  - `GitleaksReport { Leaks []Leak "vazamentos" }`, `Leak { Rule "regra"; Description "descricao"; File "arquivo"; Line "linha" int; Commit "commit"; Author "autor"; Date "data"; Entropy "entropia" float64 }`; o parser decodifica num tipo que **não tem** os campos `Secret` e `Match`
  - Achados: um `alta` por vazamento, título `"<regra> em <arquivo>:<linha>"`, evidência só com commit e data

**Testes:** `TestGitleaksDropsSecrets`: o relatório e os achados gerados a partir do fixture não contêm o valor do segredo nem o trecho casado em nenhum campo; `Bind` com caminho diferente do `Repo` do alvo → erro; montagem sempre somente leitura.

### Tarefa 13.4: Páginas do Amass e do Vazamentos

**Files:** Create `web/src/arsenal/AmassView.tsx`, `GitleaksView.tsx`; Tests

- **Amass, "subdomínios":** lista em árvore pelo domínio, cada subdomínio com endereços e fontes; destaque para os que apontam para este servidor e aviso para os sem endereço. Nota fixa: "Consulta fontes públicas na internet; o alvo não recebe tráfego."
- **Vazamentos, "segredos no código":** vazamentos agrupados por arquivo, com linha, regra, commit, autor e data; filtro por regra; o valor nunca aparece, só o texto "valor ocultado". Página título "Vazamentos", com o Gitleaks citado como a ferramenta usada.

**Testes:** subdomínio sem endereço mostra o aviso; agrupamento por arquivo; nenhum texto de segredo é renderizado a partir de um relatório que (indevidamente) contenha a chave `segredo`.

PR `feat(orquestrador): adiciona amass e gitleaks ao arsenal`, labels `funcionalidade`, `docker`, `motor` e `visual`.

---

## Sprint 14: Inspetor de pacotes

Entrega: `/inspetor` mostra os pacotes das portas dos alvos ao vivo, um por linha, com pausa e filtros, sem travar o navegador nem a captura.

### Tarefa 14.1: Resumo de pacote

**Files:** Create `internal/sniffer/summary.go`; Test `internal/sniffer/summary_test.go` (com os `.pcap` de teste existentes)

**Interfaces:**
- Produces: `type PacketSummary struct { Time time.Time "hora"; Src "origem"; Dst "destino"; SrcPort "porta_origem" int; DstPort "porta_destino" int; Protocol "protocolo"; Length "tamanho" int; Flags "flags"; Info "info" }`; `func Summarize(p gopacket.Packet, appPorts, dbPorts []int) PacketSummary`; `Info` traz, no HTTP, só método e rota normalizada (`l7` da Sprint 5) e, no PostgreSQL, só o tipo da mensagem

**Testes:** `TestSummaryNeverHasHeaders`: pacote HTTP com `Authorization: Bearer x` e corpo JSON → `Info` = `"POST /login"`, sem `Bearer` nem corpo; SYN, SYN-ACK e RST com `Flags` corretos; pacote UDP sem porta conhecida → protocolo `UDP`.

### Tarefa 14.2: Assinatura pelo WebSocket

**Files:** Modify `internal/sniffer/engine.go`, `internal/ws/handler.go`, `internal/ws/hub.go`; Create `internal/ws/inspector.go`; Tests

**Interfaces:**
- Produces:
  - `(*Engine) SetTap(func(PacketSummary))`; o tap é chamado na goroutine da captura e nunca bloqueia (envia num canal com buffer de 4096 e descarta quando cheio)
  - Comando do cliente: `{"tipo":"INSPETOR","dados":{"ativo":true,"filtro":{"ip":"","porta":0,"protocolo":""}}}`; o handler, que hoje descarta as mensagens recebidas, passa a ler esse comando; outros tipos continuam ignorados
  - Evento `PACOTES` `{"pacotes":[...], "descartados": n}`, a cada 250 ms, no máximo 500 pacotes por lote, só para os clientes com `ativo: true`, já filtrados por cliente
  - Sem nenhum cliente assinando, o tap fica desligado

**Testes:** `TestInspectorDropsWhenFull`: 100 000 resumos de uma vez não bloqueiam o tap, chegam lotes de no máximo 500 e `descartados` > 0; cliente com `ativo:false` não recebe `PACOTES`; filtro por porta entrega só os pacotes da porta; cliente sem assinatura continua recebendo os outros eventos.

### Tarefa 14.3: Tela do Inspetor

**Files:** Create `web/src/pages/InspectorPage.tsx`, `web/src/realtime/inspector.ts`; Modify `web/package.json` (`@tanstack/react-virtual` em versão exata), `web/src/router.tsx`, `web/src/components/Sidebar.tsx`; Tests

- Tabela virtualizada com no máximo 10 000 linhas em memória (as mais antigas saem); botão "Pausar"/"Retomar", que também manda `ativo:false`/`true`; filtros de IP, porta e protocolo, enviados no comando; contador de descartados; clique numa linha abre o detalhe do pacote ao lado. Sair da página manda `ativo:false`.

**Testes:** pausar para de acrescentar linhas e envia o comando; buffer nunca passa de 10 000; desmontar a página envia `ativo:false`.

PR `feat(sniffer): adiciona inspetor de pacotes ao vivo`, labels `funcionalidade`, `motor` e `visual`.

---

## Sprint 15: Capturas `.pcap`

Entrega: `/capturas` grava capturas de curta duração em `.pcap`, lista, baixa, apaga e abre cada uma na tabela do Inspetor.

### Tarefa 15.1: Gravador de capturas

**Files:** Create `internal/capture/recorder.go`, `internal/store/migrations/019_captures.sql`, `internal/store/captures.go`; Modify `internal/sniffer/engine.go`, `internal/config/config.go`, `cmd/omni/run.go`; Tests

**Interfaces:**
- Produces:
  - Config: `OMNI_CAPTURAS` (diretório, padrão `capturas` ao lado do banco) e `OMNI_RETENCAO_CAPTURAS` (duração, padrão `24h`, mínimo `1h`)
  - `(*Engine) AddRawTap(func(gopacket.Packet)) (remove func())`, com a mesma regra de nunca bloquear
  - `type Capture struct { ID int64 "id"; File string "-"; StartedAt "inicio"; Duration "duracao_s" int; Port "porta" int; Packets "pacotes" int; Bytes "bytes" int64; Status "status" /* gravando, concluida, falhou */ }`
  - `func (r *Recorder) Start(ctx, duration time.Duration, port int) (int64, error)`: duração de 10 s a 300 s; arquivo `capture-<id>.pcap` com permissão `0600`; para ao atingir 100 MiB; recusa iniciar se o diretório passar de 1 GiB ou já houver uma captura gravando (409)
  - Limpeza periódica dos arquivos e linhas mais antigos que a retenção
  - Eventos `CAPTURA_INICIADA` e `CAPTURA_CONCLUIDA`

**Testes:** captura com fonte falsa grava um `.pcap` legível pelo `pcapgo` com os pacotes enviados; limite de 100 MiB encerra a captura com status `concluida`; segunda captura simultânea → erro; retenção apaga arquivo e linha.

### Tarefa 15.2: API e tela de Capturas

**Files:** Create `internal/api/captures.go`, `web/src/pages/CapturesPage.tsx`, `web/src/api/captures.ts`; Modify `web/src/router.tsx`, `web/src/components/Sidebar.tsx`, `web/src/pages/InspectorPage.tsx`; Tests

**Interfaces:**
- Produces: `POST /api/capturas {"duracao_s": 60, "porta": 0}` → 202 `{"id"}`; `GET /api/capturas`; `GET /api/capturas/{id}/arquivo` (`Content-Type: application/vnd.tcpdump.pcap`, `Content-Disposition: attachment`); `GET /api/capturas/{id}/pacotes?pagina=` (resumos da Tarefa 14.1, 500 por página); `DELETE /api/capturas/{id}`
- Tela: botão "Gravar captura" com duração e porta, progresso da gravação, lista com início, duração, pacotes e tamanho, ações "Baixar" (busca com o token e salva por `Blob`), "Abrir no Inspetor" (a tabela do Inspetor em modo arquivo, paginada) e "Apagar" (confirmação). `Notice` fixo sobre dados pessoais nas capturas e o prazo de retenção.

**Testes:** id inexistente → 404; download devolve os bytes do arquivo; abrir no Inspetor lista os resumos; apagar remove da lista.

PR `feat(sniffer): adiciona gravação de capturas pcap`, labels `funcionalidade`, `motor` e `visual`.

---

## Sprint 16: Exportação de relatório em PDF

Entrega: em Relatórios, "Exportar PDF" monta um relatório imprimível do alvo e do período escolhidos e abre a janela de impressão do navegador, que salva em PDF.

### Tarefa 16.1: Período nas varreduras e alertas

**Files:** Modify `internal/api/scans.go`, `internal/alerts/http.go`; Tests

**Interfaces:**
- Produces: `GET /api/varreduras?alvo_id=&desde=&ate=` e `GET /api/alertas?desde=&ate=` filtram pelo período (RFC 3339); `ate` antes de `desde` → 400

**Testes:** período filtra as varreduras e o histórico de alertas; período invertido → 400.

### Tarefa 16.2: Página imprimível

**Files:** Create `web/src/pages/ReportExportPage.tsx`, `web/src/print.css` (importado só por essa página); Modify `web/src/pages/ReportsPage.tsx`, `web/src/router.tsx`; Tests

- `/relatorios/exportar?alvo=&desde=&ate=&modulos=monitoramento,alertas,varreduras`: capa com o nome do alvo e o período, resumo de disponibilidade e alertas, gráficos de requisições e status do período, e, por varredura, os achados por severidade e o que mudou desde a anterior. CSS de impressão: `@page { size: A4; margin: 16mm }`, sem barra superior nem menu, quebras de página antes de cada varredura, cores mantidas (`print-color-adjust: exact`).
- Em Relatórios, o formulário "Exportar PDF" (alvo, período e módulos) abre essa página, que chama `window.print()` quando termina de carregar.
- Decisão: o PDF é gerado pelo navegador, sem biblioteca nova no backend; o README passa a dizer isso.

**Testes:** a página monta as seções escolhidas e só elas; `window.print` é chamado depois que os dados carregam (mock); período sem dados mostra "Nenhum dado no período" na seção.

PR `feat(frontend): adiciona exportação de relatório em pdf`, labels `funcionalidade` e `visual`.

---

## Sprint 17: Controle das varreduras

Entrega: parar uma varredura em andamento e rodar várias ao mesmo tempo, inclusive no mesmo alvo, enquanto os containers aguentarem.

**Limite:** a capacidade é a que o Docker informa em `/info` (`NCPU` e `MemTotal`). Cada varredura reserva 1 CPU e o maior `Memory` das suas ferramentas, que rodam em sequência (zero vale 512 MiB). Uma nova só começa se a reserva couber no que sobra, somando todos os alvos; sem nenhuma em andamento, é sempre aceita. Sem espaço, é recusada, sem fila e sem gravar nada no banco.

### Tarefa 17.1: Orquestrador sem trava por alvo e com cancelamento

**Files:** Modify `internal/orchestrator/orchestrator.go`, `internal/orchestrator/docker.go`, `internal/store/scans.go`; Tests

**Interfaces:**
- Produces: `func (o *Orchestrator) Cancel(scanID int64) error`; `ErrVarreduraNaoEstaRodando`; `ErrLimiteDeVarreduras`; `Runner.Capacity(ctx) (cpus int, memory int64, err error)`; status `store.ScanCanceled = "cancelada"`; evento `SCAN_CANCELADO` com `{"scan_id", "alvo_id", "achados"}`
- Removes: `ErrVarreduraEmAndamento`; o mapa `running` passa a ser de ID da varredura para o `context.CancelFunc` dela

**Testes:** duas varreduras do mesmo alvo rodam ao mesmo tempo; cancelar uma não afeta a outra; a cancelada fica `cancelada`, mantém os achados das ferramentas concluídas e não roda as seguintes; o container é removido; cancelar varredura terminada → `ErrVarreduraNaoEstaRodando`; desligar o Omni continua marcando `falhou`; com um runner falso de 2 CPUs, a terceira varredura devolve `ErrLimiteDeVarreduras` sem criar registro; o mesmo quando só a memória falta (uma segunda com Nuclei não cabe); a primeira é aceita mesmo numa capacidade menor que a reserva; erro ao ler a capacidade devolve `ErrDockerIndisponivel`; terminar, falhar ou cancelar libera a reserva; pedidos concorrentes nunca passam da capacidade (a checagem e a reserva ficam sob o mesmo mutex); `go test -race` sem corrida.

### Tarefa 17.2: API de cancelamento

**Files:** Modify `internal/api/scans.go`, `cmd/omni/run.go`; Tests

**Interfaces:**
- Produces: `POST /api/varreduras/{id}/cancelar` → 202; id inexistente → 404; varredura que não está rodando → 409; `POST /api/varreduras` no limite → 429 com a mensagem "sem CPU ou memória para mais uma varredura"
- Removes: o 409 de `POST /api/varreduras` para alvo com varredura em andamento

**Testes:** cada código de resposta; abaixo do limite, duas criações seguidas no mesmo alvo respondem 202; no limite, a seguinte responde 429.

### Tarefa 17.3: Interface

**Files:** Modify `web/src/pages/ArsenalPage.tsx`, `web/src/pages/ToolPage.tsx`, `web/src/arsenal/common.tsx`, `web/src/realtime/events.ts`; Tests

- O botão de iniciar continua habilitado com uma varredura em andamento enquanto o limite não for atingido. Ao receber o 429, a tela mostra o aviso do limite e sugere parar uma varredura ou esperar o fim de uma.
- Lista das varreduras em andamento do alvo, cada uma com status, console e botão "Parar".
- Status "Cancelada" no histórico e em Relatórios; `SCAN_CANCELADO` atualiza a tela e mostra um aviso.

**Testes:** "Parar" chama a rota da varredura certa; é possível iniciar a segunda varredura; a recusa pelo limite aparece na tela; o console de cada uma mostra só a própria saída.

### Tarefa 17.4: Verificação com Docker e documentação

- Contra a cobaia: parar um Nuclei no meio e conferir que o container sumiu; rodar duas varreduras ao mesmo tempo, inclusive duas com Nuclei na primeira execução, porque dividem o volume `omni-nuclei-templates`.
- Conferir a comparação entre scans quando a varredura mais antiga termina depois da mais nova.
- Iniciar varreduras até a recusa, conferir que o número bate com as CPUs e a memória de `docker info` e que, depois de parar uma, a nova é aceita.
- Atualizar o README, `docs/SPRINTS.md` e `docs/TESTES-NAVEGACAO.md`.

PR `feat(orquestrador): permite parar e rodar varreduras em paralelo`, labels `funcionalidade`, `docker`, `motor` e `visual`.
