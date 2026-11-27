# Vitrine cobaia

A vitrine é uma loja online de verdade, só que vulnerável de propósito. Ela tem catálogo, carrinho, login, checkout, "meus pedidos" e painel de admin, e dá para usar no navegador como qualquer loja. Por trás, ela tem as falhas mais comuns de aplicações web e um comando pronto para cada ataque. Serve para mostrar o Omni detectando ataques ao vivo.

> **Atenção:** as falhas são reais. A API se recusa a subir fora do loopback (`127.0.0.1`), e todos os dados (clientes, CPFs, segredos do `.env`) são inventados.

| Peça | Endereço | O que é |
|---|---|---|
| Site (Vite + React) | <http://127.0.0.1:5180> | O que você abre no navegador; repassa `/api` para a API |
| API (Express) | `127.0.0.1:4000` | É esta porta que o Omni observa |
| PostgreSQL 17 | `127.0.0.1:5434` | Docker, rede do host, para o sniffer ver as consultas |

## Como subir

Pré-requisitos: Node 20 ou mais novo e Docker com Compose v2.

```bash
make vitrine
```

O comando instala as dependências, sobe o banco e roda `npm run dev` até Ctrl+C. Na primeira vez, a API cria as tabelas, 24 produtos, 6 contas e 180 pedidos. Se você passar `OMNI_TOKEN` e `OMNI_URL` (o token e o `painel=` que o `make init-reset` imprimiu), ele ainda cadastra o alvo no Omni antes de subir:

```bash
OMNI_TOKEN=<token> OMNI_URL=http://127.0.0.1:<porta> make vitrine
```

Sem o `make`, de dentro de `subjects/vitrine`:

```bash
npm install
npm run banco
npm run dev
```

Se subir com `make vitrine` sem as variáveis, cadastre o alvo depois (porta 4000 e banco 5434) com o token e o endereço que o `make init-reset` imprimiu:

```bash
OMNI_TOKEN=<token> OMNI_URL=http://127.0.0.1:<porta> make vitrine-cadastrar
```

Você também pode criar o alvo na página Alvos do Omni.

### Contas de teste

| Usuário | Senha | Observação |
|---|---|---|
| `ana` | `ana123` | Alvo do `ataque:forca-bruta` |
| `bia` | `bia123` | Usada pelo `ataque:idor` |
| `carlos` | `carlos123` | |
| `davi` | `123456` | Cai no `ataque:spraying` |
| `elis` | `elis2026` | |
| `admin` | `admin` | Vê o menu Admin |

## Comandos

Os comandos rodam de dentro de `subjects/vitrine`. Da raiz do repositório, use `make vitrine-ataque a=<nome>`, com as opções em `ARGS`, como em `make vitrine-ataque a=ddos ARGS="--taxa 300"`.

| Comando | O que faz |
|---|---|
| `npm run dev` | Sobe a API e o site até Ctrl+C. Se a API cair, volta sozinha em 45 s (`VITRINE_REINICIO=20 npm run dev` muda o tempo) |
| `npm run banco` / `banco:parar` / `banco:apagar` | Sobe o PostgreSQL; para mantendo os dados; para e apaga tudo (a API recria os dados na próxima partida) |
| `npm run trafego` | Simula clientes navegando (5 req/s) até Ctrl+C. É a "linha de base" de tráfego normal |
| `npm run cadastrar` | Cria o alvo `vitrine` no Omni (precisa de `OMNI_URL` e `OMNI_TOKEN`) |
| `npm run ataque:ddos` | 150 req/s vindas de 254 IPs forjados, por 2 min |
| `npm run ataque:forca-bruta` | Testa 181 senhas na conta `ana`, 3 por segundo, até acertar |
| `npm run ataque:spraying` | Testa a senha `123456` em 18 contas, 2 por segundo, por 90 s |
| `npm run ataque:varredura` | Procura 50 arquivos e rotas sensíveis (`.env`, `.git`, `backup.sql`, `/admin`…) e lista o que respondeu |
| `npm run ataque:idor` | Logada como `bia`, lê os pedidos 1 a 250 e lista os dados vazados de outros clientes |
| `npm run ataque:sqli` | Mostra quatro injeções de SQL na busca e repete as cargas por 30 s |
| `npm run ataque:lento` | Martela o relatório que espera 2 s no banco; a loja inteira fica lenta |
| `npm run ataque:derrubar` | Chama a rota de depuração que encerra a API e conta quanto tempo ela ficou fora |

Todo ataque aceita `--taxa` e `--duracao` (como `90s`, `2m`); os específicos estão no topo de cada arquivo em `scripts/ataques/`. Ctrl+C para o ataque e mostra o placar. Os IPs forjados vão no `X-Forwarded-For` e são de blocos reservados para documentação (RFC 5737).

## O que dá para ver na loja

| Página | Endereço | O que tem |
|---|---|---|
| Vitrine | `/` | Ofertas e produtos com filtro por categoria |
| Produto | `/produtos/1` | Descrição, preço, estoque e botão de comprar |
| Busca | `/busca?q=fone` | Resultado da busca do topo |
| Carrinho | `/carrinho` | Itens, quantidades e checkout (precisa de login) |
| Entrar | `/entrar` | Login, com as contas de teste na tela |
| Meus pedidos | `/meus-pedidos` | Histórico do cliente logado |
| Pedido | `/pedidos/2` | Detalhes do pedido, com setas para o anterior e o próximo |
| Admin | `/admin` | Lista de clientes com hash da senha e relatório de vendas (lento) |

## Falhas de propósito

| Falha | Onde | Como ver no navegador ou no terminal |
|---|---|---|
| Injeção de SQL | `GET /api/busca?q=` | Busque `' OR 1=1--`: aparecem o "Protótipo Console X" e o "Cupom interno", que são ocultos. Busque `'` e a página mostra o SQL inteiro |
| XSS refletido | Página de busca | Busque `<img src=x onerror=alert(localStorage['vitrine:credencial'])>` logado: o alerta mostra a sua senha em Base64 |
| IDOR | `GET /api/pedidos/:id` | Logado como `bia`, abra `/pedidos/2`: é o pedido da Ana, com CPF e endereço. As setas navegam pelos pedidos de todo mundo |
| Controle de acesso só na tela | `GET /api/admin/usuarios` | O menu Admin só aparece para `admin`, mas `/admin` abre logado como qualquer um, ou sem login nenhum |
| Senhas em md5 sem sal | Tabela `usuarios` | Os hashes do Admin quebram em sites de busca de md5 (`21232f29…` é `admin`) |
| Login sem limite de tentativas | `POST /api/entrar` | É o que deixa a força bruta e o spraying funcionarem |
| Enumeração de usuários | `POST /api/entrar` | Entre como `zeca`: "usuário zeca não existe". Entre como `ana` com a senha errada: "senha incorreta" |
| Preço vindo do navegador | `POST /api/pedidos` | `curl -u ana:ana123 -H 'Content-Type: application/json' -d '{"itens":[{"id":5,"nome":"Smartwatch","preco":0.01,"quantidade":1}],"endereco":"Rua X","cartao":"4111111111111111"}' http://127.0.0.1:4000/api/pedidos` compra o relógio por R$ 0,01 |
| Cartão no log | Terminal do `npm run dev` | Cada compra imprime o número completo do cartão |
| Senha no localStorage | Navegador | DevTools → Application → Local Storage: `vitrine:credencial` é `usuario:senha` em Base64 |
| Arquivos expostos | `/.env`, `/.git/config`, `/.git/HEAD`, `/backup.sql` | Abra em `http://127.0.0.1:4000/.env` |
| `robots.txt` que vira mapa | `/robots.txt` | "Esconde" justamente `/admin`, `/backup.sql` e `/api/debug/` |
| Rotas de depuração | `/api/debug/config`, `POST /api/debug/derrubar` | A primeira mostra a URL do banco com a senha; a segunda derruba a API sem pedir login |
| Erros detalhados | Toda resposta 500 | O JSON traz a mensagem do banco, o SQL e a pilha do Node |
| Cabeçalhos que revelam a tecnologia | Todas as respostas | `X-Powered-By: Express` e `Server: Vitrine/1.4.2 (Node …)` |
| CORS aberto e sem cabeçalhos de segurança | Todas as respostas | `Access-Control-Allow-Origin: *`, sem CSP, sem `X-Frame-Options` |
| Consulta que trava o banco | `GET /api/relatorios/vendas?ms=` | O botão "Gerar relatório" do Admin demora; o `ataque:lento` esgota o pool de 10 conexões |

## O que o Omni mostra em cada ataque

O Omni tem detectores próprios para DDoS, força bruta e spraying, e o alerta sai com o nome do ataque. Os outros aparecem nos gráficos de tráfego, de erros, de latência e de consultas do banco, sem um alerta com o nome do ataque.

> **Nota técnica:** os scripts mandam `Connection: close`, abrindo uma conexão TCP por requisição. Sem isso, o Node concentra um ataque de taxa alta numa única conexão reusada, e o sniffer do Omni — que remonta o HTTP por conexão — captura só uma fração das requisições (num teste, ~2% a 150 req/s). Com uma conexão por requisição a captura passa de ~90%, e é também como um DDoS real (cada bot) ou muitos visitantes se comportam. Que uma única conexão de taxa muito alta seja sub-amostrada é um comportamento do próprio Omni, que vale investigar à parte.

| Ataque | Onde olhar no Omni | O que aparece |
|---|---|---|
| `ataque:ddos` | Cartão Segurança e alertas | "Pico de tráfego (possível DDoS)" amarelo a partir de 3x o normal e vermelho a partir de 6x. Precisa de tráfego normal antes (veja o roteiro) |
| `ataque:forca-bruta` | Cartão Segurança | "Logins falhos por IP" e "Logins falhos por conta" (`ana`) em vermelho, acima de 30 por minuto |
| `ataque:spraying` | Cartão Segurança | "Contas diferentes testadas por IP" em vermelho, acima de 10 por minuto, vindo de 198.51.100.23 |
| `ataque:varredura` | Requisições HTTP | Rajada de 404 em rotas como `/.env`, `/wp-login.php` e `/phpmyadmin` |
| `ataque:idor` | Requisições HTTP | Centenas de `GET /api/pedidos/:id` em sequência, terminando em 404 |
| `ataque:sqli` | Requisições HTTP e banco | 5xx em `/api/busca`; consultas com erro e a busca com `?` no lugar do literal |
| `ataque:lento` | Requisições HTTP e banco | p95 de vários segundos em `/api/relatorios/vendas`; `SELECT pg_sleep($1)` entre as consultas mais lentas |
| `ataque:derrubar` | Alvos e notificações | `ALERTA_QUEDA` na rodada seguinte do Poller e `SERVICO_RESTAURADO` quando a API volta |
| Arsenal (Nmap, Nuclei, ffuf) no alvo `vitrine` | Varreduras e Relatórios | Portas 4000 e 5434; `.env`, `.git/config`, cabeçalhos ausentes e as rotas expostas |

## Roteiro para uma apresentação

Use três terminais, mais o Omni e a vitrine abertos lado a lado no navegador.

1. **Omni:** suba com o Poller rápido, para a queda aparecer em segundos e não em 30 s:

   ```bash
   OMNI_INTERVALO_POLLER=5s make init-reset
   ```

2. **Vitrine:** `make vitrine` e, em outro terminal, `OMNI_TOKEN=<token> OMNI_URL=<painel> make vitrine-cadastrar`.
3. **Uso normal:** abra <http://127.0.0.1:5180>, navegue, entre como `ana`, compre algo. No Omni, mostre as requisições e as consultas do PostgreSQL aparecendo.
4. **Linha de base:** `npm run trafego` em um terminal e deixe rodando. O detector de DDoS compara o volume do último minuto com a média do tráfego normal, então ele precisa de pelo menos 1 minuto de `trafego` antes do ataque. Não dispare outros ataques pesados antes do DDoS: eles sobem a linha de base e o pico pode não cruzar o limiar.
5. **DDoS:** `npm run ataque:ddos`. Em cerca de 20 a 30 s o pico fica vermelho no cartão Segurança (o ataque manda 150 req/s, bem acima do normal).
6. **Força bruta:** `npm run ataque:forca-bruta`. O terminal termina com "SENHA ENCONTRADA: ana / ana123", e o Omni mostra o IP e a conta.
7. **Spraying:** `npm run ataque:spraying`. A conta `davi` cai com `123456`.
8. **Vazamentos:** `npm run ataque:varredura` lista o que está exposto; abra `http://127.0.0.1:4000/.env` no navegador. Depois rode o Nuclei do Omni no alvo `vitrine` e mostre que ele achou o mesmo.
9. **Dados de clientes:** logado como `bia`, abra `/pedidos/2` no navegador; depois `npm run ataque:idor` para o vazamento em massa.
10. **Injeção de SQL:** busque `' OR 1=1--` na loja, depois `npm run ataque:sqli` para ver os hashes de senha saindo pela busca.
11. **Lentidão:** `npm run ataque:lento` e tente navegar na loja enquanto ele roda; no Omni, o p95 sobe e o `pg_sleep` lidera as consultas lentas.
12. **Queda:** `npm run ataque:derrubar`. O Omni avisa a queda e, 45 s depois, a volta.

Um item do roteiro que não aparecer no Omni é um achado sobre o Omni, não sobre a vitrine.

## Variáveis

| Variável | Padrão | Para quê |
|---|---|---|
| `VITRINE_ADDR` | `127.0.0.1:4000` | Endereço da API; só aceita loopback |
| `VITRINE_BANCO` | `postgres://vitrine:vitrine-cobaia@127.0.0.1:5434/vitrine?sslmode=disable` | Conexão com o banco |
| `VITRINE_REINICIO` | `45` | Segundos até o `npm run dev` subir a API de novo depois de uma queda |
| `VITRINE_URL` | `http://127.0.0.1:4000` | Para onde os scripts de tráfego e de ataque apontam |
| `OMNI_URL`, `OMNI_TOKEN` | — | Usadas pelo `npm run cadastrar` |

Se mudar a porta da API, o proxy do site em `vite.config.js` continua apontando para a 4000; ajuste os dois.
