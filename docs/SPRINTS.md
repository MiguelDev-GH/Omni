# Sprints do Omni

Visão geral do que é feito em cada sprint da implementação básica. Cada sprint vira **um branch e um pull request**, aberto depois que a sprint anterior entra na `main`, e entrega um binário funcional e testado.

Para os detalhes técnicos (arquivos, interfaces e testes de cada tarefa), veja o [plano de implementação](superpowers/plans/2026-11-18-implementacao-basica.md). Para a arquitetura, veja o [README](../README.md).

## Resumo

| Sprint | Fase | Entrega | Status |
|---|---|---|---|
| [1](#sprint-1-núcleo-do-backend) | Fundação | Núcleo do backend | ✅ Concluída |
| [2](#sprint-2-poller-e-websocket) | Fundação | Poller, WebSocket e métricas do sistema | ✅ Concluída |
| [3](#sprint-3-frontend-base) | Fundação | Frontend base embutido no binário | ✅ Concluída |
| [4](#sprint-4-sniffer-camadas-3-e-4) | Observação passiva | Captura de tráfego e gráficos de banda e protocolos | ✅ Concluída |
| [5](#sprint-5-decodificação-http) | Observação passiva | Requisições por rota e status HTTP | 🚧 Em andamento |
| [6](#sprint-6-decodificação-postgresql) | Observação passiva | Consultas ao banco | 🚧 Em andamento |
| [7](#sprint-7-orquestrador-docker-e-nmap) | Arsenal | Orquestrador Docker e varredura Nmap | 🚧 Em andamento |
| [8](#sprint-8-nuclei-testssl-e-comparação-entre-scans) | Arsenal | Nuclei, testssl e comparação entre scans | 🚧 Em andamento |
| [Expansão](#fase-4-expansão) | Expansão | Restante das pendências do README | 📋 Backlog |

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

Itens fora do escopo básico. Cada um vira uma sprint própria quando entrar na fila:

- **Aplicação:** HTTP/2, MySQL, `pg_stat_statements`, Poller de DNS e SMTP, Nikto, SQLMap, ffuf, Amass, Gitleaks, agendamento de varreduras, alertas externos e relatório em PDF.
- **TLS:** metadados do handshake (SNI, versão, cifra).
- **Transporte:** RTT, retransmissões, RST, janela zero, conexões por estado e amplificação UDP.
- **Rede:** ICMP, fragmentação e GeoIP.
- **Enlace:** detecção de ARP spoofing e interfaces virtuais do Docker.
- **Sistema:** contadores da placa, captura de alto desempenho, Inspetor de Tráfego pacote a pacote e retenção das capturas.

---

## Como atualizar este arquivo

Ao abrir o PR de uma sprint, mude o status dela para 🚧 Em andamento; ao mesclar, para ✅ Concluída. Ao planejar uma sprint nova da Fase 4, adicione uma seção para ela neste arquivo e uma linha na tabela de resumo.
