import { expect, test } from "vitest";
import { formatMs, l7Series, L7_POINTS, mergeL7 } from "./l7";

const t0 = Date.parse("2026-11-21T12:00:00Z");
const w = (i: number, n: number) => ({ inicio: new Date(t0 + i * 10_000).toISOString(), n });

test("mergeL7 dedupes by 10 s slot and caps the window", () => {
  expect(mergeL7([w(1, 1), w(0, 0)], [w(1, 9)]).map((x) => x.n)).toEqual([0, 9]);
  const many = Array.from({ length: L7_POINTS + 10 }, (_, i) => w(i, i));
  const capped = mergeL7([], many);
  expect(capped).toHaveLength(L7_POINTS);
  expect(capped[0]?.n).toBe(10);
});

test("l7Series fills gaps with the empty value", () => {
  const s = l7Series([w(0, 4), w(3, 7)], (x) => ({ v: x.n }), { v: 0 });
  expect(s.map((p) => p.v)).toEqual([4, 0, 0, 7]);
  expect(s[1]?.t).toBe(t0 + 10_000);
  expect(l7Series([], (x: { inicio: string }) => x, { inicio: "" })).toEqual([]);
});

test("formatMs", () => {
  expect(formatMs(1.25)).toBe("1,3 ms");
  expect(formatMs(120)).toBe("120 ms");
  expect(formatMs(1500)).toBe("1,5 s");
});
