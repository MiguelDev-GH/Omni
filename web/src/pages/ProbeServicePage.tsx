import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router";
import { listHistory } from "../api/targets";
import CountChart from "../charts/CountChart";
import { Card, Notice, PageHeader, Stat } from "../components/Card";
import { summarizeProbes } from "../lib/probes";
import { portKey, useAppStore, type PortStatus } from "../store/app";

type Kind = "dns" | "smtp";

// Os mesmos limites das regras dns_latencia e smtp_latencia do backend.
const config = {
  dns: { title: "DNS", field: "porta_dns", limits: { aviso: 200, critico: 1000 } },
  smtp: { title: "SMTP", field: "porta_smtp", limits: { aviso: 2000, critico: 5000 } },
} as const;

const TABLE_ROWS = 20;

const percent = (v: number | null) => (v === null ? "—" : `${v.toLocaleString("pt-BR", { maximumFractionDigits: 0 })}%`);
const ms = (v: number | null) => (v === null ? "—" : `${v.toLocaleString("pt-BR", { maximumFractionDigits: 1 })} ms`);
const toneFor = (v: number | null, limits: { aviso: number; critico: number }) =>
  v === null ? undefined : v >= limits.critico ? "text-erro" : v >= limits.aviso ? "text-aviso" : undefined;
const clock = (iso: string) => new Date(iso).toLocaleTimeString("pt-BR");

// ProbeServicePage é a tela de um serviço testado por sonda ativa (DNS ou
// SMTP) na porta do alvo selecionado.
export default function ProbeServicePage({ kind }: { kind: Kind }) {
  const { title, field, limits } = config[kind];
  const target = useAppStore((s) => s.targets.find((t) => t.id === s.selectedTargetId));
  const targetsLoaded = useAppStore((s) => s.targetsLoaded);
  const port = target?.[field];
  const live = useAppStore((s) => (target && port ? s.targetStatus[portKey(target.id, port)] : undefined));
  // O histórico e a falha guardam o alvo a que pertencem: trocar de alvo não
  // mostra os números do anterior.
  const targetId = target?.id;
  const [loaded, setLoaded] = useState<{ id: number; list: PortStatus[] } | null>(null);
  const [failedId, setFailedId] = useState<number | null>(null);
  const history = loaded && loaded.id === targetId ? loaded.list : null;
  const loadError = targetId !== undefined && failedId === targetId;

  useEffect(() => {
    if (targetId === undefined || !port) return;
    let current = true;
    listHistory(targetId).then(
      (list) => {
        if (!current) return;
        setLoaded({ id: targetId, list });
        setFailedId(null);
      },
      () => current && setFailedId(targetId),
    );
    return () => {
      current = false;
    };
  }, [targetId, port]);

  // O histórico da porta, continuado pelo último STATUS_ALVOS ao vivo.
  const results = useMemo(() => {
    const list = (history ?? []).filter((r) => r.porta === port && r.tipo === kind);
    const last = list[list.length - 1];
    if (live && live.tipo === kind && (!last || Date.parse(live.hora) > Date.parse(last.hora))) list.push(live);
    return list;
  }, [history, live, port, kind]);

  const summary = useMemo(() => summarizeProbes(results), [results]);
  const latency = useMemo(() => results.filter((r) => r.no_ar).map((r) => ({ t: Date.parse(r.hora), n: r.latencia_ms })), [results]);

  if (targetsLoaded && (!target || !port)) {
    return (
      <div className="space-y-8">
        <PageHeader title={title} />
        <Notice tone="info">
          {target ? `O alvo ${target.nome} não tem porta ${title}.` : "Nenhum alvo selecionado."}{" "}
          <Link to="/alvos" className="link">
            Cadastre a porta {title} em Alvos
          </Link>{" "}
          para acompanhar o serviço.
        </Notice>
      </div>
    );
  }

  const ultimo = summary.ultimo;
  return (
    <div className="space-y-8">
      <PageHeader title={title}>
        {kind === "dns"
          ? `Resolução do domínio ${target?.dominio ?? ""} na porta ${port}, testada a cada rodada do poller.`
          : `Conversa SMTP na porta ${port} (saudação, EHLO e destinatário postmaster), sem enviar e-mail.`}{" "}
        Os números cobrem as últimas 24 horas.
      </PageHeader>

      {loadError && <p className="text-sm text-erro">Não foi possível carregar o histórico do {title}.</p>}

      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4 sm:gap-6">
        <Stat label="Disponibilidade" value={percent(summary.disponibilidade)} />
        <Stat label="Latência média" value={ms(summary.latenciaMedia)} tone={toneFor(summary.latenciaMedia, limits)} />
        <Stat label="Latência p95" value={ms(summary.latenciaP95)} tone={toneFor(summary.latenciaP95, limits)} />
        <Stat label="Taxa de falhas" value={percent(summary.taxaFalhas)} />
      </div>

      {ultimo && (
        <p className="text-sm text-suave">
          Última verificação às {clock(ultimo.hora)}:{" "}
          <span className={ultimo.no_ar ? (ultimo.degradado ? "text-aviso" : "text-texto") : "text-erro"}>
            {ultimo.no_ar ? (ultimo.detalhe ?? "respondeu") : (ultimo.erro ?? "fora do ar")}
          </span>
          {kind === "smtp" && ultimo.starttls !== undefined && (
            <>
              {" · "}
              <span className={ultimo.starttls ? "text-texto" : "text-aviso"}>
                {ultimo.starttls ? "Com STARTTLS" : "Sem STARTTLS"}
              </span>
            </>
          )}
        </p>
      )}

      <Card title="Latência da sonda (ms)">
        <CountChart id={`latencia-${kind}`} points={latency} label="Latência (ms)" empty="Sem verificações ainda." />
      </Card>

      <Card title="Últimas verificações" plane>
        {results.length === 0 ? (
          <p className="text-sm text-suave">Sem verificações ainda.</p>
        ) : (
          <table className="tabela">
            <thead>
              <tr>
                <th>Hora</th>
                <th>Estado</th>
                <th className="text-right">Latência</th>
                <th>Detalhe</th>
              </tr>
            </thead>
            <tbody>
              {results
                .slice(-TABLE_ROWS)
                .reverse()
                .map((r) => (
                  <tr key={r.hora}>
                    <td className="font-mono text-xs">{clock(r.hora)}</td>
                    <td className={r.no_ar ? (r.degradado ? "text-aviso" : "") : "text-erro"}>
                      {r.no_ar ? (r.degradado ? "Com erro" : "No ar") : "Fora do ar"}
                    </td>
                    <td className="text-right font-mono text-xs tabular-nums">{r.no_ar ? ms(r.latencia_ms) : "—"}</td>
                    <td className="text-xs break-all">{r.no_ar ? (r.detalhe ?? "") : (r.erro ?? "")}</td>
                  </tr>
                ))}
            </tbody>
          </table>
        )}
      </Card>
    </div>
  );
}
