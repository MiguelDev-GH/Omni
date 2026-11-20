import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { expect, test, vi } from "vitest";
import Layout from "../components/Layout";
import { useAppStore } from "../store/app";
import { json, mockFetch, type FetchCall } from "../test/fetch";
import TargetsPage from "./TargetsPage";

const api = { id: 1, nome: "api", host: "127.0.0.1", portas_app: [3000, 8080] };

function renderPage() {
  useAppStore.setState({ token: "t" });
  const router = createMemoryRouter(
    [{ element: <Layout />, children: [{ path: "/alvos", element: <TargetsPage /> }] }],
    { initialEntries: ["/alvos"] },
  );
  return render(<RouterProvider router={router} />);
}

// backend devolve a lista atual em GET e delega o resto a other.
function backend(list: unknown[], other: (c: FetchCall) => Response = () => json(500, { erro: "inesperado" })) {
  return mockFetch((c) => (c.method === "GET" && c.url === "/api/alvos" ? json(200, list) : other(c)));
}

test("lists targets", async () => {
  backend([api]);
  renderPage();
  const row = (await screen.findByRole("cell", { name: "api" })).closest("tr")!;
  expect(within(row).getByText("3000, 8080")).toBeInTheDocument();
  expect(within(row).getAllByText("—")).toHaveLength(2);
  expect(screen.getByRole("combobox", { name: "Alvo" })).toHaveDisplayValue("api");
});

test("shows empty state", async () => {
  backend([]);
  renderPage();
  expect(
    await screen.findByText("Nenhum alvo cadastrado. Cadastre o primeiro para começar o monitoramento."),
  ).toBeInTheDocument();
  expect(screen.getByRole("combobox", { name: "Alvo" })).toHaveDisplayValue("Nenhum alvo");
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
  await waitFor(() => expect(calls.filter((c) => c.method === "POST")).toHaveLength(1));
  const post = calls.find((c) => c.method === "POST")!;
  expect(post.url).toBe("/api/alvos");
  expect(post.body).toBe('{"nome":"api","host":"127.0.0.1","portas_app":[3000,8080]}');
  await waitFor(() => expect(calls.filter((c) => c.method === "GET")).toHaveLength(2));
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
  expect(calls.filter((c) => c.method === "POST")).toHaveLength(0);
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
