# subjects

Aplicações cobaia, usadas como alvo para testar o Omni de ponta a ponta.

> **Atenção:** as cobaias têm falhas de segurança de propósito (arquivos `.env` e `.git` expostos, cabeçalhos de versão, certificado autoassinado). Elas só podem rodar no loopback (`127.0.0.1`), e a loja e a vitrine se recusam a subir em outro endereço.

| Cobaia | O que é |
|---|---|
| [loja](loja/README.md) | API de loja em Go com PostgreSQL e Caddy (TLS), mais gerador de carga e cadastro automático no Omni |
| [vitrine](vitrine/README.md) | Loja online com interface (React + Express + PostgreSQL) que dá para usar no navegador, com falhas de propósito e um comando para cada ataque simulado |
