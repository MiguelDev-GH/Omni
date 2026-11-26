import { expect, test } from "vitest";
import { connSnapshot } from "../test/connections";
import { connectionSeries, latestPools, mergeConnections, MAX_CONN_POINTS } from "./connections";

const at = (s: number) => new Date(Date.parse("2026-11-25T12:00:00Z") + s * 1000).toISOString();

test("connectionSeries sums the established connections of one kind", () => {
  const two = connSnapshot(at(0), 3);
  two.portas.push({ porta: 3001, tipo: "app", estabelecidas: 2, estados: {}, clientes: [] });
  expect(connectionSeries([two, connSnapshot(at(5), 7)], "app")).toEqual([
    { t: Date.parse(at(0)), n: 5 },
    { t: Date.parse(at(5)), n: 7 },
  ]);
  expect(connectionSeries([two], "banco")).toEqual([{ t: Date.parse(at(0)), n: 5 }]);
});

test("mergeConnections dedupes by time, keeps order and caps the window", () => {
  const merged = mergeConnections([connSnapshot(at(5), 1)], [connSnapshot(at(0), 0), connSnapshot(at(5), 9)]);
  expect(merged.map((s) => s.portas[0]?.estabelecidas)).toEqual([0, 9]);
  const many = Array.from({ length: MAX_CONN_POINTS + 5 }, (_, i) => connSnapshot(at(i * 5), i));
  const capped = mergeConnections([], many);
  expect(capped).toHaveLength(MAX_CONN_POINTS);
  expect(capped[0]?.portas[0]?.estabelecidas).toBe(5);
});

test("latestPools lists the clients of the database ports in the newest snapshot", () => {
  expect(latestPools([])).toEqual([]);
  expect(latestPools([connSnapshot(at(0), 1)])).toEqual([
    { porta: 5432, processo: "loja", conexoes: 4 },
    { porta: 5432, processo: "", conexoes: 1 },
  ]);
});
