import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { expect, test } from "vitest";
import type { Finding, Scan } from "../api/scans";
import Layout from "../components/Layout";
import { useAppStore } from "../store/app";
import { FakeWebSocket, installFakeWebSocket } from "../test/fakeSocket";
import { json, mockFetch, type FetchCall } from "../test/fetch";
import ArsenalPage from "./ArsenalPage";

const target = { id: 1, nome: "api", host: "127.0.0.1", portas_app: [3000] };
const tools = [{ nome: "nmap", descricao: "Portas abertas e versões dos serviços", imagem: "instrumentisto/nmap:7.95" }];
const done: Scan = { id: 5, alvo_id: 1, ferramentas: ["nmap"], status: "concluida", criada_em: "2026-11-21T12:00:00Z", achados: { info: 1 } };
const finding: Finding = { ferramenta: "nmap", titulo: "Porta 3000/tcp aberta: http", evidencia: "", severidade: "info", porta: 3000, assinatura: "a" };

function renderPage(handler: (c: FetchCall, scans: Scan[]) => Response | undefined = () => undefined) {
  const scans: Scan[] = [];
  const calls = mockFetch((c) => {
    const custom = handler(c, scans);
    if (custom) return custom;
    if (c.url === "/api/ws/ticket") return json(200, { ticket: "t" });
    if (c.url === "/api/alvos") return json(200, [target]);
    if (c.url === "/api/ferramentas") return json(200, tools);
    if (c.url.startsWith("/api/varreduras?")) return json(200, scans);
    if (c.url === "/api/varreduras/5/achados") return json(200, [finding]);
    return json(500, { erro: "inesperado" });
  });
  installFakeWebSocket();
  useAppStore.setState({ token: "t" });
  const router = createMemoryRouter([{ element: <Layout />, children: [{ path: "/arsenal", element: <ArsenalPage /> }] }], {
    initialEntries: ["/arsenal"],
  });
  render(<RouterProvider router={router} />);
  return { calls, scans };
}

test("starts a scan, streams the console and shows findings", async () => {
  const { calls, scans } = renderPage((c, scans) => {
    if (c.method === "POST" && c.url === "/api/varreduras") {
      scans.unshift({ ...done, status: "rodando" });
      return json(202, { id: 5 });
    }
    return undefined;
  });
  await screen.findByLabelText(/nmap/);
  await screen.findByRole("option", { name: "api (127.0.0.1)" });
  await userEvent.click(screen.getByRole("button", { name: "Iniciar varredura" }));

  const post = calls.find((c) => c.method === "POST" && c.url === "/api/varreduras");
  expect(JSON.parse(post!.body!)).toEqual({ alvo_id: 1, ferramentas: ["nmap"] });
  expect(await screen.findByText("Varredura #5 · Rodando")).toBeInTheDocument();

  const ws = FakeWebSocket.instances.at(-1)!;
  act(() => ws.open());
  act(() => ws.message({ tipo: "SCAN_LOG", dados: { scan_id: 5, ferramenta: "nmap", linhas: ["Starting Nmap"] } }));
  expect(screen.getByRole("log")).toHaveTextContent("Starting Nmap");

  scans[0]!.status = "concluida";
  act(() => ws.message({ tipo: "SCAN_CONCLUIDO", dados: { scan_id: 5, alvo_id: 1, ferramentas: ["nmap"], achados: 1, novos_desde_ultimo: 1 } }));
  const table = await screen.findByRole("table", { name: "Achados" });
  expect(within(table).getByText("Porta 3000/tcp aberta: http")).toBeInTheDocument();
  expect(screen.getByRole("status")).toHaveTextContent("1 achado, 1 novo desde a anterior");
  expect(screen.getByRole("link", { name: "Ver achados" })).toHaveAttribute("href", "/relatorios?varredura=5");
  expect(screen.getAllByText("Visão interna").length).toBeGreaterThan(0);
});

test("shows the API error when Docker is unavailable", async () => {
  renderPage((c) => (c.method === "POST" ? json(503, { erro: "Docker indisponível" }) : undefined));
  await screen.findByLabelText(/nmap/);
  await screen.findByRole("option", { name: "api (127.0.0.1)" });
  await userEvent.click(screen.getByRole("button", { name: "Iniciar varredura" }));
  expect(await screen.findByText("Docker indisponível", { selector: "p" })).toBeInTheDocument();
});

test("warns when the Docker engine is unavailable", async () => {
  renderPage();
  await screen.findByLabelText(/nmap/);
  const ws = FakeWebSocket.instances.at(-1)!;
  act(() => ws.open());
  act(() => ws.message({ tipo: "ESTADO_MOTORES", dados: { docker: "indisponivel" } }));
  expect(screen.getByText(/Docker indisponível: inicie o Docker/)).toBeInTheDocument();
  expect(screen.getByText("Docker: indisponível")).toBeInTheDocument();
});
