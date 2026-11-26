import { render, screen, within } from "@testing-library/react";
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

type Traffic = { dns?: unknown[]; smtp?: unknown[]; fila?: unknown };

function renderPage(kind: "dns" | "smtp", t: Target = target, traffic: Traffic = {}) {
  const calls = mockFetch((c) => {
    if (c.url.startsWith(`/api/alvos/${t.id}/historico`)) return json(200, history);
    if (c.url.startsWith("/api/dns")) return json(200, traffic.dns ?? []);
    if (c.url.startsWith("/api/smtp/fila")) return json(200, traffic.fila ?? { configurado: false, contagens: [] });
    if (c.url.startsWith("/api/smtp")) return json(200, traffic.smtp ?? []);
    return json(500, { erro: "inesperado" });
  });
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

const ago = (seconds: number) => new Date(Date.now() - seconds * 1000).toISOString();

// trafficValue lê o número de um Stat dentro da seção "Tráfego real".
const trafficValue = async (label: string) => {
  const section = await screen.findByRole("region", { name: "Tráfego real" });
  return within(section).getByText(label, { selector: "p" }).nextElementSibling?.textContent;
};

test("shows queries per second, failure rate and rcodes from traffic", async () => {
  renderPage("dns", target, {
    dns: [
      { inicio: ago(25), consultas: 40, falhas: 2, rcodes: { NOERROR: 38, NXDOMAIN: 2 }, p50_ms: 1, p95_ms: 4 },
      { inicio: ago(15), consultas: 60, falhas: 3, rcodes: { NOERROR: 57, SERVFAIL: 1, TIMEOUT: 2 }, p50_ms: 1, p95_ms: 12.5 },
    ],
  });
  expect(await trafficValue("Consultas por segundo")).toBe("6");
  expect(await trafficValue("Falhas em 15 minutos")).toBe("5%");
  expect(await trafficValue("Resolução p95 (10 s)")).toBe("13 ms");
  const rows = within(screen.getByRole("table", { name: "Respostas DNS" })).getAllByRole("row").slice(1);
  expect(rows.map((r) => r.textContent)).toEqual(["NOERROR95", "NXDOMAIN2", "TIMEOUT2", "SERVFAIL1"]);
});

test("shows delivery rate, send latency, volume per minute and per hour, and queue length", async () => {
  const smtp = [
    { inicio: ago(3000), tentativas: 100, entregues: 100, rejeitados: 0, abortados: 0, codigos: {}, p50_ms: 30, p95_ms: 50 },
    { inicio: ago(55), tentativas: 4, entregues: 3, rejeitados: 1, abortados: 0, codigos: { "550": 1 }, p50_ms: 30, p95_ms: 50 },
    { inicio: ago(15), tentativas: 6, entregues: 5, rejeitados: 0, abortados: 1, codigos: {}, p50_ms: 35, p95_ms: 80 },
  ];
  renderPage("smtp", target, { smtp, fila: { configurado: true, contagens: [{ hora: ago(20), mensagens: 7 }] } });
  expect(await trafficValue("Taxa de entrega")).toBe("98%"); // 108 de 110 na última hora
  expect(await trafficValue("Latência de envio p95 (10 s)")).toBe("80 ms");
  expect(await trafficValue("E-mails no último minuto")).toBe("10");
  expect(await trafficValue("E-mails na última hora")).toBe("110");
  expect(await trafficValue("Fila de saída")).toBe("7");
  const rows = within(screen.getByRole("table", { name: "Códigos de recusa" })).getAllByRole("row").slice(1);
  expect(rows.map((r) => r.textContent)).toEqual(["5501"]);
});

test("SMTP queue shows when it is not configured", async () => {
  renderPage("smtp");
  expect(await trafficValue("Fila de saída")).toBe("não configurada");
  expect(await trafficValue("Taxa de entrega")).toBe("—");
});
