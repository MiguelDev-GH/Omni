import type { SourceCount, TrafficWindow } from "../api/traffic";

// MAX_POINTS é a janela deslizante dos gráficos: 15 minutos, um ponto por segundo.
export const MAX_POINTS = 900;

const second = (w: TrafficWindow) => Math.floor(Date.parse(w.inicio) / 1000);

// mergeWindows junta janelas novas às antigas, sem repetir o mesmo segundo
// (a mais nova vence), em ordem cronológica e com no máximo MAX_POINTS.
export function mergeWindows(prev: TrafficWindow[], next: TrafficWindow[]): TrafficWindow[] {
  const bySecond = new Map<number, TrafficWindow>();
  for (const w of [...prev, ...next]) {
    if (!Number.isNaN(second(w))) bySecond.set(second(w), w);
  }
  return [...bySecond.entries()]
    .sort(([a], [b]) => a - b)
    .slice(-MAX_POINTS)
    .map(([, w]) => w);
}

export type BandwidthPoint = { t: number; bytes: number };

// bandwidthSeries devolve um ponto por segundo nos últimos MAX_POINTS segundos
// até a janela mais recente; segundos sem janela valem zero.
export function bandwidthSeries(windows: TrafficWindow[]): BandwidthPoint[] {
  const first = windows[0];
  const last = windows[windows.length - 1];
  if (!first || !last) return [];
  const bySecond = new Map(windows.map((w) => [second(w), w.bytes]));
  const end = second(last);
  const start = Math.max(end - MAX_POINTS + 1, second(first));
  const out: BandwidthPoint[] = [];
  for (let s = start; s <= end; s++) out.push({ t: s * 1000, bytes: bySecond.get(s) ?? 0 });
  return out;
}

// protocolTotals soma os bytes por protocolo, do maior para o menor.
export function protocolTotals(windows: TrafficWindow[]): { name: string; bytes: number }[] {
  const sum = new Map<string, number>();
  for (const w of windows) {
    for (const [name, bytes] of Object.entries(w.protocolos ?? {})) sum.set(name, (sum.get(name) ?? 0) + bytes);
  }
  return [...sum.entries()].map(([name, bytes]) => ({ name, bytes })).sort((a, b) => b.bytes - a.bytes);
}

// topSources soma as origens de todas as janelas e devolve as limit maiores.
// Cada janela só traz as dez maiores dela, então o total é um piso.
export function topSources(windows: TrafficWindow[], limit = 10): SourceCount[] {
  const sum = new Map<string, SourceCount>();
  for (const w of windows) {
    for (const s of w.top_origens ?? []) {
      const acc = sum.get(s.ip) ?? { ip: s.ip, bytes: 0, pacotes: 0 };
      acc.bytes += s.bytes;
      acc.pacotes += s.pacotes;
      sum.set(s.ip, acc);
    }
  }
  return [...sum.values()].sort((a, b) => b.bytes - a.bytes || a.ip.localeCompare(b.ip)).slice(0, limit);
}

const units = ["B", "KB", "MB", "GB"];

// formatBytes escreve bytes em base 1000, como "1,5 MB".
export function formatBytes(n: number): string {
  let i = 0;
  while (n >= 1000 && i < units.length - 1) {
    n /= 1000;
    i++;
  }
  const digits = i === 0 || n >= 100 ? 0 : 1;
  return `${n.toLocaleString("pt-BR", { maximumFractionDigits: digits })} ${units[i]}`;
}
