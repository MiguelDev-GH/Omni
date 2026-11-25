import { apiFetch } from "./client";
import { sinceQuery } from "./traffic";

// DBWindow resume 10 s de consultas ao banco, como em METRICAS_BANCO.
export type DBWindow = {
  inicio: string;
  consultas: number;
  erros: number;
  tipos: Record<string, number>; // "SELECT", "ROLLBACK"...
  transacoes_falhas?: number; // transações abortadas por erro
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

// DBSize é o tamanho de um banco do PostgreSQL; crescimento_gb_dia fica null
// enquanto as medições cobrem menos de 1 h.
export type DBSize = {
  banco: string;
  bytes: number;
  crescimento_gb_dia: number | null;
  serie: { hora: string; bytes: number }[];
};

// DBSizes diz se a medição está configurada (OMNI_PG_DSN) e os bancos das
// últimas 24 h, do maior para o menor.
export type DBSizes = { configurado: boolean; bancos: DBSize[] };

export function getDBSizes(): Promise<DBSizes> {
  return apiFetch<DBSizes>("/api/banco/tamanho");
}
