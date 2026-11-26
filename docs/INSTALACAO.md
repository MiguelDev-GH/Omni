# Guia de instalação do Omni

Este guia leva uma máquina Linux do zero até o Omni monitorando a loja cobaia, com alertas por e-mail e os detectores de segurança ligados. Os comandos foram escritos para Arch Linux e Ubuntu/Debian; a arquitetura e os motores estão descritos no [README](../README.md).

## 1. Requisitos

| Item | Versão | Para quê |
|---|---|---|
| Linux | qualquer distribuição recente (no Windows, use WSL2) | o sniffer usa `libpcap` e capabilities do Linux |
| Go | 1.24 ou mais novo | compilar o backend |
| Node.js e npm | Node 22.12 ou mais novo | compilar o frontend |
| libpcap | `libpcap` (Arch) ou `libpcap-dev` (Ubuntu) | captura de pacotes |
| Docker Engine e Compose | qualquer versão recente | Arsenal e serviços de teste |
| openssl | qualquer | gerar o token |

```bash
# Arch Linux
sudo pacman -S --needed git go nodejs npm libpcap docker docker-compose openssl

# Ubuntu / Debian (o Go do apt pode ser antigo; prefira https://go.dev/dl)
sudo apt install git golang nodejs npm libpcap-dev docker.io docker-compose-v2 openssl
```

Libere o Docker para o seu usuário (o Arsenal fala com o socket do Docker) e entre de novo na sessão:

```bash
sudo systemctl enable --now docker
sudo usermod -aG docker $USER
# saia e entre de novo; depois confira:
docker ps
```

## 2. Baixar e compilar

```bash
git clone https://github.com/MiguelDev-GH/Omni.git
cd Omni
make build                                           # frontend + binário ./omni
sudo setcap cap_net_raw,cap_net_admin=eip ./omni     # captura sem root
```

O `setcap` dá ao binário só as duas permissões de captura, em vez de rodá-lo como root (princípio do menor privilégio). **Refaça depois de cada `make build`**, porque o binário novo perde as capabilities e o sniffer aparece como `sem-permissao`.

## 3. Configurar

Toda a configuração é feita por variáveis de ambiente.

| Variável | Padrão | Descrição |
|---|---|---|
| `OMNI_TOKEN` | — (obrigatória) | token de acesso ao painel e à API, com pelo menos 32 caracteres. Gere com `openssl rand -hex 32` |
| `OMNI_ADDR` | `127.0.0.1:0` (porta livre aleatória) | endereço do painel. Use `127.0.0.1:8080` para ter uma porta fixa |
| `OMNI_PERMITIR_REMOTO` | desligado | `1` permite escutar fora do loopback. Só faça isso atrás de um proxy com TLS |
| `OMNI_ORIGENS` | — | origens extras aceitas no WebSocket, separadas por vírgula |
| `OMNI_DB` | `omni.db` | arquivo do banco SQLite |
| `OMNI_INTERFACE` | `any` | interface de captura do sniffer |
| `OMNI_INTERVALO_POLLER` | `30s` | intervalo das verificações ativas |
| `OMNI_RETENCAO_VERIFICACOES` | `168h` | por quanto tempo guardar as verificações, as métricas de tráfego, HTTP, banco, DNS, SMTP, conexões e sistema e o histórico de alertas. Abaixo de `168h`, o relatório em PDF não tem dados para a semana inteira |
| `OMNI_CAPTURAS` | `capturas`, ao lado do banco | diretório das capturas `.pcap`, criado com permissão `0700` |
| `OMNI_RETENCAO_CAPTURAS` | `24h` | por quanto tempo guardar as capturas `.pcap`, no mínimo `1h`. Elas têm o tráfego inteiro e podem conter dados pessoais |
| `OMNI_SMTP_HOST`, `OMNI_SMTP_PORTA`, `OMNI_SMTP_USUARIO`, `OMNI_SMTP_SENHA` ou `OMNI_SMTP_SENHA_ARQUIVO`, `OMNI_SMTP_REMETENTE` | e-mail desligado | servidor que envia os alertas. Veja [Alertas e notificação por e-mail](../README.md#alertas-e-notificação-por-e-mail) |
| `OMNI_ALERTA_EMAILS` | — | destinatários dos alertas, separados por vírgula |
| `OMNI_ARQUIVOS_MONITORADOS` | integridade desligada | arquivos de configuração cuja integridade é vigiada, separados por vírgula |

Uma forma prática é guardar as variáveis num arquivo só seu (permissão 600, porque tem o token):

```bash
cat > ~/.omni.env <<'EOF'
export OMNI_TOKEN=COLE_AQUI_O_RESULTADO_DE_openssl_rand_-hex_32
export OMNI_ADDR=127.0.0.1:8080
export OMNI_SMTP_HOST=127.0.0.1
export OMNI_SMTP_PORTA=1025
export OMNI_SMTP_REMETENTE=omni@localhost
export OMNI_ALERTA_EMAILS=equipe@exemplo.com
export OMNI_ARQUIVOS_MONITORADOS="$HOME/Projetos/OMNI/Omni/subjects/loja/Caddyfile,$HOME/Projetos/OMNI/Omni/subjects/loja/compose.yaml"
EOF
chmod 600 ~/.omni.env
```

## 4. Subir

```bash
source ~/.omni.env
./omni
```

Abra o endereço da linha `painel=` do log (com a configuração acima, http://127.0.0.1:8080) e cole o token. O log de início também mostra se o e-mail e a integridade de arquivos estão ligados.

Serviços de apoio para testes (servidor de e-mail falso e DNS):

```bash
cd exemplos/dns-smtp && docker compose up -d      # Mailpit em :1025/:8025 e CoreDNS em :1053
```

## 5. Primeiro alvo: a loja cobaia

Noutro terminal, com o mesmo `OMNI_TOKEN` e o endereço do painel:

```bash
source ~/.omni.env
export OMNI_URL=http://127.0.0.1:8080
cd subjects/loja && make demo
```

O `make demo` sobe o PostgreSQL e o Caddy, inicia a loja na porta 3000, cadastra os alvos no Omni e gera tráfego até Ctrl+C. Em **Alvos**, edite a loja para incluir o domínio `loja.local`, a porta DNS `1053` e a porta SMTP `1025`.

## 6. Conferir a instalação

| O que olhar | Resultado esperado |
|---|---|
| Barra superior do painel | sniffer `ativo` (se aparecer `sem-permissao`, refaça o `setcap`) |
| Tela **Geral** | gráficos de tráfego se mexendo |
| Tela **Alertas** | cartões Web, Banco, DNS, SMTP, Sistema e Segurança preenchidos |
| http://127.0.0.1:8025 | e-mails de alerta chegando quando algo muda de nível |
| `make check` | todas as verificações passam |

## 7. Rodar como serviço (opcional)

Para o Omni subir com a máquina, um serviço do systemd com as mesmas variáveis:

```ini
# /etc/systemd/system/omni.service
[Unit]
Description=Omni - monitoramento e segurança
After=network-online.target docker.service

[Service]
User=omni
WorkingDirectory=/opt/omni
EnvironmentFile=/etc/omni/omni.env
ExecStart=/opt/omni/omni
AmbientCapabilities=CAP_NET_RAW CAP_NET_ADMIN
CapabilityBoundingSet=CAP_NET_RAW CAP_NET_ADMIN
NoNewPrivileges=true
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

O `/etc/omni/omni.env` usa o formato `CHAVE=valor` (sem `export`) e deve ter permissão 600, com dono `root`. Depois: `sudo systemctl daemon-reload && sudo systemctl enable --now omni`.

## 8. Desinstalar

```bash
cd subjects/loja && make derrubar-tudo
cd ../../exemplos/dns-smtp && docker compose down -v
docker volume rm omni-nuclei-templates
rm -f omni omni.db* integridade.json
```
