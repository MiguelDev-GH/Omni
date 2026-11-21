import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { expect, test } from "vitest";
import type { Comparison, Finding, Scan } from "../api/scans";
import { useAppStore } from "../store/app";
import { json, mockFetch } from "../test/fetch";
import ReportsPage from "./ReportsPage";

const f = (titulo: string, severidade: Finding["severidade"], porta = 443): Finding => ({
  ferramenta: "testssl",
  titulo,
  evidencia: "",
  severidade,
  porta,
  assinatura: titulo,
});

const scans: Scan[] = [
  { id: 9, alvo_id: 2, ferramentas: ["testssl"], status: "concluida", criada_em: "2026-11-21T12:00:00Z", achados: { critica: 1, baixa: 1 } },
  { id: 4, alvo_id: 1, ferramentas: ["nmap"], status: "concluida", criada_em: "2026-11-20T12:00:00Z", achados: { info: 2 } },
  { id: 3, alvo_id: 1, ferramentas: ["nmap"], status: "falhou", erro: "nmap: código 1", criada_em: "2026-11-19T12:00:00Z", achados: {} },
];

const cmp: Comparison = {
  novos: [f("HSTS: not offered", "baixa"), f("cert_chain_of_trust: failed (self signed).", "critica")],
  resolvidos: [f("Nota TLS: A", "info")],
  anteriores: { testssl: 7 },
};

function renderPage(entry: string) {
  mockFetch((c) => {
    if (c.url === "/api/varreduras") return json(200, scans);
    if (c.url === "/api/varreduras/9/comparacao") return json(200, cmp);
    if (c.url === "/api/varreduras/9/achados") return json(200, cmp.novos);
    return json(500, { erro: "inesperado" });
  });
  useAppStore.setState({
    token: "t",
    targets: [
      { id: 1, nome: "api", host: "127.0.0.1", portas_app: [3000] },
      { id: 2, nome: "site", host: "127.0.0.1", portas_app: [443] },
    ],
    selectedTargetId: 1,
  });
  const router = createMemoryRouter([{ path: "/relatorios", element: <ReportsPage /> }], { initialEntries: [entry] });
  render(<RouterProvider router={router} />);
}

test("lists the scans of the selected target with severity counts", async () => {
  renderPage("/relatorios");
  const table = await screen.findByRole("table", { name: "Varreduras" });
  const rows = within(table).getAllByRole("row").slice(1);
  expect(rows).toHaveLength(2);
  expect(rows[0]).toHaveTextContent("#4");
  expect(rows[0]).toHaveTextContent("2 info");
  expect(rows[1]).toHaveTextContent("Falhou");
});

test("opening a scan shows new findings first by severity and resolved ones struck through", async () => {
  renderPage("/relatorios?varredura=9");
  const novos = await screen.findByRole("region", { name: "Achados novos" });
  const items = within(novos).getAllByRole("listitem");
  expect(items[0]).toHaveTextContent("Crítica");
  expect(items[0]).toHaveTextContent("cert_chain_of_trust");
  expect(items[1]).toHaveTextContent("HSTS: not offered");
  expect(within(novos).getByRole("heading")).toHaveTextContent("Novos (2)");

  const resolvidos = screen.getByRole("region", { name: "Achados resolvidos" });
  const gone = within(resolvidos).getByRole("listitem");
  expect(gone).toHaveTextContent("Nota TLS: A");
  expect(gone).toHaveClass("line-through");

  // A varredura aberta pelo link seleciona o alvo dela.
  expect(useAppStore.getState().selectedTargetId).toBe(2);
  expect(screen.getByRole("table", { name: "Varreduras" })).toHaveTextContent("#9");
});

test("changing the target closes the opened scan", async () => {
  renderPage("/relatorios?varredura=9");
  await screen.findByRole("region", { name: "Achados novos" });
  await userEvent.selectOptions(screen.getByLabelText("Alvo"), "1");
  expect(screen.queryByRole("region", { name: "Achados novos" })).toBeNull();
  expect(screen.getByRole("table", { name: "Varreduras" })).toHaveTextContent("#4");
});
