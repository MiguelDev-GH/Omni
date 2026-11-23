import { useEffect, useMemo, useState } from "react";
import { listSystem } from "../api/system";
import UsageChart, { type UsagePoint } from "../charts/UsageChart";
import { Card, PageHeader, Stat } from "../components/Card";
import EngineBadge, { engineNames } from "../components/EngineBadge";
import Select from "../components/Select";
import { useAppStore, type SystemMetrics } from "../store/app";

type Period = "15m" | "1h" | "24h";

const periods: { value: Period; label: string; ms: number }[] = [
  { value: "15m", label: "Últimos 15 minutos", ms: 15 * 60_000 },
  { value: "1h", label: "Última hora", ms: 60 * 60_000 },
  { value: "24h", label: "Últimas 24 horas", ms: 24 * 60 * 60_000 },
];

const periodMs = (p: Period) => periods.find((o) => o.value === p)!.ms;

// Os mesmos limites das regras de alerta do backend.
const charts = [
  { key: "cpu", title: "CPU", limits: { aviso: 80, critico: 95 } },
  { key: "memoria", title: "Memória", limits: { aviso: 85, critico: 95 } },
  { key: "disco", title: "Disco", limits: { aviso: 85, critico: 95 } },
] as const;

const percent = (v: number | undefined) =>
  v === undefined ? "—" : `${v.toLocaleString("pt-BR", { maximumFractionDigits: 1 })}%`;

// toneFor pinta o valor atual quando ele passa de um limite.
const toneFor = (v: number | undefined, limits: { aviso: number; critico: number }) =>
  v === undefined ? undefined : v >= limits.critico ? "text-erro" : v >= limits.aviso ? "text-aviso" : undefined;

export default function SystemPage() {
  const live = useAppStore((s) => s.system);
  const engines = useAppStore((s) => s.engines);
  const [period, setPeriod] = useState<Period>("15m");
  // O histórico anterior continua na tela enquanto o do novo período carrega.
  const [samples, setSamples] = useState<SystemMetrics[] | null>(null);
  const [failed, setFailed] = useState<Period | null>(null);
  const loadError = failed === period;

  // Cada período refaz a busca; uma resposta antiga não sobrescreve a nova.
  useEffect(() => {
    let current = true;
    listSystem(new Date(Date.now() - periodMs(period))).then(
      (list) => {
        if (!current) return;
        setSamples(list);
        setFailed(null);
      },
      () => current && setFailed(period),
    );
    return () => {
      current = false;
    };
  }, [period]);

  // As leituras ao vivo de METRICAS_SISTEMA continuam a série carregada.
  useEffect(
    () =>
      useAppStore.subscribe((st, before) => {
        const m = st.system;
        if (!m || m === before.system) return;
        setSamples((prev) => {
          if (prev === null) return prev;
          const last = prev[prev.length - 1];
          if (last && Date.parse(last.hora) >= Date.parse(m.hora)) return prev;
          const oldest = Date.parse(m.hora) - periodMs(period);
          return [...prev.filter((x) => Date.parse(x.hora) >= oldest), m];
        });
      }),
    [period],
  );

  const series = useMemo(() => {
    const list = samples ?? [];
    const of = (key: "cpu" | "memoria" | "disco"): UsagePoint[] =>
      list.map((s) => ({ t: Date.parse(s.hora), v: s[key] }));
    return { cpu: of("cpu"), memoria: of("memoria"), disco: of("disco") };
  }, [samples]);

  const now = live ?? samples?.[samples.length - 1];

  return (
    <div className="space-y-8">
      <PageHeader
        title="Sistema"
        aside={
          <Select
            label="Período"
            value={period}
            onChange={setPeriod}
            options={periods.map(({ value, label }) => ({ value, label }))}
          />
        }
      >
        CPU, memória e disco da máquina onde o Omni roda, com as linhas dos limites de alerta. As medições ficam guardadas
        por 24 horas.
      </PageHeader>

      {loadError && <p className="text-sm text-erro">Não foi possível carregar o histórico do sistema.</p>}

      <div className="grid grid-cols-3 gap-4 sm:gap-6">
        {charts.map((c) => (
          <Stat key={c.key} label={`${c.title} agora`} value={percent(now?.[c.key])} tone={toneFor(now?.[c.key], c.limits)} />
        ))}
      </div>

      <section aria-labelledby="motores" className="space-y-2">
        <h2 id="motores" className="text-sm font-medium text-texto">
          Motores
        </h2>
        <ul className="flex flex-wrap gap-x-6 gap-y-2 text-sm text-suave">
          {engineNames.map((name) => (
            <li key={name}>
              <EngineBadge name={name} state={engines[name]} />
            </li>
          ))}
        </ul>
      </section>

      <div className="grid gap-x-10 gap-y-8 xl:grid-cols-3">
        {charts.map((c) => (
          <Card key={c.key} title={c.title}>
            <UsageChart id={`sistema-${c.key}`} label={c.title} points={series[c.key]} limits={c.limits} />
          </Card>
        ))}
      </div>
    </div>
  );
}
