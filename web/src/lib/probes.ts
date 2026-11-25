import type { PortStatus } from "../store/app";

export type ProbeSummary = {
  disponibilidade: number | null; // % das verificações em que o servidor respondeu
  latenciaMedia: number | null; // ms, só das verificações no ar
  latenciaP95: number | null;
  taxaFalhas: number | null; // % fora do ar ou com resposta de erro
  ultimo?: PortStatus;
};

// summarizeProbes resume as verificações de uma porta DNS ou SMTP, em ordem
// cronológica. Respostas de erro (degradado) contam como no ar para a
// disponibilidade e como falha para a taxa, como no backend.
export function summarizeProbes(results: PortStatus[]): ProbeSummary {
  if (results.length === 0) {
    return { disponibilidade: null, latenciaMedia: null, latenciaP95: null, taxaFalhas: null, ultimo: undefined };
  }
  const up = results.filter((r) => r.no_ar);
  const failed = results.filter((r) => !r.no_ar || r.degradado).length;
  const latencies = up.map((r) => r.latencia_ms).sort((a, b) => a - b);
  return {
    disponibilidade: (up.length / results.length) * 100,
    latenciaMedia: latencies.length ? latencies.reduce((s, v) => s + v, 0) / latencies.length : null,
    latenciaP95: latencies.length ? (latencies[Math.ceil(0.95 * latencies.length) - 1] ?? null) : null,
    taxaFalhas: (failed / results.length) * 100,
    ultimo: results[results.length - 1],
  };
}
