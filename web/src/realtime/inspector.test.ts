import { afterEach, expect, test, vi } from "vitest";
import { handleEvent } from "./events";
import {
  appendRows,
  bindInspectorSocket,
  linkAcks,
  onPackets,
  pruneAcks,
  setInspector,
  type AckIndex,
  type Packet,
  type PacketBatch,
} from "./inspector";

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
  seq: 1000,
  ack: 7000,
  carga: 18,
};

// tcp monta um pacote TCP com id; quem envia é a, quem recebe é b.
const tcp = (id: number, a: string, b: string, flags: string, seq: number, ack: number, carga = 0) => {
  const [origem, po] = a.split(":");
  const [destino, pd] = b.split(":");
  const p: Packet = {
    ...packet,
    origem: origem!,
    porta_origem: Number(po),
    destino: destino!,
    porta_destino: Number(pd),
    flags,
    seq,
    ack,
    carga,
  };
  return { ...p, id };
};

const C = "10.0.0.1:40000";
const S = "10.0.0.2:3000";

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

test("linkAcks points each ACK to the segment it confirms", () => {
  const index: AckIndex = new Map();
  const rows = linkAcks(index, [
    tcp(0, C, S, "SYN", 100, 0),
    tcp(1, S, C, "SYN,ACK", 500, 101),
    tcp(2, C, S, "ACK", 101, 501),
    tcp(3, C, S, "PSH,ACK", 101, 501, 18),
    tcp(4, S, C, "ACK", 501, 119),
    tcp(5, S, C, "PSH,ACK", 501, 119, 40),
    tcp(6, C, S, "FIN,ACK", 119, 541),
    tcp(7, S, C, "ACK", 541, 120),
  ]);
  expect(rows.map((r) => r.confirma)).toEqual([undefined, 0, 1, 1, 3, 3, 5, 6]);
});

test("linkAcks ignores pure ACKs as targets and keeps the latest retransmission", () => {
  const index: AckIndex = new Map();
  const [, , , ack] = linkAcks(index, [
    tcp(0, C, S, "PSH,ACK", 101, 501, 18),
    tcp(1, C, S, "ACK", 119, 501),
    tcp(2, C, S, "PSH,ACK", 101, 501, 18),
    tcp(3, S, C, "ACK", 501, 119),
  ]);
  expect(ack!.confirma).toBe(2);
});

test("linkAcks wraps the sequence space and leaves unmatched ACKs alone", () => {
  const index: AckIndex = new Map();
  const rows = linkAcks(index, [
    tcp(0, C, S, "PSH,ACK", 2 ** 32 - 10, 1, 20),
    tcp(1, S, C, "ACK", 1, 10),
    tcp(2, S, C, "ACK", 1, 999),
    { ...tcp(3, C, S, "", 0, 0), protocolo: "UDP" },
  ]);
  expect(rows.map((r) => r.confirma)).toEqual([undefined, 0, undefined, undefined]);
});

test("pruneAcks forgets segments that left the table", () => {
  const index: AckIndex = new Map();
  linkAcks(index, [tcp(0, C, S, "PSH,ACK", 1, 1, 10), tcp(1, C, S, "PSH,ACK", 11, 1, 10)]);
  pruneAcks(index, 1);
  const [a, b] = linkAcks(index, [tcp(2, S, C, "ACK", 1, 11), tcp(3, S, C, "ACK", 1, 21)]);
  expect([a!.confirma, b!.confirma]).toEqual([undefined, 1]);
});
