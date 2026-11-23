<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/img/logo-escuro.png" />
    <img src="docs/img/logo-claro.png" alt="Omni" width="360" />
  </picture>
</p>

# Omni

Plataforma de **monitoramento e segurança** que roda **na mesma máquina do servidor** que observa. Ela une três visões em um único painel:

- **Monitoramento passivo:** captura de tráfego com gopacket, um "mini-Wireshark" embutido, que decodifica HTTP, PostgreSQL, DNS e SMTP.
- **Monitoramento ativo:** testes periódicos de HTTP, DNS e SMTP, mais métricas do sistema (CPU, memória, disco e I/O), conexões por porta e tamanho do banco.
- **Arsenal de segurança:** ferramentas como Nmap e Nuclei executadas em containers Docker efêmeros.
- **Detecção de ataques:** brute-force e password spraying, picos de tráfego (DDoS) e alteração de arquivos de configuração.

**Documentação:** [guia de instalação](docs/INSTALACAO.md) · [runbooks de operação](docs/RUNBOOKS.md) · [playbooks de incidente](docs/PLAYBOOKS.md) · [testes de navegação](docs/TESTES-NAVEGACAO.md)

> **Status:** as Sprints 1 a 8 (núcleo, Poller, interface, sniffer das camadas 3 e 4, HTTP, PostgreSQL e Arsenal com Nmap, Nuclei e testssl) estão prontas. Da Fase 4 já estão na `main` a página própria de cada ferramenta do Arsenal (Nmap, Nuclei, testssl, ffuf, Nikto, SQLMap e Amass), a tela Sistema e o relatório em PDF. Também estão prontos os alertas por e-mail, as telas de DNS e SMTP (sonda ativa e tráfego real), as conexões por porta, o I/O de disco e o tamanho do PostgreSQL. Faltam Gitleaks (Vazamentos), o Inspetor de pacotes e as capturas `.pcap`. Veja [docs/SPRINTS.md](docs/SPRINTS.md) e a [cobertura do enunciado](#cobertura-do-enunciado).

> **▶️ Quer rodar agora?** Vá para [Como inicializar e testar](#como-inicializar-e-testar): passo a passo para distribuições Linux, Windows com WSL2 e Windows nativo.

---

## Sumário

- [Como inicializar e testar](#como-inicializar-e-testar)
- [Visão geral](#visão-geral)
- [Arquitetura](#arquitetura)
- [Componentes](#componentes)
- [Fluxo de funcionamento](#fluxo-de-funcionamento)
- [Comunicação em tempo real (WebSocket)](#comunicação-em-tempo-real-websocket)
- [Interface (frontend)](#interface-frontend)
- [Aplicação cobaia](#aplicação-cobaia)
- [Alertas e notificação por e-mail](#alertas-e-notificação-por-e-mail)
- [Monitoramento de DNS e SMTP](#monitoramento-de-dns-e-smtp)
- [Conexões, disco e tamanho do banco](#conexões-disco-e-tamanho-do-banco)
- [Cobertura do enunciado](#cobertura-do-enunciado)
- [Segurança: brute-force, DDoS e integridade de arquivos](#segurança-brute-force-ddos-e-integridade-de-arquivos)
- [Requisitos de implantação](#requisitos-de-implantação)
- [O que falta implementar, por camada](#o-que-falta-implementar-por-camada)
- [Segurança da própria plataforma](#segurança-da-própria-plataforma)
- [Limitações conhecidas](#limitações-conhecidas)
- [Roadmap](#roadmap)
- [Stack](#stack)

---

## Como inicializar e testar

Este é o caminho curto para rodar e testar. A configuração completa (todas as variáveis, serviço do systemd e desinstalação) está no [guia de instalação](docs/INSTALACAO.md).

| Onde | O que funciona | Passo a passo |
|---|---|---|
| Linux | Tudo | [Linux](#linux) |
| Windows com WSL2 | Tudo, para o que roda dentro do WSL | [Windows com WSL2](#windows-com-wsl2) |
| Windows nativo | Interface e Poller; sem Sniffer e sem Arsenal | [Windows nativo](#windows-nativo) |

### Linux

#### 1. Instalar as dependências

| Ferramenta | Versão | Para quê |
|---|---|---|
| Go | 1.24 ou mais nova | Compilar o backend |
| Node.js e npm | Node 22.12 ou mais novo | Compilar a interface |
| gcc e libpcap (com os cabeçalhos) | | Captura de pacotes |
| make, git, openssl, `setcap` | | Build, token e permissões |
| Docker Engine com Compose v2 | | Arsenal e aplicação cobaia |

Debian e Ubuntu:

```bash
sudo apt update
sudo apt install -y git make gcc openssl libpcap-dev libcap2-bin golang-go
```

Arch e derivadas:

```bash
sudo pacman -S --needed git make gcc openssl libpcap libcap go nodejs npm docker docker-compose
```

Fedora:

```bash
sudo dnf install -y git make gcc openssl libpcap-devel libcap golang nodejs npm
```

- **Go:** confira com `go version`. Se o pacote da distribuição for anterior a 1.24, instale pelo [site oficial](https://go.dev/doc/install).
- **Node.js:** confira com `node --version`. Os repositórios do Debian e do Ubuntu podem trazer uma versão antiga demais; nesse caso, use o [nvm](https://github.com/nvm-sh/nvm) (`nvm install 24`) ou o [site oficial](https://nodejs.org/).
- **Docker:** no Debian, Ubuntu e Fedora, siga o [guia oficial do Docker Engine](https://docs.docker.com/engine/install/). Depois, em qualquer distribuição, ligue o serviço e libere o seu usuário (saia da sessão e entre de novo):
  ```bash
  sudo systemctl enable --now docker
  sudo usermod -aG docker $USER
  docker compose version   # precisa responder sem sudo
  ```

#### 2. Baixar e compilar

```bash
git clone https://github.com/MiguelDev-GH/Omni.git
cd Omni
make build               # compila a interface e gera ./omni
```

#### 3. Iniciar o Omni

```bash
export OMNI_TOKEN=$(openssl rand -hex 32)   # mínimo de 32 caracteres
echo $OMNI_TOKEN                            # guarde: é a senha do painel
OMNI_ADDR=127.0.0.1:8080 make backend       # pede a senha do sudo (setcap)
```

Abra <http://127.0.0.1:8080> e informe o token. Ctrl+C encerra o Omni.

Sem `OMNI_ADDR`, o Omni escolhe uma porta livre a cada execução, e o endereço sai no log (`painel=http://127.0.0.1:<porta>`). Sem `make backend`, o binário roda com `./omni`, mas o sniffer fica em `sem-permissao` até você aplicar `sudo setcap cap_net_raw,cap_net_admin=eip ./omni`.

#### 4. Testar com a aplicação cobaia

Com o Omni no ar, abra mais dois terminais na raiz do repositório.

Terminal 2, a loja de exemplo (PostgreSQL, Caddy, API e gerador de carga):

```bash
make loja                # fica rodando até Ctrl+C
```

Terminal 3, o cadastro dos alvos `loja` e `loja-tls`:

```bash
OMNI_TOKEN=<token> OMNI_URL=http://127.0.0.1:8080 make loja-cadastrar
```

No painel, confira:

| Faça | Deve aparecer |
|---|---|
| Abra a tela Geral | Requisições, status 2xx/4xx/5xx e protocolos `HTTP` e `PostgreSQL` |
| Abra Requisições na API e Consultas no banco | Rotas como `/produtos/:id` e as consultas mais lentas |
| Pare o terminal 2 com Ctrl+C | Alerta de queda |
| Rode `make loja` de novo | Serviço restaurado |
| No Arsenal, rode o Nmap no alvo `loja` | Portas 3000 e 5433 (a primeira execução baixa a imagem) |

O roteiro completo está em [subjects/loja/README.md](subjects/loja/README.md#roteiro-de-teste-do-omni). Para desligar os containers da loja (o Ctrl+C não os para): `make -C subjects/loja derrubar`.

Para testar as sondas de DNS e SMTP e os e-mails de alerta, veja [Monitoramento de DNS e SMTP](#monitoramento-de-dns-e-smtp) e [Configurar o e-mail](#configurar-o-e-mail).

#### 5. Rodar os testes automatizados

`make check` roda formatação, lint, typecheck, build e testes do backend, da interface e da loja, parando na primeira falha. Ele precisa do [golangci-lint v2](https://golangci-lint.run/welcome/install/) e das dependências da interface (instaladas pelo `make build`).

```bash
make check
```

Para rodar só uma parte:

```bash
go test ./...                            # backend
(cd web && npm test)                     # interface
(cd subjects/loja && go test ./...)      # loja cobaia
```

### Windows com WSL2

É o caminho para ter o Omni completo no Windows.

1. No PowerShell como administrador, instale uma distribuição e reinicie o computador se for pedido:
   ```powershell
   wsl --install -d Ubuntu
   ```
2. No Docker Desktop, se ele estiver instalado, desligue a integração da distribuição em *Settings → Resources → WSL integration*.
3. Dentro do terminal da distribuição, siga os passos de [Linux](#linux), clonando o repositório na sua pasta do Linux (`cd ~`), e não em `/mnt/c`.
4. Abra o painel no navegador do Windows, em <http://localhost:8080>.

Os motivos de cada cuidado estão em [Windows](#windows).

### Windows nativo

Serve para desenvolver e para testar a interface e o Poller. O Sniffer fica inativo e o Arsenal indisponível. Os comandos abaixo são para o PowerShell.

**Dependências:** [Git](https://git-scm.com/download/win), [Go](https://go.dev/doc/install) 1.24 ou mais novo, [Node.js](https://nodejs.org/) 22.12 ou mais novo e, para a loja cobaia, o [Docker Desktop](https://www.docker.com/products/docker-desktop/). O `make` não é necessário.

#### 1. Baixar e compilar

O `core.autocrlf=input` mantém o fim de linha dos arquivos como no Linux; sem ele, testes que leem arquivos de exemplo falham.

```powershell
git clone -c core.autocrlf=input https://github.com/MiguelDev-GH/Omni.git
cd Omni
```

Compile a interface, copie o resultado para dentro do backend e gere o executável:

```powershell
cd web
npm ci
npm run build
cd ..
Get-ChildItem internal\web\static -Exclude placeholder.html | Remove-Item -Recurse -Force
Copy-Item web\dist\* internal\web\static -Recurse -Force
go build -o omni.exe ./cmd/omni
```

#### 2. Iniciar o Omni

```powershell
$bytes = New-Object byte[] 32
[Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
$env:OMNI_TOKEN = -join ($bytes | ForEach-Object { $_.ToString('x2') })
$env:OMNI_TOKEN          # guarde: é a senha do painel
$env:OMNI_ADDR = "127.0.0.1:8080"
.\omni.exe
```

Abra <http://127.0.0.1:8080> e informe o token. Ctrl+C encerra o Omni.

#### 3. Testar com a aplicação cobaia

O `compose.yaml` da loja usa a rede do host, que não serve no Docker Desktop. Suba só o PostgreSQL, publicando a porta:

```powershell
docker run -d --name loja-postgres -p 127.0.0.1:5433:5432 -e POSTGRES_USER=loja -e POSTGRES_PASSWORD=loja-cobaia -e POSTGRES_DB=loja postgres:17.6-alpine
```

Em um segundo terminal, na pasta `subjects\loja`, inicie a API:

```powershell
go run ./cmd/loja
```

Em um terceiro terminal, também em `subjects\loja`, cadastre os alvos e gere carga:

```powershell
$env:OMNI_TOKEN = "<token>"
$env:OMNI_URL = "http://127.0.0.1:8080"
go run ./cmd/cadastrar
go run ./cmd/carga
```

No painel, o alvo `loja` aparece no ar nas portas 3000 e 5433. Pare a API com Ctrl+C para ver o alerta de queda e inicie de novo para ver o serviço restaurado. O alvo `loja-tls` aparece fora do ar, porque o Caddy não foi iniciado. Os gráficos de requisições e consultas ficam vazios, porque dependem do Sniffer.

Para remover o banco depois do teste:

```powershell
docker rm -f -v loja-postgres
```

#### 4. Rodar os testes automatizados

```powershell
go test ./...
```

```powershell
cd web; npm run lint; npm run typecheck; npm test; cd ..
```

```powershell
cd subjects\loja; go test ./...; cd ..\..
```

No Windows nativo, alguns testes de `internal/config`, `internal/security`, `internal/sniffer` e `internal/store` falham, porque dependem do comportamento do Linux (caminhos, permissões e nomes de arquivo). Os testes da interface e da loja passam. O `go test -race` exige um gcc de 64 bits. Para a verificação completa, use o Linux ou o WSL2.

---

## Visão geral

O Omni é distribuído como um **binário Go único**, com o frontend React embutido via `go:embed`, instalado no próprio servidor da aplicação monitorada. Como compartilha a máquina com a aplicação, o Omni enxerga:

- o tráfego que chega à aplicação, inclusive o HTTP em texto puro atrás do reverse proxy, na interface `lo`;
- as consultas que a aplicação faz ao banco, quando o banco está na mesma máquina e é acessado via TCP local;
- o consumo de recursos do servidor (CPU, RAM, disco, conexões).

## Arquitetura

```mermaid
graph TD
    subgraph Servidor["Servidor (Linux)"]
        Proxy["Reverse proxy<br/>Nginx/Caddy (TLS :443)"]
        App["Aplicação alvo<br/>(ex: :3000)"]
        DB["Banco de dados<br/>Postgres/MySQL (127.0.0.1)"]
        Proxy -->|HTTP via lo| App
        App -->|TCP via lo| DB

        subgraph Omni["Omni (binário Go único)"]
            Sniffer["Sniffer<br/>gopacket + reassembly<br/>HTTP · PostgreSQL · DNS · SMTP<br/>(CAP_NET_RAW)"]
            Poller["Poller<br/>sondas HTTP / TCP / DNS / SMTP"]
            Coletores["Coletores<br/>CPU, memória, disco e I/O<br/>conexões por porta<br/>tamanho do PostgreSQL · fila SMTP"]
            Orq["Orquestrador Docker<br/>(cliente da API do Engine)"]
            Agg["Agregador<br/>(janelas de 1 s e 10 s)"]
            Alertas["Motor de alertas<br/>+ e-mail"]
            API["API REST"]
            WS["Hub WebSocket"]
            Store[("SQLite (WAL)<br/>agregados")]
        end

        subgraph Docker["Containers efêmeros"]
            Tools["Nmap · Nuclei · Nikto · SQLMap<br/>ffuf · Amass · testssl · Gitleaks"]
        end
    end

    UI["Frontend React<br/>(embutido no binário)"]

    Sniffer -.->|escuta lo / any| Proxy
    Sniffer -.-> DB
    Sniffer --> Agg
    Poller --> Agg
    Poller -.->|testes ativos| Proxy
    Coletores -.->|consulta opcional| DB
    Coletores --> Store
    Coletores --> WS
    Agg --> Store
    Agg --> WS
    Agg --> Alertas
    Poller --> Alertas
    Coletores --> Alertas
    Alertas --> Store
    Alertas --> WS
    Orq -->|inicia / destrói| Tools
    Tools -->|JSON / XML| Orq
    Orq -->|achados normalizados| Store
    Orq -->|scan concluído| WS
    API <--> Store
    API --> Orq
    UI <-->|comandos| API
    UI <-->|tempo real| WS
```

## Componentes

| Componente | Responsabilidade |
|---|---|
| **Sniffer** | Captura nas interfaces `lo`/`any` com filtro BPF nas portas do alvo (TCP da aplicação, do banco e do SMTP; UDP do DNS). Remonta fluxos TCP e decodifica HTTP/1.1, PostgreSQL e SMTP; pareia consultas e respostas DNS por UDP. |
| **Poller** | Testes ativos periódicos: disponibilidade e latência HTTP, resolução DNS, conversa SMTP e STARTTLS. |
| **Coletores do sistema** | CPU, memória, disco e I/O do disco (`gopsutil`), conexões TCP abertas por porta e por processo cliente (`/proc`), tamanho do PostgreSQL (conexão opcional) e fila de saída do servidor de e-mail (diretórios opcionais). |
| **Orquestrador Docker** | Sobe containers efêmeros do arsenal com limites de CPU e RAM, coleta a saída, normaliza os achados num esquema único e destrói o container. |
| **Agregador** | Consolida o tráfego em janelas de 1 s (banda, pacotes, protocolos, principais origens) e as métricas de aplicação em janelas de 10 s (requisições por rota, status e códigos HTTP, consultas ao banco, DNS e SMTP) e grava no SQLite. |
| **Motor de alertas** | Compara as métricas com os limites de cada regra, classifica cada recurso em verde, amarelo ou vermelho e avisa as mudanças pelo WebSocket, no banco e por e-mail. |
| **API REST** | CRUD de alvos, disparo de scans, consultas ao histórico e aos relatórios de cada ferramenta. `GET /api/alvos/{id}/relatorio` junta o que o PDF precisa; o navegador monta o arquivo com jsPDF. |
| **Hub WebSocket** | Envia alertas, métricas e o andamento das varreduras ao frontend, com buffer por cliente. |
| **Armazenamento** | SQLite em modo WAL, só com agregados, achados e relatórios das ferramentas. Capturas completas ficam em arquivos `.pcap` com retenção configurável (Sprint 15). |

### Arsenal (containers)

Cada ferramenta tem uma página própria em `/arsenal/<ferramenta>`, desenhada a partir do que ela faz. Os achados de todas seguem um esquema único (para a comparação entre scans e os contadores por severidade), e cada uma guarda também um relatório estruturado, que alimenta a sua página. A saída bruta fica disponível, recolhida, em "Saída da ferramenta".

| Ferramenta | Função | Página na interface | Situação |
|---|---|---|---|
| **Nmap** | Portas abertas e serviços | Mapa de portas: todas as portas com estado, serviço e versão, e o que abriu ou fechou desde a varredura anterior | Pronta |
| **Nuclei** | Vulnerabilidades conhecidas (CVEs) e más configurações, via templates | Resultados por template, com filtros por severidade e tag, descrição e referências | Pronta |
| **testssl.sh** | Validade do certificado e força da criptografia TLS | Boletim TLS: nota, protocolos oferecidos, dias até o certificado vencer e vulnerabilidades testadas | Pronta |
| **ffuf** | Descoberta de diretórios e rotas ocultas | Árvore de rotas, marcando as que nunca apareceram no tráfego real | Pronta |
| **Nikto** | Falhas em servidores web | Checagens do servidor agrupadas em cabeçalhos, arquivos expostos, versão e outros | Pronta |
| **SQLMap** | Injeção de SQL | Escolha de uma rota vista pelo sniffer e do parâmetro; veredito, técnicas e banco identificado | Pronta |
| **Amass** | Descoberta de subdomínios do domínio da aplicação (modo passivo) | Subdomínios com endereços, destacando os que apontam para este servidor | Pronta |
| **Gitleaks** (Vazamentos) | Vazamento de segredos no código | Vazamentos por arquivo, com regra, linha e commit, sem nunca mostrar o segredo | A fazer (Sprint 13) |

## Fluxo de funcionamento

1. **Cadastro do alvo:** o usuário informa as portas da aplicação, a porta do banco e o domínio. O formulário sugere os serviços que já escutam portas TCP na máquina (aplicações e bancos, sem os serviços do sistema), e um clique preenche host e portas.
2. **Captura passiva:** o Sniffer aplica um filtro BPF nessas portas. A partir do tráfego gera métricas de requisições, status HTTP, latência, consultas ao banco, banda e protocolos.
3. **Testes ativos:** o Poller verifica periodicamente se a aplicação está no ar e respondendo bem, e coleta métricas do sistema.
4. **Agregação:** a cada segundo (tráfego) e a cada 10 s (HTTP, banco, DNS e SMTP), o Agregador salva os agregados no SQLite e os envia pelo WebSocket, e os gráficos se atualizam.
5. **Varredura de segurança:** sob demanda (o agendamento ainda não existe), o Orquestrador roda as ferramentas selecionadas, consolida as vulnerabilidades, compara com o scan anterior e avisa o fim pelo WebSocket.
6. **Relatórios:** o usuário escolhe o alvo e o período (último dia, última semana ou de/até, no máximo 168 h) e baixa um PDF com alertas, disponibilidade, varreduras do Arsenal com os achados e os gráficos do período. Alertas, tráfego, HTTP, banco e sistema são do servidor inteiro.

## Comunicação em tempo real (WebSocket)

- **Protocolo:** WebSocket puro com `gorilla/websocket` no Go e `react-use-websocket` no React. A reconexão automática fica a cargo da biblioteca do frontend.
- **Comandos** que alteram estado (criar alvo, iniciar scan) usam a API REST. O WebSocket só transporta eventos do servidor para o navegador.
- **Cada cliente tem um canal com buffer próprio.** Se o cliente ficar lento, mensagens antigas são descartadas em vez de travar o hub.
- **Saídas longas vão em lotes:** a saída das ferramentas do Arsenal sai a cada 500 ms, nunca linha a linha, e os pacotes do Inspetor saem a cada 250 ms, com no máximo 500 por lote.

**Conexão:** o cliente pede um ticket em `POST /api/ws/ticket`, com o token da API, e abre a conexão em `GET /ws?ticket=<ticket>`. O ticket vale por 30 s e só pode ser usado uma vez:

```bash
curl -s -X POST -H "Authorization: Bearer $OMNI_TOKEN" 127.0.0.1:<porta>/api/ws/ticket
# {"ticket":"..."}  →  ws://127.0.0.1:<porta>/ws?ticket=...
```

Toda mensagem vem no envelope `{ "tipo": ..., "dados": ... }`. Os eventos:

```json
{ "tipo": "STATUS_ALVOS", "dados": { "resultados": [ { "alvo_id": 1, "porta": 3000, "tipo": "http", "no_ar": true, "status_http": 200, "latencia_ms": 1.2, "hora": "2026-11-20T17:31:46Z" } ] } }
{ "tipo": "ALERTA_QUEDA", "dados": { "alvo_id": 1, "alvo": "api", "porta": 3000, "erro": "dial tcp 127.0.0.1:3000: connect: connection refused", "hora": "2026-11-20T17:31:52Z" } }
{ "tipo": "SERVICO_RESTAURADO", "dados": { "alvo_id": 1, "alvo": "api", "porta": 3000, "hora": "2026-11-20T17:31:56Z" } }
{ "tipo": "METRICAS_SISTEMA", "dados": { "cpu": 10.9, "memoria": 50.2, "disco": 4.7, "io": true, "leitura_bps": 204800, "escrita_bps": 51200, "iops": 42, "latencia_disco_ms": 0.8, "disco_ocupado": 3.5, "hora": "2026-11-20T17:31:49Z" } }
{ "tipo": "METRICAS_TRAFEGO", "dados": { "inicio": "2026-11-21T12:00:00Z", "bytes": 48210, "pacotes": 61, "descartados": 0, "protocolos": { "HTTP": 40100, "PostgreSQL": 8110 }, "top_origens": [ { "ip": "127.0.0.1", "bytes": 30020, "pacotes": 33 } ] } }
{ "tipo": "ESTADO_MOTORES", "dados": { "sniffer": "ativo" } }
{ "tipo": "METRICAS_HTTP", "dados": { "inicio": "2026-11-21T12:00:00Z", "requisicoes": 42, "status": { "2xx": 39, "4xx": 1, "5xx": 2 }, "codigos": { "404": 1, "503": 2 }, "p50_ms": 1.4, "p95_ms": 12.8, "rotas": [ { "metodo": "GET", "rota": "/users/:id", "requisicoes": 30, "erros": 0, "p50_ms": 1.2, "p95_ms": 3.1 } ] } }
{ "tipo": "METRICAS_BANCO", "dados": { "inicio": "2026-11-21T12:00:00Z", "consultas": 18, "erros": 1, "tipos": { "SELECT": 15, "UPDATE": 2, "ROLLBACK": 1 }, "transacoes_falhas": 1, "p50_ms": 0.6, "p95_ms": 4.2, "top": [ { "consulta": "SELECT * FROM u WHERE id = ?", "tipo": "SELECT", "execucoes": 15, "erros": 0, "p50_ms": 0.5, "p95_ms": 1.1 } ] } }
{ "tipo": "METRICAS_DNS", "dados": { "inicio": "2026-11-21T12:00:00Z", "consultas": 50, "falhas": 2, "rcodes": { "NOERROR": 48, "NXDOMAIN": 1, "TIMEOUT": 1 }, "p50_ms": 0.4, "p95_ms": 1.9 } }
{ "tipo": "METRICAS_SMTP", "dados": { "inicio": "2026-11-21T12:00:00Z", "tentativas": 5, "entregues": 4, "rejeitados": 1, "abortados": 0, "codigos": { "550": 1 }, "p50_ms": 12, "p95_ms": 40 } }
{ "tipo": "FILA_SMTP", "dados": { "hora": "2026-11-21T12:00:00Z", "mensagens": 3 } }
{ "tipo": "METRICAS_CONEXOES", "dados": { "hora": "2026-11-21T12:00:00Z", "portas": [ { "porta": 5432, "tipo": "banco", "estabelecidas": 6, "estados": { "ESTABLISHED": 6 }, "clientes": [ { "processo": "loja", "conexoes": 5 }, { "processo": "psql", "conexoes": 1 } ] } ] } }
{ "tipo": "ESTADO_MOTORES", "dados": { "docker": "ativo" } }
{ "tipo": "SCAN_INICIADO", "dados": { "scan_id": 3, "alvo_id": 1, "ferramentas": ["nmap"] } }
{ "tipo": "SCAN_LOG", "dados": { "scan_id": 3, "ferramenta": "nmap", "linhas": ["Starting Nmap 7.95"] } }
{ "tipo": "SCAN_CONCLUIDO", "dados": { "scan_id": 3, "alvo_id": 1, "ferramentas": ["nmap"], "achados": 2, "novos_desde_ultimo": 1 } }
{ "tipo": "SCAN_FALHOU", "dados": { "scan_id": 4, "alvo_id": 1, "erro": "nmap: a ferramenta terminou com código 1", "achados": 0 } }
```

- `STATUS_ALVOS` sai a cada rodada do poller (`OMNI_INTERVALO_POLLER`, padrão 30 s, mínimo 1 s), com uma verificação por porta: HTTP nas portas da aplicação, TCP na porta do banco e as sondas DNS e SMTP nas portas desses serviços. Os resultados ficam no banco por `OMNI_RETENCAO_VERIFICACOES` (padrão `168h`, ou 7 dias; mínimo `1h`, no formato de duração do Go, como `48h`); a mesma retenção vale para as métricas de tráfego, HTTP, banco, sistema e para o histórico de alertas, e o histórico de cada alvo traz até 2880 resultados por porta.
- `ALERTA_QUEDA` e `SERVICO_RESTAURADO` saem só na mudança de estado de uma porta.
- `METRICAS_SISTEMA` sai a cada 5 s, com CPU, memória e disco (do volume do banco) em porcentagem e o I/O desse disco no intervalo: bytes lidos e escritos por segundo, IOPS, tempo médio por operação (`latencia_disco_ms`) e percentual de tempo ocupado. `io` é `false` na primeira leitura e quando o dispositivo não é encontrado (como no Windows nativo). As leituras ficam no banco por `OMNI_RETENCAO_VERIFICACOES` e saem em `GET /api/sistema` (últimos 15 minutos; aceita `?desde=<RFC3339>`, até 24 h, com uma média por minuto acima de 1 h), que alimenta a tela Sistema.
- O histórico das verificações das últimas 24 h fica em `GET /api/alvos/{id}/historico` (aceita `?desde=<RFC3339>`).
- `METRICAS_TRAFEGO` sai a cada segundo enquanto o sniffer captura: bytes, pacotes, bytes por protocolo, os dez IPs de origem com mais tráfego e os pacotes que o pcap descartou (`descartados` maior que zero indica métricas incompletas). O protocolo é deduzido pela porta: as portas da aplicação contam como HTTP, e as portas DNS e SMTP dos alvos, como DNS e SMTP.
- `ESTADO_MOTORES` sai a cada 5 s e a cada mudança, com o estado do sniffer: `ativo`, `inativo` (sem alvos cadastrados) ou `sem-permissao` (falta `CAP_NET_RAW`; o resto do Omni segue no ar).
- As janelas com tráfego ficam no banco por `OMNI_RETENCAO_VERIFICACOES` e saem em `GET /api/trafego` (últimos 15 minutos; aceita `?desde=<RFC3339>`, até 3600 janelas).
- `METRICAS_HTTP` sai a cada 10 s: o sniffer remonta os fluxos TCP das portas da aplicação (`gopacket/reassembly`, até 4 MiB em buffer por conexão, fluxos ociosos fechados em 2 min), pareia requisição e resposta (inclusive em pipeline e fora de ordem) e agrega contagem, classes de status e latência p50/p95 por rota normalizada (`/users/123` vira `/users/:id`). Cabeçalhos, query strings e corpos nunca são guardados, e o tráfego com User-Agent `Omni-` (Poller e Arsenal) fica fora. Rotas: `GET /api/requisicoes` (janelas, últimos 15 minutos) e `GET /api/requisicoes/rotas` (as 100 rotas de mais volume das últimas 24 h).
- `METRICAS_BANCO` sai a cada 10 s com as consultas ao PostgreSQL na porta do banco dos alvos (consultas simples e Parse/Bind/Execute, com erros). O SQL é normalizado: literais viram `?`, comentários somem e listas viram `(?)`. Mensagens de senha nunca são lidas, e conexões com TLS são ignoradas. Rotas: `GET /api/consultas` e `GET /api/consultas/lentas?limite=` (padrão 20, as de maior p95 em 24 h). As métricas HTTP, de banco, DNS e SMTP seguem a mesma retenção (`OMNI_RETENCAO_VERIFICACOES`).
- `METRICAS_HTTP` traz também `codigos`, a contagem das respostas de erro (status 400 ou maior) pelo código exato.
- `METRICAS_BANCO` traz os rollbacks pedidos pela aplicação em `tipos.ROLLBACK` e as transações abortadas por erro em `transacoes_falhas` (o `ReadyForQuery` do PostgreSQL passa a informar o estado `E`; cada transação conta uma vez).
- `METRICAS_DNS` e `METRICAS_SMTP` saem a cada 10 s com o que o sniffer viu nas portas DNS e SMTP dos alvos; veja [Tráfego real](#tráfego-real). Rotas: `GET /api/dns`, `GET /api/smtp` e `GET /api/smtp/fila`.
- `METRICAS_CONEXOES` sai a cada 5 s com as conexões TCP de cada porta dos alvos (aplicação, banco e SMTP): as abertas, os estados e, do lado cliente, os processos e quantas conexões cada um mantém (o pool). Rota: `GET /api/conexoes`.
- `FILA_SMTP` sai a cada 30 s quando `OMNI_SMTP_FILA_DIRS` está definido.
- O Docker informa o estado no `ESTADO_MOTORES` a cada 5 s: `ativo` ou `indisponivel`.
- `ALERTA` sai quando uma métrica muda de nível (verde, amarelo ou vermelho); veja [Alertas e notificação por e-mail](#alertas-e-notificação-por-e-mail).
- `SCAN_LOG` leva a saída da ferramenta em lotes a cada 500 ms (no máximo 20 000 linhas por ferramenta); `SCAN_CONCLUIDO` traz quantos achados são novos em relação à varredura anterior.
- `PACOTES` só vai aos navegadores que assinaram o Inspetor com `{"tipo":"INSPETOR","dados":{"ativo":true,"filtro":{"ip":"","porta":0,"protocolo":""}}}`: a cada 250 ms, no máximo 500 pacotes já filtrados, cada um com hora, origem, destino, portas, protocolo, tamanho, flags TCP e `info` (no HTTP só método e rota, no PostgreSQL só o tipo da mensagem). `descartados` conta o que não coube. Sem nenhum assinante, o sniffer nem monta os resumos.

## Interface (frontend)

**Como rodar:** `make build` compila a interface (em `web/`) e gera o binário `./omni` com ela embutida. Sem `OMNI_ADDR`, o Omni escuta em `127.0.0.1` numa porta alta livre escolhida pelo sistema, diferente a cada execução; o endereço sai no log de início (`omni no ar ... painel=http://127.0.0.1:<porta>`). Abra esse endereço e informe o token. Para fixar a porta, defina `OMNI_ADDR` (ex.: `OMNI_ADDR=127.0.0.1:8080`).

Para desenvolver a interface com recarga automática, `make dev` sobe o backend e o Vite, cada um numa porta alta livre, e imprime o endereço do painel:

```bash
OMNI_TOKEN=<token> make dev
```

Para subir só o backend com um token aleatório novo, `make backend` compila o binário, aplica `setcap` com `sudo` (para a captura de pacotes), gera o `OMNI_TOKEN`, imprime no terminal e inicia o Omni. Se `OMNI_TOKEN` já estiver no ambiente, ele é reaproveitado.

`make loja` sobe a loja cobaia (PostgreSQL, Caddy, API e carga) até Ctrl+C, sem credenciais: ela não conhece o Omni. `make loja port=3001` troca a porta da API (padrão 3000) e o Caddy acompanha. Para cadastrar os alvos no Omni, rode `OMNI_TOKEN=<token> OMNI_URL=http://127.0.0.1:<porta> make loja-cadastrar` (token e porta saem no terminal do `make backend`) ou use a página de Alvos.

**Barra superior:** seletor global de alvo, estado dos motores (Sniffer, Poller, Docker), troca de tema e botão Sair. Os avisos (quedas, alertas, fim de varreduras) aparecem como notificações flutuantes no canto da tela.

**Barra lateral:**

- **Monitoramento:** Geral, Alertas, Requisições na API, Consultas no banco, DNS, SMTP, Status HTTP, Sistema
- **Tráfego:** Inspetor de pacotes, Capturas `.pcap` (Sprint 15)
- **Arsenal:** visão geral em `/arsenal` e uma página por ferramenta: Nmap, Nuclei, Nikto, SQLMap, ffuf, Amass, testssl e Vazamentos (Sprint 13)
- **Relatórios:** histórico, comparação entre scans e exportação em PDF (o PDF é gerado na própria página Relatórios; o item "Exportação em PDF" do menu ainda aparece como "em breve")

Os itens com sprint ainda aparecem no menu como "em breve". O que cada sprint entrega está em [docs/SPRINTS.md](docs/SPRINTS.md).

**Tela Geral:**

- Requisições e consultas por segundo, conexões ativas, banda e pacotes por segundo
- Gráfico de requisições na API
- Gráfico de consultas no banco
- Barras empilhadas de status HTTP (2xx, 3xx, 4xx, 5xx)
- Consumo de banda
- Distribuição de protocolos
- Logs de **eventos agregados**, como quedas, início e fim de scans e picos de erro (ainda não implementado; hoje esses eventos aparecem como notificações)

O detalhe pacote a pacote fica **apenas no Inspetor de pacotes** (Sprint 14), que usa uma tabela virtualizada com botão de pausa e filtros. Colocá-lo na tela Geral travaria o navegador. O navegador só recebe pacotes enquanto a página do Inspetor está aberta, em lotes limitados, e no HTTP vê só método e rota, nunca cabeçalhos ou corpos.

As telas laterais de métricas são **drill-down**: mostram o gráfico e, abaixo dele, uma tabela detalhada, por exemplo as consultas mais lentas das últimas 24 horas.

## Aplicação cobaia

Para testar o Omni de ponta a ponta sem uma aplicação real, a pasta [subjects/](subjects/README.md) tem uma loja de exemplo (Go, PostgreSQL e Caddy com TLS) com gerador de carga, cadastro automático dos alvos e falhas de propósito para o Arsenal encontrar. Com o Omni no ar, `cd subjects/loja && OMNI_TOKEN=<token> make demo` sobe tudo.

## Alertas e notificação por e-mail

O motor de alertas (`internal/alerts`) observa os mesmos eventos que o frontend recebe (`STATUS_ALVOS`, `METRICAS_HTTP`, `METRICAS_BANCO`, `METRICAS_DNS`, `METRICAS_SMTP`, `FILA_SMTP` e `METRICAS_SISTEMA`), mais os detectores de segurança, compara cada métrica com dois limites e classifica o recurso em três níveis:

| Nível | Significado | Ação |
|---|---|---|
| 🟢 Verde | Estado normal | Nenhuma; a volta ao verde gera um e-mail de normalização |
| 🟡 Amarelo | Degradação que exige observação | E-mail |
| 🔴 Vermelho | Serviço degradado ou indisponível | E-mail imediato |

Um nível só muda depois que a condição se mantém pelo tempo da regra (coluna "Por"), para que um pico isolado não vire alerta. Queda de porta não espera: vira vermelho na primeira verificação que falha.

| Regra | Serviço | Amarelo a partir de | Vermelho a partir de | Por |
|---|---|---|---|---|
| Disponibilidade (por porta) | Web / Banco / DNS / SMTP | respondeu com erro (HTTP 5xx, RCODE de erro, comando SMTP recusado) | não respondeu | imediato |
| Latência da verificação ativa | Web / Banco | 1000 ms | 3000 ms | 1 min |
| Taxa de erros 5xx | Web | 10% | 25% | 30 s |
| Latência p95 das requisições | Web | 1000 ms | 2500 ms | 30 s |
| Taxa de erros nas consultas | Banco | 10% | 25% | 30 s |
| Latência p95 das consultas | Banco | 500 ms | 2000 ms | 30 s |
| Tempo de resolução DNS | DNS | 200 ms | 1000 ms | 30 s |
| Latência da conversa SMTP | SMTP | 2000 ms | 5000 ms | 30 s |
| Taxa de falhas da sonda (últimas 10, mínimo 5) | DNS / SMTP | 20% | 50% | imediato |
| SMTP sem STARTTLS | SMTP | servidor não oferece STARTTLS | — | imediato |
| Uso de CPU / memória / disco | Sistema | 80% / 85% / 85% | 95% | 1 min |
| Latência do disco (por operação) | Sistema | 20 ms | 100 ms | 1 min |
| Falhas de resolução no tráfego (mínimo 20 consultas) | DNS | 5% | 20% | 30 s |
| Envios recusados ou interrompidos (mínimo 10 tentativas) | SMTP | 10% | 30% | 30 s |
| Tamanho da fila de e-mails | SMTP | 50 mensagens | 200 mensagens | 5 min |
| Logins falhos por IP / por conta | Segurança | 10 por minuto | 30 por minuto | imediato |
| Contas diferentes testadas por IP (password spraying) | Segurança | 5 por minuto | 10 por minuto | imediato |
| Requisições por IP | Segurança | 3000 por minuto | 6000 por minuto | imediato |
| Pico de tráfego (possível DDoS) | Segurança | 3x o normal | 6x o normal | imediato |
| Arquivo de configuração alterado | Segurança | conteúdo, permissões ou dono mudaram (por 5 min) | arquivo removido ou ilegível | imediato |

As taxas e o p95 só contam com volume mínimo na janela de 10 s (20 requisições, consultas ao banco ou consultas DNS; 10 envios SMTP), para uma única falha não virar "50% de erro". Cada mudança de nível sai no WebSocket como `ALERTA`, aparece na tela **Alertas** (`/alertas`) e em `GET /api/alertas` (nível atual por recurso, últimas 200 mudanças, regras e estado do e-mail):

```json
{ "tipo": "ALERTA", "dados": { "id": 3, "regra": "http_p95", "servico": "web", "nome": "Latência p95 das requisições", "recurso": "", "nivel": "amarelo", "nivel_anterior": "verde", "valor": 1234.5, "limite": 1000, "unidade": "ms", "hora": "2026-11-21T20:10:00Z" } }
```

### Configurar o e-mail

Sem `OMNI_SMTP_HOST` o envio fica desligado e o restante funciona normalmente.

| Variável | Uso |
|---|---|
| `OMNI_SMTP_HOST` | Servidor SMTP. Liga o envio |
| `OMNI_SMTP_PORTA` | Padrão 587 (STARTTLS). Na 465 o TLS começa já na conexão |
| `OMNI_SMTP_USUARIO` | Usuário do SMTP (opcional) |
| `OMNI_SMTP_SENHA` ou `OMNI_SMTP_SENHA_ARQUIVO` | Senha, ou caminho de um arquivo com ela (preferível: variáveis de ambiente ficam visíveis em `/proc/<pid>/environ`) |
| `OMNI_SMTP_REMETENTE` | Endereço que envia; se vazio, usa o usuário quando ele é um e-mail |
| `OMNI_ALERTA_EMAILS` | Destinatários, separados por vírgula |

**Teste local sem conta de e-mail,** com o [Mailpit](https://mailpit.axllent.org/), que captura as mensagens e as mostra numa página:

```bash
docker run -d --name mailpit -p 127.0.0.1:1025:1025 -p 127.0.0.1:8025:8025 axllent/mailpit
OMNI_TOKEN=... OMNI_SMTP_HOST=127.0.0.1 OMNI_SMTP_PORTA=1025 \
  OMNI_SMTP_REMETENTE=omni@localhost OMNI_ALERTA_EMAILS=equipe@exemplo.com ./omni
# derrube a loja cobaia e abra http://127.0.0.1:8025
```

**Gmail:** `OMNI_SMTP_HOST=smtp.gmail.com`, porta 587, o seu endereço como usuário e uma [senha de app](https://support.google.com/accounts/answer/185833) (exige verificação em duas etapas) no lugar da senha da conta.

**Proteções do envio:**

- **TLS obrigatório para a senha.** Se o servidor não oferece STARTTLS, o Omni se recusa a autenticar, exceto em servidor local (`127.0.0.1`/`localhost`).
- **Sem injeção de cabeçalho.** Quebras de linha são removidas dos campos que vêm do usuário (como o nome do alvo), e os endereços são validados na inicialização: um nome como `x\r\nBcc: alguem@exemplo.com` não cria cópias ocultas.
- **Limite de 20 e-mails por minuto.** Um ataque que derruba o serviço não vira, de quebra, uma enxurrada de e-mails; o e-mail seguinte informa quantos alertas foram suprimidos.
- **Envio fora do caminho crítico.** Os e-mails saem de uma fila numa goroutine própria: um SMTP lento ou fora do ar nunca atrasa o Poller, o sniffer ou o WebSocket.

## Monitoramento de DNS e SMTP

Cada alvo pode ter uma **porta DNS** e uma **porta SMTP** (campos `porta_dns` e `porta_smtp`), no mesmo host do alvo. O Poller testa as duas a cada rodada, junto com as portas da aplicação, e o resultado sai no `STATUS_ALVOS` com `tipo` `dns` ou `smtp`:

```json
{ "alvo_id": 1, "porta": 1053, "tipo": "dns", "no_ar": true, "degradado": true, "detalhe": "NXDOMAIN", "latencia_ms": 0.8, "hora": "..." }
{ "alvo_id": 1, "porta": 1025, "tipo": "smtp", "no_ar": true, "starttls": false, "detalhe": "aceita destinatário postmaster", "latencia_ms": 3.1, "hora": "..." }
```

**Sonda DNS** (`internal/probe/dns.go`): um cliente DNS feito só com a biblioteca padrão, que monta o pacote byte a byte (RFC 1035). Pergunta pelo registro A do **domínio** do alvo (obrigatório quando há porta DNS) e mede o tempo de resolução. Responder com qualquer código conta como "no ar"; `NXDOMAIN`, `SERVFAIL`, `REFUSED` ou uma resposta sem endereços contam como degradado (amarelo). Respostas truncadas (bit TC) são repetidas por TCP. Por segurança, a sonda:

- usa um ID de consulta aleatório (a porta de origem também é aleatória) e descarta respostas com ID diferente ou para outro nome, a defesa básica contra envenenamento de cache (ataque de Kaminsky);
- limita os saltos de ponteiros de compressão, para um pacote malicioso com ponteiros em ciclo não travar a leitura.

**Sonda SMTP** (`internal/probe/smtp.go`): conversa com o servidor **sem enviar e-mail**: espera a saudação `220`, manda `EHLO` e anota se o servidor oferece `STARTTLS`. Com domínio no alvo, testa também `MAIL FROM:<>` e `RCPT TO:<postmaster@domínio>` e desfaz com `RSET`; todo domínio que recebe e-mail precisa aceitar `postmaster` (RFC 5321, seção 4.5.1). Um comando recusado (4xx/5xx) é degradado; conexão recusada, timeout ou saudação diferente de 220 é fora do ar. Servidor sem STARTTLS gera o alerta amarelo "SMTP sem STARTTLS": os e-mails trafegariam em texto puro.

A taxa de falhas usa as últimas 10 verificações de cada porta (precisa de pelo menos 5). Com o intervalo padrão de 30 s, isso cobre os últimos 5 minutos.

**Ambiente de teste** em `exemplos/dns-smtp/`: um CoreDNS que resolve `loja.local` na porta 1053 e um Mailpit na porta 1025.

```bash
cd exemplos/dns-smtp && docker compose up -d
dig @127.0.0.1 -p 1053 loja.local +short        # 127.0.0.1
```

Cadastre (ou edite) o alvo com domínio `loja.local`, porta DNS `1053` e porta SMTP `1025`. Para ver os níveis mudarem:

- **Amarelo no DNS:** troque o domínio para `nao-existe.loja.local` (NXDOMAIN).
- **Vermelho no DNS ou SMTP:** `docker compose stop coredns` ou `docker compose stop mailpit`.
- **Amarelo "SMTP sem STARTTLS":** já aparece com o Mailpit padrão, que não tem TLS.

### Tráfego real

Além da sonda, o sniffer captura a porta DNS (UDP) e a porta SMTP de cada alvo. As telas **DNS** (`/dns`) e **SMTP** (`/smtp`) mostram as duas visões: a sonda ativa e o tráfego real.

**DNS** (`internal/sniffer/l7/dns.go`): cada consulta é pareada com a resposta pelo cliente (IP e porta) e pelo ID. Daí saem as consultas por segundo, o tempo de resolução (p50/p95), as falhas (RCODE diferente de `NOERROR`) e os códigos de resposta. Consulta sem resposta em 5 s conta como `TIMEOUT`. O nome consultado nunca é guardado, e no máximo 10.000 consultas ficam esperando resposta, para um ataque não esgotar a memória.

**SMTP** (`internal/sniffer/l7/smtp.go`): cada transação, do `MAIL FROM` ao fim, termina **entregue** (`250` depois do `DATA`), **rejeitada** (`MAIL FROM`, todos os `RCPT TO` ou o fim do `DATA` recusados com 4xx/5xx) ou **abortada** (`RSET`, `QUIT` ou conexão encerrada ou ociosa no meio, que são os timeouts). A tela mostra a taxa de entrega, a latência de envio (do fim do `DATA` até a resposta), o volume no último minuto e na última hora e os códigos de recusa. Endereços, cabeçalhos e corpo nunca são guardados, as linhas depois de `AUTH` não são lidas, e a conexão é ignorada depois que o servidor aceita o `STARTTLS` ou depois de um buraco na captura (a transação aberta conta como abortada). A conversa da sonda do próprio Omni (`EHLO omni-sonda.localhost`) fica de fora, como o tráfego `Omni-` no HTTP.

**Fila de saída:** com `OMNI_SMTP_FILA_DIRS` (diretórios separados por vírgula; no Postfix, `/var/spool/postfix/active,/var/spool/postfix/deferred`), o Omni conta a cada 30 s os arquivos da fila, sem abrir nenhum e sem seguir links. O usuário do Omni precisa de leitura nesses diretórios (por exemplo, com uma ACL: `sudo setfacl -R -m u:omni:rx /var/spool/postfix/active /var/spool/postfix/deferred`). Sem a variável, a tela mostra "não configurada".

**Carga de teste:** `make carga ARGS="-dns 127.0.0.1:1053 -smtp 127.0.0.1:1025"`, em `subjects/loja` e com `exemplos/dns-smtp` no ar, envia consultas e e-mails de pedido junto com a carga HTTP (`-dns-taxa` e `-emails-por-minuto` mudam o ritmo).

**Limites:** DNS sobre TCP e o SMTP depois do `STARTTLS` não são decodificados (o segundo é criptografado).

## Conexões, disco e tamanho do banco

**Conexões por porta** (`internal/connstats`): a cada 5 s, o Omni lê as conexões TCP do sistema e conta, em cada porta dos alvos, as abertas (`ESTABLISHED`) e as demais por estado. Do lado cliente, agrupa as conexões pelo processo que as abriu: é o pool de conexões de cada aplicação com o banco. As telas Requisições e Geral mostram as conexões ativas da aplicação; a tela Consultas mostra as abertas no banco e os pools. Sem root, processos de outros usuários aparecem como "(sem permissão para ver)".

**I/O do disco** (`internal/sysmetrics`): o dispositivo é o que guarda o banco do Omni (o mesmo do percentual de disco); no LVM, o link `/dev/mapper/...` é resolvido para o `dm-N` dos contadores. A tela Sistema mostra leitura e escrita por segundo, IOPS e a latência média por operação, com os limites do alerta.

**Tamanho do PostgreSQL** (`internal/dbsize`): o tamanho só existe dentro do banco (a pasta de dados pertence ao usuário `postgres`, com permissão 700). Com `OMNI_PG_DSN_ARQUIVO` (ou `OMNI_PG_DSN`), o Omni abre a cada 5 minutos uma conexão só de leitura, mede cada banco com `pg_database_size` e estima o crescimento em GB/dia pela regressão linear das últimas 24 h (precisa de pelo menos 1 h de medições). Crie um papel só para isso:

```sql
CREATE ROLE omni LOGIN PASSWORD '...' IN ROLE pg_monitor;
```

```bash
echo 'postgres://omni:...@127.0.0.1:5432/postgres' > ~/omni-pg.dsn && chmod 600 ~/omni-pg.dsn
OMNI_PG_DSN_ARQUIVO=~/omni-pg.dsn ./omni
```

O arquivo é preferível à variável, que fica visível em `/proc/<pid>/environ`. A senha nunca aparece no log: uma falha de conexão troca a senha por `xxxxx`, e um DSN malformado vira só "OMNI_PG_DSN inválido". Sem a conexão, a tela Consultas explica como ligar. Rota: `GET /api/banco/tamanho`.

## Segurança: brute-force, DDoS e integridade de arquivos

Três detectores em `internal/security` alimentam o mesmo motor de alertas, no serviço **Segurança**: aparecem na tela Alertas, como aviso no painel e por e-mail. O que fazer quando cada um dispara está nos [playbooks de incidente](docs/PLAYBOOKS.md).

### Brute-force e password spraying

O sniffer entrega cada requisição HTTP decodificada ao detector, que conta, numa janela deslizante de 1 minuto:

- **logins falhos por IP**: respostas `401` vindas do mesmo IP (brute-force clássico);
- **logins falhos por conta**: senhas erradas na mesma conta, de qualquer IP (brute-force distribuído);
- **contas diferentes por IP**: um IP testando muitas contas com poucas tentativas cada (password spraying).

A conta vem do cabeçalho `Authorization: Basic`: o decodificador guarda só o usuário e descarta a senha na hora (`internal/clientinfo`). O **IP de origem** tem um cuidado de rede: o sniffer captura depois do proxy reverso, então toda conexão parece vir de `127.0.0.1`. O IP real vem no `X-Forwarded-For`, que o proxy acrescenta. Como qualquer cliente pode escrever esse cabeçalho, o Omni só o usa quando a conexão vem do loopback (o proxy local) e, mesmo assim, só o **último** endereço da lista, que é o que o proxy acrescentou; os anteriores vieram do cliente e podem ser falsos.

### Picos de tráfego (DDoS)

- **Requisições por IP:** um IP sozinho passando de 3000 requisições por minuto (DoS simples ou robô).
- **Pico de tráfego:** o volume total do último minuto comparado com o normal, aprendido por uma média móvel exponencial. A média **fica congelada enquanto o tráfego está acima de 3x o normal**; senão, o próprio ataque viraria o novo normal e o alerta sumiria no meio do incidente. Volumes abaixo de 600 requisições por minuto não contam, e os primeiros minutos servem só para aprender.

O detector acompanha no máximo 10.000 IPs e contas ao mesmo tempo: num ataque com IPs forjados, ele não pode virar o alvo e esgotar a memória do Omni.

### Integridade de arquivos de configuração

Com `OMNI_ARQUIVOS_MONITORADOS` (caminhos separados por vírgula), o Omni guarda a impressão digital de cada arquivo (SHA-256, tamanho, permissões e dono) e confere a cada 10 segundos. Uma mudança fica **amarela por 5 minutos**; arquivo removido ou ilegível fica **vermelho** até voltar. O alerta diz o que mudou (hashes, tamanho, permissões) mas **nunca o conteúdo**, porque configuração costuma ter senhas e o alerta vai por e-mail. A linha de base é salva em `integridade.json` (permissão 600) ao lado do banco, então uma alteração feita com o Omni desligado também é detectada quando ele volta. É o mesmo princípio de ferramentas de FIM (*file integrity monitoring*) como Tripwire, AIDE e Wazuh.

### Como testar

A loja cobaia ganhou a rota `POST /entrar` (autenticação Basic; contas `ana`, `bia`, `carlos` e `admin`, com senhas fracas e sem limite de tentativas, de propósito) e o gerador de carga ganhou a opção `-ataque`. Os ataques falam direto com a porta da loja mandando `X-Forwarded-For`, como se passassem pelo proxy, com IPs dos blocos reservados para documentação (RFC 5737).

1. Suba o Omni vigiando os arquivos da loja, e a loja com a carga normal:
   ```bash
   OMNI_ARQUIVOS_MONITORADOS="$PWD/subjects/loja/Caddyfile,$PWD/subjects/loja/compose.yaml" ./omni
   # noutro terminal
   cd subjects/loja && make demo
   ```
2. Noutro terminal, em `subjects/loja`, rode um ataque de cada vez e acompanhe o cartão **Segurança** em Alertas:

   | Para ver | Comando | Resultado esperado |
   |---|---|---|
   | Brute-force (vermelho) | `make carga ARGS="-ataque forca-bruta -taxa 2"` | `IP 203.0.113.66` e `conta ana` em "Logins falhos" |
   | Password spraying (vermelho) | `make carga ARGS="-ataque spraying -taxa 2"` | `IP 198.51.100.23` em "Contas diferentes testadas" |
   | Amarelo em vez de vermelho | `-taxa 1 -duracao 15s` em qualquer um dos dois | entre 10 e 30 falhas no minuto |
   | DDoS (vermelho) | depois de 3 min de carga normal: `make carga ARGS="-ataque ddos -taxa 200 -workers 32"` | "Pico de tráfego" com o volume atual e o normal no detalhe |
   | Arquivo alterado (amarelo) | `echo '# teste' >> Caddyfile` | hashes e tamanho antigos e novos no detalhe |
   | Permissão alterada (amarelo) | `chmod 666 Caddyfile` (desfaça com `chmod 644`) | `permissões -rw-r--r-- → -rw-rw-rw-` |
   | Arquivo removido (vermelho) | `mv Caddyfile /tmp/` (desfaça com `mv /tmp/Caddyfile .`) | "arquivo removido ou ilegível" |

3. Pare o ataque: os contadores saem da janela e o nível volta ao verde em cerca de 1 minuto (5 minutos para arquivo alterado), com e-mail de normalização.

## Requisitos de implantação

- **Sistema:** servidor Linux. No Windows, rode o Omni dentro do WSL2 (veja [Windows](#windows)).
- **Reverse proxy terminando o TLS** e repassando à aplicação via `localhost`. Sem isso, o tráfego HTTPS fica ilegível para o Sniffer.
- **Banco acessado via TCP em `127.0.0.1`,** e não por unix socket, que o pcap não enxerga.
- **libpcap:** a captura ao vivo usa cgo e precisa do `libpcap-dev` (Debian/Ubuntu) ou `libpcap` (Arch) para compilar. Um binário compilado com `CGO_ENABLED=0` roda, mas com o sniffer sempre inativo.
- **Privilégios mínimos:** a captura usa só `CAP_NET_RAW` e `CAP_NET_ADMIN`, sem root. Refaça depois de cada build, porque o binário novo perde as capabilities:
  ```bash
  sudo setcap cap_net_raw,cap_net_admin=eip ./omni
  ```
- **Interface de captura:** `OMNI_INTERFACE` (padrão `any`, que inclui o loopback). O filtro BPF cobre as portas TCP de todos os alvos (aplicação, banco e SMTP) e a porta UDP do DNS, e é refeito quando os alvos mudam.
- **Docker** instalado, só para o Arsenal. O Omni fala com a API do Docker Engine pelo socket Unix (`/var/run/docker.sock` ou `DOCKER_HOST=unix://...`), então o usuário que roda o Omni precisa de acesso a ele. Os containers rodam com `network_mode: host`, 1 CPU e 512 MiB; o Nuclei recebe 1,5 GiB, porque carregar os templates passa de 512 MiB. Os templates do Nuclei ficam no volume `omni-nuclei-templates` e são baixados na primeira varredura.

**Arsenal pela API:** `GET /api/ferramentas` lista as ferramentas; `POST /api/varreduras` com `{"alvo_id": 1, "ferramentas": ["nmap", "nuclei", "testssl"]}` responde 202 `{"id": ...}` e roda em segundo plano (400 para ferramenta desconhecida ou alvo fora do escopo, 409 se o alvo já tem varredura em andamento, 503 com o Docker parado). `GET /api/varreduras?alvo_id=`, `GET /api/varreduras/{id}`, `GET /api/varreduras/{id}/achados` e `GET /api/varreduras/{id}/comparacao` (novos e resolvidos em relação à varredura concluída anterior de cada ferramenta) completam o fluxo. Varreduras interrompidas pelo desligamento do Omni ficam como `falhou`.

### Windows

O Omni não roda completo direto no Windows. O binário compila e a interface abre, mas dois motores ficam desligados:

| Motor | No Windows nativo | Motivo |
|---|---|---|
| Poller | Funciona | Só faz conexões TCP e requisições HTTP |
| Sniffer | Inativo | A interface padrão `any` não existe no Npcap, e `OMNI_INTERFACE` só aceita nomes no estilo Linux (até 15 caracteres), o que recusa os nomes do Npcap (`\Device\NPF_...`) |
| Arsenal (Docker) | Indisponível | O Omni só fala com o Docker por socket Unix (`unix://`), e o Docker Desktop atende no Windows por named pipe (`npipe://`) |

**Caminho recomendado: WSL2.** O Omni roda dentro de uma distribuição Linux do WSL2, e a interface é aberta pelo navegador do Windows no endereço do log de início (`http://localhost:<porta>`), porque o WSL2 repassa as portas do loopback para o Windows. Dentro do WSL, o Omni se comporta como num servidor Linux: siga os requisitos acima (`libpcap-dev`, `setcap`, Docker pelo socket Unix).

Cuidados no WSL2:

- **Use o Docker Engine instalado na própria distribuição**, e não a integração do Docker Desktop. Com a integração, o comando `docker` da distribuição fala com o motor do Docker Desktop, que roda em outra máquina virtual: containers com `network_mode: host` ficam no loopback dessa outra máquina, fora do alcance da aplicação e do Sniffer. Desligue a integração em *Settings → Resources → WSL integration* e instale o Docker Engine pelo repositório oficial da distribuição.
- **O Sniffer só enxerga o tráfego de dentro do WSL.** Aplicações rodando no Windows podem ser vigiadas pelo Poller e varridas pelo Arsenal, mas o tráfego delas não passa pela interface do WSL.
- **O `setcap` não funciona em `/mnt/c`.** Compile o binário para o sistema de arquivos do Linux (por exemplo, `go build -o ~/omni-bin/omni ./cmd/omni`) e aplique as capabilities nele.
- **Guarde o banco do Omni no sistema de arquivos do Linux** (`OMNI_DB=~/omni.db`): o SQLite em `/mnt/c` pode ter problemas de trava de arquivo.

## O que falta implementar, por camada

### Camada 7: Aplicação

- [x] **Decodificador HTTP/1.1:** método, rota, status e latência por requisição.
- [ ] **Decodificador HTTP/2:** protocolo binário, usado pela maioria dos proxies modernos.
- [x] **Normalização de rotas:** agrupar `/users/123` e `/users/456` como `/users/:id`.
- [x] **Decodificador PostgreSQL:** mensagens `Query` e `Parse`, com pareamento entre consulta e resposta para medir o tempo.
- [ ] **Decodificador MySQL:** `COM_QUERY`, com o mesmo pareamento.
- [x] **Tamanho e crescimento do PostgreSQL** por uma conexão opcional com `pg_monitor`. Veja [Conexões, disco e tamanho do banco](#conexões-disco-e-tamanho-do-banco).
- [ ] **Complemento opcional para PostgreSQL:** coletar estatísticas de `pg_stat_statements`.
- [x] **DNS (sonda ativa):** latência e respostas com erro (NXDOMAIN, SERVFAIL, REFUSED). Veja [Monitoramento de DNS e SMTP](#monitoramento-de-dns-e-smtp).
- [x] **DNS (tráfego real):** consultas por segundo, tempo de resolução, falhas e timeouts. Veja [Tráfego real](#tráfego-real).
- [ ] **DNS:** registros MX e TXT (SPF/DKIM) e DNS sobre TCP.
- [x] **SMTP (sonda ativa):** tempo do handshake (`220`) e suporte a STARTTLS.
- [x] **SMTP (tráfego real):** taxa de entrega, rejeições, latência de envio, volume e fila de saída.
- [x] **Mascaramento de dados sensíveis** (`Authorization`, `Cookie`, senhas) antes de qualquer gravação.
- [x] **Parsers do Arsenal** com um esquema único de achado (alvo, ferramenta, severidade, evidência) e severidade normalizada.
- [x] **Relatório estruturado por ferramenta,** para a página própria de cada uma.
- [ ] **Severidade por CVSS** e deduplicação entre ferramentas.
- [x] **Comparação entre scans:** indicar o que mudou desde o anterior, por exemplo "porta 22 abriu desde ontem".
- [x] **Alertas por e-mail.** Veja [Alertas e notificação por e-mail](#alertas-e-notificação-por-e-mail).
- [x] **Detecção de brute-force e password spraying** pelas respostas 401, com IP de origem atrás do proxy e usuário da autenticação Basic.
- [x] **Integridade de arquivos de configuração** (SHA-256, permissões e dono).
- [ ] **Agendamento de scans** e alertas externos (Telegram, Discord).
- [x] **Novas ferramentas do Arsenal:** ffuf, Nikto, SQLMap e Amass.
- [ ] **Gitleaks** (Vazamentos, Sprint 13).
- [x] **Exportação de relatório em PDF** por alvo e intervalo (até 168 h).

### Camada 6: Apresentação (TLS)

- [x] **Captura após o reverse proxy** (`lo`) para ler o HTTP em texto puro.
- [ ] **Metadados do handshake TLS,** visíveis mesmo com criptografia: SNI, versão, cifra e validade do certificado.
- [ ] **Alternativa sem proxy, mais complexa:** eBPF em `SSL_read`/`SSL_write`.

### Camada 4: Transporte

- [x] **Remontagem TCP** com `gopacket/reassembly` (o `tcpassembly` está obsoleto), com limite de memória por fluxo e timeout para conexões abandonadas.
- [ ] **RTT** medido pelo par SYN → SYN-ACK.
- [ ] **Retransmissões e pacotes fora de ordem,** que indicam rede degradada.
- [ ] **RST em excesso** (serviço recusando conexões) e **SYN sem resposta** (possível SYN flood ou varredura).
- [ ] **Janela TCP zero,** sinal de que a aplicação não está dando conta.
- [x] **Conexões por estado** (ESTABLISHED, TIME_WAIT) via `/proc/net/tcp`, cruzadas com o processo dono da porta. Veja [Conexões, disco e tamanho do banco](#conexões-disco-e-tamanho-do-banco).
- [ ] **UDP:** volume por porta e detecção de amplificação (resposta muito maior que a requisição).

### Camada 3: Rede

- [x] **Detecção de abuso e DDoS:** requisições por IP e pico de tráfego em relação ao normal. Veja [Segurança](#segurança-brute-force-ddos-e-integridade-de-arquivos).
- [ ] **Top IPs de origem por número de conexões** na camada 3 (o volume em bytes já aparece na tela Geral).
- [ ] **ICMP:** destino inalcançável e TTL excedido.
- [ ] **Fragmentação IP.**
- [ ] **Suporte completo a IPv6.**
- [ ] **GeoIP opcional** (MaxMind GeoLite) para mapear a origem do tráfego.

### Camada 2: Enlace

- [ ] **Detecção de ARP spoofing:** o mesmo IP anunciado com MACs diferentes.
- [ ] **Interfaces virtuais** (`docker0`, `veth*`, bridges) quando a aplicação roda em containers.

### Camada 1 e sistema

- [ ] **Contadores da placa** (`/sys/class/net/*/statistics`): erros, descartes e utilização do link.
- [ ] **Captura de alto desempenho:** AF_PACKET com ring buffer e `DecodingLayerParser`, para não sobrecarregar o garbage collector.
- [x] **Limites de CPU e RAM nos containers do Arsenal** (1 CPU e 512 MiB; 1,5 GiB para o Nuclei).
- [ ] **Limites de CPU e RAM para o próprio Omni,** para não derrubar a produção.
- [x] **Histórico de CPU, memória, disco e I/O** numa página própria.
- [ ] **Inspetor de pacotes** ao vivo (Sprint 14) e **capturas `.pcap`** sob demanda, com retenção configurável (Sprint 15).

## Segurança da própria plataforma

O Omni roda no servidor de produção, com acesso ao Docker e privilégio de captura. **Se ele for comprometido, o servidor inteiro é comprometido.** Por isso:

- **A API e o WebSocket escutam apenas em `127.0.0.1`.** O acesso remoto é feito por túnel SSH:
  ```bash
  ssh -L <porta>:127.0.0.1:<porta> usuario@servidor
  ```
- **Autenticação obrigatória** por token, mesmo em acesso local.
- **Verificação de origem no WebSocket** (`CheckOrigin`), contra sequestro de WebSocket entre sites (CSWSH). Acesso por túnel numa porta local diferente da do Omni exige a origem em `OMNI_ORIGENS` (ex.: `OMNI_ORIGENS=http://localhost:9999`). O `make dev` coloca ali a origem do Vite.
- **Descoberta de serviços sem segredos:** `GET /api/descoberta` exige o token e devolve só nome do processo, PID, host e portas, nunca a linha de comando, que costuma conter senhas. Sem root, processos de outros usuários aparecem sem nome.
- **Validação estrita do alvo** (IP, domínio ou porta), com argumentos passados aos containers **como array, nunca como string concatenada**, para evitar injeção de comando.
- **Escopo travado:** o Arsenal só roda contra `localhost` e os domínios cadastrados da aplicação.
- **Imagens Docker com versão fixa,** de preferência oficiais ou construídas pelo próprio projeto: `instrumentisto/nmap:7.95`, `projectdiscovery/nuclei:v3.4.10`, `drwetter/testssl.sh:3.2`, `ghcr.io/sullo/nikto:2.6.1`, `secsi/ffuf:2.3.0`, `caffix/amass:v3.23.3` e o SQLMap fixado num commit (`googlesky/sqlmap:upstream-5db2e62…`).
- **Conexão com o PostgreSQL só de leitura e opcional:** papel com `pg_monitor`, DSN num arquivo com permissão 600 e senha fora dos logs.
- **Retenção de dados configurável,** já que capturas podem conter dados pessoais (LGPD). As capturas `.pcap` (Sprint 15) ficam com permissão `0600`, têm tamanho máximo e são apagadas depois do prazo de `OMNI_RETENCAO_CAPTURAS`.
- **Regras das ferramentas novas (Sprints 11 a 13):** toda opção vinda da interface (rota e parâmetro do SQLMap, lista de palavras do ffuf, caminho do código-fonte do Gitleaks) é validada no backend contra uma lista fechada ou um padrão estrito; o SQLMap roda só com nível 1 e risco 1 e nunca com opções de extração (`--dump`, `--os-shell`, `--file-*`); o código-fonte é montado **somente leitura** no container do Gitleaks, e o valor dos segredos encontrados nunca é gravado nem mostrado.

## Limitações conhecidas

- **Scans locais não representam a visão de um atacante externo.** Eles enxergam portas ligadas só a `127.0.0.1` e ignoram firewall e security groups da nuvem. Por isso aparecem rotulados como "visão interna".
- **O tráfego gerado pelo próprio Omni** fica fora das métricas só quando é identificável: o HTTP do Poller e do Arsenal pelo User-Agent `Omni-` e a sonda SMTP pelo `EHLO omni-sonda.localhost`. O Nmap e o testssl não mandam esse User-Agent, então as varreduras deles ainda aparecem no tráfego.
- **Aplicações que usam SQLite como banco** não geram tráfego de rede, então não há gráfico de consultas para elas.
- **Traceroute** só faz sentido para destinos externos ao servidor.
- **O Amass precisa de acesso à internet:** em modo passivo, ele consulta fontes públicas (certificados, DNS passivo) sobre o domínio do alvo, sem mandar tráfego ao alvo. Sem internet, a varredura termina sem subdomínios.
- **Windows nativo** roda só com o Poller: o Sniffer e o Arsenal ficam desligados. Use o WSL2 (veja [Windows](#windows)).
- **DNS sobre TCP e SMTP depois do STARTTLS** não são decodificados; essas conversas ficam fora das métricas do tráfego real.
- **A fila do SMTP e o tamanho do banco** dependem de permissões que o administrador concede (leitura dos diretórios da fila e um papel `pg_monitor`).

## Cobertura do enunciado

Onde cada métrica pedida no trabalho (seção 3 do enunciado) aparece na interface e de onde vem o dado.

| Serviço | Métrica | Onde aparece | Fonte |
|---|---|---|---|
| Web | Disponibilidade (status HTTP) | Alvos, Alertas | Sonda ativa (Poller) |
| Web | Requisições por segundo | Geral, Requisições na API | Sniffer (HTTP) |
| Web | Latência | Requisições na API (p50/p95 por rota) | Sniffer (HTTP) e sonda |
| Web | Taxa de erro 4xx/5xx e contagem por código | Status HTTP, Requisições na API (Códigos de erro) | Sniffer (HTTP) |
| Web | Conexões ativas | Geral, Requisições na API | `/proc` (connstats) |
| Banco | Disponibilidade | Alvos, Alertas | Sonda ativa (TCP) |
| Banco | Consultas por segundo | Geral, Consultas no banco | Sniffer (PostgreSQL) |
| Banco | Erros, rollbacks e transações que falharam | Consultas no banco | Sniffer (PostgreSQL) |
| Banco | Conexões abertas e pools | Consultas no banco | `/proc` (connstats) |
| Banco | CPU, memória, I/O e latência de disco | Sistema | `gopsutil` |
| Banco | Tamanho e crescimento (GB/dia) | Consultas no banco | PostgreSQL (`pg_database_size`, opcional) |
| Banco | Consultas lentas | Consultas no banco | Sniffer (PostgreSQL) |
| DNS | Tempo de resolução | DNS (sonda e tráfego) | Sonda ativa e sniffer |
| DNS | Falhas de resolução | DNS, Alertas | Sonda ativa e sniffer |
| DNS | Consultas por segundo | DNS (tráfego real) | Sniffer (DNS por UDP) |
| DNS | Disponibilidade | DNS, Alertas | Sonda ativa |
| SMTP | Taxa de entrega | SMTP (tráfego real) | Sniffer (SMTP) |
| SMTP | Tamanho da fila | SMTP, Alertas | Diretórios da fila (opcional) |
| SMTP | Latência de envio | SMTP (tráfego real e sonda) | Sniffer e sonda ativa |
| SMTP | Taxa de erros (rejeições, timeouts) | SMTP, Alertas | Sniffer (SMTP) |
| SMTP | Volume por minuto e por hora | SMTP (tráfego real) | Sniffer (SMTP) |

Os alertas seguem os três níveis do enunciado (verde, amarelo e vermelho, com e-mail no amarelo e no vermelho), e a seção 5 está em [Segurança](#segurança-brute-force-ddos-e-integridade-de-arquivos) (DDoS, brute-force e integridade de arquivos) e no [Arsenal](#arsenal-containers) (vulnerabilidades conhecidas por serviço, com Nuclei, Nmap e testssl).

## Roadmap

A implementação está dividida em **4 fases**. As Sprints 1 a 8 formam a versão básica, já entregue; as Sprints 9 a 16 tiram do "em breve" as seções restantes do menu. Cada sprint entrega uma versão funcional que serve de base para a seguinte:

| Fase | Sprints |
|---|---|
| 1. Fundação | 1. Núcleo do backend · 2. Poller e WebSocket · 3. Frontend base |
| 2. Observação passiva | 4. Sniffer (camadas 3 e 4) · 5. HTTP · 6. PostgreSQL |
| 3. Arsenal | 7. Orquestrador Docker e Nmap · 8. Nuclei, testssl e comparação entre scans |
| 4. Expansão | 9. Página por ferramenta do Arsenal · 10. Sistema · 11. ffuf e Nikto · 12. SQLMap · 13. Amass e Gitleaks · 14. Inspetor de pacotes · 15. Capturas `.pcap` · 16. PDF · backlog com o restante das pendências por camada |

**➡️ O que é feito em cada sprint, com entregas e status: [docs/SPRINTS.md](docs/SPRINTS.md)**

O plano técnico detalhado (arquivos, interfaces e testes de cada tarefa) fica em [docs/superpowers/plans/2026-11-18-implementacao-basica.md](docs/superpowers/plans/2026-11-18-implementacao-basica.md) (Sprints 1 a 8) e em [docs/superpowers/plans/2026-11-21-fase-4-arsenal-e-paginas.md](docs/superpowers/plans/2026-11-21-fase-4-arsenal-e-paginas.md) (Sprints 9 a 16).

## Stack

| Camada | Tecnologias |
|---|---|
| **Backend** | Go, `gopacket/gopacket` (fork mantido), `gopacket/reassembly`, cliente mínimo da API do Docker Engine (sem o Docker SDK), `gorilla/websocket`, `gopsutil`, SQLite (`modernc.org/sqlite`), `jackc/pgx` (tamanho do PostgreSQL) |
| **Frontend** | React + TypeScript, React Router, Tailwind CSS, Zustand, TanStack Table, Recharts, `react-use-websocket`, jsPDF |
| **Arsenal** | Nmap, Nuclei, Nikto, SQLMap, ffuf, Amass, testssl.sh (Gitleaks na Sprint 13) |
