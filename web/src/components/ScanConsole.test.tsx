import { act, render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { handleEvent } from "../realtime/events";
import { MAX_LOG_LINES, useAppStore } from "../store/app";
import ScanConsole from "./ScanConsole";

const log = (scanId: number, linhas: string[]) => handleEvent(JSON.stringify({ tipo: "SCAN_LOG", dados: { scan_id: scanId, ferramenta: "nmap", linhas } }));

test("appends SCAN_LOG lines of its scan only", () => {
  render(<ScanConsole scanId={7} />);
  expect(screen.getByRole("log")).toHaveTextContent("Aguardando a saída da ferramenta...");
  act(() => {
    log(7, ["Starting Nmap 7.95", "PORT STATE SERVICE"]);
    log(8, ["outra varredura"]);
    log(7, ["3000/tcp open http"]);
  });
  expect(screen.getByRole("log").textContent).toBe("Starting Nmap 7.95\nPORT STATE SERVICE\n3000/tcp open http");
});

test("keeps at most 5000 lines", () => {
  act(() => {
    log(1, Array.from({ length: MAX_LOG_LINES - 10 }, (_, i) => `a${i}`));
    log(1, Array.from({ length: 20 }, (_, i) => `b${i}`));
  });
  const lines = useAppStore.getState().scanLogs[1]!;
  expect(lines).toHaveLength(MAX_LOG_LINES);
  expect(lines[0]).toBe("a10");
  expect(lines.at(-1)).toBe("b19");
});

test("shows a placeholder without a scan", () => {
  render(<ScanConsole scanId={null} />);
  expect(screen.getByRole("log")).toHaveTextContent("Nenhuma varredura iniciada nesta sessão.");
});
