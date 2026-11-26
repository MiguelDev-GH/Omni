import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { bindInspectorSocket, handlePackets, MAX_ROWS, type Packet } from "../realtime/inspector";
import InspectorPage from "./InspectorPage";

let sent: { ativo: boolean; filtro: { ip: string; porta: number; protocolo: string } }[];

beforeEach(() => {
  sent = [];
  bindInspectorSocket((msg) => sent.push(JSON.parse(msg).dados));
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
