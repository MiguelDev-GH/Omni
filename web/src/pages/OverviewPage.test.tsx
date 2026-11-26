import { act, render, screen, within } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { expect, test } from "vitest";
import type { TrafficWindow } from "../api/traffic";
import Layout from "../components/Layout";
import { MAX_POINTS } from "../lib/traffic";
import { useAppStore } from "../store/app";
import { FakeWebSocket, installFakeWebSocket } from "../test/fakeSocket";
import { json, mockFetch, type FetchCall } from "../test/fetch";
import OverviewPage from "./OverviewPage";

const t0 = Date.parse("2026-11-21T12:00:00Z");

function win(sec: number, bytes: number, extra: Partial<TrafficWindow> = {}): TrafficWindow {
  return {
    inicio: new Date(t0 + sec * 1000).toISOString(),
    bytes,
    pacotes: 2,
    descartados: 0,
    protocolos: { HTTP: bytes },
    top_origens: [{ ip: "10.0.0.1", bytes, pacotes: 2 }],
    ...extra,
  };
}

function renderPage(history: TrafficWindow[] = [], targets: unknown[] = [{ id: 1, nome: "api", host: "127.0.0.1", portas_app: [3000] }]) {
  const calls = mockFetch((c: FetchCall) => {
    if (c.url === "/api/ws/ticket") return json(200, { ticket: "t" });
    if (c.url === "/api/alvos") return json(200, targets);
    if (c.url.startsWith("/api/trafego")) return json(200, history);
    if (c.url.startsWith("/api/requisicoes")) return json(200, []);
    if (c.url.startsWith("/api/conexoes")) return json(200, []);
    if (c.url.startsWith("/api/consultas")) return json(200, []);
    return json(500, { erro: "inesperado" });
  });
  installFakeWebSocket();
  useAppStore.setState({ token: "t" });
  const router = createMemoryRouter(
    [{ element: <Layout />, children: [{ path: "/geral", element: <OverviewPage /> }] }],
    { initialEntries: ["/geral"] },
  );
  render(<RouterProvider router={router} />);
  return calls;
}

async function socket(): Promise<FakeWebSocket> {
  await screen.findByRole("heading", { name: "Geral" });
  let ws: FakeWebSocket | undefined;
  await act(async () => {
    for (let i = 0; i < 50 && !(ws = FakeWebSocket.instances.at(-1)); i++) await new Promise((r) => setTimeout(r, 10));
  });
  if (!ws) throw new Error("WebSocket não abriu");
  act(() => ws!.open());
  return ws;
}

test("loads history and shows protocols and top sources", async () => {
  const calls = renderPage([
    win(0, 3000, { protocolos: { HTTP: 2000, PostgreSQL: 1000 } }),
    win(1, 1000, { top_origens: [{ ip: "10.0.0.9", bytes: 5000, pacotes: 7 }] }),
  ]);
  const protocols = await screen.findByRole("list", { name: "Protocolos" });
  expect(within(protocols).getByText("HTTP")).toBeInTheDocument();
  expect(within(protocols).getByText("PostgreSQL")).toBeInTheDocument();
  expect(within(protocols).getByText("75%")).toBeInTheDocument();

  const rows = screen.getAllByRole("row").slice(1);
  expect(within(rows[0]!).getByText("10.0.0.9")).toBeInTheDocument();
  expect(within(rows[1]!).getByText("10.0.0.1")).toBeInTheDocument();

  expect(currentRate()).toHaveTextContent("—");
  act(() => useAppStore.getState().setEngine("sniffer", "ativo"));
  expect(currentRate()).toHaveTextContent("1 KB/s");
  expect(screen.getByRole("img", { name: "Banda por segundo, 2 pontos" })).toBeInTheDocument();
  expect(calls.some((c) => c.url.startsWith("/api/trafego?desde="))).toBe(true);
});

const currentRate = () => screen.getByText("Banda agora").nextElementSibling;

test("live events feed the charts and the window stays at 900 points", async () => {
  renderPage(Array.from({ length: MAX_POINTS - 5 }, (_, i) => win(i, 50)));
  await screen.findByRole("img", { name: `Banda por segundo, ${MAX_POINTS - 5} pontos` });
  const ws = await socket();
  act(() => ws.message({ tipo: "ESTADO_MOTORES", dados: { sniffer: "ativo" } }));
  act(() => {
    for (let i = MAX_POINTS - 5; i < MAX_POINTS + 10; i++) ws.message({ tipo: "METRICAS_TRAFEGO", dados: win(i, 100) });
  });
  expect(useAppStore.getState().traffic).toHaveLength(MAX_POINTS);
  expect(screen.getByRole("img", { name: `Banda por segundo, ${MAX_POINTS} pontos` })).toBeInTheDocument();
  expect(currentRate()).toHaveTextContent("100 B/s");
});

test("shows capture permission warning", async () => {
  renderPage();
  const ws = await socket();
  expect(screen.queryByRole("alert")).toBeNull();
  act(() => ws.message({ tipo: "ESTADO_MOTORES", dados: { sniffer: "sem-permissao" } }));
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Captura desativada: o Omni está sem permissão de captura (CAP_NET_RAW)",
  );
  expect(screen.getByText("Sniffer: sem permissão")).toBeInTheDocument();
});

test("asks for a target when there is none", async () => {
  renderPage([], []);
  expect(await screen.findByRole("link", { name: "Cadastre um alvo" })).toHaveAttribute("href", "/alvos");
  expect(screen.getAllByText("Sem tráfego capturado ainda.").length).toBeGreaterThan(0);
});

test("METRICAS_HTTP feeds the requests chart and status bars", async () => {
  renderPage();
  const ws = await socket();
  expect(screen.getAllByText("Sem requisições capturadas ainda.")).toHaveLength(2);
  act(() => {
    ws.message({ tipo: "METRICAS_HTTP", dados: { inicio: "2026-11-21T12:00:00Z", requisicoes: 5, status: { "2xx": 4, "5xx": 1 }, p50_ms: 1, p95_ms: 3, rotas: [] } });
    ws.message({ tipo: "METRICAS_HTTP", dados: { inicio: "2026-11-21T12:00:20Z", requisicoes: 2, status: { "4xx": 2 }, p50_ms: 1, p95_ms: 3, rotas: [] } });
  });
  expect(screen.getByRole("img", { name: "Requisições a cada 10 s, 3 pontos" })).toBeInTheDocument();
  const legend = screen.getByRole("list", { name: "Status HTTP" });
  expect(within(legend).getByText("5xx").nextElementSibling).toHaveTextContent("1");
  expect(within(legend).getByText("4xx").nextElementSibling).toHaveTextContent("2");
});

test("METRICAS_BANCO feeds the queries chart", async () => {
  renderPage();
  const ws = await socket();
  act(() => ws.message({ tipo: "METRICAS_BANCO", dados: { inicio: "2026-11-21T12:00:00Z", consultas: 9, erros: 0, tipos: { SELECT: 9 }, p50_ms: 1, p95_ms: 2, top: [] } }));
  expect(screen.getByRole("img", { name: "Consultas a cada 10 s, 1 pontos" })).toBeInTheDocument();
});

test("shows requests and queries per second from the newest windows", async () => {
  renderPage();
  await socket();
  const inicio = new Date(Date.now() - 12_000).toISOString();
  act(() => {
    useAppStore.getState().addHTTP([{ inicio, requisicoes: 30, status: { "2xx": 30 }, p50_ms: 1, p95_ms: 2 }]);
    useAppStore.getState().addDB([{ inicio, consultas: 90, erros: 0, tipos: { SELECT: 90 }, p50_ms: 1, p95_ms: 2 }]);
  });
  expect(screen.getByText("Requisições por segundo").nextElementSibling).toHaveTextContent("3");
  expect(screen.getByText("Consultas por segundo").nextElementSibling).toHaveTextContent("9");
});

test("METRICAS_CONEXOES feeds the active connections stat", async () => {
  renderPage();
  const ws = await socket();
  expect(screen.getByText("Conexões ativas").nextElementSibling).toHaveTextContent("—");
  act(() =>
    ws.message({ tipo: "METRICAS_CONEXOES", dados: { hora: new Date().toISOString(), portas: [{ porta: 3000, tipo: "app", estabelecidas: 12, estados: {}, clientes: [] }] } }),
  );
  expect(screen.getByText("Conexões ativas").nextElementSibling).toHaveTextContent("12");
});
