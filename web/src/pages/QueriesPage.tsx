import { useEffect, useMemo, useState } from "react";
import { listQueries, listSlowQueries, type QueryStats } from "../api/queries";
import CountChart from "../charts/CountChart";
import { queryColor } from "../charts/colors";
import { Card, PageHeader, Stat } from "../components/Card";
import { formatMs, formatRate, l7Series, lastRate, L7_POINTS, L7_STEP_MS } from "../lib/l7";
import { latestEstablished, latestPools, useConnHistory } from "../lib/connections";
import { useAppStore } from "../store/app";

const REFRESH_MS = 30_000;

// useDBHistory carrega os últimos 15 minutos de janelas de consultas uma vez.
export function useDBHistory(): boolean {
  const [loadError, setLoadError] = useState(false);
  useEffect(() => {
    listQueries(new Date(Date.now() - L7_POINTS * L7_STEP_MS)).then(
      (list) => useAppStore.getState().addDB(list),
      () => setLoadError(true),
    );
  }, []);
  return loadError;
}

export function useDBSeries() {
  const db = useAppStore((s) => s.db);
  return useMemo(() => l7Series(db, (w) => ({ n: w.consultas }), { n: 0 }), [db]);
}

export default function QueriesPage() {
  const loadError = useDBHistory();
  const series = useDBSeries();
  const db = useAppStore((s) => s.db);
  const [slow, setSlow] = useState<QueryStats[] | null>(null);
  const [slowError, setSlowError] = useState(false);

  // As consultas mais lentas das últimas 24 h são recarregadas a cada 30 s.
  useEffect(() => {
    const load = () =>
      listSlowQueries().then(
        (list) => {
          setSlow(list);
          setSlowError(false);
        },
        () => setSlowError(true),
      );
    void load();
    const id = setInterval(load, REFRESH_MS);
    return () => clearInterval(id);
  }, []);

  const total = db.reduce((sum, w) => sum + w.consultas, 0);
  const errors = db.reduce((sum, w) => sum + w.erros, 0);
  const last = db[db.length - 1];
  const connections = useAppStore((s) => s.connections);
  const connError = useConnHistory();
  const pools = useMemo(() => latestPools(connections), [connections]);
  const dbConns = latestEstablished(connections, "banco");

  return (
    <div className="space-y-8">
      <PageHeader title="Consultas no banco">
        Decodificado do protocolo do PostgreSQL na porta do banco dos alvos. As consultas são normalizadas: valores
        literais viram <code className="font-mono">?</code> e senhas nunca são lidas. Conexões com TLS ficam de fora.
      </PageHeader>
      {loadError && <p className="text-sm text-erro">Não foi possível carregar o histórico de consultas.</p>}
      {connError && <p className="text-sm text-erro">Não foi possível carregar o histórico de conexões.</p>}

      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 sm:gap-6 lg:grid-cols-5">
        <Stat label="Conexões abertas no banco" value={dbConns === null ? "—" : dbConns.toLocaleString("pt-BR")} />
        <Stat label="Consultas por segundo" value={formatRate(lastRate(db, (w) => w.consultas))} />
        <Stat label="Consultas em 15 minutos" value={total.toLocaleString("pt-BR")} />
        <Stat label="Erros em 15 minutos" value={errors.toLocaleString("pt-BR")} />
        <Stat label="p95 dos últimos 10 s" value={last ? formatMs(last.p95_ms) : "—"} />
      </div>

      <Card title="Consultas nos últimos 15 minutos">
        <CountChart id="db-page" points={series} label="Consultas" color={queryColor} empty="Sem consultas capturadas ainda." />
      </Card>

      <Card title="Pools de conexões ao banco" plane>
        {pools.length === 0 ? (
          <p className="text-sm text-suave">Nenhuma conexão aberta com o banco agora.</p>
        ) : (
          <table aria-label="Pools de conexões" className="tabela">
            <thead>
              <tr>
                <th>Processo cliente</th>
                <th className="text-right">Porta</th>
                <th className="text-right">Conexões</th>
              </tr>
            </thead>
            <tbody>
              {pools.map((p) => (
                <tr key={`${p.porta}-${p.processo}`}>
                  <td className={`font-mono text-xs ${p.processo ? "" : "text-suave"}`}>{p.processo || "(sem permissão para ver)"}</td>
                  <td className="text-right font-mono text-xs tabular-nums">{p.porta}</td>
                  <td className="text-right font-mono text-xs tabular-nums">{p.conexoes.toLocaleString("pt-BR")}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>

      <Card title="Consultas mais lentas nas últimas 24 horas" plane>
        {slowError && <p className="text-sm text-erro">Não foi possível carregar as consultas mais lentas.</p>}
        {slow !== null && slow.length === 0 ? (
          <p className="text-sm text-suave">Nenhuma consulta registrada nas últimas 24 horas.</p>
        ) : (
          <div className="overflow-x-auto">
            <table aria-label="Consultas mais lentas" className="tabela">
              <thead>
                <tr>
                  <th>Consulta</th>
                  <th className="text-right">Execuções</th>
                  <th className="text-right">Erros</th>
                  <th className="text-right">p50</th>
                  <th className="text-right">p95</th>
                </tr>
              </thead>
              <tbody>
                {(slow ?? []).map((q) => (
                  <tr key={q.consulta} className="align-top">
                    <td className="pr-3">
                      <code className="font-mono text-xs break-all">{q.consulta}</code>
                    </td>
                    <td className="text-right font-mono text-xs tabular-nums">{q.execucoes.toLocaleString("pt-BR")}</td>
                    <td className={`text-right font-mono text-xs tabular-nums ${q.erros > 0 ? "text-erro" : ""}`}>
                      {q.erros.toLocaleString("pt-BR")}
                    </td>
                    <td className="text-right font-mono text-xs whitespace-nowrap tabular-nums">{formatMs(q.p50_ms)}</td>
                    <td className="text-right font-mono text-xs whitespace-nowrap tabular-nums">{formatMs(q.p95_ms)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  );
}
