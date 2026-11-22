# Sprints do Omni

Visão geral do que é feito em cada sprint da implementação básica. Cada sprint vira **um branch e um pull request**, aberto depois que a sprint anterior entra na `main`, e entrega um binário funcional e testado.

Para os detalhes técnicos (arquivos, interfaces e testes de cada tarefa), veja o [plano de implementação básica](superpowers/plans/2026-11-18-implementacao-basica.md) (Sprints 1 a 8) e o [plano da Fase 4](superpowers/plans/2026-11-21-fase-4-arsenal-e-paginas.md) (Sprints 9 a 16). Para a arquitetura, veja o [README](../README.md).

## Resumo

| Sprint | Fase | Entrega | Status |
|---|---|---|---|
| [1](#sprint-1-núcleo-do-backend) | Fundação | Núcleo do backend | ✅ Concluída |
| [2](#sprint-2-poller-e-websocket) | Fundação | Poller, WebSocket e métricas do sistema | ✅ Concluída |
| [3](#sprint-3-frontend-base) | Fundação | Frontend base embutido no binário | ✅ Concluída |
| [4](#sprint-4-sniffer-camadas-3-e-4) | Observação passiva | Captura de tráfego e gráficos de banda e protocolos | ✅ Concluída |
| [5](#sprint-5-decodificação-http) | Observação passiva | Requisições por rota e status HTTP | ✅ Concluída |
| [6](#sprint-6-decodificação-postgresql) | Observação passiva | Consultas ao banco | ✅ Concluída |
| [7](#sprint-7-orquestrador-docker-e-nmap) | Arsenal | Orquestrador Docker e varredura Nmap | ✅ Concluída |
| [8](#sprint-8-nuclei-testssl-e-comparação-entre-scans) | Arsenal | Nuclei, testssl e comparação entre scans | ✅ Concluída |
| [9](#sprint-9-base-de-relatórios-e-páginas-de-nmap-nuclei-e-testssl) | Expansão | Página própria para cada ferramenta do Arsenal (Nmap, Nuclei, testssl) | 🚧 Em andamento |
| [10](#sprint-10-página-sistema) | Expansão | Página Sistema com histórico de CPU, memória e disco | ⏳ A fazer |
| [11](#sprint-11-ffuf-e-nikto) | Expansão | ffuf e Nikto | 🚧 Em andamento |
| [12](#sprint-12-sqlmap-guiado-pelo-tráfego) | Expansão | SQLMap guiado pelas rotas vistas no tráfego | 🚧 Em andamento |
| [13](#sprint-13-amass-e-gitleaks) | Expansão | Amass e Gitleaks (Vazamentos) | ⏳ A fazer |
| [14](#sprint-14-inspetor-de-pacotes) | Expansão | Inspetor de pacotes ao vivo | ⏳ A fazer |
| [15](#sprint-15-capturas-pcap) | Expansão | Capturas `.pcap` sob demanda | ⏳ A fazer |
| [16](#sprint-16-exportação-em-pdf) | Expansão | Exportação de relatório em PDF | ⏳ A fazer |
| [Backlog](#backlog) | Expansão | Restante das pendências do README | 📋 Backlog |

Legenda: ⏳ A fazer · 🚧 Em andamento · ✅ Concluída · 📋 Backlog

---

## Fase 1: Fundação

### Sprint 1: Núcleo do backend

**Ao final:** o `omni` sobe em `127.0.0.1:8080`, responde `GET /api/saude` e permite cadastrar, listar, editar e apagar alvos pela API, com tudo salvo no SQLite.

- Configuração por variáveis de ambiente (`OMNI_ADDR`, `OMNI_DB`, `OMNI_TOKEN`), recusando endereço fora do loopback e token curto.
- Modelo de alvo com validação estrita (host, portas, domínio), bloqueando valores com comandos ou opções disfarçadas.
- SQLite em modo WAL com migrações.
- API REST de alvos protegida por token.
- Binário com desligamento gracioso e CI com as verificações do Go.

**Branch:** `feat/nucleo-backend` · **Labels:** `funcionalidade`, `motor`

### Sprint 2: Poller e WebSocket

**Ao final:** o Omni testa periodicamente as portas de cada alvo, guarda o histórico e avisa em tempo real quando um serviço cai ou volta.

- Hub WebSocket com ticket de uso único e verificação de origem; clientes lentos não travam os demais.
- Verificações HTTP e TCP com latência e status.
- Agendador que publica `ALERTA_QUEDA` e `SERVICO_RESTAURADO` só na mudança de estado.
- Métricas do sistema (CPU, memória, disco) a cada 5 segundos.

**Branch:** `feat/poller-e-websocket` · **Labels:** `funcionalidade`, `motor`

**Pendências da Sprint 2:**

- O histórico de verificações devolve no máximo 5000 resultados somando todas as portas, o que cobre menos de 24 h com intervalos curtos. Decidir limite por porta, amostragem ou sinal de truncamento antes dos gráficos da Sprint 4.
- Retenção configurável da tabela `check_results`.

### Sprint 3: Frontend base

**Ao final:** abrir `http://127.0.0.1:8080` mostra a interface do Omni, embutida no próprio binário.

- Projeto React + TypeScript (Vite, Tailwind) com lint, typecheck e testes no CI.
- Tela de token, barra superior com seletor de alvo e estado dos motores, barra lateral com as seções Monitoramento, Tráfego, Arsenal e Relatórios.
- Página de Alvos com tabela e formulário.
- Descoberta de serviços: ao cadastrar um alvo, o formulário sugere os processos que já escutam portas TCP na máquina (aplicações e bancos), sem expor a linha de comando deles; um clique preenche host e portas.
- Alertas em tempo real (toasts) pela conexão WebSocket.
- `make build` gera um binário único com o frontend embutido.

**Branch:** `feat/frontend-base` · **Labels:** `funcionalidade`, `visual`

---

## Fase 2: Observação passiva

### Sprint 4: Sniffer, camadas 3 e 4

**Ao final:** a tela Geral mostra o consumo de banda, a distribuição de protocolos e os IPs que mais geram tráfego, em tempo real.

- Captura ao vivo nas portas dos alvos (filtro BPF) e leitura de arquivos `.pcap` para testes.
- Agregação por segundo: bytes, pacotes, protocolos e top IPs de origem.
- Sem permissão de captura, o Omni continua no ar e a interface avisa "Sniffer: sem permissão".
- Gráfico de área de banda e gráfico de rosca de protocolos.

**Branch:** `feat/sniffer-camadas-3-4` · **Labels:** `funcionalidade`, `motor`, `visual`

**Decisões da Sprint 4:**

- O tráfego é global: um único filtro BPF cobre as portas de todos os alvos, e a tela Geral não separa por alvo.
- Só janelas com pacotes vão para o banco, com retenção de 24 h; a interface completa com zero os segundos sem janela.
- A tela Geral passa a ser a página inicial.

### Sprint 5: Decodificação HTTP

**Ao final:** os gráficos de requisições na API e de status HTTP mostram o tráfego real da aplicação.

- Remontagem TCP e pareamento de requisição com resposta, inclusive em pipeline e fora de ordem.
- Normalização de rotas (`/users/123` vira `/users/:id`).
- Nenhum valor de `Authorization`, `Cookie` ou corpo é guardado; tráfego gerado pelo próprio Omni fica fora das métricas.
- Latência p50 e p95 por rota, barras empilhadas de status e tela "Requisições na API" com as rotas mais lentas.

**Branch:** `feat/sprints-5-a-8` (as Sprints 5 a 8 foram entregues juntas, num PR com um commit por parte) · **Labels:** `funcionalidade`, `motor`, `visual`, `docker`

**Decisões da Sprint 5:**

- O assembler recebe uma função de callback em vez de um canal: o motor processa os pacotes numa só goroutine, e um canal cheio travaria a captura.
- A latência é o tempo até o primeiro byte da resposta.
- Os percentis de cada janela são exatos; os somados de várias janelas (ranking de 24 h) vêm de histogramas com faixas de 25%.
- Cada janela guarda no máximo 100 rotas; o excedente vira o grupo `(outras)`.

### Sprint 6: Decodificação PostgreSQL

**Ao final:** o gráfico de consultas ao banco e a tabela das consultas mais lentas mostram o uso real do PostgreSQL.

- Decodificação do protocolo do PostgreSQL (consultas simples e preparadas, erros).
- Consultas normalizadas, sem valores literais nem senhas.
- Tela "Consultas no banco" com as mais lentas das últimas 24 horas.

**Branch:** `feat/sprints-5-a-8` · **Labels:** `funcionalidade`, `motor`, `visual`

**Decisões da Sprint 6:**

- A porta de banco 3306 (MySQL) não passa pelo decodificador do PostgreSQL.
- Uma conexão que não parece PostgreSQL, ou que aceitou SSL, é ignorada até fechar.

---

## Fase 3: Arsenal

### Sprint 7: Orquestrador Docker e Nmap

**Ao final:** a tela Arsenal dispara uma varredura Nmap num container efêmero, mostra a saída ao vivo e lista as portas e serviços encontrados.

- Registro de varreduras e achados com severidade normalizada.
- Escopo travado no loopback e no domínio do alvo; argumentos sempre passados como lista.
- Containers com limite de CPU e memória, removidos ao final.
- Docker parado devolve 503 sem derrubar o resto da aplicação.
- Tela Arsenal com console ao vivo e tabela de achados, rotulados como "Visão interna".

**Branch:** `feat/sprints-5-a-8` · **Labels:** `funcionalidade`, `docker`, `visual`

**Decisões da Sprint 7:**

- Em vez do Docker SDK, um cliente mínimo da API do Docker Engine pelo socket Unix (ping, imagem, criar, iniciar, logs, esperar e remover): evita uma árvore grande de dependências para seis chamadas.
- Uma varredura pode rodar várias ferramentas em sequência, e cada alvo tem no máximo uma varredura em andamento (409 na segunda).
- Imagem do Nmap: `instrumentisto/nmap:7.95` (Alpine).

### Sprint 8: Nuclei, testssl e comparação entre scans

**Ao final:** o Arsenal tem três ferramentas e a tela de Relatórios mostra o que mudou desde a varredura anterior.

- Parsers do Nuclei (vulnerabilidades) e do testssl (criptografia TLS).
- Comparação entre scans: achados novos e resolvidos.
- Tela de Relatórios com o histórico de varreduras por alvo.

**Branch:** `feat/sprints-5-a-8` · **Labels:** `funcionalidade`, `docker`, `visual`

**Decisões da Sprint 8:**

- O Nuclei roda os templates HTTP com as tags `exposure,misconfig,tech,cve`, concorrência 10 e 100 requisições por segundo, para não sobrecarregar a produção. Ele precisa de cerca de 1 GiB e por isso tem limite de 1,5 GiB, em vez dos 512 MiB das outras ferramentas.
- O testssl escreve o JSON na saída padrão junto com o texto do console; o parser extrai os objetos e guarda só os problemas de severidade baixa ou maior, mais a nota geral.
- A comparação usa, para cada ferramenta, a última varredura concluída do mesmo alvo que rodou aquela ferramenta.

---

## Fase 4: Expansão

Já entregue fora das sprints numeradas: alertas com notificação por e-mail (PR #13), monitoramento de DNS e SMTP no Poller (PR #15) e a aplicação cobaia em `subjects/` (PR #12).

As Sprints 9 a 16 tiram do "em breve" todas as seções do menu. A **Sprint 9 vem primeiro**, porque cria a base que as Sprints 11 a 13 usam; as Sprints 10, 14, 15 e 16 não dependem do Arsenal e podem entrar em qualquer ordem depois dela.

**A ideia central das páginas do Arsenal:** hoje toda ferramenta reduz a saída a uma lista genérica de achados (título, severidade, evidência, porta). Isso continua existindo, porque alimenta a comparação entre scans e os alertas, mas cada ferramenta passa a guardar também um **relatório estruturado** e a ter uma página em `/arsenal/<ferramenta>` desenhada a partir do que ela faz. A saída bruta continua disponível, recolhida em "Saída da ferramenta".

### Sprint 9: Base de relatórios e páginas de Nmap, Nuclei e testssl

**Ao final:** cada ferramenta existente tem a sua página, e `/arsenal` vira a visão geral, que dispara várias ferramentas de uma vez e resume o último resultado de cada uma.

- Cada ferramenta passa a aceitar **opções** validadas no backend e a produzir um **relatório** em JSON, guardado por varredura (`GET /api/varreduras/{id}/relatorio/{ferramenta}`).
- **Nmap, mapa de portas:** todas as portas, inclusive fechadas e filtradas, com serviço, produto e versão, e o que abriu ou fechou desde a varredura anterior.
- **Nuclei, o que os templates acharam:** filtros por severidade e tag, resultados agrupados por template, com descrição, URL casada e referências.
- **testssl, boletim TLS:** a nota geral, os protocolos oferecidos (SSLv2 a TLS 1.3), os dias até o certificado vencer e a grade das vulnerabilidades testadas.
- Varreduras antigas, sem relatório, abrem com a tabela de achados de hoje.

**Branch:** `feat/arsenal-paginas` · **Labels:** `funcionalidade`, `docker`, `visual`

### Sprint 10: Página Sistema

**Ao final:** `/sistema` mostra CPU, memória e disco dos últimos 15 minutos, da última hora ou das últimas 24 horas, com as linhas dos limites de alerta, e o estado dos motores.

- As métricas que hoje só passam pelo WebSocket passam a ser gravadas, com retenção de 24 h (`GET /api/sistema`).

**Branch:** `feat/pagina-sistema` · **Labels:** `funcionalidade`, `motor`, `visual`

### Sprint 11: ffuf e Nikto

**Ao final:** o Arsenal descobre rotas escondidas da aplicação e falhas conhecidas do servidor web.

- **ffuf, árvore de rotas:** os caminhos encontrados em árvore, com status, tamanho e redirecionamento, e a marcação "não aparece no tráfego real" para os que o sniffer nunca viu: são as rotas escondidas. Listas de palavras do SecLists embutidas no binário (pequena e média), enviadas ao container como arquivo.
- **Nikto, checagens do servidor:** o servidor identificado e os itens agrupados em cabeçalhos, arquivos expostos, versão desatualizada e outros.

**Branch:** `feat/arsenal-ffuf-nikto` · **Labels:** `funcionalidade`, `docker`, `visual`

**Decisões da Sprint 11:**

- O ffuf não publica imagem oficial; a imagem é `secsi/ffuf:2.3.0` (Alpine).
- O ffuf roda a 100 requisições por segundo, o mesmo teto do Nuclei, em vez de 50: assim a lista média (43 mil palavras) termina em cerca de 7 minutos, dentro do limite de 10 minutos da varredura. `-maxtime 540` encerra o ffuf antes desse limite, e ele ainda grava o JSON.
- A saída cortada antes do JSON final ainda gera achados, a partir das palavras que o modo `-s` escreve a cada caminho encontrado, mas sem status nem porta; o relatório fica indisponível.
- O Nikto usa a imagem oficial `ghcr.io/sullo/nikto:2.6.1`. Ele só grava o JSON num arquivo com extensão `.json` (`-o -` e `/dev/stdout` não funcionam), por isso as ferramentas ganharam `OutputFile`: o runner lê `/tmp/nikto.json` do container depois que ele termina e entrega ao parser a saída padrão seguida do arquivo, separados por uma linha marcadora.
- O JSON do Nikto 2.6.1 grava o banner do servidor sempre como `null`; o servidor vem da linha `+ Server:` do console. Sem o JSON, os achados vêm das linhas `+ [id] url: mensagem` do console, sem porta.
- `-ask no` desliga a pergunta sobre enviar banners ao CIRT.net, que `-nointeractive` não desliga. As categorias do Nikto saem de palavras da mensagem, nesta ordem: `versao` ("outdated", "end of life"), `cabecalhos` ("header", "cookie"), `arquivos` ("file found", "directory indexing", "backup", "directory found") e `outros`.
- A marcação "não aparece no tráfego real" compara com as rotas vistas pelo sniffer nas últimas 24 horas. Sem nenhuma rota registrada (sniffer sem permissão, por exemplo), a página não marca nada e explica o motivo.

### Sprint 12: SQLMap guiado pelo tráfego

**Ao final:** o usuário escolhe uma rota que o sniffer já viu e o parâmetro a testar, e a página mostra se ele é injetável, por qual técnica e qual banco foi identificado.

- Só nível 1 e risco 1, sem nenhuma opção de extração de dados (`--dump`, `--os-shell` e afins são proibidos e testados).
- Rota e parâmetro validados no backend contra um padrão estrito.

**Decisões da Sprint 12:**

- O SQLMap não publica imagem oficial; a imagem é `googlesky/sqlmap`, fixada pela tag do commit do upstream (`upstream-5db2e62884d91f3ed3b43ec55904737e4051e9d7`). **Não verificado:** a imagem não foi baixada nem executada nesta sprint, então o entrypoint e o formato exato da saída seguem a documentação do SQLMap e os fixtures são escritos à mão; rode o teste de integração com Docker antes de confiar no resultado.
- Rota e parâmetro são opções obrigatórias: `Args` recusa a falta de qualquer uma com 400. Por isso o SQLMap só roda pela página dele; na visão geral aparece desabilitado, explicando o motivo.
- Só parâmetros GET são testados; a página lista apenas as rotas `GET` vistas pelo sniffer nas últimas 24 horas. `:id` na rota vira `1` na URL testada.
- O SQLMap sai com código diferente de zero quando não acha injeção, então a ferramenta usa `AnyExit`.

**Branch:** `feat/arsenal-sqlmap` · **Labels:** `funcionalidade`, `docker`, `visual`

### Sprint 13: Amass e Gitleaks

**Ao final:** o Arsenal descobre subdomínios do domínio do alvo e procura segredos vazados no código-fonte da aplicação.

- **Amass, subdomínios:** modo passivo (consulta fontes públicas, não manda tráfego ao alvo), com destaque para os que apontam para este servidor e aviso para os sem endereço. Só aparece habilitado para alvos com domínio.
- **Vazamentos (Gitleaks), segredos no código:** o alvo ganha o campo opcional "Código-fonte", montado somente leitura no container. Vazamentos agrupados por arquivo, com regra, linha, commit e autor. **O valor do segredo nunca é gravado nem mostrado.**

**Branch:** `feat/arsenal-amass-gitleaks` · **Labels:** `funcionalidade`, `docker`, `motor`, `visual`

### Sprint 14: Inspetor de pacotes

**Ao final:** `/inspetor` mostra os pacotes das portas dos alvos ao vivo, um por linha, com pausa e filtros por IP, porta e protocolo.

- O navegador assina o fluxo pelo WebSocket só enquanto a página está aberta; os pacotes chegam em lotes limitados, e o excesso é contado como descartado, sem travar a captura.
- Tabela virtualizada com no máximo 10 000 linhas; no HTTP, só método e rota, nunca cabeçalhos ou corpos.

**Branch:** `feat/inspetor-pacotes` · **Labels:** `funcionalidade`, `motor`, `visual`

### Sprint 15: Capturas `.pcap`

**Ao final:** `/capturas` grava capturas de 10 s a 5 min em `.pcap`, lista, baixa, apaga e abre cada uma na tabela do Inspetor.

- Limites: 100 MiB por arquivo, 1 GiB no diretório, uma captura por vez, retenção configurável (`OMNI_RETENCAO_CAPTURAS`, padrão 24 h), arquivos com permissão `0600`.

**Branch:** `feat/capturas-pcap` · **Labels:** `funcionalidade`, `motor`, `visual`

### Sprint 16: Exportação em PDF

**Ao final:** em Relatórios, "Exportar PDF" monta um relatório do alvo e do período escolhidos (disponibilidade, alertas, gráficos e varreduras) e abre a janela de impressão do navegador, que salva em PDF.

- O PDF é gerado pelo navegador, com uma página própria para impressão em A4; o backend só ganha filtro por período nas varreduras e nos alertas.

**Branch:** `feat/relatorio-pdf` · **Labels:** `funcionalidade`, `visual`

### Backlog

Itens ainda sem sprint, planejados quando entrarem na fila:

- **Aplicação:** HTTP/2, MySQL, `pg_stat_statements`, agendamento de varreduras e alertas externos além do e-mail (Telegram, Discord).
- **TLS:** metadados do handshake (SNI, versão, cifra).
- **Transporte:** RTT, retransmissões, RST, janela zero, conexões por estado e amplificação UDP.
- **Rede:** top IPs por número de conexões, ICMP, fragmentação e GeoIP.
- **Enlace:** detecção de ARP spoofing e interfaces virtuais do Docker.
- **Sistema:** contadores da placa e captura de alto desempenho.

---

## Como atualizar este arquivo

Ao abrir o PR de uma sprint, mude o status dela para 🚧 Em andamento; ao mesclar, para ✅ Concluída. Ao planejar uma sprint nova a partir do backlog, adicione uma seção para ela neste arquivo, uma linha na tabela de resumo e as tarefas no plano da Fase 4.
