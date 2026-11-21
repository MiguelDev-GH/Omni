import { useEffect, useMemo, useState } from "react";
import {
  formatAlertValue,
  getAlerts,
  levelLabel,
  levelRank,
  type AlertRule,
  type AlertStatus,
  type Level,
} from "../api/alerts";
import { Card } from "../components/Card";
import { useAppStore } from "../store/app";

const REFRESH_MS = 10_000;

const serviceTitle: Record<string, string> = {
  web: "Web server",
  banco: "Banco de dados",
  dns: "DNS",
  smtp: "SMTP",
  sistema: "Sistema",
};
const serviceOrder = ["web", "banco", "dns", "smtp", "sistema"];

const levelDot: Record<Level, string> = {
  verde: "bg-emerald-500",
  amarelo: "bg-amber-400",
  vermelho: "bg-red-500",
};

const levelText: Record<Level, string> = {
  verde: "text-emerald-400",
  amarelo: "text-amber-400",
  vermelho: "text-red-400",
};

const timeFmt = new Intl.DateTimeFormat("pt-BR", { dateStyle: "short", timeStyle: "medium" });
const formatTime = (iso: string) => timeFmt.format(new Date(iso));

export function LevelBadge({ level }: { level: Level }) {
  return (
    <span className={`inline-flex items-center gap-1.5 text-xs font-medium ${levelText[level]}`}>
      <span aria-hidden="true" className={`size-2 rounded-full ${levelDot[level]}`} />
      {levelLabel[level]}
    </span>
  );
}

function worst(list: AlertStatus[]): Level {
  return list.reduce<Level>((w, s) => (levelRank[s.nivel] > levelRank[w] ? s.nivel : w), "verde");
}

function limits(regra: string, s: { limite_amarelo: number; limite_vermelho: number; unidade: string }) {
  if (regra === "disponibilidade") return "resposta com erro é amarelo; fora do ar é vermelho";
  if (regra === "smtp_sem_starttls") return "sem STARTTLS é amarelo";
  return `amarelo ≥ ${formatAlertValue(regra, s.limite_amarelo, s.unidade)} · vermelho ≥ ${formatAlertValue(regra, s.limite_vermelho, s.unidade)}`;
}

function ServiceCard({ service, list }: { service: string; list: AlertStatus[] }) {
  const level = worst(list);
  return (
    <section aria-label={serviceTitle[service] ?? service} className="min-w-0 rounded-lg border border-borda bg-superficie p-4">
      <div className="mb-3 flex items-center justify-between gap-2">
        <h2 className="text-sm font-medium text-suave">{serviceTitle[service] ?? service}</h2>
        {list.length > 0 && <LevelBadge level={level} />}
      </div>
      {list.length === 0 ? (
        <p className="text-sm text-suave">Sem medições ainda.</p>
      ) : (
        <ul className="space-y-2 text-sm">
          {list.map((s) => (
            <li key={`${s.regra}|${s.recurso}`} className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <p className="truncate">
                  {s.nome}
                  {s.recurso && <span className="text-suave"> · {s.recurso}</span>}
                </p>
                <p className="text-xs text-zinc-500">{limits(s.regra, s)}</p>
              </div>
              <div className="shrink-0 text-right">
                <p className="tabular-nums">{formatAlertValue(s.regra, s.valor, s.unidade)}</p>
                <LevelBadge level={s.nivel} />
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

export default function AlertsPage() {
  const statusMap = useAppStore((s) => s.alertStatus);
  const history = useAppStore((s) => s.alertHistory);
  const [rules, setRules] = useState<AlertRule[]>([]);
  const [email, setEmail] = useState<{ ativo: boolean; destinatarios: number } | null>(null);
  const [loadError, setLoadError] = useState(false);

  // O nível muda pelo WebSocket; a recarga periódica atualiza os valores atuais.
  useEffect(() => {
    const load = () =>
      getAlerts().then(
        (ov) => {
          useAppStore.getState().setAlertsOverview(ov.estado, ov.historico);
          setRules(ov.regras);
          setEmail(ov.email);
          setLoadError(false);
        },
        () => setLoadError(true),
      );
    void load();
    const id = setInterval(load, REFRESH_MS);
    return () => clearInterval(id);
  }, []);

  const statuses = useMemo(() => Object.values(statusMap), [statusMap]);
  const byService = useMemo(() => {
    const groups: Record<string, AlertStatus[]> = Object.fromEntries(serviceOrder.map((s) => [s, []]));
    for (const s of statuses) (groups[s.servico] ??= []).push(s);
    for (const list of Object.values(groups)) {
      list.sort((a, b) => levelRank[b.nivel] - levelRank[a.nivel] || a.recurso.localeCompare(b.recurso) || a.nome.localeCompare(b.nome));
    }
    return groups;
  }, [statuses]);
  const counts = useMemo(() => {
    const c: Record<Level, number> = { verde: 0, amarelo: 0, vermelho: 0 };
    for (const s of statuses) c[s.nivel]++;
    return c;
  }, [statuses]);
  const overall = worst(statuses);
  const services = [...serviceOrder, ...Object.keys(byService).filter((s) => !serviceOrder.includes(s))];

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-xl font-semibold">Alertas</h1>
        {statuses.length > 0 && (
          <p className="text-sm">
            Situação geral: <LevelBadge level={overall} />
          </p>
        )}
      </div>
      <p className="text-sm text-suave">
        Cada métrica é comparada com dois limites. Verde é o normal, amarelo pede observação e vermelho indica serviço
        degradado ou fora do ar. Um nível só muda depois que a condição se mantém pelo tempo da regra, para um pico
        isolado não virar alerta.
      </p>
      {loadError && <p className="text-sm text-erro">Não foi possível carregar os alertas.</p>}
      {email && (
        <p role="note" className={`text-sm ${email.ativo ? "text-suave" : "text-amber-400"}`}>
          {email.ativo
            ? `Notificação por e-mail ativa para ${email.destinatarios} ${email.destinatarios === 1 ? "destinatário" : "destinatários"}.`
            : "Notificação por e-mail desligada: defina OMNI_SMTP_HOST e OMNI_ALERTA_EMAILS ao iniciar o Omni."}
        </p>
      )}

      <div className="grid gap-4 sm:grid-cols-3">
        {(["vermelho", "amarelo", "verde"] as const).map((l) => (
          <div key={l} className="rounded-lg border border-borda bg-superficie p-4">
            <p className="text-xs text-suave">{levelLabel[l]}</p>
            <p className={`mt-1 text-2xl font-semibold tabular-nums ${levelText[l]}`}>{counts[l]}</p>
          </div>
        ))}
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        {services.map((s) => (
          <ServiceCard key={s} service={s} list={byService[s] ?? []} />
        ))}
      </div>

      <Card title="Histórico de mudanças de nível">
        {history.length === 0 ? (
          <p className="text-sm text-suave">Nenhuma mudança de nível desde que o Omni foi iniciado.</p>
        ) : (
          <div className="overflow-x-auto">
            <table aria-label="Histórico de alertas" className="w-full text-sm">
              <thead className="text-left text-xs text-suave">
                <tr>
                  <th className="py-1 pr-3 font-medium">Hora</th>
                  <th className="py-1 pr-3 font-medium">Nível</th>
                  <th className="py-1 pr-3 font-medium">Métrica</th>
                  <th className="py-1 pr-3 font-medium">Recurso</th>
                  <th className="py-1 pr-3 font-medium">Valor</th>
                  <th className="py-1 font-medium">Detalhe</th>
                </tr>
              </thead>
              <tbody>
                {history.map((a) => (
                  <tr key={a.id} className="border-t border-borda align-top">
                    <td className="py-1.5 pr-3 whitespace-nowrap text-suave tabular-nums">{formatTime(a.hora)}</td>
                    <td className="py-1.5 pr-3 whitespace-nowrap">
                      <LevelBadge level={a.nivel} />
                      <span className="text-xs text-zinc-500"> (era {levelLabel[a.nivel_anterior].toLowerCase()})</span>
                    </td>
                    <td className="py-1.5 pr-3">{a.nome}</td>
                    <td className="py-1.5 pr-3 font-mono text-xs">{a.recurso || serviceTitle[a.servico] || a.servico}</td>
                    <td className="py-1.5 pr-3 whitespace-nowrap tabular-nums">{formatAlertValue(a.regra, a.valor, a.unidade)}</td>
                    <td className="py-1.5 break-words text-suave">{a.detalhe ?? ""}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      {rules.length > 0 && (
        <Card title="Regras em uso">
          <ul className="grid gap-1 text-sm sm:grid-cols-2">
            {rules.map((r) => (
              <li key={r.id}>
                <span>{r.nome}</span>
                <span className="text-xs text-zinc-500"> · {serviceTitle[r.servico] ?? r.servico} · {limits(r.id, r)}</span>
              </li>
            ))}
          </ul>
        </Card>
      )}
    </div>
  );
}
