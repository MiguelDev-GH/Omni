import { useEffect, useMemo, useState } from "react";
import { listQueries, listSlowQueries, type QueryStats } from "../api/queries";
import CountChart from "../charts/CountChart";
import { queryColor } from "../charts/colors";
import { Card, Stat } from "../components/Card";
import { formatMs, l7Series, L7_POINTS, L7_STEP_MS } from "../lib/l7";
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

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-semibold">Consultas no banco</h1>
      <p className="text-sm text-suave">
        Decodificado do protocolo do PostgreSQL na porta do banco dos alvos. As consultas são normalizadas: valores
        literais viram <code className="font-mono">?</code> e senhas nunca são lidas. Conexões com TLS ficam de fora.
      </p>
      {loadError && <p className="text-sm text-erro">Não foi possível carregar o histórico de consultas.</p>}

      <div className="grid gap-4 sm:grid-cols-3">
        <Stat label="Consultas em 15 minutos" value={total.toLocaleString("pt-BR")} />
        <Stat label="Erros em 15 minutos" value={errors.toLocaleString("pt-BR")} />
        <Stat label="p95 dos últimos 10 s" value={last ? formatMs(last.p95_ms) : "—"} />
      </div>

      <Card title="Consultas nos últimos 15 minutos">
        <CountChart id="db-page" points={series} label="Consultas" color={queryColor} empty="Sem consultas capturadas ainda." />
      </Card>

      <Card title="Consultas mais lentas nas últimas 24 horas">
        {slowError && <p className="text-sm text-erro">Não foi possível carregar as consultas mais lentas.</p>}
        {slow !== null && slow.length === 0 ? (
          <p className="text-sm text-suave">Nenhuma consulta registrada nas últimas 24 horas.</p>
        ) : (
          <div className="overflow-x-auto">
            <table aria-label="Consultas mais lentas" className="w-full text-sm">
              <thead className="text-left text-xs text-suave">
                <tr>
                  <th className="pb-2 font-medium">Consulta</th>
                  <th className="pb-2 text-right font-medium">Execuções</th>
                  <th className="pb-2 text-right font-medium">Erros</th>
                  <th className="pb-2 text-right font-medium">p50</th>
                  <th className="pb-2 text-right font-medium">p95</th>
                </tr>
              </thead>
              <tbody>
                {(slow ?? []).map((q) => (
                  <tr key={q.consulta} className="border-t border-borda align-top">
                    <td className="py-1.5 pr-3">
                      <code className="font-mono text-xs break-all">{q.consulta}</code>
                    </td>
                    <td className="py-1.5 text-right tabular-nums">{q.execucoes.toLocaleString("pt-BR")}</td>
                    <td className={`py-1.5 text-right tabular-nums ${q.erros > 0 ? "text-erro" : ""}`}>
                      {q.erros.toLocaleString("pt-BR")}
                    </td>
                    <td className="py-1.5 text-right whitespace-nowrap tabular-nums">{formatMs(q.p50_ms)}</td>
                    <td className="py-1.5 text-right whitespace-nowrap tabular-nums">{formatMs(q.p95_ms)}</td>
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
