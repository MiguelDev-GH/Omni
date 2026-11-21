import { expect, test } from "vitest";
import type { TrafficWindow } from "../api/traffic";
import { bandwidthSeries, formatBytes, MAX_POINTS, mergeWindows, protocolTotals, topSources } from "./traffic";

const t0 = Date.parse("2026-11-21T12:00:00Z");

function win(sec: number, bytes: number, extra: Partial<TrafficWindow> = {}): TrafficWindow {
  return {
    inicio: new Date(t0 + sec * 1000).toISOString(),
    bytes,
    pacotes: 1,
    descartados: 0,
    protocolos: { HTTP: bytes },
    top_origens: [{ ip: "10.0.0.1", bytes, pacotes: 1 }],
    ...extra,
  };
}

test("mergeWindows dedupes, sorts and keeps at most MAX_POINTS", () => {
  const merged = mergeWindows([win(2, 20), win(1, 10)], [win(2, 99), win(0, 5)]);
  expect(merged.map((w) => w.bytes)).toEqual([5, 10, 99]);

  const many = Array.from({ length: MAX_POINTS + 50 }, (_, i) => win(i, i));
  const capped = mergeWindows([], many);
  expect(capped).toHaveLength(MAX_POINTS);
  expect(capped[0]?.bytes).toBe(50);
});

test("bandwidthSeries fills missing seconds with zero", () => {
  const series = bandwidthSeries([win(0, 10), win(3, 30)]);
  expect(series.map((p) => p.bytes)).toEqual([10, 0, 0, 30]);
  expect(series[0]?.t).toBe(t0);
  expect(bandwidthSeries([])).toEqual([]);
});

test("bandwidthSeries never passes MAX_POINTS", () => {
  expect(bandwidthSeries([win(0, 1), win(5000, 1)])).toHaveLength(MAX_POINTS);
});

test("protocolTotals and topSources sum all windows", () => {
  const ws = [
    win(0, 100, { protocolos: { HTTP: 60, DNS: 40 }, top_origens: [{ ip: "10.0.0.2", bytes: 100, pacotes: 2 }] }),
    win(1, 50, { protocolos: { DNS: 50 }, top_origens: [{ ip: "10.0.0.1", bytes: 50, pacotes: 1 }] }),
  ];
  expect(protocolTotals(ws)).toEqual([
    { name: "DNS", bytes: 90 },
    { name: "HTTP", bytes: 60 },
  ]);
  expect(topSources(ws)).toEqual([
    { ip: "10.0.0.2", bytes: 100, pacotes: 2 },
    { ip: "10.0.0.1", bytes: 50, pacotes: 1 },
  ]);
});

test("formatBytes", () => {
  expect(formatBytes(0)).toBe("0 B");
  expect(formatBytes(1500)).toBe("1,5 KB");
  expect(formatBytes(2_500_000)).toBe("2,5 MB");
});
