import { jsPDF } from "jspdf";
import { autoTable, type RowInput } from "jspdf-autotable";
import { formatAlertValue, levelLabel, type Level } from "../api/alerts";
import { severities, severityLabels, sortFindings, type Severity } from "../api/scans";
import { targetPorts } from "../api/targets";
import type { CheckPoint, ReportScan, TargetReport } from "../api/targetReport";
import { statusLabels } from "../arsenal/common";
import { formatMs } from "./l7";
import { formatBytes } from "./traffic";

// O PDF é desenhado direto com jsPDF: texto, tabelas e gráficos vetoriais,
// sem capturar a tela. Assim o resultado não depende do tema nem do tamanho
// da janela, e o texto continua selecionável.

type RGB = [number, number, number];

const ink: RGB = [22, 33, 30];
const soft: RGB = [93, 106, 102];
const faint: RGB = [147, 160, 155];
const grid: RGB = [221, 227, 224];
const accent: RGB = [11, 138, 118];
const tint: RGB = [224, 241, 237];
const danger: RGB = [194, 53, 43];

// Paleta categórica das séries, na mesma ordem para todos os gráficos.
const palette: [RGB, RGB, RGB, RGB, RGB, RGB] = [
  [11, 138, 118],
  [217, 119, 6],
  [124, 58, 237],
  [37, 99, 235],
  [219, 39, 119],
  [101, 163, 13],
];

const levelColors: Record<Level, RGB> = { verde: [22, 128, 61], amarelo: [180, 120, 4], vermelho: danger };

const severityColors: Record<Severity, { text: RGB; fill: RGB }> = {
  critica: { text: [153, 27, 27], fill: [254, 226, 226] },
  alta: { text: [154, 52, 18], fill: [255, 237, 213] },
  media: { text: [146, 64, 14], fill: [254, 243, 199] },
  baixa: { text: [7, 89, 133], fill: [224, 242, 254] },
  info: { text: [93, 106, 102], fill: [238, 241, 240] },
};

const PAGE_W = 210;
const PAGE_H = 297;
const MARGIN = 16;
const CONTENT_W = PAGE_W - 2 * MARGIN;
const BOTTOM = PAGE_H - 18;

const dateTime = (d: Date | string) =>
  new Date(d).toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" });

const num = (n: number, digits = 0) => n.toLocaleString("pt-BR", { maximumFractionDigits: digits });

function durationText(ms: number): string {
  const h = ms / 3_600_000;
  if (h >= 24 && Number.isInteger(h / 24)) return h === 24 ? "1 dia" : `${h / 24} dias`;
  return `${num(h, 1)} h`;
}

// reportFileName monta "omni-relatorio-<alvo>-<data>.pdf", sem acentos nem espaços.
export function reportFileName(rep: TargetReport): string {
  const slug =
    rep.alvo.nome
      .normalize("NFD")
      .replace(/[̀-ͯ]/g, "")
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-|-$/g, "") || `alvo-${rep.alvo.id}`;
  return `omni-relatorio-${slug}-${rep.ate.slice(0, 10)}.pdf`;
}

type Point = { t: number; v: number };
type Series = { name: string; color: RGB; points: Point[] };

type ChartOptions = {
  title: string;
  subtitle?: string;
  series: Series[];
  from: number;
  to: number;
  stepMs: number;
  format: (v: number) => string;
  yMax?: number;
  // marks são instantes destacados em vermelho abaixo do gráfico (quedas).
  marks?: number[];
  marksLabel?: string;
};

// Writer guarda o cursor vertical e quebra a página quando falta espaço.
class Writer {
  doc: jsPDF;
  y = MARGIN;

  constructor(doc: jsPDF) {
    this.doc = doc;
  }

  ensure(h: number) {
    if (this.y + h > BOTTOM) {
      this.doc.addPage();
      this.y = MARGIN;
    }
  }

  color(c: RGB) {
    this.doc.setTextColor(c[0], c[1], c[2]);
  }

  text(s: string, opts: { size?: number; bold?: boolean; color?: RGB; gap?: number; x?: number } = {}) {
    const size = opts.size ?? 10;
    const lines = this.doc.setFont("helvetica", opts.bold ? "bold" : "normal").setFontSize(size).splitTextToSize(s, CONTENT_W - ((opts.x ?? MARGIN) - MARGIN)) as string[];
    const lineH = size * 0.42;
    this.ensure(lines.length * lineH);
    this.color(opts.color ?? ink);
    for (const l of lines) {
      this.y += lineH;
      this.doc.text(l, opts.x ?? MARGIN, this.y - lineH * 0.2);
    }
    this.y += opts.gap ?? 1.5;
  }

  section(title: string) {
    this.ensure(34);
    this.y += 6;
    this.doc.setFillColor(...accent).rect(MARGIN, this.y, 2.5, 6, "F");
    this.doc.setFont("helvetica", "bold").setFontSize(14);
    this.color(ink);
    this.doc.text(title, MARGIN + 5, this.y + 4.8);
    this.y += 10;
  }

  table(head: string[], body: RowInput[], opts: { columnStyles?: Record<number, object> } = {}) {
    autoTable(this.doc, {
      startY: this.y,
      head: [head],
      body,
      margin: { left: MARGIN, right: MARGIN, bottom: PAGE_H - BOTTOM },
      theme: "plain",
      styles: { font: "helvetica", fontSize: 8, textColor: ink, cellPadding: { top: 1.6, bottom: 1.6, left: 1.5, right: 1.5 }, overflow: "linebreak" },
      headStyles: { textColor: soft, fontStyle: "bold", lineWidth: { bottom: 0.3 }, lineColor: grid },
      bodyStyles: { lineWidth: { bottom: 0.1 }, lineColor: grid },
      columnStyles: opts.columnStyles,
    });
    this.y = (this.doc as unknown as { lastAutoTable: { finalY: number } }).lastAutoTable.finalY + 4;
  }

  chart(o: ChartOptions) {
    const doc = this.doc;
    const h = 46;
    const hasMarks = (o.marks?.length ?? 0) > 0;
    const legendH = o.series.length > 1 || hasMarks ? 5 : 0;
    const marksH = hasMarks ? 4 : 0;
    this.ensure(10 + h + 8 + legendH + marksH);

    doc.setFont("helvetica", "bold").setFontSize(10);
    this.color(ink);
    doc.text(o.title, MARGIN, this.y + 4);
    if (o.subtitle) {
      doc.setFont("helvetica", "normal").setFontSize(8);
      this.color(soft);
      doc.text(o.subtitle, PAGE_W - MARGIN, this.y + 4, { align: "right" });
    }
    this.y += 8;

    const axisW = 18;
    const x0 = MARGIN + axisW;
    const w = CONTENT_W - axisW;
    const y0 = this.y;
    const all = o.series.flatMap((s) => s.points.map((p) => p.v));
    const top = o.yMax ?? niceMax(all.length > 0 ? Math.max(...all) : 0);
    const span = Math.max(o.to - o.from, 1);
    const px = (t: number) => x0 + ((t - o.from) / span) * w;
    const py = (v: number) => y0 + h - (Math.min(v, top) / top) * h;

    // Grade horizontal com os valores à esquerda.
    doc.setFont("helvetica", "normal").setFontSize(7);
    doc.setLineWidth(0.15).setDrawColor(...grid);
    for (let i = 0; i <= 4; i++) {
      const v = (top * i) / 4;
      const y = py(v);
      doc.setLineDashPattern(i === 0 ? [] : [0.6, 0.8], 0).line(x0, y, x0 + w, y);
      this.color(faint);
      doc.text(o.format(v), x0 - 2, y + 1, { align: "right" });
    }
    doc.setLineDashPattern([], 0);

    // Horas no eixo de baixo.
    const longSpan = span > 36 * 3_600_000;
    for (let i = 0; i <= 5; i++) {
      const t = o.from + (span * i) / 5;
      const d = new Date(t);
      const label = longSpan
        ? d.toLocaleString("pt-BR", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" })
        : d.toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
      doc.text(label, px(t), y0 + h + 4, { align: i === 0 ? "left" : i === 5 ? "right" : "center" });
    }

    if (all.length === 0) {
      this.color(faint);
      doc.setFontSize(9).text("Sem dados no intervalo.", x0 + w / 2, y0 + h / 2, { align: "center" });
    }

    // Linhas: um buraco maior que 1,5 faixa quebra a linha em vez de ligar os pontos.
    doc.setLineWidth(0.45);
    for (const s of o.series) {
      doc.setDrawColor(...s.color);
      const pts = [...s.points].sort((a, b) => a.t - b.t);
      const near = (a?: Point, b?: Point) => a !== undefined && b !== undefined && b.t - a.t <= o.stepMs * 1.5;
      pts.forEach((p, i) => {
        const prev = pts[i - 1];
        if (prev && near(prev, p)) doc.line(px(prev.t), py(prev.v), px(p.t), py(p.v));
        else if (!near(p, pts[i + 1])) doc.setFillColor(...s.color).circle(px(p.t), py(p.v), 0.5, "F");
      });
    }
    this.y = y0 + h + 6;

    // Quedas: um traço vermelho por faixa, numa régua logo abaixo das horas.
    if (hasMarks && o.marks) {
      doc.setLineWidth(0.4).setDrawColor(...danger);
      for (const t of o.marks) doc.line(px(t), this.y, px(t), this.y + 2.2);
      this.y += marksH;
    }
    this.y += legendH > 0 ? 3 : 0;

    if (legendH > 0) {
      let lx = x0;
      doc.setFontSize(7.5);
      for (const s of o.series) {
        doc.setFillColor(...s.color).rect(lx, this.y - 2, 3, 1.4, "F");
        this.color(soft);
        doc.text(s.name, lx + 4.5, this.y - 0.8);
        lx += 4.5 + doc.getTextWidth(s.name) + 6;
      }
      if (hasMarks) {
        doc.setFillColor(...danger).rect(lx + 1, this.y - 2.8, 0.6, 2.8, "F");
        this.color(danger);
        doc.text(o.marksLabel ?? "", lx + 3, this.y - 0.8);
      }
      this.y += legendH;
    }
    this.y += 3;
  }
}

// niceMax arredonda o topo do eixo para 1, 2, 2,5 ou 5 vezes uma potência de 10.
export function niceMax(v: number): number {
  if (!(v > 0)) return 1;
  const p = 10 ** Math.floor(Math.log10(v));
  for (const m of [1, 2, 2.5, 5, 10]) if (v <= m * p) return m * p;
  return 10 * p;
}

const t = (iso: string) => new Date(iso).getTime();

// sumCounts soma os mapas de contagem das janelas.
function sumCounts(maps: (Record<string, number> | undefined)[]): Record<string, number> {
  const out: Record<string, number> = {};
  for (const m of maps) for (const [k, v] of Object.entries(m ?? {})) out[k] = (out[k] ?? 0) + v;
  return out;
}

// topCounts lista as limit maiores contagens, como "404: 12, 500: 3".
function topCounts(m: Record<string, number>, limit = 5): string {
  return Object.entries(m)
    .filter(([, v]) => v > 0)
    .sort(([ka, a], [kb, b]) => b - a || ka.localeCompare(kb))
    .slice(0, limit)
    .map(([k, v]) => `${k}: ${num(v)}`)
    .join(", ");
}

// bySeries agrupa os pontos pelo nome da série, uma cor por série.
function bySeries(points: { name: string; t: number; v: number }[]): Series[] {
  const groups = new Map<string, Point[]>();
  for (const p of points) groups.set(p.name, [...(groups.get(p.name) ?? []), { t: p.t, v: p.v }]);
  return [...groups.entries()].map(([name, pts], i) => ({ name, color: palette[i % palette.length] ?? accent, points: pts }));
}

// isLoopback diz se o host aponta para a própria máquina do Omni.
export function isLoopback(host: string): boolean {
  const h = host.trim().toLowerCase().replace(/^\[|\]$/g, "");
  return h === "localhost" || h === "::1" || /^127\.\d+\.\d+\.\d+$/.test(h);
}

function header(w: Writer, rep: TargetReport, generatedAt: Date) {
  const doc = w.doc;
  const from = new Date(rep.desde);
  const to = new Date(rep.ate);
  const ports = targetPorts(rep.alvo);
  // Com 127.0.0.1 o host é igual em todo relatório; quem identifica a
  // máquina é o nome do servidor.
  const meta: [string, string][] = [
    ["Host", isLoopback(rep.alvo.host) ? `${rep.alvo.host} (esta máquina)` : rep.alvo.host],
    ["Portas", ports.length > 0 ? ports.join(", ") : "-"],
    ["Intervalo", `${dateTime(from)} até ${dateTime(to)} (${durationText(to.getTime() - from.getTime())})`],
    ["Gerado em", dateTime(generatedAt)],
  ];
  if (rep.alvo.dominio) meta.unshift(["Domínio", rep.alvo.dominio]);
  if (rep.servidor) meta.unshift(["Servidor", rep.servidor]);

  doc.setFillColor(...tint).rect(0, 0, PAGE_W, Math.max(50, 33 + meta.length * 4), "F");
  doc.setFont("helvetica", "bold").setFontSize(9);
  w.color(accent);
  doc.text("OMNI", MARGIN, 14);
  doc.setFont("helvetica", "normal");
  w.color(soft);
  doc.text("Relatório do alvo", MARGIN + 11, 14);
  doc.setFont("helvetica", "bold").setFontSize(22);
  w.color(ink);
  doc.text(rep.alvo.nome, MARGIN, 26);

  doc.setFontSize(8.5);
  let y = 33;
  for (const [k, v] of meta) {
    doc.setFont("helvetica", "bold");
    w.color(soft);
    doc.text(k, MARGIN, y);
    doc.setFont("helvetica", "normal");
    w.color(ink);
    doc.text(v, MARGIN + 20, y);
    y += 4;
  }
  w.y = Math.max(56, y + 4);
  w.text(
    "Alertas, tráfego, HTTP, banco, DNS, SMTP, conexões e sistema refletem o servidor inteiro, não só este alvo. Disponibilidade e Arsenal são deste alvo. " +
      "Nos gráficos, cada ponto agrupa uma faixa de tempo; o p50 e o p95 agrupados são uma aproximação.",
    { size: 8, color: soft, gap: 2 },
  );
}

function alertsSection(w: Writer, rep: TargetReport) {
  w.section("Alertas");
  const doc = w.doc;
  const boxW = (CONTENT_W - 8) / 3;
  w.ensure(20);
  (["verde", "amarelo", "vermelho"] as Level[]).forEach((lvl, i) => {
    const x = MARGIN + i * (boxW + 4);
    const c = levelColors[lvl];
    doc.setFillColor(248, 250, 249).roundedRect(x, w.y, boxW, 16, 1.5, 1.5, "F");
    doc.setFillColor(...c).rect(x, w.y, 1.2, 16, "F");
    doc.setFont("helvetica", "bold").setFontSize(16);
    w.color(c);
    doc.text(num(rep.contagem_alertas?.[lvl] ?? 0), x + 5, w.y + 8.5);
    doc.setFont("helvetica", "normal").setFontSize(8);
    w.color(soft);
    doc.text(`mudanças para ${levelLabel[lvl].toLowerCase()}`, x + 5, w.y + 13);
  });
  w.y += 21;

  const summary = rep.resumo_alertas ?? [];
  const list = [...(rep.alertas ?? [])].sort((a, b) => t(a.hora) - t(b.hora));
  if (summary.length === 0 && list.length === 0) {
    w.text("Nenhuma mudança de nível no intervalo. Os alertas são gravados desde a versão que passou a guardá-los no banco.", {
      color: soft,
      size: 9,
    });
    return;
  }

  // Por regra: as contagens são exatas, mesmo quando um ataque com muitos
  // IPs gera mais mudanças do que a lista abaixo mostra.
  if (summary.length > 0) {
    w.text("Por regra", { bold: true, size: 10.5, gap: 1 });
    const countCell = (n: number, lvl: Level) =>
      n > 0 ? { content: num(n), styles: { textColor: levelColors[lvl], fontStyle: "bold" as const } } : { content: "-", styles: { textColor: faint } };
    w.table(
      ["Regra", "Recursos", "Para vermelho", "Para amarelo", "Para verde"],
      summary.map((r) => [
        r.nome || r.regra,
        num(r.recursos),
        countCell(r.mudancas.vermelho ?? 0, "vermelho"),
        countCell(r.mudancas.amarelo ?? 0, "amarelo"),
        countCell(r.mudancas.verde ?? 0, "verde"),
      ]),
      { columnStyles: { 1: { cellWidth: 20 }, 2: { cellWidth: 24 }, 3: { cellWidth: 24 }, 4: { cellWidth: 24 } } },
    );
  }

  if (list.length === 0) return;
  const total = (["verde", "amarelo", "vermelho"] as Level[]).reduce((n, l) => n + (rep.contagem_alertas?.[l] ?? 0), 0);
  w.text("Mudanças de nível", { bold: true, size: 10.5, gap: 0.5 });
  if (total > list.length) {
    w.text(`Mostrando as ${num(list.length)} mais graves de ${num(total)} mudanças, em ordem de hora.`, { size: 8.5, color: soft, gap: 1 });
  }
  w.table(
    ["Hora", "Regra", "Recurso", "Mudança", "Valor"],
    list.map((a) => [
      dateTime(a.hora),
      a.nome || a.regra,
      a.recurso,
      {
        content: `${levelLabel[a.nivel_anterior]} para ${levelLabel[a.nivel]}`,
        styles: { textColor: levelColors[a.nivel], fontStyle: "bold" },
      },
      formatAlertValue(a.regra, a.valor, a.unidade) + (a.detalhe ? ` - ${a.detalhe}` : ""),
    ]),
    { columnStyles: { 0: { cellWidth: 26 }, 3: { cellWidth: 34 } } },
  );
}

function availabilitySection(w: Writer, rep: TargetReport, from: number, to: number, stepMs: number) {
  w.section("Disponibilidade");
  const uptime = rep.disponibilidade ?? [];
  if (uptime.length === 0) {
    w.text("Nenhuma verificação deste alvo no intervalo.", { color: soft, size: 9 });
    return;
  }
  w.table(
    ["Porta", "Tipo", "Verificações", "No ar", "Disponibilidade", "Latência média"],
    uptime.map((u) => [
      String(u.porta),
      u.tipo.toUpperCase(),
      num(u.verificacoes),
      num(u.no_ar),
      { content: `${num(u.percentual, 2)}%`, styles: { textColor: u.percentual < 99 ? danger : ink, fontStyle: "bold" } },
      formatMs(u.latencia_media_ms),
    ]),
  );

  const checks = rep.verificacoes ?? [];
  const byPort = new Map<string, CheckPoint[]>();
  for (const c of checks) {
    const k = `${c.porta}/${c.tipo}`;
    byPort.set(k, [...(byPort.get(k) ?? []), c]);
  }
  const series = [...byPort.entries()].map(([name, list], i) => ({
    name,
    color: palette[i % palette.length] ?? accent,
    points: list.filter((c) => c.no_ar).map((c) => ({ t: t(c.hora), v: c.latencia_ms })),
  }));
  const down = checks.filter((c) => !c.no_ar).map((c) => t(c.hora));
  w.chart({
    title: "Latência das verificações",
    subtitle: "média por faixa, por porta",
    series,
    from,
    to,
    stepMs,
    format: formatMs,
    marks: down,
    marksLabel: `${down.length} faixa${down.length === 1 ? "" : "s"} com queda`,
  });
}

function serverCharts(w: Writer, rep: TargetReport, from: number, to: number, stepMs: number) {
  w.section("Gráficos do servidor");
  const step = Math.max(rep.passo_s, 1);
  const perMin = (n: number) => (n / step) * 60;
  const rate = (n: number) => `${num(n, n < 10 ? 1 : 0)}/min`;
  const pct = (n: number) => `${num(n)}%`;

  const traffic = rep.trafego ?? [];
  const totalBytes = traffic.reduce((s, x) => s + x.bytes, 0);
  w.chart({
    title: "Tráfego",
    subtitle: `banda média por faixa · total ${formatBytes(totalBytes)}`,
    series: [{ name: "Banda", color: palette[0], points: traffic.map((x) => ({ t: t(x.inicio), v: x.bytes / step })) }],
    from,
    to,
    stepMs,
    format: (v) => `${formatBytes(v)}/s`,
  });

  const http = rep.http ?? [];
  const status = sumCounts(http.map((x) => x.status));
  const codes = topCounts(sumCounts(http.map((x) => x.codigos)));
  const totalReq = http.reduce((s, x) => s + x.requisicoes, 0);
  const statusText = Object.entries(status)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([k, v]) => `${k} ${num(v)}`)
    .join(", ");
  w.chart({
    title: "Requisições HTTP",
    subtitle: `total ${num(totalReq)}${statusText ? ` (${statusText})` : ""}`,
    series: [
      { name: "Requisições", color: palette[0], points: http.map((x) => ({ t: t(x.inicio), v: perMin(x.requisicoes) })) },
      {
        name: "Erros 5xx",
        color: danger,
        points: http.map((x) => ({ t: t(x.inicio), v: perMin(x.status?.["5xx"] ?? 0) })),
      },
    ],
    from,
    to,
    stepMs,
    format: rate,
  });
  if (codes) w.text(`Códigos de erro mais frequentes: ${codes}.`, { size: 8, color: soft, gap: 3 });
  w.chart({
    title: "Latência HTTP",
    series: [
      { name: "p50", color: palette[0], points: http.filter((x) => x.requisicoes > 0).map((x) => ({ t: t(x.inicio), v: x.p50_ms })) },
      { name: "p95", color: palette[1], points: http.filter((x) => x.requisicoes > 0).map((x) => ({ t: t(x.inicio), v: x.p95_ms })) },
    ],
    from,
    to,
    stepMs,
    format: formatMs,
  });

  const db = rep.banco ?? [];
  const totalQ = db.reduce((s, x) => s + x.consultas, 0);
  const totalErr = db.reduce((s, x) => s + x.erros, 0);
  const totalTx = db.reduce((s, x) => s + (x.transacoes_falhas ?? 0), 0);
  w.chart({
    title: "Consultas ao banco",
    subtitle: `total ${num(totalQ)} · ${num(totalErr)} com erro · ${num(totalTx)} transaç${totalTx === 1 ? "ão" : "ões"} com falha`,
    series: [
      { name: "Consultas", color: palette[0], points: db.map((x) => ({ t: t(x.inicio), v: perMin(x.consultas) })) },
      { name: "Erros", color: danger, points: db.map((x) => ({ t: t(x.inicio), v: perMin(x.erros) })) },
      { name: "Transações com falha", color: palette[1], points: db.map((x) => ({ t: t(x.inicio), v: perMin(x.transacoes_falhas ?? 0) })) },
    ],
    from,
    to,
    stepMs,
    format: rate,
  });
  w.chart({
    title: "Latência do banco",
    series: [
      { name: "p50", color: palette[0], points: db.filter((x) => x.consultas > 0).map((x) => ({ t: t(x.inicio), v: x.p50_ms })) },
      { name: "p95", color: palette[1], points: db.filter((x) => x.consultas > 0).map((x) => ({ t: t(x.inicio), v: x.p95_ms })) },
    ],
    from,
    to,
    stepMs,
    format: formatMs,
  });

  const sys = rep.sistema ?? [];
  w.chart({
    title: "Sistema",
    subtitle: "uso médio por faixa",
    series: [
      { name: "CPU", color: palette[0], points: sys.map((x) => ({ t: t(x.hora), v: x.cpu })) },
      { name: "Memória", color: palette[2], points: sys.map((x) => ({ t: t(x.hora), v: x.memoria })) },
      { name: "Disco", color: palette[1], points: sys.map((x) => ({ t: t(x.hora), v: x.disco })) },
    ],
    from,
    to,
    stepMs,
    format: pct,
    yMax: 100,
  });

  const io = sys.filter((x) => x.io);
  const avg = (f: (x: (typeof io)[number]) => number) => (io.length > 0 ? io.reduce((n, x) => n + f(x), 0) / io.length : 0);
  w.chart({
    title: "Disco: leitura e escrita",
    subtitle:
      io.length > 0
        ? `média ${num(avg((x) => x.iops ?? 0), 1)} IOPS · ${formatMs(avg((x) => x.latencia_disco_ms ?? 0))} por operação · ${num(avg((x) => x.disco_ocupado ?? 0))}% ocupado`
        : "dispositivo do disco não encontrado",
    series: [
      { name: "Leitura", color: palette[3], points: io.map((x) => ({ t: t(x.hora), v: x.leitura_bps ?? 0 })) },
      { name: "Escrita", color: palette[1], points: io.map((x) => ({ t: t(x.hora), v: x.escrita_bps ?? 0 })) },
    ],
    from,
    to,
    stepMs,
    format: (v) => `${formatBytes(v)}/s`,
  });

  const conns = rep.conexoes ?? [];
  w.chart({
    title: "Conexões estabelecidas",
    subtitle: "pico por faixa, por porta",
    series: bySeries(conns.flatMap((c) => c.portas.map((p) => ({ name: `${p.porta}/${p.tipo}`, t: t(c.hora), v: p.estabelecidas })))),
    from,
    to,
    stepMs,
    format: (v) => num(v),
  });

  const sizes = rep.tamanho_banco ?? [];
  if (sizes.length > 0) {
    w.chart({
      title: "Tamanho do banco",
      subtitle: "maior medição por faixa",
      series: bySeries(sizes.map((x) => ({ name: x.banco, t: t(x.hora), v: x.bytes }))).slice(0, palette.length),
      from,
      to,
      stepMs,
      format: formatBytes,
    });
  }

  const dns = rep.dns ?? [];
  const dnsQ = dns.reduce((n, x) => n + x.consultas, 0);
  const dnsFail = dns.reduce((n, x) => n + x.falhas, 0);
  const rcodes = topCounts(sumCounts(dns.map((x) => x.rcodes)));
  w.chart({
    title: "Consultas DNS",
    subtitle: `total ${num(dnsQ)} · ${num(dnsFail)} com falha${rcodes ? ` (${rcodes})` : ""}`,
    series: [
      { name: "Consultas", color: palette[0], points: dns.map((x) => ({ t: t(x.inicio), v: perMin(x.consultas) })) },
      { name: "Falhas", color: danger, points: dns.map((x) => ({ t: t(x.inicio), v: perMin(x.falhas) })) },
    ],
    from,
    to,
    stepMs,
    format: rate,
  });

  const smtp = rep.smtp ?? [];
  const sum = (f: (x: (typeof smtp)[number]) => number) => smtp.reduce((n, x) => n + f(x), 0);
  const smtpCodes = topCounts(sumCounts(smtp.map((x) => x.codigos)));
  w.chart({
    title: "Envios SMTP",
    subtitle: `${num(sum((x) => x.tentativas))} tentativas · ${num(sum((x) => x.entregues))} entregues · ${num(sum((x) => x.rejeitados))} rejeitados · ${num(sum((x) => x.abortados))} abortados`,
    series: [
      { name: "Tentativas", color: palette[0], points: smtp.map((x) => ({ t: t(x.inicio), v: perMin(x.tentativas) })) },
      { name: "Entregues", color: palette[5], points: smtp.map((x) => ({ t: t(x.inicio), v: perMin(x.entregues) })) },
      { name: "Rejeitados", color: danger, points: smtp.map((x) => ({ t: t(x.inicio), v: perMin(x.rejeitados) })) },
    ],
    from,
    to,
    stepMs,
    format: rate,
  });
  if (smtpCodes) w.text(`Recusas mais frequentes: ${smtpCodes}.`, { size: 8, color: soft, gap: 3 });

  const queue = rep.fila_smtp ?? [];
  if (queue.length > 0) {
    w.chart({
      title: "Fila do SMTP",
      subtitle: `pico por faixa · maior ${num(Math.max(...queue.map((x) => x.mensagens)))} mensagens`,
      series: [{ name: "Mensagens", color: palette[2], points: queue.map((x) => ({ t: t(x.hora), v: x.mensagens })) }],
      from,
      to,
      stepMs,
      format: (v) => num(v),
    });
  }
}

function scanBlock(w: Writer, s: ReportScan) {
  w.ensure(24);
  w.y += 2;
  w.text(`Varredura #${s.id} · ${s.ferramentas.join(", ")}`, { bold: true, size: 10.5, gap: 0.5 });
  const status = statusLabels[s.status] ?? s.status;
  const counts = severities
    .filter((sev) => (s.achados?.[sev] ?? 0) > 0)
    .map((sev) => `${s.achados[sev]} ${severityLabels[sev].toLowerCase()}`)
    .join(", ");
  w.text(`${dateTime(s.criada_em)} · ${status}${s.erro ? ` (${s.erro})` : ""} · ${counts || "nenhum achado"}`, {
    size: 8.5,
    color: s.status === "falhou" ? danger : soft,
    gap: 2,
  });
  const list = sortFindings(s.lista_achados ?? []);
  if (list.length === 0) return;
  w.table(
    ["Severidade", "Ferramenta", "Porta", "Achado", "Evidência"],
    list.map((f) => {
      const c = severityColors[f.severidade] ?? severityColors.info;
      return [
        { content: severityLabels[f.severidade] ?? f.severidade, styles: { textColor: c.text, fillColor: c.fill, fontStyle: "bold" } },
        f.ferramenta,
        f.porta > 0 ? String(f.porta) : "-",
        f.titulo,
        f.evidencia.length > 240 ? `${f.evidencia.slice(0, 240)}...` : f.evidencia,
      ];
    }),
    { columnStyles: { 0: { cellWidth: 20 }, 1: { cellWidth: 20 }, 2: { cellWidth: 13 }, 3: { cellWidth: 60 } } },
  );
}

function arsenalSection(w: Writer, rep: TargetReport) {
  w.section("Arsenal");
  const scans = rep.varreduras ?? [];
  if (scans.length === 0) {
    w.text("Nenhuma varredura deste alvo no intervalo.", { color: soft, size: 9 });
    return;
  }
  const tools = [...new Set(scans.flatMap((s) => s.ferramentas))].sort();
  const findings = scans.reduce((n, s) => n + (s.lista_achados?.length ?? 0), 0);
  w.text(
    `${scans.length} varredura${scans.length === 1 ? "" : "s"} no intervalo, com ${tools.join(", ")}: ${findings} achado${findings === 1 ? "" : "s"} no total.`,
    { size: 9, color: soft, gap: 2 },
  );
  for (const s of scans) scanBlock(w, s);
}

function footers(doc: jsPDF, rep: TargetReport) {
  const n = doc.getNumberOfPages();
  for (let i = 1; i <= n; i++) {
    doc.setPage(i);
    doc.setLineWidth(0.15).setDrawColor(...grid).line(MARGIN, PAGE_H - 12, PAGE_W - MARGIN, PAGE_H - 12);
    doc.setFont("helvetica", "normal").setFontSize(7.5).setTextColor(...faint);
    doc.text(`Omni · Relatório de ${rep.alvo.nome}`, MARGIN, PAGE_H - 8);
    doc.text(`Página ${i} de ${n}`, PAGE_W - MARGIN, PAGE_H - 8, { align: "right" });
  }
}

// buildReportPdf monta o PDF do relatório; quem chama decide se salva ou mostra.
export function buildReportPdf(rep: TargetReport, generatedAt = new Date()): jsPDF {
  const doc = new jsPDF({ unit: "mm", format: "a4" });
  doc.setProperties({ title: `Relatório Omni - ${rep.alvo.nome}`, creator: "Omni" });
  const w = new Writer(doc);
  const from = t(rep.desde);
  const to = t(rep.ate);
  const stepMs = Math.max(rep.passo_s, 1) * 1000;

  header(w, rep, generatedAt);
  alertsSection(w, rep);
  availabilitySection(w, rep, from, to, stepMs);
  arsenalSection(w, rep);
  serverCharts(w, rep, from, to, stepMs);
  footers(doc, rep);
  return doc;
}
