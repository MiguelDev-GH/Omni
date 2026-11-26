import { useEffect, useMemo, useState } from "react";
import { getQueue, listDNS, listSMTP, type QueueInfo, type SMTPWindow } from "../api/dnsSmtp";
import CountChart from "../charts/CountChart";
import { Card, Stat } from "../components/Card";
import { formatMs, formatRate, l7Series, lastFresh, lastRate, L7_POINTS, L7_STEP_MS, mergeL7 } from "../lib/l7";
import { useAppStore } from "../store/app";

const HOUR_MS = 60 * 60_000;
const MINUTE_MS = 60_000;
const REFRESH_MS = 30_000;

const percent = (part: number, total: number) =>
  total === 0 ? "—" : `${((part / total) * 100).toLocaleString("pt-BR", { maximumFractionDigits: 0 })}%`;

// sumCounts soma os mapas de contagem das janelas, do maior para o menor e,
// no empate, pelo nome.
export function sumCounts(maps: Record<string, number>[]): { nome: string; total: number }[] {
  const totals = new Map<string, number>();
  for (const m of maps) for (const [k, n] of Object.entries(m)) totals.set(k, (totals.get(k) ?? 0) + n);
  return [...totals.entries()]
    .map(([nome, total]) => ({ nome, total }))
    .sort((a, b) => b.total - a.total || a.nome.localeCompare(b.nome));
}

const within = <T extends { inicio: string }>(list: T[], ms: number, now = Date.now()) =>
  list.filter((w) => now - Date.parse(w.inicio) <= ms);

function CountsTable({ label, rows, head }: { label: string; rows: { nome: string; total: number }[]; head: string }) {
  return (
    <table aria-label={label} className="tabela">
      <thead>
        <tr>
          <th>{head}</th>
          <th className="text-right">Total</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={r.nome}>
            <td className="font-mono text-xs">{r.nome}</td>
            <td className="text-right font-mono text-xs tabular-nums">{r.total.toLocaleString("pt-BR")}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

// DNSTraffic mostra as consultas DNS vistas pelo sniffer nos últimos 15 minutos.
function DNSTraffic() {
  const dns = useAppStore((s) => s.dns);
  const [loadError, setLoadError] = useState(false);

  useEffect(() => {
    listDNS(new Date(Date.now() - L7_POINTS * L7_STEP_MS)).then(
      (list) => useAppStore.getState().addDNS(list),
      () => setLoadError(true),
    );
  }, []);

  const queries = dns.reduce((s, w) => s + w.consultas, 0);
  const failures = dns.reduce((s, w) => s + w.falhas, 0);
  // A janela mais recente com respostas, se for recente: janelas vazias ou só
  // com timeouts não têm tempo de resolução.
  const lastAnswered = lastFresh(dns, (w) => w.consultas - (w.rcodes.TIMEOUT ?? 0) > 0);
  const rcodes = useMemo(() => sumCounts(dns.map((w) => w.rcodes)), [dns]);
  const series = useMemo(() => l7Series(dns, (w) => ({ n: w.consultas }), { n: 0 }), [dns]);

  return (
    <>
      {loadError && <p className="text-sm text-erro">Não foi possível carregar as consultas DNS do tráfego.</p>}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 sm:gap-6">
        <Stat label="Consultas por segundo" value={formatRate(lastRate(dns, (w) => w.consultas))} />
        <Stat label="Falhas em 15 minutos" value={percent(failures, queries)} tone={failures > 0 ? "text-aviso" : undefined} />
        <Stat label="Resolução p95 (10 s)" value={lastAnswered ? formatMs(lastAnswered.p95_ms) : "—"} />
      </div>
      <div className="grid gap-x-10 gap-y-8 xl:grid-cols-2">
        <Card title="Consultas nos últimos 15 minutos">
          <CountChart id="dns-trafego" points={series} label="Consultas DNS" empty="Sem consultas DNS capturadas ainda." />
        </Card>
        <Card title="Respostas" plane>
          {rcodes.length === 0 ? (
            <p className="text-sm text-suave">Sem consultas DNS capturadas ainda.</p>
          ) : (
            <CountsTable label="Respostas DNS" head="Código" rows={rcodes} />
          )}
        </Card>
      </div>
    </>
  );
}

// SMTPTraffic mostra as transações SMTP vistas pelo sniffer e a fila de saída.
function SMTPTraffic() {
  const live = useAppStore((s) => s.smtp);
  const liveQueue = useAppStore((s) => s.queue);
  const [hour, setHour] = useState<SMTPWindow[]>([]);
  const [queue, setQueue] = useState<QueueInfo | null>(null);
  const [loadError, setLoadError] = useState(false);

  // A última hora é recarregada a cada 30 s; os eventos ao vivo completam o intervalo.
  useEffect(() => {
    const load = () => {
      listSMTP(new Date(Date.now() - HOUR_MS)).then(
        (list) => {
          setHour(list);
          setLoadError(false);
        },
        () => setLoadError(true),
      );
      getQueue().then(setQueue, () => setLoadError(true));
    };
    load();
    const id = setInterval(load, REFRESH_MS);
    return () => clearInterval(id);
  }, []);

  const windows = useMemo(() => within(mergeL7Hour(hour, live), HOUR_MS), [hour, live]);
  const attempts = windows.reduce((s, w) => s + w.tentativas, 0);
  const delivered = windows.reduce((s, w) => s + w.entregues, 0);
  const lastMinute = within(windows, MINUTE_MS).reduce((s, w) => s + w.tentativas, 0);
  const lastDelivered = [...windows].reverse().find((w) => w.entregues > 0);
  const codes = useMemo(() => sumCounts(windows.map((w) => w.codigos)), [windows]);
  const recent = windows.slice(-L7_POINTS);
  const series = useMemo(() => l7Series(recent, (w) => ({ n: w.tentativas }), { n: 0 }), [recent]);

  const lastCount = liveQueue ?? queue?.contagens[queue.contagens.length - 1];
  const queueValue =
    queue && !queue.configurado && !liveQueue
      ? "não configurada"
      : lastCount
        ? lastCount.erro
          ? "erro ao contar"
          : lastCount.mensagens.toLocaleString("pt-BR")
        : "—";

  return (
    <>
      {loadError && <p className="text-sm text-erro">Não foi possível carregar os envios SMTP do tráfego.</p>}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 sm:gap-6 lg:grid-cols-5">
        <Stat label="Taxa de entrega" value={percent(delivered, attempts)} />
        <Stat label="Latência de envio p95 (10 s)" value={lastDelivered ? formatMs(lastDelivered.p95_ms) : "—"} />
        <Stat label="E-mails no último minuto" value={lastMinute.toLocaleString("pt-BR")} />
        <Stat label="E-mails na última hora" value={attempts.toLocaleString("pt-BR")} />
        <Stat label="Fila de saída" value={queueValue} />
      </div>
      {lastCount?.erro && <p className="text-sm text-aviso">Não foi possível contar a fila: {lastCount.erro}</p>}
      <div className="grid gap-x-10 gap-y-8 xl:grid-cols-2">
        <Card title="Envios nos últimos 15 minutos">
          <CountChart id="smtp-trafego" points={series} label="Envios SMTP" empty="Sem envios SMTP capturados ainda." />
        </Card>
        <Card title="Recusas na última hora" plane>
          {codes.length === 0 ? (
            <p className="text-sm text-suave">Nenhuma recusa na última hora.</p>
          ) : (
            <CountsTable label="Códigos de recusa" head="Código" rows={codes} />
          )}
        </Card>
      </div>
    </>
  );
}

// mergeL7Hour junta as janelas da última hora às ao vivo, sem repetir o mesmo
// intervalo de 10 s. mergeL7 guarda só 15 minutos, por isso o corte é refeito.
function mergeL7Hour(hour: SMTPWindow[], live: SMTPWindow[]): SMTPWindow[] {
  const recent = mergeL7(hour.slice(-L7_POINTS), live);
  const older = hour.slice(0, Math.max(hour.length - L7_POINTS, 0));
  const firstRecent = recent[0] ? Date.parse(recent[0].inicio) : Infinity;
  return [...older.filter((w) => Date.parse(w.inicio) < firstRecent), ...recent];
}

// TrafficServiceSection é a parte "Tráfego real" das telas de DNS e SMTP: o
// que o sniffer vê na porta do serviço, além da sonda ativa.
export default function TrafficServiceSection({ kind }: { kind: "dns" | "smtp" }) {
  const id = `trafego-${kind}`;
  return (
    <section aria-labelledby={id} className="space-y-6">
      <div className="space-y-1">
        <h2 id={id} className="text-lg font-semibold tracking-tight">
          Tráfego real
        </h2>
        <p className="text-sm text-suave">
          {kind === "dns"
            ? "Consultas DNS por UDP vistas pelo sniffer na porta DNS do alvo. Os nomes consultados não são guardados."
            : "Transações SMTP vistas pelo sniffer na porta SMTP do alvo. Endereços e conteúdo não são guardados, e conexões com STARTTLS ficam de fora depois do aceite."}
        </p>
      </div>
      {kind === "dns" ? <DNSTraffic /> : <SMTPTraffic />}
    </section>
  );
}
