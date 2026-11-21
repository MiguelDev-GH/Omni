import { apiFetch } from "./client";
import { sinceQuery } from "./traffic";

// HTTPWindow resume 10 s de requisições, como em METRICAS_HTTP.
export type HTTPWindow = {
  inicio: string;
  requisicoes: number;
  status: Record<string, number>; // "2xx", "3xx", "4xx", "5xx"
  p50_ms: number;
  p95_ms: number;
  rotas?: RouteStats[];
};

export type RouteStats = {
  metodo: string;
  rota: string;
  requisicoes: number;
  erros: number;
  p50_ms: number;
  p95_ms: number;
};

export function listRequests(since?: Date): Promise<HTTPWindow[]> {
  return apiFetch<HTTPWindow[]>(`/api/requisicoes${sinceQuery(since)}`);
}

export function listRoutes(since?: Date): Promise<RouteStats[]> {
  return apiFetch<RouteStats[]>(`/api/requisicoes/rotas${sinceQuery(since)}`);
}
