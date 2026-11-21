import { apiFetch } from "./client";

export type SourceCount = { ip: string; bytes: number; pacotes: number };

// TrafficWindow é o resumo de um segundo de tráfego, como em METRICAS_TRAFEGO.
export type TrafficWindow = {
  inicio: string;
  bytes: number;
  pacotes: number;
  descartados: number;
  protocolos: Record<string, number>;
  top_origens: SourceCount[];
};

// sinceQuery monta "?desde=<RFC3339>" sem milissegundos, ou "" sem data.
export function sinceQuery(since?: Date): string {
  return since ? `?desde=${encodeURIComponent(since.toISOString().replace(/\.\d{3}Z$/, "Z"))}` : "";
}

export function listTraffic(since?: Date): Promise<TrafficWindow[]> {
  return apiFetch<TrafficWindow[]>(`/api/trafego${sinceQuery(since)}`);
}
