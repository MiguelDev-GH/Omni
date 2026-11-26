import { expect, test } from "vitest";
import type { PortStatus } from "../store/app";
import { summarizeProbes } from "./probes";

const r = (over: Partial<PortStatus>): PortStatus => ({
  alvo_id: 1,
  porta: 1053,
  tipo: "dns",
  no_ar: true,
  latencia_ms: 10,
  hora: "2026-11-25T12:00:00Z",
  ...over,
});

test("an empty list has no summary values", () => {
  expect(summarizeProbes([])).toEqual({
    disponibilidade: null,
    latenciaMedia: null,
    latenciaP95: null,
    taxaFalhas: null,
    ultimo: undefined,
  });
});

test("availability counts degraded as up and the failure rate counts it as a failure", () => {
  const s = summarizeProbes([r({}), r({ degradado: true }), r({}), r({ no_ar: false, latencia_ms: 0 })]);
  expect(s.disponibilidade).toBe(75);
  expect(s.taxaFalhas).toBe(50);
});

test("latency ignores results that were down", () => {
  const s = summarizeProbes([r({ latencia_ms: 10 }), r({ latencia_ms: 30 }), r({ no_ar: false, latencia_ms: 5000 })]);
  expect(s.latenciaMedia).toBe(20);
  expect(s.latenciaP95).toBe(30);
});

test("the last result is the newest one", () => {
  const last = r({ hora: "2026-11-25T12:01:00Z", detalhe: "NXDOMAIN" });
  expect(summarizeProbes([r({}), last]).ultimo).toBe(last);
});
