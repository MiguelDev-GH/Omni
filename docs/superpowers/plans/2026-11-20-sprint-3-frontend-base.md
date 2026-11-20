# Sprint 3: Frontend base, Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Abrir `http://127.0.0.1:8080` mostra a interface do Omni, embutida no próprio binário: tela de token, barra superior com seletor de alvo e estado dos motores, barra lateral com as seções, página de Alvos funcionando (com sugestões dos serviços que já escutam portas na máquina) e alertas de queda e retorno em tempo real.

**Architecture:** Um projeto React + TypeScript em `web/` (Vite, Tailwind v4, Zustand, React Router, TanStack Table, `react-use-websocket`), compilado para `web/dist/` e copiado pelo `Makefile` para `internal/web/static/`, de onde o pacote Go novo `internal/web` serve os arquivos com `go:embed`. O `api.Server` passa a autenticar só o prefixo `/api/` e entrega o resto ao frontend, que tem fallback para `index.html` (rotas do React) e cabeçalhos de segurança próprios. No navegador, um store Zustand único guarda token, alvos, estado dos motores, último status de cada porta, métricas do sistema e toasts; o hook `useOmniSocket` pede um ticket novo a cada (re)conexão e despacha os eventos para esse store.

**Tech Stack:** Go 1.24 (sem dependências Go novas), Node 24 LTS no CI, versões npm exatas na seção abaixo.

**Spec:** `docs/superpowers/plans/2026-11-18-implementacao-basica.md` (seção "Sprint 3"), `docs/SPRINTS.md` (seção "Sprint 3"), `README.md` (seções "Interface (frontend)", "Comunicação em tempo real" e "Segurança da própria plataforma"). Convenções: `.claude/skills/convencoes-omni/SKILL.md`. Este documento expande a Sprint 3 com o código real das Sprints 1 e 2 à vista; onde divergir do plano básico, vale este. A Task 7 (descoberta de serviços) foi acrescentada depois do plano básico, a pedido do dono do projeto, e não tem equivalente lá. Divergências deliberadas: o fallback do frontend recebe o `fs.FS` por parâmetro (`web.Handler(fsys)`) para ser testável sem build; o `Makefile` entra na Task 2, junto com o projeto que ele compila; a ordem das tarefas começa pelo backend, que é o único bloqueio real para servir a interface.

## Global Constraints

- Linha `go 1.24.0` no `go.mod` continua igual; nenhuma dependência Go nova.
- Versões npm exatas (`npm install --save-exact`), todas publicadas antes de 2026-11-06, com `package-lock.json` commitado:
  - dependências: `react@19.3.1`, `react-dom@19.3.1`, `react-router@8.5.1`, `zustand@5.0.16`, `@tanstack/react-table@9.3.1`, `react-use-websocket@4.13.0`
  - dev: `vite@8.3.6`, `@vitejs/plugin-react@6.2.1`, `tailwindcss@4.3.4`, `@tailwindcss/vite@4.3.4`, `typescript@6.0.3` (o `typescript-eslint` 8.72 não aceita TS 7), `@types/react@19.3.0`, `@types/react-dom@19.3.0`, `eslint@10.14.0`, `@eslint/js@10.0.1`, `typescript-eslint@8.72.1`, `eslint-plugin-react-hooks@7.1.1`, `globals@17.14.0`, `vitest@5.1.0`, `jsdom@31.0.0`, `@testing-library/react@16.3.3`, `@testing-library/user-event@14.6.8`, `@testing-library/jest-dom@7.0.1`
  - Se um `npm install` reclamar de peer dependency, desça a versão do pacote que reclamou; nunca use `--force` nem `--legacy-peer-deps`.
- Nenhuma dependência de UI além das listadas (sem shadcn/ui nesta sprint, sem biblioteca de toasts): os componentes são feitos com Tailwind.
- Identificadores em inglês; todo texto visível e campos JSON em português, com acentuação correta. A interface é `lang="pt-BR"` e usa tema escuro como padrão.
- O token fica em `sessionStorage` na chave `omni.token`, com todo acesso em try/catch (sem armazenamento, vale só em memória). Nunca em `localStorage`, cookie ou URL.
- Nenhum `<script>` ou `<style>` inline em HTML servido pelo Omni, inclusive no `placeholder.html`: a CSP proíbe.
- Toda chamada à API passa por `apiFetch`; toda conexão WebSocket por `useOmniSocket`. Nenhum componente chama `fetch` ou `new WebSocket` diretamente.
- Testes do frontend com Vitest + Testing Library em jsdom; `fetch` e `WebSocket` simulados com `vi.stubGlobal`, nunca rede real. Testes Go com `testify`, como nas sprints anteriores.
- Branch `feat/frontend-base`, a partir da `main` com a Sprint 2 mesclada. Commits e PR seguem a skill `convencoes-omni`; PR `feat(frontend): adiciona interface base embutida no binário` com labels `funcionalidade`, `visual` e `motor` (a Task 1 mexe na API). Antes de cada commit, rode a verificação das partes alteradas (`make check` depois da Task 2) e relate o resultado real.

## Review Focus

1. **Token errado, expirado ou trocado no servidor enquanto a aba está aberta:** qualquer 401 da API (inclusive o `POST /api/ws/ticket` de uma reconexão) apaga o token e volta à tela de token com a mensagem "Sessão expirada. Informe o token novamente."; nunca fica em laço de reconexão com token inválido. Testes `apiFetch clears token on 401` (Task 3) e `stops reconnecting after ticket 401` (Task 5).
2. **Recarregar o navegador em `/alvos` ou abrir um link direto:** o Go devolve o `index.html`, mas um arquivo inexistente em `/assets/` devolve 404 (não HTML com status 200, que quebraria o carregamento com erro de MIME), e `/api/qualquer-coisa` continua exigindo token. Testes `TestSPAFallback`, `TestMissingAssetIs404` (Task 1) e `TestUnknownAPIRouteNeedsToken` (já existe, continua passando).
3. **Portas digitadas de formas variadas no formulário** (`3000, 8080`, `3000 8080`, `3000,,8080`, `abc`, `70000`): separadores vírgula e espaço são aceitos, vazios ignorados; texto não numérico vira erro local embaixo do campo; número fora de 1–65535 vai ao servidor e o erro dele aparece embaixo do campo `portas_app`. Teste `parsePorts` (Task 4).
4. **Binário compilado sem o frontend** (`go build` puro, como no CI do Go): `GET /` mostra a página de instrução com `make build`, e a API funciona normalmente. Teste `TestPlaceholderWithoutBuild` (Task 1).
5. **Acesso pelo túnel SSH numa porta local diferente** (`ssh -L 9999:127.0.0.1:8080`) **e servidor de desenvolvimento do Vite:** a URL do WebSocket é montada a partir de `location` (`ws://localhost:9999/ws?...`, ou `wss:` em página `https:`), nunca de um host fixo; a CSP usa `connect-src 'self'`, que cobre `ws:` na mesma origem. Teste `buildSocketUrl` (Task 5).

## Pré-requisitos

- Sprint 2 mesclada na `main`, Go, golangci-lint v2 e Node 24+ instalados. Ferramenta ausente é relatada como "não verificado", nunca como "passou".
- Branch: `git switch main && git pull --ff-only && git switch -c feat/frontend-base`.
- Fora do escopo: a pendência da Sprint 2 sobre o limite de 5000 resultados do histórico fica para a Sprint 4, que traz os primeiros gráficos; esta sprint não consome `GET /api/alvos/{id}/historico`.

## Estrutura de arquivos

| Arquivo | Responsabilidade |
|---|---|
| `internal/api/server.go` | Token só em `/api/`; `HandleFrontend` registra o handler da raiz |
| `internal/web/web.go` | `Assets()` (embed) e `Handler(fsys)`: arquivos, fallback, cabeçalhos |
| `internal/web/static/placeholder.html` | Página servida quando o frontend não foi compilado |
| `cmd/omni/run.go` | Liga `web.Handler(web.Assets())` |
| `Makefile` | `build`, `web`, `check` |
| `.github/workflows/ci.yml` | Job `web` |
| `web/` (config) | `package.json`, `vite.config.ts` (com o bloco `test`), `tsconfig*.json`, `eslint.config.js`, `index.html` |
| `web/src/main.tsx`, `web/src/App.tsx`, `web/src/index.css` | Entrada, escolha entre tela de token e layout, tema |
| `web/src/lib/session.ts` | Leitura e escrita do token com try/catch |
| `web/src/store/app.ts` | `useAppStore`: todo o estado global |
| `web/src/api/client.ts` | `apiFetch`, `ApiError`, `verifyToken` |
| `web/src/api/targets.ts` | Tipo `Target` e CRUD |
| `web/src/router.tsx` | Rotas |
| `web/src/components/Layout.tsx`, `Topbar.tsx`, `Sidebar.tsx`, `Toasts.tsx`, `TargetForm.tsx` | Estrutura da tela e componentes |
| `web/src/pages/TokenPage.tsx`, `TargetsPage.tsx`, `NotFoundPage.tsx` | Páginas |
| `web/src/realtime/events.ts` | `handleEvent`: evento do WebSocket → store |
| `web/src/realtime/useOmniSocket.ts` | Ticket, URL, conexão e reconexão |
| `internal/discovery/discovery.go` | `Listener`, `Suggestion`, `SystemListeners` (gopsutil) e `Suggest` (heurística pura) |
| `internal/api/discovery.go` | `GET /api/descoberta` |
| `web/src/api/discovery.ts` | Tipo `Suggestion` e `listSuggestions` |
| `web/src/components/DiscoveredServices.tsx` | Lista "Detectado nesta máquina" no formulário de novo alvo |

---

### Task 1: Rotas públicas do frontend e pacote `internal/web`

**Files:**
- Modify: `internal/api/server.go`, `cmd/omni/run.go`, `.gitignore`
- Create: `internal/web/web.go`, `internal/web/static/placeholder.html`
- Test: `internal/api/server_test.go`, `internal/web/web_test.go`, `cmd/omni/run_test.go`

**Interfaces:**
- Produces:
  - `func (s *Server) HandleFrontend(h http.Handler)` — registra `h` em `/` do mux externo. O `requireToken` passa a ficar em `/api/` (hoje está em `/`). `GET /api/saude` e `GET /ws` continuam públicos.
  - `func Assets() fs.FS` — `fs.Sub` de `//go:embed all:static`.
  - `func Handler(fsys fs.FS) http.Handler`, com estas regras, nesta ordem:
    1. Método diferente de `GET`/`HEAD` → 405 com `Allow: GET, HEAD`.
    2. Caminho com prefixo `/api` ou igual a `/ws` → 404 (nunca fallback).
    3. `path.Clean` do caminho aponta para um arquivo regular em `fsys` → serve o arquivo (`http.ServeFileFS`).
    4. Último segmento contém `.` → 404 (asset ausente não vira HTML).
    5. Senão → `index.html`; sem `index.html` em `fsys`, `placeholder.html`.
  - Em toda resposta: `Content-Security-Policy: default-src 'self'; connect-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`. `Cache-Control: public, max-age=31536000, immutable` para `/assets/*` e `no-cache` para o resto.
  - `placeholder.html`: HTML puro, sem estilo nem script, título "Omni", texto "A interface não foi compilada neste binário. Rode `make build` na raiz do repositório e inicie o omni de novo. A API continua disponível em /api."

- [ ] **Step 1: Escrever os testes que falham**

Em `internal/api/server_test.go`:
- `TestFrontendIsPublic`: `HandleFrontend` com um handler que responde 200 `"spa"`; `GET /` e `GET /alvos` sem token → 200 `"spa"`; `GET /api/alvos` sem token → 401.
- `TestRootWithoutFrontendIs404`: sem `HandleFrontend`, `GET /` sem token → 404 (não 401).
- Os testes existentes, em especial `TestUnknownAPIRouteNeedsToken` e `TestHandleIsBehindAuth`, continuam sem mudança.

Em `internal/web/web_test.go`, com `fstest.MapFS{"index.html": "<!doctype html>app", "assets/app-1.js": "js"}`:
- `TestServesIndex`: `GET /` → 200, `Content-Type` começa com `text/html`, corpo contém `app`.
- `TestSPAFallback`: `GET /alvos` e `GET /monitoramento/geral` → 200 com o mesmo corpo de `/`.
- `TestServesAsset`: `GET /assets/app-1.js` → 200, corpo `js`, `Cache-Control` igual a `public, max-age=31536000, immutable`.
- `TestMissingAssetIs404`: `GET /assets/nao-existe.js` e `GET /favicon.ico` → 404.
- `TestAPIPathsNeverFallBack`: `GET /api`, `GET /api/x` e `GET /ws` → 404.
- `TestRejectsPost`: `POST /` → 405, `Allow` igual a `GET, HEAD`.
- `TestNoTraversalOrListing`: `GET /../go.mod` → corpo do index (fallback), nunca conteúdo de fora do FS; `GET /assets/` → corpo do index, nunca listagem de diretório.
- `TestSecurityHeaders`: os três cabeçalhos exatos acima em `GET /`, e `Cache-Control: no-cache`.
- `TestPlaceholderWithoutBuild`: `fstest.MapFS{"placeholder.html": "make build"}`; `GET /` e `GET /alvos` → 200 com `make build`.
- `TestEmbeddedAssetsHavePlaceholder`: `fs.Stat(Assets(), "placeholder.html")` sem erro.

Em `cmd/omni/run_test.go`:
- `TestRunServesFrontend`: com `startRun`, `GET /` → 200 `text/html`; `GET /api/saude` → 200 JSON `{"status":"ok"}`; `GET /api/alvos` sem token → 401.

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/api/ ./internal/web/ ./cmd/omni/`
Expected: FAIL (`HandleFrontend` e o pacote `web` não existem).

- [ ] **Step 3: Implementar**

`server.go`: troque `s.outer.Handle("/", requireToken(...))` por `s.outer.Handle("/api/", requireToken(...))` e adicione `HandleFrontend`. `web.go` conforme as regras acima. `run.go`: `srv.HandleFrontend(web.Handler(web.Assets()))` logo depois do `HandlePublic` do `/ws`. `.gitignore`: acrescente `internal/web/static/*` e `!internal/web/static/placeholder.html`.

- [ ] **Step 4: Rodar a verificação do backend**

Run: `test -z "$(gofmt -l .)" && go vet ./... && golangci-lint run ./... && go test -race ./...`
Expected: tudo passa.

- [ ] **Step 5: Commit**

```bash
git add internal/api internal/web cmd/omni .gitignore
git commit -m "feat(api): serve o frontend embutido fora da autenticação por token"
```

---

### Task 2: Projeto Vite, Makefile e CI do frontend

**Files:**
- Create: `web/package.json`, `web/package-lock.json`, `web/index.html`, `web/vite.config.ts`, `web/tsconfig.json`, `web/tsconfig.app.json`, `web/tsconfig.node.json`, `web/eslint.config.js`, `web/src/main.tsx`, `web/src/App.tsx`, `web/src/index.css`, `web/src/test/setup.ts`, `Makefile`
- Modify: `.github/workflows/ci.yml`
- Test: `web/src/App.test.tsx`

**Interfaces:**
- Produces:
  - Scripts em `web/package.json`: `dev` (`vite`), `build` (`tsc -b && vite build`), `lint` (`eslint .`), `typecheck` (`tsc -b`), `test` (`vitest run`). `"type": "module"`, `"private": true`, `"engines": { "node": ">=22.12" }`.
  - `vite.config.ts` (`defineConfig` de `vitest/config`): plugins `react()` e `tailwindcss()`; `server.proxy` com `/api` → `http://127.0.0.1:8080` e `/ws` → `ws://127.0.0.1:8080` com `ws: true`, sem `changeOrigin` (a origem `http://localhost:5173` precisa chegar ao Go, que só a aceita com `OMNI_DEV=1`); `test: { environment: "jsdom", setupFiles: ["src/test/setup.ts"], restoreMocks: true }`.
  - TS em modo `strict`, com `noUncheckedIndexedAccess`. ESLint flat config com `@eslint/js` recommended, `typescript-eslint` recommended e `react-hooks` recommended; ignora `dist`.
  - `index.html`: `<html lang="pt-BR" class="dark">`, `<title>Omni</title>`, só `<div id="root">` e o `<script type="module" src="/src/main.tsx">`.
  - `index.css`: `@import "tailwindcss";` e cores do tema escuro em `@theme` (fundo `zinc-950`, superfície `zinc-900`, texto `zinc-100`, destaque `emerald-500`, erro `red-500`).
  - `setup.ts`: importa `@testing-library/jest-dom/vitest`; `afterEach` com `cleanup()` e `sessionStorage.clear()`.
  - `Makefile` (alvos `.PHONY`):
    - `web`: `cd web && npm ci && npm run build`, depois `find internal/web/static -mindepth 1 ! -name placeholder.html -delete` e `cp -R web/dist/. internal/web/static/`.
    - `build`: depende de `web`; `go build -o omni ./cmd/omni`.
    - `check`: as verificações do Go e do frontend da tabela da skill `convencoes-omni`, nessa ordem, parando na primeira falha.
  - CI: job `web` (`runs-on: ubuntu-latest`) com `actions/checkout@v4`, `actions/setup-node@v4` (`node-version: 24`, `cache: npm`, `cache-dependency-path: web/package-lock.json`), `actions/setup-go@v5` (`go-version-file: go.mod`); passos com `working-directory: web`: `npm ci`, `npm run lint`, `npm run typecheck`, `npm test`; último passo na raiz: `make build`.

- [ ] **Step 1: Criar os arquivos de configuração e instalar as versões fixadas**

Run: `cd web && npm install --save-exact <dependências> && npm install --save-exact -D <dev>` com a lista da seção Global Constraints.
Expected: `package-lock.json` criado, sem aviso de peer dependency não resolvida.

- [ ] **Step 2: Escrever o teste que falha**

`App.test.tsx`: `render(<App />)` sem lançar erro e `screen.getByRole("heading", { name: "Omni" })` presente.

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd web && npm test`
Expected: FAIL (`App` não existe).

- [ ] **Step 4: Implementar `App.tsx` (só o título `Omni` em `<h1>`) e `main.tsx` (`createRoot` com `StrictMode`)**

- [ ] **Step 5: Verificar tudo, inclusive o binário**

Run: `make check && make build && OMNI_TOKEN=$(openssl rand -hex 16) ./omni` e, em outro terminal, `curl -si 127.0.0.1:8080/ | head -20` e `curl -sI 127.0.0.1:8080/assets/<arquivo .js listado no HTML>`.
Expected: `make check` passa; o HTML referencia `/assets/index-*.js`; o asset responde 200 com `Cache-Control: public, max-age=31536000, immutable`. `git status` não mostra nada em `internal/web/static/` além do `placeholder.html`.

- [ ] **Step 6: Commit**

```bash
git add web Makefile .github/workflows/ci.yml
git commit -m "build(frontend): cria projeto vite com lint, testes e ci"
```

---

### Task 3: Token, cliente da API, store e layout

**Files:**
- Create: `web/src/lib/session.ts`, `web/src/store/app.ts`, `web/src/api/client.ts`, `web/src/router.tsx`, `web/src/components/Layout.tsx`, `web/src/components/Topbar.tsx`, `web/src/components/Sidebar.tsx`, `web/src/pages/TokenPage.tsx`, `web/src/pages/TargetsPage.tsx` (só o título "Alvos"; a Task 4 completa), `web/src/pages/NotFoundPage.tsx`
- Modify: `web/src/App.tsx`
- Test: `web/src/api/client.test.ts`, `web/src/components/Sidebar.test.tsx`, `web/src/App.test.tsx`

**Interfaces:**
- Produces:
  - `session.ts`: `readToken(): string | null`, `writeToken(t: string): void`, `removeToken(): void` — chave `omni.token`, try/catch em cada acesso.
  - `store/app.ts`:
    ```ts
    export type EngineState = "ativo" | "inativo" | "sem-permissao" | "indisponivel";
    export type EngineName = "poller" | "sniffer" | "docker";
    export type AppState = {
      token: string | null;
      sessionExpired: boolean;
      setToken: (t: string) => void;          // grava em session.ts, zera sessionExpired
      clearToken: (expired: boolean) => void; // remove de session.ts
      engines: Record<EngineName, EngineState>;
      setEngine: (name: EngineName, s: EngineState) => void;
    };
    export const useAppStore: UseBoundStore<StoreApi<AppState>>;
    ```
    Estado inicial: `token: readToken()`, `poller: "indisponivel"`, `sniffer: "inativo"`, `docker: "inativo"` (motores das Sprints 4 e 7). As Tasks 4 e 5 acrescentam campos a este mesmo store.
  - `api/client.ts`:
    - `class ApiError extends Error { status: number; field?: string }`
    - `apiFetch<T>(path: string, init?: RequestInit): Promise<T>` — usa o token do store em `Authorization: Bearer`; `Content-Type: application/json` quando há corpo; 204 → `undefined`; 401 → `clearToken(true)` e lança `ApiError` 401; outro status ≥ 400 → `ApiError` com `message` = `erro` do corpo JSON e `field` = `campo`, ou `"erro HTTP <status>"` se o corpo não for JSON.
    - `verifyToken(t: string): Promise<"ok" | "invalido" | "sem-conexao">` — `GET /api/alvos` com o token informado, sem passar pelo store.
  - `TokenPage`: campo `type="password"` com rótulo "Token de acesso" e botão "Entrar". Menos de 32 caracteres → "O token tem pelo menos 32 caracteres." sem ir ao servidor; `"invalido"` → "Token inválido."; `"sem-conexao"` → "Não foi possível falar com o Omni. Ele está rodando?"; `"ok"` → `setToken`. Com `sessionExpired`, mostra acima do campo "Sessão expirada. Informe o token novamente.".
  - `App`: sem token → `TokenPage`; com token → `RouterProvider`.
  - `router.tsx` (`createBrowserRouter`): `/` → `<Navigate to="/alvos" replace />`; `/alvos` → `TargetsPage`; `*` → `NotFoundPage` ("Página não encontrada" e link "Ir para Alvos"); todas dentro de `Layout` (Topbar + Sidebar + `<Outlet />`).
  - `Sidebar`: link "Alvos" (`NavLink` para `/alvos`) e as quatro seções do README, cada uma com `<h2>`: **Monitoramento** (Geral, Requisições na API, Consultas no banco, Status HTTP, Sistema), **Tráfego** (Inspetor de pacotes, Capturas), **Arsenal** (Nmap, Nuclei, Nikto, SQLMap, ffuf, Amass, testssl, Vazamentos), **Relatórios** (Histórico, Comparação entre scans, Exportação em PDF). Itens futuros são `<span aria-disabled="true">` com o selo "em breve", nunca links.
  - `Topbar`: nome "Omni", um selo por motor com o texto `<Motor>: <estado>` (ex.: "Poller: indisponível", "Sniffer: inativo") e botão "Sair" (`clearToken(false)`). O seletor de alvo entra na Task 4 e as métricas do sistema na Task 5.

- [ ] **Step 1: Escrever os testes que falham**

`client.test.ts` (`fetch` com `vi.stubGlobal`):
- `sends bearer token`: com token `t`, `apiFetch("/api/alvos")` chama `fetch` com header `Authorization: Bearer t`.
- `apiFetch clears token on 401`: resposta 401 → `useAppStore.getState().token` é `null`, `sessionExpired` é `true` e a promessa rejeita com `ApiError` de `status` 401.
- `maps server field error`: 400 `{"erro":"host inválido","campo":"host"}` → `ApiError` com `message` "host inválido" e `field` "host".
- `handles 204 and non-JSON errors`: 204 → `undefined`; 500 com corpo `oops` → `message` "erro HTTP 500".
- `verifyToken`: 200 → `"ok"`, 401 → `"invalido"`, `fetch` rejeitado → `"sem-conexao"`; nenhum dos três altera o store.

`Sidebar.test.tsx`: os quatro `<h2>` (Monitoramento, Tráfego, Arsenal, Relatórios) presentes; "Alvos" é link para `/alvos`; "Nmap" tem `aria-disabled="true"` e não é link.

`App.test.tsx` (substitui o teste da Task 2):
- `shows token page without token`: campo "Token de acesso" visível.
- `rejects short token locally`: digitar 10 caracteres e "Entrar" → "O token tem pelo menos 32 caracteres." e `fetch` não chamado.
- `logs in with valid token`: `fetch` → 200 `[]`; após "Entrar", a barra lateral aparece, `sessionStorage.getItem("omni.token")` é o token e `localStorage.length` é 0.
- `shows invalid token`: `fetch` → 401 → "Token inválido." e continua na tela de token.
- `works without sessionStorage`: `vi.spyOn(Storage.prototype, "setItem")` lançando erro; o login ainda leva ao layout.
- `logout returns to token page`: com token, clicar "Sair" → tela de token, sem a mensagem de sessão expirada.

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd web && npm test`
Expected: FAIL nos testes novos.

- [ ] **Step 3: Implementar os arquivos da lista Files conforme as interfaces**

- [ ] **Step 4: Rodar a verificação**

Run: `make check`
Expected: tudo passa.

- [ ] **Step 5: Commit**

```bash
git add web/src
git commit -m "feat(frontend): adiciona tela de token, layout e cliente da api"
```

---

### Task 4: Página de Alvos e seletor de alvo

**Files:**
- Create: `web/src/api/targets.ts`, `web/src/components/TargetForm.tsx`
- Modify: `web/src/store/app.ts`, `web/src/pages/TargetsPage.tsx`, `web/src/components/Topbar.tsx`
- Test: `web/src/api/targets.test.ts`, `web/src/pages/TargetsPage.test.tsx`

**Interfaces:**
- Consumes: `apiFetch`, `ApiError` (Task 3); API `GET/POST /api/alvos`, `PUT/DELETE /api/alvos/{id}` (Sprint 1).
- Produces:
  - `api/targets.ts`:
    ```ts
    export type Target = { id: number; nome: string; host: string; portas_app: number[]; porta_banco?: number; dominio?: string };
    export type TargetInput = Omit<Target, "id">;
    export function listTargets(): Promise<Target[]>;
    export function createTarget(t: TargetInput): Promise<Target>;
    export function updateTarget(id: number, t: TargetInput): Promise<Target>;
    export function deleteTarget(id: number): Promise<void>;
    export function parsePorts(text: string): number[] | null; // separa por vírgula e espaço, ignora vazios; null se algum pedaço não for inteiro
    ```
  - Store ganha: `targets: Target[]`, `targetsLoaded: boolean`, `loadTargets(): Promise<void>`, `selectedTargetId: number | null`, `selectTarget(id: number | null): void`. Depois de cada `loadTargets`, se `selectedTargetId` não está mais na lista, vira o id do primeiro alvo (ou `null` sem alvos).
  - `Topbar`: `<select aria-label="Alvo">` com os nomes dos alvos; sem alvos, uma única opção desabilitada "Nenhum alvo".
  - `TargetsPage`: chama `loadTargets` ao montar; tabela TanStack com as colunas Nome, Host, Portas da aplicação (`3000, 8080`), Porta do banco, Domínio (vazio vira "—") e Ações (botões "Editar" e "Excluir" com `aria-label` `Editar <nome>` / `Excluir <nome>`). Sem alvos: "Nenhum alvo cadastrado. Cadastre o primeiro para começar o monitoramento." Botão "Novo alvo" abre o `TargetForm` acima da tabela.
  - Excluir usa `window.confirm('Excluir o alvo "<nome>"? O histórico de verificações dele também será apagado.')` (a FK tem `ON DELETE CASCADE`).
  - `TargetForm` (`props: { initial?: Target; onDone: () => void; onCancel: () => void }`): campos com rótulos "Nome", "Host", "Portas da aplicação" (texto, dica "Separe por vírgula, ex.: 3000, 8080"), "Porta do banco (opcional)", "Domínio (opcional)". Opcionais vazios não vão no corpo. `parsePorts` → `null` mostra "Use só números separados por vírgula." no campo, sem ir ao servidor. `ApiError` com `field` mostra a mensagem embaixo do campo correspondente (`aria-invalid` e `aria-describedby`); sem `field`, num aviso no topo do formulário. Botão "Salvar" desabilitado enquanto a requisição está em andamento; "Cancelar" fecha sem salvar.

- [ ] **Step 1: Escrever os testes que falham**

`targets.test.ts` — `parsePorts`: `"3000, 8080"` → `[3000, 8080]`; `"3000 8080"` → `[3000, 8080]`; `"3000,,8080"` → `[3000, 8080]`; `" "` → `[]`; `"abc"` → `null`; `"30.5"` → `null`; `"70000"` → `[70000]` (o servidor valida o intervalo).

`TargetsPage.test.tsx` (`fetch` simulado por URL e método):
- `lists targets`: lista com `api` (`127.0.0.1`, `[3000, 8080]`, sem banco) → linha com "3000, 8080" e "—"; o seletor "Alvo" mostra `api` selecionado.
- `shows empty state`: lista `[]` → texto do estado vazio; seletor com "Nenhum alvo".
- `creates target with parsed ports`: preencher Nome `api`, Host `127.0.0.1`, Portas `3000 8080` e salvar → `POST /api/alvos` com corpo exatamente `{"nome":"api","host":"127.0.0.1","portas_app":[3000,8080]}` e nova chamada a `GET /api/alvos`.
- `shows server error under field`: `POST` → 400 `{"erro":"host inválido","campo":"host"}` → "host inválido" visível e o campo Host com `aria-invalid="true"`.
- `rejects non-numeric ports locally`: Portas `abc` → "Use só números separados por vírgula." e nenhum `POST`.
- `edits target`: "Editar api" preenche o formulário; mudar Nome e salvar → `PUT /api/alvos/1`.
- `delete asks confirmation`: `window.confirm` → `false` não chama `DELETE`; → `true` chama `DELETE /api/alvos/1`.

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd web && npm test`
Expected: FAIL nos testes novos.

- [ ] **Step 3: Implementar os arquivos da lista Files conforme as interfaces**

- [ ] **Step 4: Rodar a verificação**

Run: `make check`
Expected: tudo passa.

- [ ] **Step 5: Commit**

```bash
git add web/src
git commit -m "feat(frontend): adiciona página de alvos e seletor de alvo"
```

---

### Task 5: Conexão em tempo real, toasts e estado dos alvos

**Files:**
- Create: `web/src/realtime/events.ts`, `web/src/realtime/useOmniSocket.ts`, `web/src/components/Toasts.tsx`
- Modify: `web/src/store/app.ts`, `web/src/components/Layout.tsx`, `web/src/components/Topbar.tsx`, `web/src/pages/TargetsPage.tsx`
- Test: `web/src/realtime/events.test.ts`, `web/src/realtime/useOmniSocket.test.tsx`, `web/src/components/Toasts.test.tsx`

**Interfaces:**
- Consumes: `POST /api/ws/ticket` → `{"ticket": "..."}`; eventos `STATUS_ALVOS`, `ALERTA_QUEDA`, `SERVICO_RESTAURADO`, `METRICAS_SISTEMA` com os campos JSON da seção "Comunicação em tempo real" do README.
- Produces:
  - Store ganha:
    ```ts
    export type PortStatus = { alvo_id: number; porta: number; tipo: "http" | "tcp"; no_ar: boolean; status_http?: number; latencia_ms: number; erro?: string; hora: string };
    export type SystemMetrics = { cpu: number; memoria: number; disco: number; hora: string };
    export type Toast = { id: number; kind: "erro" | "ok"; title: string; body: string };
    targetStatus: Record<string, PortStatus>;   // chave `${alvo_id}:${porta}`
    setPortStatus: (s: PortStatus[]) => void;
    system: SystemMetrics | null;
    setSystem: (m: SystemMetrics) => void;
    toasts: Toast[];                            // no máximo 5; o mais antigo sai
    pushToast: (t: Omit<Toast, "id">) => void;
    dismissToast: (id: number) => void;
    ```
  - `events.ts`: `handleEvent(raw: string): void` — faz `JSON.parse` e despacha pelo `tipo`; JSON inválido, `tipo` desconhecido ou `dados` sem os campos esperados são ignorados sem lançar erro.
    - `STATUS_ALVOS` → `setPortStatus(dados.resultados)`.
    - `ALERTA_QUEDA` → `pushToast({ kind: "erro", title: "Serviço fora do ar", body: "<alvo>, porta <porta>: <erro>" })` e marca a porta como `no_ar: false`.
    - `SERVICO_RESTAURADO` → `pushToast({ kind: "ok", title: "Serviço restaurado", body: "<alvo>, porta <porta>" })` e marca `no_ar: true`.
    - `METRICAS_SISTEMA` → `setSystem(dados)`.
  - `useOmniSocket.ts`:
    - `buildSocketUrl(loc: Pick<Location, "protocol" | "host">, ticket: string): string` — `wss:` quando `loc.protocol === "https:"`, senão `ws:`; ticket com `encodeURIComponent`.
    - `useOmniSocket(options?: { reconnectInterval?: (attempt: number) => number }): void` — `useWebSocket` com URL assíncrona: cada (re)conexão chama `apiFetch<{ ticket: string }>("/api/ws/ticket", { method: "POST" })` e devolve `buildSocketUrl(location, ticket)`; `shouldReconnect: () => true`, `reconnectAttempts: Infinity`, intervalo padrão `min(1000 * 2 ** attempt, 30000)` ms; `onOpen` → `setEngine("poller", "ativo")`; `onClose` → `setEngine("poller", "indisponivel")`; `onMessage` → `handleEvent(ev.data)`. Chamado uma única vez, no `Layout`; como o `Layout` desmonta quando o token é apagado, um 401 no ticket encerra as reconexões.
  - `Toasts`: lista fixa no canto inferior direito; toast `erro` com `role="alert"`, `ok` com `role="status"`; cada um com botão "Fechar" e sumindo sozinho após 10 s.
  - `Topbar` mostra `CPU 11% · Memória 50% · Disco 5%` (inteiros arredondados) quando `system` não é `null`.
  - `TargetsPage` ganha a coluna Estado: sem dados de nenhuma porta → "Aguardando"; todas no ar → "No ar"; alguma fora → "Fora do ar (porta 3000, 5432)".

- [ ] **Step 1: Escrever os testes que falham**

`events.test.ts`:
- `ALERTA_QUEDA pushes error toast`: toast com `title` "Serviço fora do ar" e `body` "api, porta 3000: connection refused"; `targetStatus["1:3000"].no_ar` é `false`.
- `SERVICO_RESTAURADO pushes ok toast` e marca `no_ar: true`.
- `STATUS_ALVOS stores each port` e `METRICAS_SISTEMA stores metrics`.
- `ignores bad input`: `"não é json"`, `{"tipo":"OUTRO","dados":{}}` e `{"tipo":"ALERTA_QUEDA"}` não lançam e não mudam o store.
- `keeps at most five toasts`: seis `ALERTA_QUEDA` → 5 toasts, sem o primeiro.

`useOmniSocket.test.tsx` (classe `FakeWebSocket` via `vi.stubGlobal("WebSocket", ...)` que guarda as instâncias e expõe `open()`, `message(data)` e `close()`; `fetch` do ticket devolve `t1`, depois `t2`; `reconnectInterval: () => 0`):
- `buildSocketUrl`: `{protocol:"http:", host:"localhost:9999"}` + `a b` → `ws://localhost:9999/ws?ticket=a%20b`; `https:` → `wss://...`.
- `connects with fresh ticket each time`: a primeira instância usa `ticket=t1`; após `close()`, a segunda usa `ticket=t2`; foram dois `POST /api/ws/ticket`.
- `updates poller engine`: após `open()`, `engines.poller` é `"ativo"`; após `close()`, `"indisponivel"`.
- `stops reconnecting after ticket 401`: renderizando `<App />` com token e o ticket respondendo 401 → aparece a tela de token com "Sessão expirada. Informe o token novamente." e nenhuma nova chamada de ticket depois disso.
- `shows toast on ALERTA_QUEDA`: com `<App />` logado, `message` de um `ALERTA_QUEDA` → "Serviço fora do ar" visível com `role="alert"`.

`Toasts.test.tsx`: "Fechar" remove o toast; com `vi.useFakeTimers()`, o toast some após 10 s.

`TargetsPage.test.tsx` ganha `shows port state`: com `targetStatus` de `1:3000` fora do ar → "Fora do ar (porta 3000)"; sem status → "Aguardando".

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd web && npm test`
Expected: FAIL nos testes novos.

- [ ] **Step 3: Implementar os arquivos da lista Files conforme as interfaces**

- [ ] **Step 4: Rodar a verificação**

Run: `make check`
Expected: tudo passa.

- [ ] **Step 5: Commit**

```bash
git add web/src
git commit -m "feat(frontend): adiciona alertas em tempo real pelo websocket"
```

---

### Task 6: Documentação e verificação ponta a ponta

**Files:**
- Modify: `README.md`, `docs/SPRINTS.md`

- [ ] **Step 1: Atualizar a documentação**

- `README.md`: na seção "Interface (frontend)", um parágrafo "Como rodar" com os três comandos (`make build`; desenvolvimento com `OMNI_DEV=1 OMNI_TOKEN=<token> go run ./cmd/omni` num terminal e `cd web && npm run dev` noutro, abrindo `http://localhost:5173`); na linha de status do topo, Sprint 2 concluída e Sprint 3 em andamento.
- `docs/SPRINTS.md`: Sprint 3 para 🚧 Em andamento (e Sprint 2 para ✅, se ainda não estiver); na lista de pendências da Sprint 2, trocar "antes dos gráficos da Sprint 3" por "antes dos gráficos da Sprint 4".

- [ ] **Step 2: Verificação final do código**

Run: `make check && make build`
Expected: tudo passa e `./omni` é gerado.

- [ ] **Step 3: Verificação no navegador**

Com `OMNI_TOKEN=$(openssl rand -hex 16) OMNI_INTERVALO_POLLER=2s ./omni` rodando e um serviço de teste (`python3 -m http.server 3000 --bind 127.0.0.1`):
1. `http://127.0.0.1:8080` mostra a tela de token; um token errado mostra "Token inválido."; o certo leva a `/alvos`.
2. Cadastrar o alvo `api` (`127.0.0.1`, porta `3000`): aparece na tabela e no seletor; a coluna Estado vira "No ar" em até 2 s; o selo mostra "Poller: ativo" e a barra superior mostra CPU, memória e disco.
3. Parar o serviço: toast "Serviço fora do ar"; subir de novo: toast "Serviço restaurado".
4. Recarregar a página em `/alvos`: continua logado e na mesma página. Fechar a aba e abrir outra: pede o token de novo.
5. Reiniciar o `omni` com outro token, com a aba aberta: a aba volta à tela de token com "Sessão expirada. Informe o token novamente.".
6. Console do navegador sem erro de CSP.

Expected: os seis itens se comportam como descrito. Relate qualquer desvio, com o que apareceu.

- [ ] **Step 4: Commit e PR**

```bash
git add README.md docs/SPRINTS.md
git commit -m "docs: documenta como rodar a interface e marca a sprint 3 em andamento"
```

Abra o PR `feat(frontend): adiciona interface base embutida no binário` contra a `main`, com as labels `funcionalidade`, `visual` e `motor`, e na descrição o resultado real do `make check` e da verificação no navegador.

---

### Task 7: Descoberta de serviços na máquina

Ao cadastrar um alvo, o formulário mostra os serviços que já escutam portas TCP nesta máquina, para preencher host e portas com um clique. A lista sai dos sockets em estado LISTEN (`/proc/net/tcp*` cruzado com `/proc/<pid>/fd`, via `gopsutil`) e de uma heurística que separa aplicação de banco e esconde serviços do sistema.

**Files:**
- Create: `internal/discovery/discovery.go`, `internal/api/discovery.go`, `web/src/api/discovery.ts`, `web/src/components/DiscoveredServices.tsx`
- Modify: `cmd/omni/run.go`, `web/src/components/TargetForm.tsx`, `README.md`
- Test: `internal/discovery/discovery_test.go`, `internal/api/discovery_test.go`, `cmd/omni/run_test.go`, `web/src/components/DiscoveredServices.test.tsx`

**Interfaces:**
- Consumes: `TargetForm` e `TargetInput` (Task 4); `apiFetch` (Task 3); `api.Server.Handle` (Sprint 2).
- Produces:
  - Em `internal/discovery`:
    ```go
    type Listener struct {
        Address string // IP local do socket, ex.: "127.0.0.1", "0.0.0.0", "::"
        Port    int
        PID     int32  // 0 quando o processo não é visível
        Process string // nome do processo (/proc/<pid>/comm); "" quando não é visível
    }
    type Suggestion struct {
        Process string `json:"processo,omitempty"`
        PID     int32  `json:"pid,omitempty"`
        Host    string `json:"host"`
        Kind    string `json:"tipo"`   // "app" ou "banco"
        Ports   []int  `json:"portas"` // em ordem crescente
    }
    func SystemListeners(ctx context.Context) ([]Listener, error)
    func Suggest(ls []Listener, ownPort int) []Suggestion
    ```
    - `SystemListeners`: `gopsutil/v4/net.ConnectionsWithContext(ctx, "tcp")` (IPv4 e IPv6), só `Status == "LISTEN"`; nome por `process.NewProcessWithContext(ctx, pid)` e `NameWithContext`. Erro ao ler o nome (processo de outro usuário, sem root) deixa `Process` vazio e `PID` zerado, sem falhar a chamada. Nunca lê a linha de comando.
    - `Suggest`, nesta ordem:
      1. Descarta a porta `ownPort` e os processos `sshd`, `systemd-resolve`, `systemd-resolved`, `cupsd`, `avahi-daemon`, `chronyd`, `rpcbind`, `dnsmasq`, `exim4`, `master` (Postfix).
      2. Une a mesma porta escutando em IPv4 e IPv6 numa só.
      3. `Host`: `127.0.0.1` quando o endereço é `0.0.0.0`, `::` ou `127.0.0.0/8`; `::1` quando é `::1`; senão o próprio IP.
      4. Agrupa por `(PID, Host)` quando o PID é conhecido; PID 0 vira uma sugestão por porta.
      5. `Kind` é `"banco"` quando o processo é `postgres`, `mysqld`, `mariadbd`, `redis-server` ou `mongod`, ou, sem nome, quando a porta é 5432, 3306, 6379 ou 27017; senão `"app"`.
      6. Ordena por `Kind` (`app` antes de `banco`) e depois pela menor porta. Sem listeners, devolve `[]Suggestion{}` (nunca `nil`).
  - Em `internal/api`: `func DiscoveryHandler(list func(context.Context) ([]discovery.Listener, error), ownPort int) http.Handler` responde `GET /api/descoberta` com `Suggest(...)` em JSON; erro de `list` → 500 `{"erro":"não foi possível listar os serviços"}`, com o erro original só no log.
  - `run.go`: `srv.Handle("GET /api/descoberta", api.DiscoveryHandler(discovery.SystemListeners, ln.Addr().(*net.TCPAddr).Port))`.
  - Em `web/src/api/discovery.ts`: `type Suggestion = { processo?: string; pid?: number; host: string; tipo: "app" | "banco"; portas: number[] }` e `listSuggestions(): Promise<Suggestion[]>`.
  - `DiscoveredServices` (`props: { targets: Target[]; onPick: (s: Suggestion) => void }`): pede `listSuggestions` ao montar; esconde sugestões cujas portas já estão todas cadastradas num alvo com o mesmo host. Título "Detectado nesta máquina"; um botão por sugestão com o texto `<processo ou "Processo desconhecido"> · <portas separadas por ", ">` e o selo "banco" quando `tipo === "banco"`. Carregando: "Procurando serviços…"; vazio: "Nenhum serviço novo encontrado."; erro: "Não foi possível listar os serviços desta máquina." (o formulário continua utilizável).
  - `TargetForm`, só no modo de criação, renderiza `DiscoveredServices` acima dos campos. Clique numa sugestão `app`: preenche Host com `host`, Portas da aplicação com `portas` e Nome com `processo` quando o Nome está vazio. Clique numa sugestão `banco`: preenche só Porta do banco com a primeira porta.
  - `README.md`: no passo 1 de "Fluxo de funcionamento", citar as sugestões; em "Segurança da própria plataforma", dizer que a descoberta não expõe a linha de comando dos processos.

**Review Focus desta task:** Omni rodando sem root numa máquina onde postgres e nginx pertencem a outros usuários: as portas aparecem como "Processo desconhecido" (postgres ainda sai como `banco` pela porta 5432), e a rota nunca devolve 500 por falta de permissão. Teste `TestSuggestUnknownProcess`.

- [ ] **Step 1: Escrever os testes Go que falham**

`internal/discovery/discovery_test.go` (tabela sobre `Suggest`, sem tocar no sistema):
- `TestSuggestGroupsByProcess`: `node` (PID 10) em `0.0.0.0:3000` e `127.0.0.1:9229` → uma sugestão `{Process:"node", PID:10, Host:"127.0.0.1", Kind:"app", Ports:[3000,9229]}`.
- `TestSuggestMergesIPv4AndIPv6`: `python` (PID 11) em `0.0.0.0:8000` e `:::8000` → uma sugestão com `Ports:[8000]`.
- `TestSuggestDetectsDatabase`: `postgres` (PID 12) em `127.0.0.1:5432` → `Kind:"banco"`.
- `TestSuggestUnknownProcess`: PID 0 em `127.0.0.1:5432` e em `0.0.0.0:80` → duas sugestões, `banco` (5432) e `app` (80), ambas com `Process` vazio.
- `TestSuggestSkipsSystemAndSelf`: `sshd` em 22, `systemd-resolved` em `127.0.0.53:53` e qualquer processo na `ownPort` 8080 → `[]Suggestion{}` (não `nil`).
- `TestSuggestKeepsSpecificAddress`: `java` em `192.168.0.10:8443` → `Host:"192.168.0.10"`; `deno` em `::1:4000` → `Host:"::1"`.
- `TestSuggestOrder`: um `banco` em 3306 e apps em 5000 e 3000 → ordem 3000, 5000, 3306.
- `TestSystemListenersRuns`: abre um `net.Listen("tcp", "127.0.0.1:0")` no próprio teste e confere que `SystemListeners` devolve essa porta com o `PID` de `os.Getpid()`.

`internal/api/discovery_test.go`:
- `TestDiscoveryReturnsSuggestions`: `list` falso com `node` em 3000 → 200, JSON `[{"processo":"node","pid":10,"host":"127.0.0.1","tipo":"app","portas":[3000]}]`.
- `TestDiscoveryEmptyIsArray`: `list` vazio → corpo `[]`.
- `TestDiscoveryError`: `list` com erro → 500 `{"erro":"não foi possível listar os serviços"}`.

`cmd/omni/run_test.go`:
- `TestRunServesDiscovery`: `GET /api/descoberta` sem token → 401; com token → 200 e a porta do próprio `run` fora da lista.

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/discovery/ ./internal/api/ ./cmd/omni/`
Expected: FAIL (pacote `discovery` e `DiscoveryHandler` não existem).

- [ ] **Step 3: Implementar `internal/discovery`, `DiscoveryHandler` e a rota em `run.go`**

- [ ] **Step 4: Escrever os testes do frontend que falham**

`DiscoveredServices.test.tsx` (`fetch` simulado):
- `lists suggestions`: `[{processo:"node",pid:10,host:"127.0.0.1",tipo:"app",portas:[3000,9229]}, {host:"127.0.0.1",tipo:"banco",portas:[5432]}]` → botões "node · 3000, 9229" e "Processo desconhecido · 5432", este com o selo "banco".
- `hides registered services`: com o alvo `{host:"127.0.0.1", portas_app:[3000, 9229]}` cadastrado, o botão do `node` não aparece.
- `shows empty and error states`: `[]` → "Nenhum serviço novo encontrado."; 500 → "Não foi possível listar os serviços desta máquina.".
- `fills form from app suggestion` (renderizando `TargetForm` em modo de criação): clicar em "node · 3000, 9229" → Nome `node`, Host `127.0.0.1`, Portas da aplicação `3000, 9229`.
- `keeps typed name`: com Nome já digitado `minha api`, o clique não troca o Nome.
- `fills database port`: clicar na sugestão `banco` → Porta do banco `5432` e Portas da aplicação intacta.
- `edit mode has no suggestions`: `TargetForm` com `initial` não mostra "Detectado nesta máquina" e não chama `/api/descoberta`.

`TargetsPage.test.tsx`: o `backend` dos testes passa a responder `/api/descoberta` com `[]`, para os testes existentes continuarem contando só as chamadas de `/api/alvos`.

- [ ] **Step 5: Rodar e ver falhar**

Run: `cd web && npm test`
Expected: FAIL nos testes novos.

- [ ] **Step 6: Implementar `discovery.ts`, `DiscoveredServices` e a mudança no `TargetForm`; atualizar o README**

- [ ] **Step 7: Verificar**

Run: `make check && make build`, depois subir o `./omni` com `python3 -m http.server 3000 --bind 127.0.0.1` rodando e abrir "Novo alvo".
Expected: `make check` passa; a lista mostra `python3 · 3000` (ou o nome do processo que o sistema der) e não mostra a porta do próprio Omni; clicar preenche Nome, Host e Portas; depois de salvar, ao abrir "Novo alvo" de novo, a sugestão some.

- [ ] **Step 8: Commit**

```bash
git add internal/discovery internal/api cmd/omni web/src README.md
git commit -m "feat(frontend): sugere serviços da máquina ao cadastrar alvo"
```
