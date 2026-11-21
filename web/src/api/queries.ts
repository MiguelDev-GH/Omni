import { apiFetch } from "./client";
import { sinceQuery } from "./traffic";

// DBWindow resume 10 s de consultas ao banco, como em METRICAS_BANCO.
export type DBWindow = {
  inicio: string;
  consultas: number;
  erros: number;
  tipos: Record<string, number>;
  p50_ms: number;
  p95_ms: number;
  top?: QueryStats[];
};

// QueryStats traz a consulta já normalizada, sem valores literais.
export type QueryStats = {
  consulta: string;
  tipo: string;
  execucoes: number;
  erros: number;
  p50_ms: number;
  p95_ms: number;
};

export function listQueries(since?: Date): Promise<DBWindow[]> {
  return apiFetch<DBWindow[]>(`/api/consultas${sinceQuery(since)}`);
}

export function listSlowQueries(limit = 20): Promise<QueryStats[]> {
  return apiFetch<QueryStats[]>(`/api/consultas/lentas?limite=${limit}`);
}
