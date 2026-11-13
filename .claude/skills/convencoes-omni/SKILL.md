---
name: convencoes-omni
description: Use quando for criar um branch, escrever uma mensagem de commit, abrir ou nomear um pull request, ou declarar uma mudança como pronta no repositório Omni (backend Go, frontend React + TypeScript).
---

# Convenções do Omni

## Visão geral

Todo trabalho no Omni segue duas regras:

1. **Nomes seguem o [Conventional Commits](https://www.conventionalcommits.org/pt-br/v1.0.0/)**: branches, commits e títulos de PR.
2. **Nada é declarado pronto sem verificação**: testes, lint e typecheck rodam antes de cada commit, e o resultado real é relatado.

## Tipos permitidos

| Tipo | Quando usar |
|---|---|
| `feat` | Nova funcionalidade |
| `fix` | Correção de bug |
| `docs` | Só documentação |
| `style` | Formatação, sem mudar comportamento |
| `refactor` | Reestruturação sem nova funcionalidade nem correção |
| `perf` | Melhoria de desempenho |
| `test` | Adição ou ajuste de testes |
| `build` | Dependências, Dockerfiles, sistema de build |
| `ci` | Pipelines de CI |
| `chore` | Manutenção que não se encaixa acima (configs, skills de agentes) |
| `revert` | Reverte um commit anterior |

**Escopos sugeridos** (opcionais, entre parênteses): `sniffer`, `poller`, `orquestrador`, `api`, `ws`, `db`, `frontend`, `relatorios`, `skills`.

## Branches

Formato: `<tipo>/<descricao-em-kebab-case>`

| Correto | Errado |
|---|---|
| `feat/decodificador-postgres` | `postgres` |
| `fix/ws-reconexao` | `Fix_WS` |
| `docs/readme-arquitetura` | `claude/gifted-sagan-ew0dji` |
| `chore/skill-convencoes-e-verificacao` | `minha-branch` |

- Descrição curta, em minúsculas, palavras separadas por hífen, sem acentos.
- Branch criado com outro nome (inclusive nome gerado automaticamente pelo ambiente): renomeie antes do primeiro push com `git branch -m <novo-nome>`.

## Commits

Formato:

```
<tipo>(<escopo opcional>): <descrição>

<corpo opcional: o porquê da mudança>

<rodapé opcional: BREAKING CHANGE, referências>
```

- **Descrição:** em português, no presente do indicativo ("adiciona", "corrige", "remove"), começando em minúscula, sem ponto final, com no máximo 72 caracteres no cabeçalho.
- **Mudança incompatível:** `!` depois do tipo/escopo e rodapé `BREAKING CHANGE: <explicação>`.
- **Um assunto por commit.** Mudança que pede dois tipos vira dois commits.

Exemplos:

```
feat(sniffer): adiciona decodificador do protocolo PostgreSQL
fix(ws): descarta mensagens antigas quando o cliente fica lento
docs: documenta arquitetura e pendências por camada no README
refactor(api)!: renomeia rota /scans para /varreduras

BREAKING CHANGE: clientes que usam /scans precisam migrar para /varreduras.
```

## Pull requests

- **Título:** mesmo formato do cabeçalho de commit, por exemplo `feat(poller): adiciona teste de STARTTLS no SMTP`.
- **Base:** `main`. A `main` é protegida: toda mudança entra por PR, nunca por push direto.
- **Descrição:** o que mudou, por quê, e o resultado da verificação (seção abaixo).

## Verificação antes de commitar

Rode as verificações de cada parte do projeto que você alterou e leia a saída:

| Parte | Lint | Typecheck / build | Testes |
|---|---|---|---|
| Backend Go (pasta com `go.mod`) | `gofmt -l .` (saída vazia) e `golangci-lint run ./...` | `go vet ./...` e `go build ./...` | `go test -race ./...` |
| Frontend (pasta com `package.json`) | `npm run lint` | `npx tsc -b` | `npm test` |

Regras:

- **A verificação vale para o estado final do código.** Uma edição depois do último teste exige rodar de novo.
- **Relate o resultado real**: o que passou, o que falhou (com a saída) e o que não foi possível rodar e por quê. Ferramenta ausente é relatada como "não verificado", nunca como "passou".
- **Nunca desative, pule ou apague um teste** para fazer a verificação passar.
- **Mudança só de documentação** (`docs:`) dispensa testes; diga isso explicitamente.
- **Repositório ainda sem código** para alguma parte: diga que aquela parte não tem o que verificar.

## Erros comuns

| Erro | Correção |
|---|---|
| Commit `update readme` | `docs: atualiza seção de stack no README` |
| Cabeçalho `Feat: Adicionado sniffer.` | `feat(sniffer): adiciona captura na interface lo` |
| `feat` e `fix` misturados num commit | Dois commits, um de cada tipo |
| "Testes passando" com base num teste rodado antes da última edição | Rodar de novo e relatar a saída atual |
| Push direto na `main` | Branch `<tipo>/<descricao>` e PR |
