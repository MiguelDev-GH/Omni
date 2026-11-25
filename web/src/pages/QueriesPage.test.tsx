import { act, render, screen, within } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { expect, test } from "vitest";
import type { QueryStats } from "../api/queries";
import { useAppStore } from "../store/app";
import { connSnapshot } from "../test/connections";
import { json, mockFetch } from "../test/fetch";
import QueriesPage from "./QueriesPage";

// A API já devolve as lentas ordenadas por p95.
const slow: QueryStats[] = [
  { consulta: "UPDATE contas SET saldo = ? WHERE id = ?", tipo: "UPDATE", execucoes: 4, erros: 1, p50_ms: 210, p95_ms: 900 },
  { consulta: "SELECT * FROM u WHERE email = ?", tipo: "SELECT", execucoes: 1200, erros: 0, p50_ms: 0.8, p95_ms: 3 },
];

function renderPage() {
  const calls = mockFetch((c) => {
    if (c.url.startsWith("/api/conexoes")) return json(200, [connSnapshot(new Date(Date.now() - 2_000).toISOString(), 1)]);
    if (c.url.startsWith("/api/consultas/lentas")) return json(200, slow);
    if (c.url.startsWith("/api/consultas")) return json(200, []);
    return json(500, { erro: "inesperado" });
  });
  useAppStore.setState({ token: "t" });
  const router = createMemoryRouter([{ path: "/", element: <QueriesPage /> }]);
  render(<RouterProvider router={router} />);
  return calls;
}

test("lists the slowest normalized queries", async () => {
  const calls = renderPage();
  const table = await screen.findByRole("table", { name: "Consultas mais lentas" });
  await within(table).findByText(slow[0]!.consulta);
  const rows = within(table).getAllByRole("row").slice(1);
  expect(rows[0]).toHaveTextContent("UPDATE contas SET saldo = ? WHERE id = ?");
  expect(rows[0]).toHaveTextContent("900 ms");
  expect(rows[1]).toHaveTextContent("SELECT * FROM u WHERE email = ?");
  expect(calls.some((c) => c.url === "/api/consultas/lentas?limite=20")).toBe(true);
});

test("live windows feed the chart and stats", async () => {
  renderPage();
  await screen.findByRole("table", { name: "Consultas mais lentas" });
  act(() =>
    useAppStore.getState().addDB([
      { inicio: "2026-11-21T12:00:00Z", consultas: 30, erros: 2, tipos: { SELECT: 30 }, p50_ms: 1, p95_ms: 4.5 },
      { inicio: "2026-11-21T12:00:10Z", consultas: 10, erros: 0, tipos: { SELECT: 10 }, p50_ms: 1, p95_ms: 12 },
    ]),
  );
  expect(screen.getByRole("img", { name: "Consultas a cada 10 s, 2 pontos" })).toBeInTheDocument();
  expect(screen.getByText("Consultas em 15 minutos").nextElementSibling).toHaveTextContent("40");
  expect(screen.getByText("p95 dos últimos 10 s").nextElementSibling).toHaveTextContent("12 ms");
});

test("shows queries per second", async () => {
  renderPage();
  await screen.findByRole("table", { name: "Consultas mais lentas" });
  act(() =>
    useAppStore.getState().addDB([
      { inicio: new Date(Date.now() - 12_000).toISOString(), consultas: 120, erros: 0, tipos: { SELECT: 120 }, p50_ms: 1, p95_ms: 2 },
    ]),
  );
  expect(screen.getByText("Consultas por segundo").nextElementSibling).toHaveTextContent("12");
});

test("Queries page lists the client pools of the database port", async () => {
  renderPage();
  const table = await screen.findByRole("table", { name: "Pools de conexões" });
  const rows = within(table).getAllByRole("row").slice(1);
  expect(rows.map((r) => r.textContent)).toEqual(["loja54324", "(sem permissão para ver)54321"]);
  expect(screen.getByText("Conexões abertas no banco").nextElementSibling).toHaveTextContent("5");
});

test("shows rollbacks and failed transactions per window", async () => {
  renderPage();
  await screen.findByRole("table", { name: "Consultas mais lentas" });
  act(() =>
    useAppStore.getState().addDB([
      { inicio: "2026-11-21T12:00:00Z", consultas: 30, erros: 2, tipos: { SELECT: 25, ROLLBACK: 3 }, transacoes_falhas: 2, p50_ms: 1, p95_ms: 4 },
      { inicio: "2026-11-21T12:00:10Z", consultas: 10, erros: 0, tipos: { ROLLBACK: 1 }, p50_ms: 1, p95_ms: 2 },
    ]),
  );
  expect(screen.getByText("Rollbacks em 15 minutos").nextElementSibling).toHaveTextContent("4");
  const failed = screen.getByText("Transações que falharam em 15 minutos").nextElementSibling;
  expect(failed).toHaveTextContent("2");
  expect(failed).toHaveClass("text-erro");
});
