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

export function listTraffic(since?: Date): Promise<TrafficWindow[]> {
  const q = since ? `?desde=${encodeURIComponent(since.toISOString().replace(/\.\d{3}Z$/, "Z"))}` : "";
  return apiFetch<TrafficWindow[]>(`/api/trafego${q}`);
}
