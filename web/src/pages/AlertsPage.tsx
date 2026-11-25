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
import { Card, PageHeader } from "../components/Card";
import { useAppStore } from "../store/app";

const REFRESH_MS = 10_000;
// GREEN_SHOWN é quantos itens verdes cada cartão mostra antes do botão
// "Mostrar todos"; amarelos e vermelhos aparecem sempre.
const GREEN_SHOWN = 5;
// HISTORY_PAGE é quantas mudanças de nível o histórico mostra por vez.
const HISTORY_PAGE = 20;

const serviceTitle: Record<string, string> = {
  web: "Web server",
  banco: "Banco de dados",
  dns: "DNS",
  smtp: "SMTP",
  sistema: "Sistema",
  seguranca: "Segurança",
};
const serviceOrder = ["seguranca", "web", "banco", "dns", "smtp", "sistema"];

const levelDot: Record<Level, string> = {
  verde: "bg-destaque",
  amarelo: "bg-aviso",
  vermelho: "bg-erro animate-pulso text-erro",
};

const levelText: Record<Level, string> = {
  verde: "text-destaque-forte",
  amarelo: "text-aviso",
  vermelho: "text-erro",
};

const timeFmt = new Intl.DateTimeFormat("pt-BR", { dateStyle: "short", timeStyle: "medium" });
const formatTime = (iso: string) => timeFmt.format(new Date(iso));

export function LevelBadge({ level }: { level: Level }) {
  return (
    <span className={`inline-flex items-center gap-1.5 text-xs font-medium ${levelText[level]}`}>
      <span aria-hidden="true" className={`size-1.5 rounded-full ${levelDot[level]}`} />
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
  if (regra === "seguranca_arquivo_alterado") return "alterado é amarelo por 5 min; removido é vermelho";
  return `amarelo ≥ ${formatAlertValue(regra, s.limite_amarelo, s.unidade)} · vermelho ≥ ${formatAlertValue(regra, s.limite_vermelho, s.unidade)}`;
}

function ServiceCard({ service, list }: { service: string; list: AlertStatus[] }) {
  const [showAll, setShowAll] = useState(false);
  const level = worst(list);
  const alerting = list.filter((s) => s.nivel !== "verde");
  const green = list.filter((s) => s.nivel === "verde");
  const hidden = showAll ? 0 : Math.max(green.length - GREEN_SHOWN, 0);
  const visible = showAll ? list : [...alerting, ...green.slice(0, GREEN_SHOWN)];
  return (
    <section aria-label={serviceTitle[service] ?? service} className="min-w-0 rounded-2xl bg-superficie/80 p-5">
      <div className="mb-3 flex items-center justify-between gap-2">
        <h2 className="text-sm font-medium">{serviceTitle[service] ?? service}</h2>
        {list.length > 0 && <LevelBadge level={level} />}
      </div>
      {list.length === 0 ? (
        <p className="text-sm text-suave">Sem medições ainda.</p>
      ) : (
        <ul className="space-y-3 text-sm">
          {visible.map((s) => (
            <li key={`${s.regra}|${s.recurso}`} className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <p className="truncate">
                  {s.nome}
                  {s.recurso && <span className="text-suave"> · {s.recurso}</span>}
                </p>
                <p className="text-xs text-apagado">{limits(s.regra, s)}</p>
              </div>
              <div className="shrink-0 text-right">
                <p className="font-mono text-xs tabular-nums">{formatAlertValue(s.regra, s.valor, s.unidade)}</p>
                <LevelBadge level={s.nivel} />
              </div>
            </li>
          ))}
        </ul>
      )}
      {(hidden > 0 || showAll) && green.length > GREEN_SHOWN && (
        <button
          type="button"
          onClick={() => setShowAll((v) => !v)}
          className="mt-3 text-xs text-suave underline-offset-2 hover:text-texto hover:underline"
        >
          {showAll ? "Mostrar menos" : `Mostrar todos (+${hidden} verdes)`}
        </button>
      )}
    </section>
  );
}

export default function AlertsPage() {
  const statusMap = useAppStore((s) => s.alertStatus);
  const history = useAppStore((s) => s.alertHistory);
  const [historyShown, setHistoryShown] = useState(HISTORY_PAGE);
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
    <div className="space-y-8">
      <PageHeader
        title="Alertas"
        aside={
          statuses.length > 0 && (
            <p className="text-sm">
              Situação geral: <LevelBadge level={overall} />
            </p>
          )
        }
      >
        Cada métrica é comparada com dois limites. Verde é o normal, amarelo pede observação e vermelho indica serviço
        degradado ou fora do ar. Um nível só muda depois que a condição se mantém pelo tempo da regra, para um pico
        isolado não virar alerta.
      </PageHeader>
      {loadError && <p className="text-sm text-erro">Não foi possível carregar os alertas.</p>}
      {email && (
        <p role="note" className={`-mt-4 text-sm ${email.ativo ? "text-suave" : "text-aviso"}`}>
          {email.ativo
            ? `Notificação por e-mail ativa para ${email.destinatarios} ${email.destinatarios === 1 ? "destinatário" : "destinatários"}.`
            : "Notificação por e-mail desligada: defina OMNI_SMTP_HOST e OMNI_ALERTA_EMAILS ao iniciar o Omni."}
        </p>
      )}

      <div className="grid grid-cols-3 gap-4 sm:gap-6">
        {(["vermelho", "amarelo", "verde"] as const).map((l) => (
          <div key={l} className="relative pl-3.5">
            <span aria-hidden="true" className={`absolute top-1 bottom-1 left-0 w-px ${levelDot[l].split(" ")[0]}`} />
            <p className="text-xs text-suave">{levelLabel[l]}</p>
            <p className={`mt-1 font-mono text-lg font-medium tabular-nums sm:text-2xl ${levelText[l]}`}>{counts[l]}</p>
          </div>
        ))}
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        {services.map((s) => (
          <ServiceCard key={s} service={s} list={byService[s] ?? []} />
        ))}
      </div>

      <Card title="Histórico de mudanças de nível" plane>
        {history.length === 0 ? (
          <p className="text-sm text-suave">Nenhuma mudança de nível desde que o Omni foi iniciado.</p>
        ) : (
          <>
          <div className="overflow-x-auto">
            <table aria-label="Histórico de alertas" className="tabela">
              <thead>
                <tr>
                  <th className="pr-3">Hora</th>
                  <th className="pr-3">Nível</th>
                  <th className="pr-3">Métrica</th>
                  <th className="pr-3">Recurso</th>
                  <th className="pr-3">Valor</th>
                  <th>Detalhe</th>
                </tr>
              </thead>
              <tbody>
                {history.slice(0, historyShown).map((a) => (
                  <tr key={a.id} className="align-top">
                    <td className="pr-3 font-mono text-xs whitespace-nowrap text-suave tabular-nums">{formatTime(a.hora)}</td>
                    <td className="pr-3 whitespace-nowrap">
                      <LevelBadge level={a.nivel} />
                      <span className="text-xs text-apagado"> (era {levelLabel[a.nivel_anterior].toLowerCase()})</span>
                    </td>
                    <td className="pr-3">{a.nome}</td>
                    <td className="pr-3 font-mono text-xs">{a.recurso || serviceTitle[a.servico] || a.servico}</td>
                    <td className="pr-3 font-mono text-xs whitespace-nowrap tabular-nums">{formatAlertValue(a.regra, a.valor, a.unidade)}</td>
                    <td className="break-words text-suave">{a.detalhe ?? ""}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
            {history.length > historyShown && (
              <button
                type="button"
                onClick={() => setHistoryShown((n) => n + HISTORY_PAGE)}
                className="mt-3 text-xs text-suave underline-offset-2 hover:text-texto hover:underline"
              >
                Mostrar mais ({history.length - historyShown} restantes)
              </button>
            )}
          </>
        )}
      </Card>

      {rules.length > 0 && (
        <Card title="Regras em uso" plane>
          <ul className="grid gap-x-8 gap-y-2 text-sm sm:grid-cols-2">
            {rules.map((r) => (
              <li key={r.id}>
                <span>{r.nome}</span>
                <span className="text-xs text-apagado"> · {serviceTitle[r.servico] ?? r.servico} · {limits(r.id, r)}</span>
              </li>
            ))}
          </ul>
        </Card>
      )}
    </div>
  );
}
