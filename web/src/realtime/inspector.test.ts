import { afterEach, expect, test, vi } from "vitest";
import { handleEvent } from "./events";
import { appendRows, bindInspectorSocket, onPackets, setInspector, type PacketBatch } from "./inspector";

afterEach(() => {
  setInspector(false);
  bindInspectorSocket(null);
});

const packet = {
  hora: "2026-11-23T12:00:00Z",
  origem: "10.0.0.1",
  destino: "10.0.0.2",
  porta_origem: 40000,
  porta_destino: 3000,
  protocolo: "HTTP",
  tamanho: 120,
  flags: "PSH,ACK",
  info: "GET /",
};

test("appendRows keeps the newest rows up to the limit", () => {
  expect(appendRows([1, 2, 3], [4, 5], 4)).toEqual([2, 3, 4, 5]);
  const rows = [1];
  expect(appendRows(rows, [], 4)).toBe(rows);
});

test("sends the command and resends it after reconnecting", () => {
  const first = vi.fn();
  bindInspectorSocket(first);
  setInspector(true, { ip: "", porta: 3000, protocolo: "" });
  expect(JSON.parse(first.mock.calls[0]![0] as string)).toEqual({
    tipo: "INSPETOR",
    dados: { ativo: true, filtro: { ip: "", porta: 3000, protocolo: "" } },
  });

  bindInspectorSocket(null);
  const second = vi.fn();
  bindInspectorSocket(second);
  expect(JSON.parse(second.mock.calls[0]![0] as string).dados.filtro.porta).toBe(3000);

  setInspector(false);
  const third = vi.fn();
  bindInspectorSocket(third);
  expect(third).not.toHaveBeenCalled();
});

test("PACOTES reaches listeners and drops malformed packets", () => {
  const got: PacketBatch[] = [];
  const off = onPackets((b) => got.push(b));
  handleEvent(JSON.stringify({ tipo: "PACOTES", dados: { pacotes: [packet, { hora: 1 }], descartados: 7 } }));
  handleEvent(JSON.stringify({ tipo: "PACOTES", dados: "lixo" }));
  off();
  handleEvent(JSON.stringify({ tipo: "PACOTES", dados: { pacotes: [packet], descartados: 0 } }));
  expect(got).toEqual([{ pacotes: [packet], descartados: 7 }]);
});
