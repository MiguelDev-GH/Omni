# Testes de navegação e uso da plataforma

Este documento tem duas partes:

1. **Testes automáticos** que já rodam no CI e no `make check`, simulando um usuário na interface.
2. **Roteiro de testes manuais** para executar no navegador, por exemplo antes da apresentação. A coluna "Resultado" fica para quem executa preencher (✅ passou, ❌ falhou, com observação).

## 1. Testes automáticos de interface

O frontend tem testes com [Vitest](https://vitest.dev/) e [Testing Library](https://testing-library.com/), que montam cada tela num navegador simulado (jsdom) e interagem como um usuário: digitam, clicam, navegam entre rotas e recebem eventos falsos do WebSocket. Rodam com:

```bash
cd web && npm test          # só o frontend
make check                  # tudo: Go, frontend e loja cobaia
```

| Arquivo | O que cobre |
|---|---|
| `src/App.test.tsx` | tela de token: token curto recusado, login válido, token inválido, erro de conexão, sessão expirada e botão Sair |
| `src/components/Sidebar.test.tsx` | menu lateral: cada item leva à rota certa |
| `src/components/Topbar.test.tsx` | barra superior: estado dos motores e troca de tema |
| `src/components/Toasts.test.tsx` | avisos na tela: aparecem, somem sozinhos e podem ser fechados |
| `src/pages/OverviewPage.test.tsx` | tela Geral: histórico, protocolos, principais origens e atualização ao vivo |
| `src/pages/AlertsPage.test.tsx` | tela Alertas: nível por serviço (Web, Banco, DNS, SMTP, Sistema, Segurança), histórico, e-mail ativo ou não, atualização ao vivo |
| `src/pages/TargetsPage.test.tsx` | tela Alvos: criar, editar, excluir, validação de portas, portas DNS/SMTP e estado de cada porta |
| `src/pages/RequestsPage.test.tsx`, `QueriesPage.test.tsx` | métricas de requisições HTTP (por segundo, códigos de erro, conexões ativas) e de consultas ao banco (por segundo, rollbacks, transações que falharam, pools e tamanho) |
| `src/pages/SystemPage.test.tsx` | tela Sistema: CPU, memória, disco e I/O do disco |
| `src/pages/ProbeServicePage.test.tsx` | telas DNS e SMTP: sonda ativa (disponibilidade, latência, STARTTLS) e tráfego real (consultas por segundo, taxa de entrega, volume, fila) |
| `src/pages/ArsenalPage.test.tsx`, `ToolPage.test.tsx`, `src/arsenal/*.test.tsx` | Arsenal: iniciar varredura, console ao vivo e relatório de cada ferramenta |
| `src/pages/ReportsPage.test.tsx` | Relatórios: histórico e comparação entre scans |
| `src/realtime/*.test.ts*` | WebSocket: ticket, reconexão e tratamento de cada tipo de evento |

No backend, `cmd/omni/run_test.go` sobe o Omni de verdade numa porta aleatória e testa o caminho completo: login, cadastro de alvo, WebSocket e alerta de queda chegando ao navegador.

## 2. Roteiro de testes manuais

**Preparação:** siga o [RB-09 dos runbooks](RUNBOOKS.md#rb-09--ambiente-de-demonstração-do-zero) (Omni com e-mail e arquivos monitorados, Mailpit/CoreDNS de `exemplos/dns-smtp` e `make demo` na loja). Use um navegador desktop e, para os testes de responsividade, as ferramentas de desenvolvedor no modo celular (F12 → ícone de dispositivo).

### 2.1 Acesso

| ID | Passos | Resultado esperado | Resultado |
|---|---|---|---|
| NAV-01 | Abrir o painel sem token salvo | Tela de token com a logo do Omni | |
| NAV-02 | Digitar um token com menos de 32 caracteres e entrar | Mensagem de token curto, sem chamar o servidor | |
| NAV-03 | Digitar um token errado (32+ caracteres) | Mensagem de token inválido | |
| NAV-04 | Digitar o token certo | Painel abre na tela Geral | |
| NAV-05 | Recarregar a página (F5) | Continua logado | |
| NAV-06 | Clicar em Sair | Volta à tela de token | |
| NAV-07 | Trocar o tema claro/escuro, na tela de token e no painel | Cores mudam e a escolha é mantida depois de recarregar | |

### 2.2 Navegação e monitoramento

| ID | Passos | Resultado esperado | Resultado |
|---|---|---|---|
| NAV-10 | Clicar em cada item do menu lateral | Cada tela abre sem erro; itens ainda não implementados aparecem desabilitados | |
| NAV-11 | Tela Geral com a loja gerando carga | Gráficos de tráfego atualizando a cada segundo | |
| NAV-12 | Barra superior | Sniffer, Poller e Docker com estado `ativo` | |
| NAV-13 | Requisições na API | Rotas da loja (`/produtos`, `/produtos/:id`…), latência p50/p95 e status 2xx/4xx/5xx | |
| NAV-14 | Consultas no banco | Consultas normalizadas com `?` e lista de consultas lentas | |
| NAV-15 | Sistema | Histórico de CPU, memória e disco | |
| NAV-16 | Alvos → Novo alvo, com porta inválida (`abc`) | Erro no campo, sem salvar | |
| NAV-17 | Alvos → Editar a loja: domínio `loja.local`, porta DNS `1053`, porta SMTP `1025` | Salva; a coluna DNS/SMTP mostra as portas e o estado fica "No ar" | |
| NAV-18 | Tela DNS, com `make carga ARGS="-dns 127.0.0.1:1053"` em `subjects/loja` | Sonda ativa com disponibilidade e latência; Tráfego real com cerca de 5 consultas por segundo e respostas `NOERROR` | |
| NAV-19 | Tela SMTP, com `make carga ARGS="-smtp 127.0.0.1:1025"` | Sonda com "Sem STARTTLS"; Tráfego real com taxa de entrega perto de 100% e e-mails no último minuto | |
| NAV-20 | Requisições na API e Consultas no banco com a loja em carga | Requisições/consultas por segundo, conexões ativas, códigos de erro, pools (processo `loja`) e rollbacks | |
| NAV-21 | Sistema | Leitura, escrita, IOPS e latência do disco com gráficos (no Linux ou WSL2) | |

### 2.3 Alertas e notificação

| ID | Passos | Resultado esperado | Resultado |
|---|---|---|---|
| ALE-01 | Abrir Alertas com tudo funcionando | Cartões preenchidos, todos verdes; aviso de e-mail ativo | |
| ALE-02 | Parar a loja (Ctrl+C no `make demo`) | Disponibilidade vermelha, aviso na tela e e-mail em http://127.0.0.1:8025 | |
| ALE-03 | Subir a loja de novo | Volta ao verde e chega e-mail de normalização | |
| ALE-04 | Gerar lentidão: `while true; do curl -s -o /dev/null "127.0.0.1:3000/lento?ms=1800" & sleep 0.05; done` por 40 s | Latência p95 amarela | |
| ALE-05 | Trocar o domínio da loja para `nao-existe.loja.local` | DNS amarelo (NXDOMAIN) | |
| ALE-06 | `docker compose stop coredns` em `exemplos/dns-smtp` | DNS vermelho | |
| ALE-07 | Observar o cartão SMTP com o Mailpit padrão | Alerta amarelo "SMTP sem STARTTLS" | |

### 2.4 Segurança

Rodar cada ataque simulado noutro terminal, em `subjects/loja`, com a loja no ar:

| ID | Passos | Resultado esperado | Resultado |
|---|---|---|---|
| SEG-01 | `make carga ARGS="-ataque forca-bruta -taxa 2"` por 30 s | Cartão Segurança: "Logins falhos por IP" e "Logins falhos por conta" (conta `ana`) amarelos e depois vermelhos para `IP 203.0.113.66` | |
| SEG-02 | Parar o ataque e esperar 1 minuto | Os dois voltam ao verde | |
| SEG-03 | `make carga ARGS="-ataque spraying -taxa 2"` por 30 s | "Contas diferentes testadas por IP (password spraying)" vermelho para `IP 198.51.100.23` | |
| SEG-04 | Com a carga normal rodando há 3 minutos, `make carga ARGS="-ataque ddos -taxa 200 -workers 32"` por 1 minuto | "Pico de tráfego (possível DDoS)" vermelho, com o volume atual e o normal no detalhe | |
| SEG-05 | Parar o ataque DDoS | Pico de tráfego volta ao verde em cerca de 1 minuto | |
| SEG-06 | Editar `subjects/loja/Caddyfile` (por exemplo, acrescentar um comentário) | Em até 10 s, "Arquivo de configuração alterado" amarelo, com hash e tamanho antigos e novos no detalhe; e-mail enviado | |
| SEG-07 | `chmod 666 subjects/loja/Caddyfile` | Novo alerta amarelo com "permissões -rw-r--r-- → -rw-rw-rw-" (desfaça com `chmod 644`) | |
| SEG-08 | Renomear o arquivo monitorado | Alerta vermelho "arquivo removido ou ilegível"; ao voltar o nome, amarelo "arquivo criado" | |

### 2.5 Arsenal e relatórios

| ID | Passos | Resultado esperado | Resultado |
|---|---|---|---|
| ARS-01 | Arsenal → escolher a loja e rodar Nmap | Console ao vivo e portas abertas listadas | |
| ARS-02 | Rodar Nuclei na loja | Achados (por exemplo, cabeçalhos `Server` e `X-Powered-By` expostos) | |
| ARS-03 | Rodar a mesma ferramenta de novo e abrir Comparação entre scans | Achados novos e resolvidos em relação à varredura anterior | |

### 2.6 Responsividade

| ID | Passos | Resultado esperado | Resultado |
|---|---|---|---|
| RES-01 | Modo celular (360 px de largura) na tela Geral | Sem rolagem horizontal na página; gráficos se ajustam | |
| RES-02 | Modo celular nas telas Alertas e Alvos | Cartões empilhados em uma coluna; tabelas com rolagem própria | |
| RES-03 | Modo tablet (768 px) | Layout em duas colunas onde couber | |

## 3. Registro da execução

| Data | Quem executou | Versão (commit) | Casos com falha | Observações |
|---|---|---|---|---|
| | | | | |
