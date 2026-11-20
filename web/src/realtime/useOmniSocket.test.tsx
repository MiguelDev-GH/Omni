import { render, renderHook, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, test } from "vitest";
import App from "../App";
import { useAppStore } from "../store/app";
import { FakeWebSocket, installFakeWebSocket } from "../test/fakeSocket";
import { json, mockFetch } from "../test/fetch";
import { buildSocketUrl, useOmniSocket } from "./useOmniSocket";

const TOKEN = "0123456789abcdef0123456789abcdef";

beforeEach(() => {
  installFakeWebSocket();
  useAppStore.setState({ token: TOKEN });
});

// ticketBackend responde t1, t2, ... a cada POST /api/ws/ticket e [] para o resto.
function ticketBackend() {
  let n = 0;
  return mockFetch((c) => (c.url === "/api/ws/ticket" ? json(200, { ticket: `t${++n}` }) : json(200, [])));
}

const ticketCalls = (calls: { url: string }[]) => calls.filter((c) => c.url === "/api/ws/ticket").length;

test("buildSocketUrl", () => {
  expect(buildSocketUrl({ protocol: "http:", host: "localhost:9999" }, "a b")).toBe("ws://localhost:9999/ws?ticket=a%20b");
  expect(buildSocketUrl({ protocol: "https:", host: "omni.local" }, "x")).toBe("wss://omni.local/ws?ticket=x");
});

test("connects with fresh ticket each time", async () => {
  const calls = ticketBackend();
  renderHook(() => useOmniSocket({ reconnectInterval: () => 0 }));
  await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1));
  expect(FakeWebSocket.instances[0]?.url).toBe(`ws://${location.host}/ws?ticket=t1`);
  FakeWebSocket.instances[0]?.open();
  FakeWebSocket.instances[0]?.serverClose();
  await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(2));
  expect(FakeWebSocket.instances[1]?.url).toBe(`ws://${location.host}/ws?ticket=t2`);
  expect(ticketCalls(calls)).toBe(2);
});

test("updates poller engine", async () => {
  ticketBackend();
  renderHook(() => useOmniSocket({ reconnectInterval: () => 60_000 }));
  await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1));
  FakeWebSocket.instances[0]?.open();
  await waitFor(() => expect(useAppStore.getState().engines.poller).toBe("ativo"));
  FakeWebSocket.instances[0]?.serverClose();
  await waitFor(() => expect(useAppStore.getState().engines.poller).toBe("indisponivel"));
});

test("retries ticket after network failure", async () => {
  let fail = true;
  const calls = mockFetch((c) => {
    if (c.url !== "/api/ws/ticket") return json(200, []);
    if (fail) {
      fail = false;
      return Promise.reject(new TypeError("rede"));
    }
    return json(200, { ticket: "t9" });
  });
  renderHook(() => useOmniSocket({ reconnectInterval: () => 0 }));
  await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1));
  expect(FakeWebSocket.instances[0]?.url).toContain("ticket=t9");
  expect(ticketCalls(calls)).toBe(2);
});

test("stops reconnecting after ticket 401", async () => {
  const calls = mockFetch((c) => (c.url === "/api/ws/ticket" ? json(401, { erro: "não autorizado" }) : json(200, [])));
  render(<App />);
  expect(await screen.findByText("Sessão expirada. Informe o token novamente.")).toBeInTheDocument();
  const n = ticketCalls(calls);
  await new Promise((r) => setTimeout(r, 1500));
  expect(ticketCalls(calls)).toBe(n);
  expect(FakeWebSocket.instances).toHaveLength(0);
});

test("shows toast on ALERTA_QUEDA", async () => {
  ticketBackend();
  render(<App />);
  await waitFor(() => expect(FakeWebSocket.instances.length).toBeGreaterThan(0));
  const ws = FakeWebSocket.instances.at(-1)!;
  ws.open();
  ws.message({ tipo: "ALERTA_QUEDA", dados: { alvo_id: 1, alvo: "api", porta: 3000, erro: "connection refused", hora: "2026-11-20T17:31:52Z" } });
  expect(await screen.findByRole("alert")).toHaveTextContent("Serviço fora do ar");
});
