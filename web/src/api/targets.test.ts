import { expect, test } from "vitest";
import { parsePorts } from "./targets";

test("parsePorts", () => {
  expect(parsePorts("3000, 8080")).toEqual([3000, 8080]);
  expect(parsePorts("3000 8080")).toEqual([3000, 8080]);
  expect(parsePorts("3000,,8080")).toEqual([3000, 8080]);
  expect(parsePorts(" ")).toEqual([]);
  expect(parsePorts("abc")).toBeNull();
  expect(parsePorts("30.5")).toBeNull();
  expect(parsePorts("3000x")).toBeNull();
  expect(parsePorts("70000")).toEqual([70000]);
});
