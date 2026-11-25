import { render, screen } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { expect, test } from "vitest";
import type { Target } from "../api/targets";
import { useAppStore, type PortStatus } from "../store/app";
import { json, mockFetch } from "../test/fetch";
import ProbeServicePage from "./ProbeServicePage";

const target: Target = { id: 7, nome: "loja", host: "127.0.0.1", portas_app: [3000], dominio: "loja.local", porta_dns: 1053, porta_smtp: 1025 };

const at = (minutesAgo: number) => new Date(Date.now() - minutesAgo * 60_000).toISOString();

const history: PortStatus[] = [
  { alvo_id: 7, porta: 3000, tipo: "http", no_ar: true, latencia_ms: 1, hora: at(3) },
  { alvo_id: 7, porta: 1053, tipo: "dns", no_ar: true, latencia_ms: 10, hora: at(3) },
  { alvo_id: 7, porta: 1053, tipo: "dns", no_ar: false, latencia_ms: 0, erro: "timeout", hora: at(2) },
  { alvo_id: 7, porta: 1053, tipo: "dns", no_ar: true, degradado: true, detalhe: "NXDOMAIN", latencia_ms: 30, hora: at(1) },
  { alvo_id: 7, porta: 1025, tipo: "smtp", no_ar: true, starttls: false, detalhe: "aceita destinatário postmaster", latencia_ms: 4, hora: at(1) },
];

// statValue lê o número mostrado logo abaixo do rótulo de um Stat.
const statValue = (label: string) => screen.getByText(label, { selector: "p" }).nextElementSibling?.textContent;

function renderPage(kind: "dns" | "smtp", t: Target = target) {
  const calls = mockFetch((c) => (c.url.startsWith(`/api/alvos/${t.id}/historico`) ? json(200, history) : json(500, { erro: "inesperado" })));
  useAppStore.setState({ token: "t", targets: [t], targetsLoaded: true, selectedTargetId: t.id, targetStatus: {} });
  const router = createMemoryRouter([
    { path: "/", element: <ProbeServicePage kind={kind} /> },
    { path: "/alvos", element: <p>alvos</p> },
  ]);
  render(<RouterProvider router={router} />);
  return calls;
}

test("shows availability, latency and the last detail for the selected target's DNS port", async () => {
  renderPage("dns");
  await screen.findByText("NXDOMAIN", { selector: "td" });
  expect(statValue("Disponibilidade")).toBe("67%"); // 2 de 3 responderam
  expect(statValue("Taxa de falhas")).toBe("67%"); // uma fora do ar e uma com erro
  expect(statValue("Latência média")).toBe("20 ms"); // média de 10 e 30
  expect(screen.getAllByText("NXDOMAIN").length).toBeGreaterThan(0);
  expect(screen.getByRole("heading", { name: "DNS" })).toBeInTheDocument();
});

test("SMTP shows the STARTTLS state", async () => {
  renderPage("smtp");
  const badge = await screen.findByText("Sem STARTTLS");
  expect(badge).toHaveClass("text-aviso");
});

test("without a DNS port on the target, shows an empty state linking to /alvos", async () => {
  renderPage("dns", { ...target, porta_dns: undefined });
  expect(await screen.findByText(/não tem porta DNS/)).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /Cadastre/ })).toHaveAttribute("href", "/alvos");
});
