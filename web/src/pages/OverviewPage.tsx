import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router";
import { listTraffic } from "../api/traffic";
import BandwidthChart from "../charts/BandwidthChart";
import ProtocolDonut from "../charts/ProtocolDonut";
import { bandwidthSeries, formatBytes, MAX_POINTS, protocolTotals, topSources } from "../lib/traffic";
import { useAppStore } from "../store/app";

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-borda bg-superficie p-4">
      <p className="text-xs text-suave">{label}</p>
      <p className="mt-1 text-2xl font-semibold tabular-nums">{value}</p>
    </div>
  );
}

function Card({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="min-w-0 rounded-lg border border-borda bg-superficie p-4">
      <h2 className="mb-3 text-sm font-medium text-suave">{title}</h2>
      {children}
    </section>
  );
}

export default function OverviewPage() {
  const traffic = useAppStore((s) => s.traffic);
  const sniffer = useAppStore((s) => s.engines.sniffer);
  const targets = useAppStore((s) => s.targets);
  const targetsLoaded = useAppStore((s) => s.targetsLoaded);
  const [loadError, setLoadError] = useState(false);

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
    <div className="space-y-4">
      <h1 className="text-xl font-semibold">Geral</h1>

      {sniffer === "sem-permissao" && (
        <p role="alert" className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm text-amber-200">
          Captura desativada: o Omni está sem permissão de captura (CAP_NET_RAW).
        </p>
      )}
      {targetsLoaded && targets.length === 0 && (
        <p className="rounded-lg border border-borda bg-superficie px-4 py-3 text-sm text-suave">
          O sniffer observa as portas dos alvos.{" "}
          <Link to="/alvos" className="text-destaque underline">
            Cadastre um alvo
          </Link>{" "}
          para começar a capturar.
        </p>
      )}
      {dropped > 0 && (
        <p className="text-sm text-amber-300">
          A captura descartou {dropped.toLocaleString("pt-BR")} pacotes nos últimos 15 minutos; as métricas estão
          incompletas.
        </p>
      )}
      {loadError && <p className="text-sm text-erro">Não foi possível carregar o histórico de tráfego.</p>}

      <div className="grid gap-4 sm:grid-cols-3">
        <Stat label="Banda agora" value={last ? `${formatBytes(last.bytes)}/s` : "—"} />
        <Stat label="Pacotes por segundo" value={last ? last.pacotes.toLocaleString("pt-BR") : "—"} />
        <Stat label="Total em 15 minutos" value={formatBytes(total)} />
      </div>

      <Card title="Banda nos últimos 15 minutos">
        <BandwidthChart points={series} />
      </Card>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card title="Protocolos">
          <ProtocolDonut data={protocols} />
        </Card>
        <Card title="IPs de origem com mais tráfego">
          {sources.length === 0 ? (
            <p className="text-sm text-suave">Sem tráfego capturado ainda.</p>
          ) : (
            <table className="w-full text-sm">
              <thead className="text-left text-xs text-suave">
                <tr>
                  <th className="pb-2 font-medium">IP</th>
                  <th className="pb-2 text-right font-medium">Bytes</th>
                  <th className="pb-2 text-right font-medium">Pacotes</th>
                </tr>
              </thead>
              <tbody>
                {sources.map((s) => (
                  <tr key={s.ip} className="border-t border-borda">
                    <td className="py-1.5 font-mono text-xs break-all">{s.ip}</td>
                    <td className="py-1.5 text-right tabular-nums">{formatBytes(s.bytes)}</td>
                    <td className="py-1.5 text-right tabular-nums">{s.pacotes.toLocaleString("pt-BR")}</td>
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
