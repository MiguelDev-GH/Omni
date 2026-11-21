import { act, render, screen, within } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { expect, test } from "vitest";
import type { Alert, AlertsOverview } from "../api/alerts";
import { useAppStore } from "../store/app";
import { json, mockFetch } from "../test/fetch";
import AlertsPage from "./AlertsPage";

const down: Alert = {
  id: 1,
  regra: "disponibilidade",
  servico: "web",
  nome: "Disponibilidade",
  recurso: "loja:3000",
  nivel: "vermelho",
  nivel_anterior: "verde",
  valor: 2,
  limite: 2,
  unidade: "",
  detalhe: "connection refused",
  hora: "2026-11-21T12:00:00Z",
};

const overview: AlertsOverview = {
  estado: [
    { ...down, limite_amarelo: 1, limite_vermelho: 2, desde: down.hora, atualizado: down.hora },
    {
      regra: "sistema_cpu",
      servico: "sistema",
      nome: "Uso de CPU",
      recurso: "",
      nivel: "verde",
      valor: 12.34,
      unidade: "%",
      limite_amarelo: 80,
      limite_vermelho: 95,
      desde: down.hora,
      atualizado: down.hora,
    },
  ],
  historico: [down],
  regras: [{ id: "sistema_cpu", servico: "sistema", nome: "Uso de CPU", unidade: "%", limite_amarelo: 80, limite_vermelho: 95 }],
  email: { ativo: false, destinatarios: 0 },
};

function renderPage(body: AlertsOverview | null = overview) {
  const calls = mockFetch((c) => {
    if (c.url === "/api/alertas" && body) return json(200, body);
    return json(500, { erro: "inesperado" });
  });
  useAppStore.setState({ token: "t" });
  const router = createMemoryRouter([{ path: "/", element: <AlertsPage /> }]);
  render(<RouterProvider router={router} />);
  return calls;
}

test("groups the current level by service", async () => {
  renderPage();
  const web = await screen.findByRole("region", { name: "Web server" });
  await within(web).findByText("fora do ar");
  expect(within(web).getAllByText("Vermelho").length).toBeGreaterThan(0);
  expect(within(web).getByText(/loja:3000/)).toBeInTheDocument();

  const sys = screen.getByRole("region", { name: "Sistema" });
  expect(within(sys).getByText("12.3%")).toBeInTheDocument();
  expect(within(sys).getByText("amarelo ≥ 80% · vermelho ≥ 95%")).toBeInTheDocument();

  expect(within(screen.getByRole("region", { name: "Banco de dados" })).getByText("Sem medições ainda.")).toBeInTheDocument();
  expect(screen.getByText("Vermelho", { selector: "p" }).nextElementSibling).toHaveTextContent("1");
});

test("shows the history and the e-mail state", async () => {
  renderPage();
  const table = await screen.findByRole("table", { name: "Histórico de alertas" });
  const rows = within(table).getAllByRole("row").slice(1);
  expect(rows).toHaveLength(1);
  expect(rows[0]).toHaveTextContent("Disponibilidade");
  expect(rows[0]).toHaveTextContent("connection refused");
  expect(rows[0]).toHaveTextContent("era verde");
  expect(screen.getByRole("note")).toHaveTextContent("Notificação por e-mail desligada");
});

test("live alerts update the page", async () => {
  renderPage();
  await screen.findByRole("table", { name: "Histórico de alertas" });
  act(() =>
    useAppStore.getState().addAlert({ ...down, id: 2, nivel: "verde", nivel_anterior: "vermelho", valor: 0, hora: "2026-11-21T12:01:00Z" }),
  );
  const rows = within(screen.getByRole("table", { name: "Histórico de alertas" })).getAllByRole("row").slice(1);
  expect(rows).toHaveLength(2);
  expect(rows[0]).toHaveTextContent("era vermelho");
  expect(within(screen.getByRole("region", { name: "Web server" })).getByText("no ar")).toBeInTheDocument();
});

test("reports a load error", async () => {
  renderPage(null);
  expect(await screen.findByText("Não foi possível carregar os alertas.")).toBeInTheDocument();
});
