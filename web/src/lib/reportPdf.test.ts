import { expect, test } from "vitest";
import type { TargetReport } from "../api/targetReport";
import { buildReportPdf, niceMax, reportFileName } from "./reportPdf";

const from = Date.parse("2026-11-24T00:00:00Z");
const to = from + 24 * 3_600_000;
const step = 300;
const iso = (ms: number) => new Date(ms).toISOString();
const slots = Array.from({ length: (to - from) / (step * 1000) }, (_, i) => from + i * step * 1000);

export function fixture(over: Partial<TargetReport> = {}): TargetReport {
  return {
    alvo: { id: 3, nome: "Loja Ação", host: "127.0.0.1", portas_app: [8080, 8443], porta_banco: 5432 },
    desde: iso(from),
    ate: iso(to),
    passo_s: step,
    contagem_alertas: { verde: 1, amarelo: 2, vermelho: 1 },
    alertas: [
      { id: 1, regra: "latencia_http", servico: "web", nome: "Latência HTTP", recurso: "loja:8080", nivel: "amarelo", nivel_anterior: "verde", valor: 812.4, limite: 500, unidade: "ms", hora: iso(from + 3_600_000) },
      { id: 2, regra: "disponibilidade", servico: "web", nome: "Disponibilidade", recurso: "loja:8443", nivel: "vermelho", nivel_anterior: "amarelo", valor: 2, limite: 2, unidade: "", detalhe: "conexão recusada", hora: iso(from + 7_200_000) },
      { id: 3, regra: "cpu", servico: "sistema", nome: "CPU", recurso: "servidor", nivel: "amarelo", nivel_anterior: "verde", valor: 87, limite: 80, unidade: "%", hora: iso(from + 9_000_000) },
      { id: 4, regra: "disponibilidade", servico: "web", nome: "Disponibilidade", recurso: "loja:8443", nivel: "verde", nivel_anterior: "vermelho", valor: 0, limite: 2, unidade: "", hora: iso(from + 9_600_000) },
    ],
    disponibilidade: [
      { porta: 5432, tipo: "tcp", verificacoes: 2880, no_ar: 2880, percentual: 100, latencia_media_ms: 1.2 },
      { porta: 8080, tipo: "http", verificacoes: 2880, no_ar: 2880, percentual: 100, latencia_media_ms: 42.5 },
      { porta: 8443, tipo: "http", verificacoes: 2880, no_ar: 2850, percentual: 98.96, latencia_media_ms: 61 },
    ],
    verificacoes: slots.flatMap((s, i) => [
      { alvo_id: 3, porta: 8080, tipo: "http", no_ar: true, latencia_ms: 40 + 20 * Math.sin(i / 10), hora: iso(s) },
      { alvo_id: 3, porta: 8443, tipo: "http", no_ar: i < 24 || i > 32, latencia_ms: 60 + 30 * Math.cos(i / 7), hora: iso(s) },
    ]),
    trafego: slots.map((s, i) => ({ inicio: iso(s), bytes: 4e6 + 3e6 * Math.sin(i / 15), pacotes: 3000, descartados: 0, protocolos: { HTTP: 2e6 }, top_origens: [] })),
    http: slots.map((s, i) => ({ inicio: iso(s), requisicoes: 600 + 200 * Math.sin(i / 20), status: { "2xx": 580, "5xx": i % 40 === 0 ? 20 : 0 }, p50_ms: 30, p95_ms: 120 + 40 * Math.sin(i / 9) })),
    banco: slots.filter((_, i) => i < 120 || i > 160).map((s, i) => ({ inicio: iso(s), consultas: 300, erros: i % 50 === 0 ? 4 : 0, tipos: { SELECT: 300 }, p50_ms: 2, p95_ms: 9 })),
    sistema: slots.map((s, i) => ({ hora: iso(s), cpu: 30 + 25 * Math.sin(i / 12), memoria: 61, disco: 44 })),
    varreduras: [
      {
        id: 12,
        alvo_id: 3,
        ferramentas: ["nmap", "nuclei"],
        status: "concluida",
        criada_em: iso(from + 4 * 3_600_000),
        achados: { alta: 1, info: 2 },
        lista_achados: [
          { ferramenta: "nmap", titulo: "Porta 5432 aberta (postgresql)", evidencia: "PostgreSQL 16.2", severidade: "info", porta: 5432, assinatura: "a" },
          { ferramenta: "nuclei", titulo: "Painel de administração exposto", evidencia: "GET /admin -> 200", severidade: "alta", porta: 8080, assinatura: "b" },
          { ferramenta: "nmap", titulo: "Porta 8080 aberta (http)", evidencia: "", severidade: "info", porta: 8080, assinatura: "c" },
        ],
      },
      { id: 13, alvo_id: 3, ferramentas: ["sqlmap"], status: "falhou", erro: "sqlmap: código 1", criada_em: iso(from + 5 * 3_600_000), achados: {}, lista_achados: [] },
    ],
    ...over,
  };
}

// Sem compressão, o texto fica legível no conteúdo do PDF.
const text = (rep: TargetReport) => {
  const doc = buildReportPdf(rep, new Date(to));
  return { doc, raw: doc.output() };
};

test("the PDF carries the target, alerts, uptime, scans and charts", () => {
  const { doc, raw } = text(fixture());
  expect(doc.getNumberOfPages()).toBeGreaterThan(1);
  for (const s of ["Loja A", "127.0.0.1", "Alertas", "Disponibilidade", "Arsenal", "Painel de administra", "Varredura #13", "Tr", "Sistema", "98,96%"]) {
    expect(raw).toContain(s);
  }
});

test("an empty interval still produces a readable report", () => {
  const { raw } = text(
    fixture({
      contagem_alertas: { verde: 0, amarelo: 0, vermelho: 0 },
      alertas: [],
      disponibilidade: [],
      verificacoes: [],
      trafego: [],
      http: [],
      banco: [],
      sistema: [],
      varreduras: [],
    }),
  );
  expect(raw).toContain("Nenhuma varredura deste alvo no intervalo.");
  expect(raw).toContain("Sem dados no intervalo.");
});

test("null lists from the backend are treated as empty", () => {
  const rep = fixture();
  const nulled = { ...rep, alertas: null, verificacoes: null, varreduras: null } as unknown as TargetReport;
  expect(() => buildReportPdf(nulled)).not.toThrow();
});

test("file name drops accents and spaces", () => {
  expect(reportFileName(fixture())).toBe("omni-relatorio-loja-acao-2026-11-25.pdf");
});

test("niceMax rounds the axis top up", () => {
  expect(niceMax(0)).toBe(1);
  expect(niceMax(7)).toBe(10);
  expect(niceMax(180)).toBe(200);
  expect(niceMax(2300)).toBe(2500);
});
