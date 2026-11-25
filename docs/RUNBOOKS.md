# Runbooks de operação do Omni

Procedimentos de rotina para manter o Omni funcionando. Cada runbook diz quando usar, os passos e como confirmar que deu certo. Para responder a um **incidente** apontado por um alerta (serviço fora do ar, brute-force, DDoS…), use os [playbooks de incidente](PLAYBOOKS.md).

Convenção: os comandos rodam na pasta do repositório, com as variáveis carregadas (`source ~/.omni.env`, veja o [guia de instalação](INSTALACAO.md)).

---

## RB-01 · Iniciar e parar o Omni

**Quando:** início do expediente, depois de uma atualização, depois de reiniciar a máquina.

```bash
source ~/.omni.env
./omni                     # ou: sudo systemctl start omni
```

Para parar: Ctrl+C no terminal (ou `sudo systemctl stop omni`). O Omni encerra as varreduras em andamento, fecha o WebSocket e grava o banco antes de sair.

**Confirmação:** o log mostra `omni no ar` com o endereço do painel; o painel abre e o sniffer aparece `ativo`.

## RB-02 · Atualizar para a versão nova

**Quando:** depois que um PR entra na `main`.

```bash
git switch main && git pull
make build
sudo setcap cap_net_raw,cap_net_admin=eip ./omni
# pare o Omni antigo (RB-01) e suba de novo
```

As migrações do banco rodam sozinhas na inicialização. **Confirmação:** `make check` passa e o painel abre com os dados antigos.

## RB-03 · Backup e restauração do banco

**Quando:** antes de atualizar, antes de apagar dados, diariamente em produção.

O banco usa WAL; copiar só o `omni.db` com o Omni rodando pode gerar uma cópia inconsistente. Use o backup online do SQLite (pacote `sqlite` no Arch, `sqlite3` no Ubuntu):

```bash
sqlite3 omni.db ".backup 'backup-$(date +%F).db'"
cp integridade.json "integridade-$(date +%F).json"     # linha de base dos arquivos monitorados
```

Restaurar: pare o Omni, troque o arquivo e suba de novo.

```bash
mv omni.db omni.db.antigo && rm -f omni.db-wal omni.db-shm
cp backup-AAAA-MM-DD.db omni.db
```

**Confirmação:** o histórico de verificações e as varreduras antigas aparecem em **Relatórios**.

## RB-04 · Trocar o token de acesso

**Quando:** o token vazou, alguém saiu da equipe, ou periodicamente.

```bash
openssl rand -hex 32           # novo token
# atualize OMNI_TOKEN no ~/.omni.env (ou /etc/omni/omni.env) e reinicie o Omni (RB-01)
```

Sessões abertas no navegador param de funcionar e voltam para a tela de token. Atualize também quem cadastra alvos pela API (por exemplo, a loja cobaia usa o mesmo `OMNI_TOKEN`).

## RB-05 · Sniffer aparece "sem-permissao" ou "inativo"

1. `sem-permissao`: o binário perdeu as capabilities (acontece a cada build). Rode `sudo setcap cap_net_raw,cap_net_admin=eip ./omni` e reinicie.
2. `inativo` com permissão: confira se há alvos cadastrados (sem alvo não há porta para capturar) e se `OMNI_INTERFACE` existe (`ip link`).
3. Compilado com `CGO_ENABLED=0` ou sem `libpcap`: instale `libpcap`/`libpcap-dev` e rode `make build` de novo.

**Confirmação:** a barra superior mostra o sniffer `ativo` e a tela **Geral** volta a receber tráfego.

## RB-06 · Os e-mails de alerta não chegam

1. No log de início, procure `alertas por e-mail ativos`. Se aparece `desligados`, faltam `OMNI_SMTP_HOST` e `OMNI_ALERTA_EMAILS`.
2. Procure no log `alertas: enviar e-mail` com o erro:
   - `connection refused`: o servidor SMTP está fora do ar ou a porta está errada.
   - `servidor SMTP sem STARTTLS`: o Omni se recusou a mandar a senha em texto puro. Use a porta 587 com STARTTLS ou a 465 com TLS.
   - `autenticação`: usuário ou senha errados. No Gmail, use uma senha de app.
3. `limite de e-mails por minuto atingido`: houve mais de 20 alertas no minuto; os seguintes foram suprimidos de propósito e o próximo e-mail informa quantos.
4. Para testar sem conta real, use o Mailpit de `exemplos/dns-smtp` e abra http://127.0.0.1:8025.

## RB-07 · Mudar a lista de arquivos monitorados

**Quando:** um arquivo de configuração novo precisa ser vigiado, ou um saiu de uso.

1. Edite `OMNI_ARQUIVOS_MONITORADOS` (caminhos separados por vírgula) e reinicie o Omni.
2. Arquivos novos entram na linha de base na primeira leitura, sem alerta.

**Depois de uma mudança autorizada** num arquivo vigiado, o alerta amarelo volta ao verde sozinho em 5 minutos; a nova versão já é a referência. Para recomeçar a linha de base do zero (por exemplo, depois de reinstalar tudo): pare o Omni, apague `integridade.json` e suba de novo.

## RB-08 · Disco enchendo

1. Veja o uso na tela **Sistema** (ou `du -h omni.db*`).
2. Reduza `OMNI_RETENCAO_VERIFICACOES` (por exemplo, `72h`) e reinicie; o histórico mais antigo é apagado.
3. Remova capturas e varreduras antigas que não forem mais necessárias, depois de um backup (RB-03).
4. Imagens do Arsenal ocupam espaço no Docker: `docker system df` e, se precisar, `docker image prune`.

## RB-09 · Ambiente de demonstração do zero

```bash
cd subjects/loja && make derrubar-tudo && cd ../..
rm -f omni.db omni.db-wal omni.db-shm integridade.json
source ~/.omni.env && ./omni
# noutro terminal:
(cd exemplos/dns-smtp && docker compose up -d)
cd subjects/loja && make demo
```

Depois, cadastre domínio `loja.local`, porta DNS `1053` e porta SMTP `1025` na loja (tela **Alvos**) e siga o [roteiro de testes de navegação](TESTES-NAVEGACAO.md).
