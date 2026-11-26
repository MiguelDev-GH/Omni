import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import type { Capture, CapturePage } from "../api/captures";
import { bindInspectorSocket, handlePackets, MAX_ROWS, type Packet } from "../realtime/inspector";
import { useAppStore } from "../store/app";
import { json, mockFetch, type FetchCall } from "../test/fetch";
import InspectorPage from "./InspectorPage";

let sent: { ativo: boolean; filtro: { ip: string; porta: number; protocolo: string } }[];

const done: Capture = {
  id: 3,
  inicio: "2026-11-26T12:00:00Z",
  fim: "2026-11-26T12:01:00Z",
  duracao_s: 60,
  ip: "",
  porta: 3000,
  protocolo: "",
  pacotes: 501,
  bytes: 2 * 1024 * 1024,
  descartados: 0,
  status: "concluida",
};

let calls: FetchCall[];

// api responde às rotas das gravações; handler cobre os casos de cada teste.
function api(handler: (c: FetchCall) => Response | undefined = () => undefined) {
  calls = mockFetch((c) => handler(c) ?? (c.url === "/api/capturas" ? json(200, []) : json(500, { erro: "inesperado" })));
}

beforeEach(() => {
  sent = [];
  bindInspectorSocket((msg) => sent.push(JSON.parse(msg).dados));
  useAppStore.setState({ token: "t" });
  api();
});

afterEach(() => bindInspectorSocket(null));

const packet = (i: number): Packet => ({
  hora: "2026-11-23T12:00:00.123Z",
  origem: "10.0.0.1",
  destino: "10.0.0.2",
  porta_origem: 40000 + (i % 1000),
  porta_destino: 3000,
  protocolo: "HTTP",
  tamanho: 100,
  flags: "PSH,ACK",
  info: `GET /rota-${i}`,
  seq: 0,
  ack: 0,
  carga: 0,
});

const batch = (from: number, n: number, descartados = 0) =>
  act(() => handlePackets({ pacotes: Array.from({ length: n }, (_, i) => packet(from + i)), descartados }));

const inTable = () => screen.getByText("Na tabela").nextElementSibling?.textContent;

test("subscribes on mount and unsubscribes on unmount", () => {
  const { unmount } = render(<InspectorPage />);
  expect(sent.at(-1)).toEqual({ ativo: true, filtro: { ip: "", porta: 0, protocolo: "" } });
  unmount();
  expect(sent.at(-1)?.ativo).toBe(false);
});

test("pausing stops adding rows and sends the command", async () => {
  const user = userEvent.setup();
  render(<InspectorPage />);
  batch(0, 3, 2);
  expect(inTable()).toBe("3");
  expect(screen.getByText("Descartados").nextElementSibling).toHaveTextContent("2");

  await user.click(screen.getByRole("button", { name: "Pausar" }));
  expect(sent.at(-1)?.ativo).toBe(false);
  batch(3, 5);
  expect(inTable()).toBe("3");

  await user.click(screen.getByRole("button", { name: "Retomar" }));
  expect(sent.at(-1)?.ativo).toBe(true);
  batch(8, 1);
  expect(inTable()).toBe("4");
});

test("buffer never passes the limit", () => {
  render(<InspectorPage />);
  for (let i = 0; i < 25; i++) batch(i * 500, 500);
  expect(inTable()).toBe(MAX_ROWS.toLocaleString("pt-BR"));
});

test("filters go in the command", async () => {
  const user = userEvent.setup();
  render(<InspectorPage />);
  await user.type(screen.getByLabelText("IP"), "10.0.0.2");
  await user.type(screen.getByLabelText("Porta"), "3000");
  await user.click(screen.getByRole("button", { name: "Filtrar" }));
  expect(sent.at(-1)).toEqual({ ativo: true, filtro: { ip: "10.0.0.2", porta: 3000, protocolo: "" } });

  await user.clear(screen.getByLabelText("Porta"));
  await user.type(screen.getByLabelText("Porta"), "99999");
  const before = sent.length;
  await user.click(screen.getByRole("button", { name: "Filtrar" }));
  expect(screen.getByRole("alert")).toHaveTextContent("porta");
  expect(sent.length).toBe(before);
});

test("clicking a row shows the packet detail", async () => {
  // O jsdom não faz layout; o virtualizador lê a altura por clientHeight.
  const height = vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockReturnValue(512);
  const user = userEvent.setup();
  render(<InspectorPage />);
  batch(0, 2);
  const rows = await screen.findAllByRole("row", { name: /GET \/rota-1/ });
  await user.click(rows[0]!);
  const detail = screen.getByRole("complementary", { name: "Detalhe do pacote" });
  expect(detail).toHaveTextContent("GET /rota-1");
  expect(detail).toHaveTextContent("10.0.0.2:3000");
  height.mockRestore();
});

test("an ACK row names the packet it confirms and the detail jumps to it", async () => {
  const height = vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockReturnValue(512);
  const user = userEvent.setup();
  render(<InspectorPage />);
  const req = { ...packet(0), seq: 101, ack: 501, carga: 18 };
  const ack: Packet = {
    ...packet(0),
    origem: req.destino,
    porta_origem: req.porta_destino,
    destino: req.origem,
    porta_destino: req.porta_origem,
    flags: "ACK",
    info: "",
    seq: 501,
    ack: 119,
  };
  act(() => handlePackets({ pacotes: [req, ack], descartados: 0 }));

  const ackRow = await screen.findByRole("row", { name: /confirma Nº 1/ });
  await user.click(ackRow);
  const detail = screen.getByRole("complementary", { name: "Detalhe do pacote" });
  expect(detail).toHaveTextContent("119");
  await user.click(screen.getByRole("button", { name: /Nº 1/ }));
  expect(detail).toHaveTextContent("GET /rota-0");
  expect(screen.getByRole("row", { name: /GET \/rota-0/ })).toHaveAttribute("aria-selected", "true");
  height.mockRestore();
});

test("records with the applied filter", async () => {
  const user = userEvent.setup();
  useAppStore.getState().setEngine("sniffer", "ativo");
  api((c) => (c.method === "POST" ? json(202, { id: 4 }) : undefined));
  render(<InspectorPage />);

  await user.type(screen.getByLabelText("Porta"), "3000");
  await user.click(screen.getByRole("button", { name: "Filtrar" }));
  expect(screen.getByText("porta 3000")).toBeInTheDocument();

  await user.clear(screen.getByLabelText("Duração (segundos)"));
  await user.type(screen.getByLabelText("Duração (segundos)"), "5");
  await user.click(screen.getByRole("button", { name: "Gravar" }));
  expect(screen.getByRole("alert")).toHaveTextContent("10 a 300");
  expect(calls.some((c) => c.method === "POST")).toBe(false);

  await user.clear(screen.getByLabelText("Duração (segundos)"));
  await user.type(screen.getByLabelText("Duração (segundos)"), "30");
  await user.click(screen.getByRole("button", { name: "Gravar" }));
  await waitFor(() =>
    expect(JSON.parse(calls.find((c) => c.method === "POST")!.body!)).toEqual({
      duracao_s: 30,
      filtro: { ip: "", porta: 3000, protocolo: "" },
    }),
  );
});

test("recording needs the sniffer and shows the server refusal", async () => {
  const user = userEvent.setup();
  api((c) => (c.method === "POST" ? json(409, { erro: "já há uma captura gravando; espere ela terminar" }) : undefined));
  render(<InspectorPage />);
  expect(screen.getByRole("button", { name: "Gravar" })).toBeDisabled();

  act(() => useAppStore.getState().setEngine("sniffer", "ativo"));
  await user.click(screen.getByRole("button", { name: "Gravar" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("já há uma captura gravando");
});

test("shows progress while a recording runs", async () => {
  const recording: Capture = { ...done, id: 9, status: "gravando", fim: undefined, inicio: new Date().toISOString() };
  api((c) => (c.url === "/api/capturas" ? json(200, [recording]) : undefined));
  render(<InspectorPage />);
  expect(await screen.findByRole("progressbar", { name: "Progresso da gravação" })).toHaveAttribute(
    "aria-valuemax",
    "60",
  );
  expect(screen.queryByRole("button", { name: "Abrir a gravação 9 na tabela" })).not.toBeInTheDocument();
});

test("opening a recording stops the live stream and pages through the file", async () => {
  const user = userEvent.setup();
  const page = (n: number): CapturePage => ({ pacotes: [packet(n)], pagina: n, por_pagina: 500, total: 501 });
  api((c) => {
    if (c.url === "/api/capturas") return json(200, [done]);
    const m = /^\/api\/capturas\/3\/pacotes\?pagina=(\d+)$/.exec(c.url);
    return m ? json(200, page(Number(m[1]))) : undefined;
  });
  render(<InspectorPage />);
  const row = (await screen.findByText("porta 3000")).closest("tr")!;
  expect(within(row).getByText("2 MiB")).toBeInTheDocument();

  await user.click(within(row).getByRole("button", { name: "Abrir a gravação 3 na tabela" }));
  expect(sent.at(-1)?.ativo).toBe(false);
  expect(screen.queryByRole("button", { name: "Pausar" })).not.toBeInTheDocument();
  // O jsdom não mede a área de rolagem, então os números mostram o que chegou.
  const stat = (label: string) => screen.getByText(label).nextElementSibling;
  await waitFor(() => expect(stat("Pacotes no arquivo")).toHaveTextContent("501"));
  expect(stat("Página")).toHaveTextContent("1 de 2");

  await user.click(screen.getByRole("button", { name: "Próxima" }));
  await waitFor(() => expect(stat("Página")).toHaveTextContent("2 de 2"));
  expect(calls.map((c) => c.url)).toContain("/api/capturas/3/pacotes?pagina=1");

  await user.click(screen.getByRole("button", { name: "Voltar ao vivo" }));
  expect(sent.at(-1)?.ativo).toBe(true);
  expect(screen.getByRole("button", { name: "Pausar" })).toBeInTheDocument();
});

test("deleting the open recording goes back to live", async () => {
  const user = userEvent.setup();
  vi.stubGlobal("confirm", () => true);
  let deleted = false;
  api((c) => {
    if (c.method === "DELETE") {
      deleted = true;
      return new Response(null, { status: 204 });
    }
    if (c.url === "/api/capturas") return json(200, deleted ? [] : [done]);
    if (c.url.startsWith("/api/capturas/3/pacotes")) return json(200, { pacotes: [], pagina: 0, por_pagina: 500, total: 0 });
    return undefined;
  });
  render(<InspectorPage />);
  await user.click(await screen.findByRole("button", { name: "Abrir a gravação 3 na tabela" }));
  expect(await screen.findByRole("button", { name: "Voltar ao vivo" })).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "Apagar a gravação 3" }));
  await waitFor(() => expect(screen.queryByRole("button", { name: "Voltar ao vivo" })).not.toBeInTheDocument());
  expect(calls.some((c) => c.method === "DELETE" && c.url === "/api/capturas/3")).toBe(true);
  expect(sent.at(-1)?.ativo).toBe(true);
});

test("downloads a recording with the token through a Blob", async () => {
  const user = userEvent.setup();
  const create = vi.fn(() => "blob:gravacao");
  vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: create, revokeObjectURL: vi.fn() }));
  api((c) => {
    if (c.url === "/api/capturas") return json(200, [done]);
    return c.url === "/api/capturas/3/arquivo" ? new Response(new Uint8Array([0xd4, 0xc3, 0xb2, 0xa1])) : undefined;
  });
  render(<InspectorPage />);
  await user.click(await screen.findByRole("button", { name: "Baixar a gravação 3" }));
  await waitFor(() => expect(create).toHaveBeenCalled());
  expect(calls.find((c) => c.url === "/api/capturas/3/arquivo")?.headers.get("Authorization")).toBe("Bearer t");
});
