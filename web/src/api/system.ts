import type { SystemMetrics } from "../store/app";
import { apiFetch } from "./client";
import { sinceQuery } from "./traffic";

// listSystem busca as leituras de CPU, memória e disco desde since. Acima de
// 1 h, a API devolve uma média por minuto.
export function listSystem(since?: Date): Promise<SystemMetrics[]> {
  return apiFetch<SystemMetrics[]>(`/api/sistema${sinceQuery(since)}`);
}
