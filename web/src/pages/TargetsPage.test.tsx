import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { expect, test, vi } from "vitest";
import Layout from "../components/Layout";
import { useAppStore } from "../store/app";
import { installFakeWebSocket } from "../test/fakeSocket";
import { json, mockFetch, type FetchCall } from "../test/fetch";
import TargetsPage from "./TargetsPage";

const api = { id: 1, nome: "api", host: "127.0.0.1", portas_app: [3000, 8080] };

function renderPage() {
  installFakeWebSocket();
  useAppStore.setState({ token: "t" });
  const router = createMemoryRouter(
    [{ element: <Layout />, children: [{ path: "/alvos", element: <TargetsPage /> }] }],
    { initialEntries: ["/alvos"] },
  );
  return render(<RouterProvider router={router} />);
}

// backend devolve a lista atual em GET, um ticket para o WebSocket e delega o resto a other.
function backend(list: unknown[], other: (c: FetchCall) => Response = () => json(500, { erro: "inesperado" })) {
  return mockFetch((c) => {
    if (c.url === "/api/ws/ticket") return json(200, { ticket: "t" });
    if (c.url === "/api/descoberta") return json(200, []);
    return c.method === "GET" && c.url === "/api/alvos" ? json(200, list) : other(c);
  });
}

test("lists targets", async () => {
  backend([api]);
  renderPage();
  const row = (await screen.findByRole("cell", { name: "api" })).closest("tr")!;
  expect(within(row).getByText("3000, 8080")).toBeInTheDocument();
  expect(within(row).getAllByText("—")).toHaveLength(3);
  expect(screen.getByRole("combobox", { name: "Alvo" })).toHaveTextContent("api");
});

test("shows empty state", async () => {
  backend([]);
  renderPage();
  expect(
    await screen.findByText("Nenhum alvo cadastrado. Cadastre o primeiro para começar o monitoramento."),
  ).toBeInTheDocument();
  expect(screen.getByRole("combobox", { name: "Alvo" })).toHaveTextContent("Nenhum alvo");
});

async function fillNew(fields: { nome?: string; host?: string; portas?: string }) {
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Novo alvo" }));
  if (fields.nome) await user.type(screen.getByLabelText("Nome"), fields.nome);
  if (fields.host) await user.type(screen.getByLabelText("Host"), fields.host);
  if (fields.portas) await user.type(screen.getByLabelText("Portas da aplicação"), fields.portas);
  await user.click(screen.getByRole("button", { name: "Salvar" }));
}

test("creates target with parsed ports", async () => {
  const calls = backend([], (c) => json(201, { ...JSON.parse(c.body ?? "{}"), id: 1 }));
  renderPage();
  await fillNew({ nome: "api", host: "127.0.0.1", portas: "3000 8080" });
  await waitFor(() => expect(calls.filter((c) => c.method === "POST" && c.url === "/api/alvos")).toHaveLength(1));
  const post = calls.find((c) => c.method === "POST" && c.url === "/api/alvos")!;
  expect(post.url).toBe("/api/alvos");
  expect(post.body).toBe('{"nome":"api","host":"127.0.0.1","portas_app":[3000,8080]}');
  // Um GET do layout, um da página ao montar e o recarregamento depois do POST.
  await waitFor(() => expect(calls.filter((c) => c.method === "GET" && c.url === "/api/alvos")).toHaveLength(3));
});

test("shows server error under field", async () => {
  backend([], () => json(400, { erro: "host inválido", campo: "host" }));
  renderPage();
  await fillNew({ nome: "api", host: "x;rm", portas: "3000" });
  expect(await screen.findByText("host inválido")).toBeInTheDocument();
  expect(screen.getByLabelText("Host")).toHaveAttribute("aria-invalid", "true");
});

test("shows server error without field at the top", async () => {
  backend([], () => json(500, { erro: "erro interno" }));
  renderPage();
  await fillNew({ nome: "api", host: "127.0.0.1", portas: "3000" });
  expect(await screen.findByRole("alert")).toHaveTextContent("erro interno");
});

test("rejects non-numeric ports locally", async () => {
  const calls = backend([]);
  renderPage();
  await fillNew({ nome: "api", host: "127.0.0.1", portas: "abc" });
  expect(await screen.findByText("Use só números separados por vírgula.")).toBeInTheDocument();
  expect(calls.filter((c) => c.method === "POST" && c.url === "/api/alvos")).toHaveLength(0);
});

test("edits target", async () => {
  const calls = backend([{ ...api, porta_banco: 5432 }], (c) => json(200, JSON.parse(c.body ?? "{}")));
  renderPage();
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Editar api" }));
  expect(screen.getByLabelText("Portas da aplicação")).toHaveValue("3000, 8080");
  expect(screen.getByLabelText("Porta do banco (opcional)")).toHaveValue("5432");
  await user.clear(screen.getByLabelText("Nome"));
  await user.type(screen.getByLabelText("Nome"), "api2");
  await user.click(screen.getByRole("button", { name: "Salvar" }));
  await waitFor(() => expect(calls.some((c) => c.method === "PUT")).toBe(true));
  const put = calls.find((c) => c.method === "PUT")!;
  expect(put.url).toBe("/api/alvos/1");
  expect(JSON.parse(put.body!)).toEqual({ nome: "api2", host: "127.0.0.1", portas_app: [3000, 8080], porta_banco: 5432 });
});

test("delete asks confirmation", async () => {
  const calls = backend([api], () => new Response(null, { status: 204 }));
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
  renderPage();
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Excluir api" }));
  expect(confirm).toHaveBeenCalledWith(
    'Excluir o alvo "api"? O histórico de verificações dele também será apagado.',
  );
  expect(calls.some((c) => c.method === "DELETE")).toBe(false);

  confirm.mockReturnValue(true);
  await user.click(screen.getByRole("button", { name: "Excluir api" }));
  await waitFor(() => expect(calls.find((c) => c.method === "DELETE")?.url).toBe("/api/alvos/1"));
});

test("shows port state", async () => {
  backend([api, { ...api, id: 2, nome: "web", portas_app: [4000] }]);
  useAppStore.setState({
    targetStatus: {
      "1:3000": { alvo_id: 1, porta: 3000, tipo: "http", no_ar: false, latencia_ms: 1, hora: "2026-11-20T17:31:46Z" },
      "1:8080": { alvo_id: 1, porta: 8080, tipo: "http", no_ar: true, latencia_ms: 1, hora: "2026-11-20T17:31:46Z" },
    },
  });
  renderPage();
  const row = (await screen.findByRole("cell", { name: "api" })).closest("tr")!;
  expect(within(row).getByText("Fora do ar (porta 3000)")).toBeInTheDocument();
  const web = screen.getByRole("cell", { name: "web" }).closest("tr")!;
  expect(within(web).getByText("Aguardando")).toBeInTheDocument();
});

test("shows all ports up", async () => {
  backend([{ ...api, portas_app: [3000] }]);
  useAppStore.setState({
    targetStatus: { "1:3000": { alvo_id: 1, porta: 3000, tipo: "http", no_ar: true, latencia_ms: 1, hora: "2026-11-20T17:31:46Z" } },
  });
  renderPage();
  const row = (await screen.findByRole("cell", { name: "api" })).closest("tr")!;
  expect(within(row).getByText("No ar")).toBeInTheDocument();
});

test("creates target with DNS and SMTP ports", async () => {
  const calls = backend([], (c) => json(201, { ...JSON.parse(c.body ?? "{}"), id: 1 }));
  renderPage();
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Novo alvo" }));
  await user.type(screen.getByLabelText("Nome"), "loja");
  await user.type(screen.getByLabelText("Host"), "127.0.0.1");
  await user.type(screen.getByLabelText("Portas da aplicação"), "3000");
  await user.type(screen.getByLabelText("Domínio (opcional)"), "loja.local");
  await user.type(screen.getByLabelText("Porta DNS (opcional)"), "1053");
  await user.type(screen.getByLabelText("Porta SMTP (opcional)"), "1025");
  await user.click(screen.getByRole("button", { name: "Salvar" }));
  const isCreate = (c: FetchCall) => c.method === "POST" && c.url === "/api/alvos";
  await waitFor(() => expect(calls.some(isCreate)).toBe(true));
  const post = calls.find(isCreate)!;
  expect(JSON.parse(post.body ?? "{}")).toEqual({
    nome: "loja",
    host: "127.0.0.1",
    portas_app: [3000],
    dominio: "loja.local",
    porta_dns: 1053,
    porta_smtp: 1025,
  });
});

test("rejects a non-numeric DNS port locally", async () => {
  const calls = backend([]);
  renderPage();
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Novo alvo" }));
  await user.type(screen.getByLabelText("Nome"), "loja");
  await user.type(screen.getByLabelText("Host"), "127.0.0.1");
  await user.type(screen.getByLabelText("Portas da aplicação"), "3000");
  await user.type(screen.getByLabelText("Porta DNS (opcional)"), "dns");
  await user.click(screen.getByRole("button", { name: "Salvar" }));
  expect(screen.getByLabelText("Porta DNS (opcional)")).toHaveAttribute("aria-invalid", "true");
  expect(calls.some((c) => c.method === "POST" && c.url === "/api/alvos")).toBe(false);
});

test("shows DNS and SMTP ports and a degraded server", async () => {
  backend([{ ...api, portas_app: [3000], dominio: "loja.local", porta_dns: 1053, porta_smtp: 1025 }]);
  useAppStore.setState({
    targetStatus: {
      "1:3000": { alvo_id: 1, porta: 3000, tipo: "http", no_ar: true, latencia_ms: 1, hora: "2026-11-21T12:00:00Z" },
      "1:1053": {
        alvo_id: 1,
        porta: 1053,
        tipo: "dns",
        no_ar: true,
        degradado: true,
        detalhe: "NXDOMAIN",
        latencia_ms: 1,
        hora: "2026-11-21T12:00:00Z",
      },
    },
  });
  renderPage();
  const row = (await screen.findByRole("cell", { name: "api" })).closest("tr")!;
  expect(within(row).getByText("DNS 1053 · SMTP 1025")).toBeInTheDocument();
  expect(within(row).getByText("Com erro (porta 1053)")).toBeInTheDocument();
});

test("creates target with the source code folder and shows its server error", async () => {
  const calls = backend([], (c) =>
    JSON.parse(c.body ?? "{}").repositorio === "/etc"
      ? json(400, { erro: "repositorio: o código-fonte não pode estar na raiz nem em um diretório do sistema", campo: "repositorio" })
      : json(201, { ...JSON.parse(c.body ?? "{}"), id: 1 }),
  );
  renderPage();
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Novo alvo" }));
  await user.type(screen.getByLabelText("Nome"), "loja");
  await user.type(screen.getByLabelText("Host"), "127.0.0.1");
  await user.type(screen.getByLabelText("Portas da aplicação"), "3000");
  const repo = screen.getByLabelText("Código-fonte (opcional)");
  await user.type(repo, "/etc");
  await user.click(screen.getByRole("button", { name: "Salvar" }));
  expect(await screen.findByText(/diretório do sistema/)).toBeInTheDocument();
  expect(repo).toHaveAttribute("aria-invalid", "true");

  await user.clear(repo);
  await user.type(repo, " /srv/loja ");
  await user.click(screen.getByRole("button", { name: "Salvar" }));
  const isCreate = (c: FetchCall) => c.method === "POST" && c.url === "/api/alvos";
  await waitFor(() => expect(calls.filter(isCreate)).toHaveLength(2));
  expect(JSON.parse(calls.filter(isCreate)[1]!.body ?? "{}")).toEqual({
    nome: "loja",
    host: "127.0.0.1",
    portas_app: [3000],
    repositorio: "/srv/loja",
  });
});
