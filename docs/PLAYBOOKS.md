# Playbooks de resposta a incidentes

Cada playbook começa num alerta do Omni e segue as fases do guia de resposta a incidentes do NIST (SP 800-61): **detecção e análise → contenção → erradicação e recuperação → pós-incidente**. Os alertas chegam na tela **Alertas**, como aviso no painel e por e-mail. Para tarefas de operação do próprio Omni (backup, token, sniffer sem permissão…), veja os [runbooks](RUNBOOKS.md).

## Níveis e tempo de resposta

| Nível | Significado | Resposta esperada |
|---|---|---|
| 🟢 Verde | Normal | Nenhuma. Um e-mail de normalização avisa quando um incidente acaba |
| 🟡 Amarelo | Degradação ou suspeita | Investigar em até 30 minutos durante o expediente |
| 🔴 Vermelho | Serviço degradado/indisponível ou ataque em curso | Começar a resposta imediatamente |

## Registro do incidente

Para todo alerta vermelho (e amarelo que vire incidente), registre: hora do primeiro alerta, quem respondeu, linha do tempo das ações, causa, impacto (usuários, dados, tempo fora) e o que muda para não se repetir. O histórico da tela **Alertas** e o relatório em **Relatórios** ajudam a montar a linha do tempo.

---

## PB-01 · Serviço fora do ar

**Alerta:** `Disponibilidade` vermelho (porta não respondeu) ou amarelo (respondeu com erro: HTTP 5xx, RCODE de erro no DNS, comando recusado no SMTP).

**Detecção e análise**
1. Em **Alvos**, veja qual porta caiu e o erro (`connection refused`, `timeout`, `HTTP 503`, `NXDOMAIN`…).
2. `connection refused`: o processo não está escutando. `timeout`: o processo travou, a máquina está sobrecarregada ou um firewall está bloqueando.
3. Confira a tela **Sistema**: CPU, memória ou disco no vermelho explicam muita queda.
4. Para a loja cobaia: `docker compose ps` em `subjects/loja` e o terminal do `make demo`.

**Contenção:** se só uma parte caiu (por exemplo, o banco), avise os usuários e, se possível, coloque a aplicação em modo de manutenção em vez de deixá-la respondendo erro.

**Erradicação e recuperação**
1. Reinicie o serviço (`docker compose restart <serviço>`, `systemctl restart <serviço>` ou o comando da aplicação).
2. Se voltar a cair, leia o log da aplicação antes de reiniciar de novo: reiniciar em loop esconde a causa.
3. Confirme no Omni: a disponibilidade volta ao verde e chega o e-mail de normalização.

**Pós-incidente:** registre a duração da queda. Se a causa foi recurso esgotado, ajuste limites ou capacidade.

## PB-02 · Latência alta ou erros 5xx na API

**Alerta:** `Latência p95 das requisições` ou `Taxa de erros 5xx` amarelo/vermelho.

**Detecção e análise**
1. Em **Requisições na API**, descubra qual rota está lenta ou falhando.
2. Em **Consultas no banco**, veja se a mesma janela tem consultas lentas ou erros: muitas vezes a API está lenta porque o banco está.
3. Cruze com **Sistema** (CPU/memória) e com um pico de tráfego (PB-07).

**Contenção:** se uma rota específica derruba o resto, desative-a ou limite seu uso temporariamente.

**Erradicação e recuperação:** corrija a consulta lenta (índice, paginação) ou a rota com erro e publique a correção. Confirme o p95 e a taxa de 5xx de volta ao verde.

**Pós-incidente:** adicione um teste que reproduza a lentidão ou o erro.

## PB-03 · Banco de dados com erros ou lento

**Alerta:** `Taxa de erros nas consultas` ou `Latência p95 das consultas`.

1. Em **Consultas no banco**, identifique as consultas com erro e as mais lentas (a tela agrupa consultas iguais com valores trocados por `?`).
2. Erros em massa depois de um deploy: provável migração faltando ou consulta inválida; reverta o deploy.
3. Lentidão: `EXPLAIN ANALYZE` na consulta, conferir índices e bloqueios (`pg_stat_activity`).
4. Confirme a volta ao verde e registre a causa.

## PB-04 · DNS falhando ou lento

**Alerta:** `Disponibilidade` do DNS, `Tempo de resolução DNS` ou `Taxa de falhas da sonda`.

1. Reproduza: `dig @<host> -p <porta> <domínio>` e compare com o detalhe do alerta (`NXDOMAIN`, `SERVFAIL`, `REFUSED`, timeout).
2. `NXDOMAIN` inesperado: registro apagado ou zona alterada. Confira se o arquivo da zona está entre os arquivos monitorados e se houve alerta de alteração (PB-08) na mesma hora.
3. `SERVFAIL`: o servidor não conseguiu consultar o autoritativo ou a zona está com erro de sintaxe; veja o log do servidor DNS.
4. Respostas que mudaram para IPs desconhecidos são suspeita de **sequestro de DNS**: trate como incidente de segurança, restaure a zona a partir do backup e troque as credenciais do painel do provedor.

## PB-05 · Servidor SMTP sem STARTTLS ou rejeitando e-mails

**Alerta:** `SMTP sem STARTTLS`, `Disponibilidade` do SMTP amarela (comando recusado) ou `Taxa de falhas da sonda`.

1. **Sem STARTTLS:** os e-mails trafegam em texto puro. Confirme com `openssl s_client -starttls smtp -connect <host>:<porta>`. Configure um certificado no servidor (Postfix: `smtpd_tls_cert_file`, `smtpd_tls_key_file`, `smtpd_tls_security_level = may`). Se o STARTTLS sumiu de repente num servidor que tinha, investigue um ataque de **downgrade** (alguém no caminho removendo a oferta de STARTTLS).
2. **RCPT recusado para postmaster:** o servidor parou de aceitar e-mail do domínio (configuração ou lista de bloqueio). Veja o log do servidor de e-mail.
3. Confirme a volta ao verde.

## PB-06 · Brute-force e password spraying

**Alerta:** `Logins falhos por IP (brute-force)`, `Logins falhos por conta` ou `Contas diferentes testadas por IP (password spraying)`.

| Padrão | O que significa |
|---|---|
| Muitas falhas de **um IP** numa **conta** | brute-force clássico: tentando adivinhar a senha de alguém |
| Muitas falhas numa **conta**, vindas de **vários IPs** | brute-force distribuído (botnet) contra uma conta |
| Um **IP** testando **muitas contas** com poucas tentativas cada | password spraying: uma senha comum ("123456") em todo mundo, para não disparar bloqueio por conta |

**Detecção e análise**
1. O recurso do alerta diz o IP (`IP 203.0.113.66`) ou a conta (`conta ana`). O detalhe diz quantas falhas houve no último minuto.
2. Verifique se alguma tentativa daquele IP **deu certo** depois das falhas (resposta 200 na rota de login). Se deu, trate a conta como comprometida.
3. Confira se o IP é conhecido (escritório, VPN, parceiro) antes de bloquear.

**Contenção**
1. Bloqueie o IP no firewall ou no proxy:
   ```bash
   sudo iptables -I INPUT -s 203.0.113.66 -j DROP
   # ou, no Caddy, um matcher remote_ip com abort
   ```
2. Se a conta foi comprometida: troque a senha, encerre as sessões ativas e revise o que foi feito com ela.

**Erradicação e recuperação**
1. Limite tentativas de login por IP e por conta (rate limit e bloqueio progressivo).
2. Exija senhas fortes e, para contas administrativas, autenticação em dois fatores.
3. Confirme que os contadores caem a zero e os alertas voltam ao verde (em até 1 minuto depois do fim do ataque).

**Pós-incidente:** registre os IPs, as contas visadas e se houve acesso indevido. Senhas fracas visadas (como as da loja cobaia) devem ser trocadas.

## PB-07 · Pico de tráfego e possível DDoS

**Alerta:** `Pico de tráfego (possível DDoS)` (volume várias vezes acima do normal aprendido) ou `Requisições por IP` (um IP sozinho mandando demais).

**Detecção e análise**
1. O detalhe do alerta compara o volume do último minuto com o normal. Um pico pode ser legítimo (campanha, notícia) ou ataque.
2. Em **Geral**, veja os principais IPs de origem. Um IP dominante aponta para um robô ou um DoS simples; muitos IPs com volume parecido apontam para um DDoS distribuído.
3. Veja quais rotas recebem o tráfego (**Requisições na API**): ataques costumam martelar rotas caras (busca, login) ou a página inicial.
4. Acompanhe o impacto: latência (PB-02) e recursos da máquina (**Sistema**).

**Contenção**
1. IP dominante: bloqueie (veja PB-06) ou aplique limite de requisições por IP no proxy.
2. DDoS distribuído: ative a proteção do provedor ou CDN (Cloudflare, AWS Shield), aplique rate limit global e, se necessário, desligue temporariamente rotas caras.
3. Priorize manter a aplicação viva para os usuários legítimos, mesmo com funcionalidade reduzida.

**Erradicação e recuperação:** remova os bloqueios temporários aos poucos, observando se o tráfego volta ao normal. O alerta volta ao verde quando o volume cai abaixo de 3 vezes o normal.

**Pós-incidente:** registre duração, pico atingido, IPs e rotas. Avalie limites de taxa permanentes e capacidade extra.

> O Omni aprende o "normal" com uma média móvel que fica **congelada durante o pico**: se ela continuasse aprendendo, o próprio ataque viraria o novo normal e o alerta sumiria no meio do incidente.

## PB-08 · Arquivo de configuração alterado

**Alerta:** `Arquivo de configuração alterado` amarelo (conteúdo, permissões ou dono mudaram) ou vermelho (arquivo removido ou ilegível).

**Detecção e análise**
1. O recurso do alerta é o caminho do arquivo; o detalhe diz o que mudou (hash SHA-256 antigo e novo, tamanho, permissões, dono). O conteúdo não aparece no alerta de propósito, porque configuração costuma ter senhas.
2. **Pergunte primeiro:** a mudança foi planejada? Há um commit, um ticket ou alguém da equipe que fez o deploy nesse horário?
3. Veja o que mudou:
   ```bash
   git diff -- subjects/loja/Caddyfile      # se o arquivo está versionado
   ls -l <arquivo>                          # permissões e dono atuais
   ```
4. Sinais de alerta: proxy apontando para um IP desconhecido, porta nova exposta, usuário ou senha adicionados, permissões abertas para todos (`-rw-rw-rw-`), mudança fora do horário de deploy.

**Contenção (mudança não autorizada)**
1. Preserve evidências antes de mexer: copie o arquivo alterado e anote a hora.
2. Restaure a versão boa (`git checkout -- <arquivo>` ou backup) e recarregue o serviço.
3. Descubra como o arquivo foi alterado: quem tinha acesso de escrita, logins recentes na máquina (`last`, `journalctl`), e troque as credenciais envolvidas.

**Erradicação e recuperação:** restrinja a escrita no arquivo ao mínimo de pessoas e processos, e passe as mudanças de configuração a acontecerem só por deploy revisado. O alerta volta ao verde 5 minutos depois da última mudança.

**Pós-incidente:** se foi uma mudança legítima sem aviso, combine que mudanças de configuração sejam anunciadas. Se foi invasão, escale para resposta completa (análise da máquina inteira).

## PB-09 · Vulnerabilidade crítica encontrada pelo Arsenal

**Gatilho:** achado de severidade alta ou crítica numa varredura (Nuclei, Nikto, SQLMap, testssl…), visível em **Arsenal** e **Relatórios**.

1. Confirme que não é falso positivo: reproduza com a evidência do achado.
2. Avalie a exposição: o serviço está acessível de fora? Há sinais de exploração nos alertas de segurança (PB-06, PB-07) ou no tráfego?
3. Contenha: aplique a correção ou uma mitigação (desligar o recurso vulnerável, regra no proxy).
4. Rode a varredura de novo e use **Comparação entre scans** para confirmar que o achado aparece como resolvido.
