import { useEffect, useMemo, useState } from "react";
import { listRequests, listRoutes, type RouteStats } from "../api/requests";
import CountChart from "../charts/CountChart";
import StatusBars from "../charts/StatusBars";
import { Card, PageHeader, Stat } from "../components/Card";
import Select from "../components/Select";
import { emptyStatus, formatMs, formatRate, l7Series, lastRate, L7_POINTS, L7_STEP_MS, type StatusPoint } from "../lib/l7";
import { useAppStore } from "../store/app";

const REFRESH_MS = 30_000;

type Order = "p95" | "volume";

// sortRoutes ordena pela latência p95 (as mais lentas primeiro) ou pelo volume.
export function sortRoutes(list: RouteStats[], order: Order): RouteStats[] {
  const key = (r: RouteStats) => (order === "p95" ? r.p95_ms : r.requisicoes);
  return [...list].sort((a, b) => key(b) - key(a) || a.rota.localeCompare(b.rota));
}

// useHTTPHistory carrega os últimos 15 minutos de janelas HTTP uma vez.
export function useHTTPHistory(): boolean {
  const [loadError, setLoadError] = useState(false);
  useEffect(() => {
    listRequests(new Date(Date.now() - L7_POINTS * L7_STEP_MS)).then(
      (list) => useAppStore.getState().addHTTP(list),
      () => setLoadError(true),
    );
  }, []);
  return loadError;
}

export function useHTTPSeries() {
  const http = useAppStore((s) => s.http);
  return useMemo(
    () => ({
      requests: l7Series(http, (w) => ({ n: w.requisicoes }), { n: 0 }),
      status: l7Series(http, (w): StatusPoint => ({ ...emptyStatus, ...w.status }), emptyStatus),
    }),
    [http],
  );
}

export default function RequestsPage() {
  const loadError = useHTTPHistory();
  const { requests, status } = useHTTPSeries();
  const [routes, setRoutes] = useState<RouteStats[] | null>(null);
  const [routesError, setRoutesError] = useState(false);
  const [order, setOrder] = useState<Order>("p95");
  const http = useAppStore((s) => s.http);

  // As rotas das últimas 24 h são recarregadas a cada 30 s.
  useEffect(() => {
    const load = () =>
      listRoutes().then(
        (list) => {
          setRoutes(list);
          setRoutesError(false);
        },
        () => setRoutesError(true),
      );
    void load();
    const id = setInterval(load, REFRESH_MS);
    return () => clearInterval(id);
  }, []);

  const sorted = useMemo(() => sortRoutes(routes ?? [], order), [routes, order]);
  const total = (routes ?? []).reduce((sum, r) => sum + r.requisicoes, 0);
  const errors = (routes ?? []).reduce((sum, r) => sum + r.erros, 0);

  return (
    <div className="space-y-8">
      <PageHeader title="Requisições na API">
        Decodificado do tráfego HTTP/1.1 das portas da aplicação. O tráfego do próprio Omni fica de fora, e cabeçalhos,
        query strings e corpos nunca são guardados.
      </PageHeader>
      {loadError && <p className="text-sm text-erro">Não foi possível carregar o histórico de requisições.</p>}

      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4 sm:gap-6">
        <Stat label="Requisições por segundo" value={formatRate(lastRate(http, (w) => w.requisicoes))} />
        <Stat label="Requisições em 24 h" value={total.toLocaleString("pt-BR")} />
        <Stat label="Erros 5xx em 24 h" value={errors.toLocaleString("pt-BR")} tone={errors > 0 ? "text-erro" : undefined} />
        <Stat label="Rotas distintas" value={(routes?.length ?? 0).toLocaleString("pt-BR")} />
      </div>

      <div className="grid gap-x-10 gap-y-8 xl:grid-cols-2">
        <Card title="Requisições nos últimos 15 minutos">
          <CountChart id="req-page" points={requests} label="Requisições" empty="Sem requisições capturadas ainda." />
        </Card>
        <Card title="Status HTTP">
          <StatusBars points={status} />
        </Card>
      </div>

      <Card
        title="Rotas nas últimas 24 horas"
        plane
        action={
          <div className="flex items-center gap-2 text-sm">
            <label htmlFor="ordem" className="text-suave">
              Ordenar por
            </label>
            <Select
              id="ordem"
              value={order}
              onChange={setOrder}
              options={[
                { value: "p95", label: "Mais lentas (p95)" },
                { value: "volume", label: "Mais acessadas" },
              ]}
            />
          </div>
        }
      >
        {routesError && <p className="text-sm text-erro">Não foi possível carregar as rotas.</p>}
        {routes !== null && routes.length === 0 ? (
          <p className="text-sm text-suave">Nenhuma requisição registrada nas últimas 24 horas.</p>
        ) : (
          <div className="overflow-x-auto">
            <table aria-label="Rotas" className="tabela">
              <thead>
                <tr>
                  <th>Rota</th>
                  <th className="text-right">Requisições</th>
                  <th className="text-right">Erros 5xx</th>
                  <th className="text-right">p50</th>
                  <th className="text-right">p95</th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((r) => (
                  <tr key={`${r.metodo} ${r.rota}`}>
                    <td className="pr-3 font-mono text-xs break-all">
                      <span className="mr-2 text-destaque-forte">{r.metodo}</span>
                      {r.rota}
                    </td>
                    <td className="text-right font-mono text-xs tabular-nums">{r.requisicoes.toLocaleString("pt-BR")}</td>
                    <td className={`text-right font-mono text-xs tabular-nums ${r.erros > 0 ? "text-erro" : ""}`}>
                      {r.erros.toLocaleString("pt-BR")}
                    </td>
                    <td className="text-right font-mono text-xs whitespace-nowrap tabular-nums">{formatMs(r.p50_ms)}</td>
                    <td className="text-right font-mono text-xs whitespace-nowrap tabular-nums">{formatMs(r.p95_ms)}</td>
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
