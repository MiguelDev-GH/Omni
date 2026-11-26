import { useEffect, useState } from "react";
import { listConnections, type ConnKind, type ConnSnapshot } from "../api/connections";
import type { CountPoint } from "../charts/CountChart";
import { useAppStore } from "../store/app";

// As coletas de conexões saem a cada 5 s; MAX_CONN_POINTS cobre 15 minutos.
export const CONN_STEP_MS = 5_000;
export const MAX_CONN_POINTS = 180;

// mergeConnections junta coletas novas às antigas, sem repetir a mesma hora
// (a mais nova vence), em ordem cronológica e com no máximo MAX_CONN_POINTS.
export function mergeConnections(prev: ConnSnapshot[], next: ConnSnapshot[]): ConnSnapshot[] {
  const byTime = new Map<number, ConnSnapshot>();
  for (const s of [...prev, ...next]) {
    const t = Date.parse(s.hora);
    if (!Number.isNaN(t)) byTime.set(t, s);
  }
  return [...byTime.entries()]
    .sort(([a], [b]) => a - b)
    .slice(-MAX_CONN_POINTS)
    .map(([, s]) => s);
}

const established = (s: ConnSnapshot, kind: ConnKind) =>
  s.portas.filter((p) => p.tipo === kind).reduce((sum, p) => sum + p.estabelecidas, 0);

// connectionSeries soma as conexões abertas das portas do tipo em cada coleta.
export function connectionSeries(snaps: ConnSnapshot[], kind: ConnKind): CountPoint[] {
  return snaps.map((s) => ({ t: Date.parse(s.hora), n: established(s, kind) }));
}

// latestEstablished devolve as conexões abertas do tipo na coleta mais nova,
// ou null sem coletas ou com a última parada há 3 coletas ou mais.
export function latestEstablished(snaps: ConnSnapshot[], kind: ConnKind, now = Date.now()): number | null {
  const last = snaps[snaps.length - 1];
  if (!last || now - Date.parse(last.hora) >= 3 * CONN_STEP_MS) return null;
  return established(last, kind);
}

// latestPools lista os clientes das portas de banco na coleta mais nova.
export function latestPools(snaps: ConnSnapshot[]): { porta: number; processo: string; conexoes: number }[] {
  const last = snaps[snaps.length - 1];
  if (!last) return [];
  return last.portas
    .filter((p) => p.tipo === "banco")
    .flatMap((p) => p.clientes.map((c) => ({ porta: p.porta, ...c })));
}

// useConnHistory carrega os últimos 15 minutos de coletas uma vez.
export function useConnHistory(): boolean {
  const [loadError, setLoadError] = useState(false);
  useEffect(() => {
    listConnections(new Date(Date.now() - MAX_CONN_POINTS * CONN_STEP_MS)).then(
      (list) => useAppStore.getState().addConnections(list),
      () => setLoadError(true),
    );
  }, []);
  return loadError;
}
