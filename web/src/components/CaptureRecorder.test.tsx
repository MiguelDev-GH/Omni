import { expect, test } from "vitest";
import { describeFilter } from "../api/captures";
import { formatBytes } from "./CaptureRecorder";

test("formatBytes picks the unit", () => {
  expect(formatBytes(24)).toBe("24 B");
  expect(formatBytes(1536)).toBe("1,5 KiB");
  expect(formatBytes(100 * 1024 * 1024)).toBe("100 MiB");
});

test("describeFilter names each part of the filter", () => {
  expect(describeFilter({ ip: "", porta: 0, protocolo: "" })).toBe("todo o tráfego");
  expect(describeFilter({ ip: "10.0.0.1", porta: 3000, protocolo: "HTTP" })).toBe("10.0.0.1 · porta 3000 · HTTP");
});
