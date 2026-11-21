import { useEffect, useState } from "react";
import { Link } from "react-router";
import { ApiError } from "../api/client";
import { listScans, listTools, scanFindings, startScan, type Finding, type Scan, type Tool } from "../api/scans";
import { Card } from "../components/Card";
import FindingsTable from "../components/FindingsTable";
import ScanConsole from "../components/ScanConsole";
import { useAppStore } from "../store/app";

// statusLabels traduz o estado de uma varredura para a interface.
export const statusLabels: Record<Scan["status"], string> = {
  fila: "Na fila",
  rodando: "Rodando",
  concluida: "Concluída",
  falhou: "Falhou",
};

const when = (iso: string) => new Date(iso).toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "medium" });

export default function ArsenalPage() {
  const targets = useAppStore((s) => s.targets);
  const selected = useAppStore((s) => s.selectedTargetId);
  const selectTarget = useAppStore((s) => s.selectTarget);
  const docker = useAppStore((s) => s.engines.docker);
  const scanEvents = useAppStore((s) => s.scanEvents);

  const [tools, setTools] = useState<Tool[]>([]);
  const [chosen, setChosen] = useState<string[]>(["nmap"]);
  const [scans, setScans] = useState<Scan[]>([]);
  const [current, setCurrent] = useState<number | null>(null);
  const [loaded, setLoaded] = useState<{ scanId: number; list: Finding[] } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    listTools().then(setTools, () => setError("Não foi possível carregar as ferramentas."));
  }, []);

  // Recarrega a lista quando o alvo muda ou uma varredura começa ou termina.
  useEffect(() => {
    if (selected === null) return;
    listScans(selected).then(setScans, () => setError("Não foi possível carregar as varreduras."));
  }, [selected, scanEvents]);

  const shown = scans.find((s) => s.id === current) ?? scans[0];
  const doneId = shown?.status === "concluida" ? shown.id : null;
  useEffect(() => {
    if (doneId === null) return;
    scanFindings(doneId).then(
      (list) => setLoaded({ scanId: doneId, list }),
      () => setError("Não foi possível carregar os achados."),
    );
  }, [doneId]);
  const found = loaded && loaded.scanId === doneId ? loaded.list : null;

  async function start() {
    if (selected === null) return;
    setBusy(true);
    setError(null);
    try {
      const { id } = await startScan(selected, chosen);
      setCurrent(id);
      setScans(await listScans(selected));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Não foi possível iniciar a varredura.");
    } finally {
      setBusy(false);
    }
  }

  const toggle = (name: string) =>
    setChosen((c) => (c.includes(name) ? c.filter((n) => n !== name) : [...c, name]));
  const running = shown?.status === "rodando" || shown?.status === "fila";

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-baseline gap-3">
        <h1 className="text-xl font-semibold">Arsenal</h1>
        <span className="rounded border border-amber-500/40 bg-amber-500/10 px-2 py-0.5 text-xs text-amber-200">
          Visão interna
        </span>
      </div>
      <p className="text-sm text-suave">
        As ferramentas rodam em containers Docker efêmeros na própria máquina, com 1 CPU e 512 MiB, só contra o
        loopback e o host ou domínio do alvo. Por isso enxergam portas que um atacante externo não vê.
      </p>

      {docker === "indisponivel" && (
        <p role="alert" className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm text-amber-200">
          Docker indisponível: inicie o Docker para usar o Arsenal. O resto do Omni continua funcionando.
        </p>
      )}

      <Card title="Nova varredura">
        {targets.length === 0 ? (
          <p className="text-sm text-suave">
            <Link to="/alvos" className="text-destaque underline">
              Cadastre um alvo
            </Link>{" "}
            para usar o Arsenal.
          </p>
        ) : (
          <div className="space-y-3">
            <div className="flex flex-wrap items-center gap-2 text-sm">
              <label htmlFor="alvo-arsenal" className="text-suave">
                Alvo
              </label>
              <select
                id="alvo-arsenal"
                value={selected ?? ""}
                onChange={(e) => selectTarget(Number(e.target.value))}
                className="rounded border border-borda bg-fundo px-2 py-1"
              >
                {targets.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.nome} ({t.host})
                  </option>
                ))}
              </select>
            </div>
            <fieldset className="space-y-1.5">
              <legend className="mb-1 text-sm text-suave">Ferramentas</legend>
              {tools.map((t) => (
                <label key={t.nome} className="flex items-start gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={chosen.includes(t.nome)}
                    onChange={() => toggle(t.nome)}
                    className="mt-1"
                  />
                  <span>
                    <span className="font-medium">{t.nome}</span>{" "}
                    <span className="text-suave">— {t.descricao}</span>
                  </span>
                </label>
              ))}
            </fieldset>
            <button
              type="button"
              onClick={() => void start()}
              disabled={busy || chosen.length === 0 || selected === null}
              className="rounded bg-emerald-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-500 disabled:opacity-50"
            >
              {busy ? "Iniciando..." : "Iniciar varredura"}
            </button>
            {error && (
              <p role="alert" className="text-sm text-erro">
                {error}
              </p>
            )}
          </div>
        )}
      </Card>

      <Card title={shown ? `Varredura #${shown.id} · ${statusLabels[shown.status]}` : "Saída ao vivo"}>
        <ScanConsole scanId={shown?.id ?? null} />
        {shown?.status === "falhou" && shown.erro && <p className="mt-2 text-sm text-erro">{shown.erro}</p>}
      </Card>

      <Card title="Achados (visão interna)">
        {running ? (
          <p className="text-sm text-suave">A varredura ainda está rodando.</p>
        ) : found === null ? (
          <p className="text-sm text-suave">Nenhuma varredura concluída para este alvo.</p>
        ) : (
          <>
            <FindingsTable list={found} label="Achados" />
            <Link to={`/relatorios?varredura=${shown?.id}`} className="mt-3 inline-block text-sm text-destaque underline">
              Comparar com a varredura anterior
            </Link>
          </>
        )}
      </Card>

      {scans.length > 0 && (
        <Card title="Varreduras deste alvo">
          <ul className="divide-y divide-borda text-sm">
            {scans.slice(0, 10).map((s) => (
              <li key={s.id}>
                <button
                  type="button"
                  onClick={() => setCurrent(s.id)}
                  aria-current={s.id === shown?.id}
                  className="flex w-full flex-wrap justify-between gap-2 py-1.5 text-left hover:text-texto aria-[current=true]:text-destaque"
                >
                  <span>
                    #{s.id} · {s.ferramentas.join(", ")}
                  </span>
                  <span className="text-suave">
                    {statusLabels[s.status]} · {when(s.criada_em)}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </Card>
      )}
    </div>
  );
}
