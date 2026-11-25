import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router";
import { listTraffic } from "../api/traffic";
import BandwidthChart from "../charts/BandwidthChart";
import { queryColor } from "../charts/colors";
import CountChart from "../charts/CountChart";
import ProtocolDonut from "../charts/ProtocolDonut";
import StatusBars from "../charts/StatusBars";
import { Card, Notice, PageHeader, Stat } from "../components/Card";
import { formatRate, lastRate } from "../lib/l7";
import { bandwidthSeries, formatBytes, MAX_POINTS, protocolTotals, topSources } from "../lib/traffic";
import { useAppStore } from "../store/app";
import { useDBHistory, useDBSeries } from "./QueriesPage";
import { useHTTPHistory, useHTTPSeries } from "./RequestsPage";

export default function OverviewPage() {
  const traffic = useAppStore((s) => s.traffic);
  const sniffer = useAppStore((s) => s.engines.sniffer);
  const targets = useAppStore((s) => s.targets);
  const targetsLoaded = useAppStore((s) => s.targetsLoaded);
  const [loadError, setLoadError] = useState(false);
  const httpError = useHTTPHistory();
  const httpSeries = useHTTPSeries();
  const dbError = useDBHistory();
  const dbSeries = useDBSeries();
  const http = useAppStore((s) => s.http);
  const db = useAppStore((s) => s.db);

  // Carrega os últimos 15 minutos uma vez; os eventos ao vivo completam a série.
  useEffect(() => {
    listTraffic(new Date(Date.now() - MAX_POINTS * 1000)).then(
      (list) => useAppStore.getState().addTraffic(list),
      () => setLoadError(true),
    );
  }, []);

  const series = useMemo(() => bandwidthSeries(traffic), [traffic]);
  const protocols = useMemo(() => protocolTotals(traffic), [traffic]);
  const sources = useMemo(() => topSources(traffic), [traffic]);
  // Fora do estado ativo, a última janela é antiga e não representa "agora".
  const last = sniffer === "ativo" ? traffic[traffic.length - 1] : undefined;
  const total = traffic.reduce((sum, w) => sum + w.bytes, 0);
  const dropped = traffic.reduce((sum, w) => sum + (w.descartados ?? 0), 0);

  return (
    <div className="space-y-8">
      <PageHeader title="Geral" />

      {sniffer === "sem-permissao" && (
        <Notice role="alert">Captura desativada: o Omni está sem permissão de captura (CAP_NET_RAW).</Notice>
      )}
      {targetsLoaded && targets.length === 0 && (
        <Notice tone="info">
          O sniffer observa as portas dos alvos.{" "}
          <Link to="/alvos" className="link">
            Cadastre um alvo
          </Link>{" "}
          para começar a capturar.
        </Notice>
      )}
      {dropped > 0 && (
        <p className="text-sm text-aviso">
          A captura descartou {dropped.toLocaleString("pt-BR")} pacotes nos últimos 15 minutos; as métricas estão
          incompletas.
        </p>
      )}
      {loadError && <p className="text-sm text-erro">Não foi possível carregar o histórico de tráfego.</p>}
      {httpError && <p className="text-sm text-erro">Não foi possível carregar o histórico de requisições.</p>}
      {dbError && <p className="text-sm text-erro">Não foi possível carregar o histórico de consultas.</p>}

      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 sm:gap-6 lg:grid-cols-5">
        <Stat label="Requisições por segundo" value={formatRate(lastRate(http, (w) => w.requisicoes))} />
        <Stat label="Consultas por segundo" value={formatRate(lastRate(db, (w) => w.consultas))} />
        <Stat label="Banda agora" value={last ? `${formatBytes(last.bytes)}/s` : "—"} />
        <Stat label="Pacotes por segundo" value={last ? last.pacotes.toLocaleString("pt-BR") : "—"} />
        <Stat label="Total em 15 minutos" value={formatBytes(total)} />
      </div>

      <div className="grid gap-x-10 gap-y-8 xl:grid-cols-2">
        <Card title="Requisições na API">
          <CountChart id="req-geral" points={httpSeries.requests} label="Requisições" empty="Sem requisições capturadas ainda." />
        </Card>
        <Card title="Consultas no banco">
          <CountChart id="db-geral" points={dbSeries} label="Consultas" color={queryColor} empty="Sem consultas capturadas ainda." />
        </Card>
      </div>

      <Card title="Status HTTP">
        <StatusBars points={httpSeries.status} />
      </Card>

      <Card title="Banda nos últimos 15 minutos">
        <BandwidthChart points={series} />
      </Card>

      <div className="grid gap-x-10 gap-y-8 lg:grid-cols-2">
        <Card title="Protocolos">
          <ProtocolDonut data={protocols} />
        </Card>
        <Card title="IPs de origem com mais tráfego" plane>
          {sources.length === 0 ? (
            <p className="text-sm text-suave">Sem tráfego capturado ainda.</p>
          ) : (
            <table className="tabela">
              <thead>
                <tr>
                  <th>IP</th>
                  <th className="text-right">Bytes</th>
                  <th className="text-right">Pacotes</th>
                </tr>
              </thead>
              <tbody>
                {sources.map((s) => (
                  <tr key={s.ip}>
                    <td className="font-mono text-xs break-all">{s.ip}</td>
                    <td className="text-right font-mono text-xs tabular-nums">{formatBytes(s.bytes)}</td>
                    <td className="text-right font-mono text-xs tabular-nums">{s.pacotes.toLocaleString("pt-BR")}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Card>
      </div>
    </div>
  );
}
