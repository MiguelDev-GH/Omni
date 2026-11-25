import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { useAppStore } from "../store/app";
import { json, mockFetch } from "../test/fetch";
import ReportPdfForm, { rangeError } from "./ReportPdfForm";

const save = vi.fn();
vi.mock("../lib/reportPdf", () => ({
  buildReportPdf: vi.fn(() => ({ save })),
  reportFileName: vi.fn(() => "omni-relatorio-api.pdf"),
}));

const target = { id: 7, nome: "api", host: "127.0.0.1", portas_app: [3000] };
const report = { alvo: target, desde: "2026-11-24T00:00:00Z", ate: "2026-11-25T00:00:00Z", passo_s: 300 };

beforeEach(() => {
  useAppStore.setState({ token: "t" });
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date("2026-11-25T12:00:00Z"));
});

afterEach(() => {
  vi.useRealTimers();
  save.mockClear();
});

test("the week preset asks the backend for the last 168 hours and saves the PDF", async () => {
  const calls = mockFetch(() => json(200, report));
  render(<ReportPdfForm target={target} />);
  await userEvent.click(screen.getByRole("radio", { name: "Última semana" }));
  await userEvent.click(screen.getByRole("button", { name: "Baixar PDF" }));
  await waitFor(() => expect(save).toHaveBeenCalledWith("omni-relatorio-api.pdf"));
  const url = new URL(calls.at(0)?.url ?? "", "http://x");
  expect(url.pathname).toBe("/api/alvos/7/relatorio");
  expect(url.searchParams.get("ate")).toBe("2026-11-25T12:00:00Z");
  expect(url.searchParams.get("desde")).toBe("2026-11-18T12:00:00Z");
});

test("a custom interval longer than 7 days disables the button", async () => {
  mockFetch(() => json(200, report));
  render(<ReportPdfForm target={target} />);
  await userEvent.click(screen.getByRole("radio", { name: "Personalizado" }));
  const from = screen.getByLabelText("De");
  await userEvent.clear(from);
  await userEvent.type(from, "2026-11-01T00:00");
  expect(screen.getByRole("alert")).toHaveTextContent("no máximo 168 horas");
  expect(screen.getByRole("button", { name: "Baixar PDF" })).toBeDisabled();
});

test("a backend error is shown and nothing is saved", async () => {
  mockFetch(() => json(400, { erro: "intervalo maior que 168h0m0s" }));
  render(<ReportPdfForm target={target} />);
  await userEvent.click(screen.getByRole("button", { name: "Baixar PDF" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("intervalo maior que 168h0m0s");
  expect(save).not.toHaveBeenCalled();
});

test("rangeError checks order and the 168 h limit", () => {
  const d = (s: string) => new Date(s);
  expect(rangeError(d("2026-11-20T00:00Z"), d("2026-11-27T00:00Z"))).toBeNull();
  expect(rangeError(d("2026-11-20T00:00Z"), d("2026-11-27T00:01Z"))).toMatch(/168/);
  expect(rangeError(d("2026-11-21T00:00Z"), d("2026-11-20T00:00Z"))).toMatch(/antes/);
  expect(rangeError(d("x"), d("2026-11-20T00:00Z"))).toMatch(/Preencha/);
});
