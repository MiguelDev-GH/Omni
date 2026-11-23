import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { expect, test } from "vitest";
import { useAppStore, type SystemMetrics } from "../store/app";
import { json, mockFetch } from "../test/fetch";
import SystemPage from "./SystemPage";

const at = (minutesAgo: number) => new Date(Date.now() - minutesAgo * 60_000).toISOString();

const history: SystemMetrics[] = [
  { cpu: 10, memoria: 40, disco: 60, hora: at(2) },
  { cpu: 20, memoria: 41, disco: 60, hora: at(1) },
];

function renderPage(list: SystemMetrics[] = history) {
  const calls = mockFetch((c) => (c.url.startsWith("/api/sistema") ? json(200, list) : json(500, { erro: "inesperado" })));
  useAppStore.setState({ token: "t" });
  const router = createMemoryRouter([{ path: "/", element: <SystemPage /> }]);
  render(<RouterProvider router={router} />);
  return calls;
}

// sinceOf devolve quantos minutos antes de agora vale o desde da chamada.
const sinceOf = (url: string) => {
  const desde = new URL(url, "http://omni").searchParams.get("desde")!;
  return Math.round((Date.now() - Date.parse(desde)) / 60_000);
};

test("loads the history and appends live METRICAS_SISTEMA points", async () => {
  const calls = renderPage();
  expect(await screen.findByRole("img", { name: "CPU em percentual, 2 pontos" })).toBeInTheDocument();
  expect(sinceOf(calls[0]!.url)).toBe(15);
  expect(screen.getByText("CPU agora").nextElementSibling).toHaveTextContent("20%");

  act(() => useAppStore.getState().setSystem({ cpu: 91.5, memoria: 42, disco: 61, hora: at(0) }));
  expect(screen.getByRole("img", { name: "CPU em percentual, 3 pontos" })).toBeInTheDocument();
  expect(screen.getByRole("img", { name: "Disco em percentual, 3 pontos" })).toBeInTheDocument();
  const cpu = screen.getByText("CPU agora").nextElementSibling;
  expect(cpu).toHaveTextContent("91,5%");
  expect(cpu).toHaveClass("text-aviso");
});

test("changing the period refetches with the matching desde", async () => {
  const calls = renderPage();
  await screen.findByRole("img", { name: "CPU em percentual, 2 pontos" });

  await userEvent.click(screen.getByRole("combobox", { name: "Período" }));
  await userEvent.click(screen.getByRole("option", { name: "Últimas 24 horas" }));
  expect(calls).toHaveLength(2);
  expect(sinceOf(calls[1]!.url)).toBe(24 * 60);

  await userEvent.click(screen.getByRole("combobox", { name: "Período" }));
  await userEvent.click(screen.getByRole("option", { name: "Última hora" }));
  expect(calls).toHaveLength(3);
  expect(sinceOf(calls[2]!.url)).toBe(60);
});

test("shows an empty state without samples and the engine states", async () => {
  renderPage([]);
  useAppStore.getState().setEngine("docker", "ativo");
  expect(await screen.findAllByText("Sem medições ainda.")).toHaveLength(3);
  expect(screen.getByText("CPU agora").nextElementSibling).toHaveTextContent("—");
  expect(screen.getByText("Docker: ativo")).toBeInTheDocument();
  expect(screen.getByText("Sniffer: inativo")).toBeInTheDocument();
  expect(screen.getByText("Poller: indisponível")).toBeInTheDocument();
});
