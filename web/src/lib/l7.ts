// Janelas de 10 s das métricas de aplicação (HTTP e banco).
export const L7_STEP_MS = 10_000;
// L7_POINTS cobre 15 minutos de janelas de 10 s.
export const L7_POINTS = 90;

type Windowed = { inicio: string };

const slot = (w: Windowed) => Math.floor(Date.parse(w.inicio) / L7_STEP_MS);

// mergeL7 junta janelas novas às antigas, sem repetir o mesmo intervalo de
// 10 s (a mais nova vence), em ordem cronológica e com no máximo L7_POINTS.
export function mergeL7<T extends Windowed>(prev: T[], next: T[]): T[] {
  const bySlot = new Map<number, T>();
  for (const w of [...prev, ...next]) {
    if (!Number.isNaN(slot(w))) bySlot.set(slot(w), w);
  }
  return [...bySlot.entries()]
    .sort(([a], [b]) => a - b)
    .slice(-L7_POINTS)
    .map(([, w]) => w);
}

// l7Series devolve um ponto por intervalo de 10 s até a janela mais recente,
// no máximo L7_POINTS; intervalos sem janela recebem empty.
export function l7Series<T extends Windowed, P>(windows: T[], pick: (w: T) => P, empty: P): (P & { t: number })[] {
  const first = windows[0];
  const last = windows[windows.length - 1];
  if (!first || !last) return [];
  const bySlot = new Map(windows.map((w) => [slot(w), w]));
  const end = slot(last);
  const start = Math.max(end - L7_POINTS + 1, slot(first));
  const out: (P & { t: number })[] = [];
  for (let s = start; s <= end; s++) {
    const w = bySlot.get(s);
    out.push({ ...(w ? pick(w) : empty), t: s * L7_STEP_MS });
  }
  return out;
}

export const statusClasses = ["2xx", "3xx", "4xx", "5xx"] as const;
export type StatusPoint = Record<(typeof statusClasses)[number], number>;

export const emptyStatus: StatusPoint = { "2xx": 0, "3xx": 0, "4xx": 0, "5xx": 0 };

// formatMs escreve uma latência em ms ou s, como "12 ms" ou "1,5 s".
export function formatMs(ms: number): string {
  if (ms >= 1000) return `${(ms / 1000).toLocaleString("pt-BR", { maximumFractionDigits: 1 })} s`;
  return `${ms.toLocaleString("pt-BR", { maximumFractionDigits: ms < 10 ? 1 : 0 })} ms`;
}

// perSecond converte a contagem de uma janela em taxa por segundo.
export function perSecond(count: number, stepMs = L7_STEP_MS): number {
  return count / (stepMs / 1000);
}

// STALE_MS é a idade a partir da qual a última janela não representa "agora":
// uma janela chega ao fim dos seus 10 s, então a mais nova tem entre 10 e 20 s.
const STALE_MS = 3 * L7_STEP_MS;

// lastRate devolve a taxa por segundo da janela mais recente, ou null sem
// janelas ou com a última parada há STALE_MS ou mais.
export function lastRate<T extends Windowed>(windows: T[], pick: (w: T) => number, now = Date.now()): number | null {
  const last = windows[windows.length - 1];
  if (!last || now - Date.parse(last.inicio) >= STALE_MS) return null;
  return perSecond(pick(last));
}

// formatRate mostra uma taxa por segundo com até uma casa decimal.
export function formatRate(rate: number | null): string {
  return rate === null ? "—" : rate.toLocaleString("pt-BR", { maximumFractionDigits: 1 });
}
