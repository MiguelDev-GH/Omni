import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { isRunning, statusLabels, TargetPicker, when } from "../arsenal/common";
import { missingRequirement, toolViews } from "../arsenal/registry";
import { ApiError } from "../api/client";
import { getReport } from "../api/reports";
import { listScans, listTools, scanFindings, startScan, type Finding, type Scan, type Tool } from "../api/scans";
import { Card, InternalBadge, Notice, PageHeader } from "../components/Card";
import FindingsTable from "../components/FindingsTable";
import ScanConsole from "../components/ScanConsole";
import { useAppStore } from "../store/app";
import NotFoundPage from "./NotFoundPage";

type Loaded = { scanId: number; report: unknown | null; findings: Finding[] };

// ToolPage é a página de uma ferramenta do Arsenal: opções, última varredura
// do alvo selecionado e a visualização própria do relatório dela.
export default function ToolPage() {
  const { ferramenta = "" } = useParams();
  const view = toolViews[ferramenta];
  // A key zera o estado ao trocar de ferramenta pelo menu.
  return view ? <ToolPageFor key={ferramenta} name={ferramenta} /> : <NotFoundPage />;
}

function ToolPageFor({ name }: { name: string }) {
  const view = toolViews[name]!;
  const targets = useAppStore((s) => s.targets);
  const selected = useAppStore((s) => s.selectedTargetId);
  const docker = useAppStore((s) => s.engines.docker);
  const scanEvents = useAppStore((s) => s.scanEvents);
  const target = targets.find((t) => t.id === selected);

  const [tool, setTool] = useState<Tool | null>(null);
  const [scans, setScans] = useState<Scan[]>([]);
  const [current, setCurrent] = useState<number | null>(null);
  const [options, setOptions] = useState<Record<string, string>>({});
  const [loaded, setLoaded] = useState<Loaded | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    listTools().then(
      (list) => setTool(list.find((t) => t.nome === name) ?? null),
      () => setError("Não foi possível carregar as ferramentas."),
    );
  }, [name]);

  // Recarrega quando o alvo muda ou uma varredura começa ou termina.
  useEffect(() => {
    if (selected === null) return;
    listScans(selected).then(
      (list) => setScans(list.filter((s) => s.ferramentas.includes(name))),
      () => setError("Não foi possível carregar as varreduras."),
    );
  }, [selected, scanEvents, name]);

  const shown = scans.find((s) => s.id === current) ?? scans[0];
  const finishedId = shown && !isRunning(shown) ? shown.id : null;
  useEffect(() => {
    if (finishedId === null) return;
    let live = true;
    Promise.all([getReport(finishedId, name), scanFindings(finishedId)]).then(
      ([report, list]) => live && setLoaded({ scanId: finishedId, report, findings: list.filter((f) => f.ferramenta === name) }),
      () => live && setError("Não foi possível carregar o relatório."),
    );
    return () => {
      live = false;
    };
  }, [finishedId, name]);
  const data = loaded && loaded.scanId === finishedId ? loaded : null;

  async function start() {
    if (selected === null) return;
    setBusy(true);
    setError(null);
    try {
      const opts = Object.keys(options).length > 0 ? { [name]: options } : undefined;
      const { id } = await startScan(selected, [name], opts);
      setCurrent(id);
      const list = await listScans(selected);
      setScans(list.filter((s) => s.ferramentas.includes(name)));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Não foi possível iniciar a varredura.");
    } finally {
      setBusy(false);
    }
  }

  const missing = tool ? missingRequirement(tool, target) : null;
  const running = isRunning(shown);

  return (
    <div className="space-y-8">
      <PageHeader title={view.title} aside={<InternalBadge />}>
        {tool?.descricao}
        {tool && (
          <>
            {" "}
            · roda em container Docker efêmero (<span className="font-mono text-xs">{tool.imagem}</span>).{" "}
            <Link to="/arsenal" className="link">
              Ver todas as ferramentas
            </Link>
          </>
        )}
      </PageHeader>

      {docker === "indisponivel" && (
        <Notice role="alert">
          Docker indisponível: inicie o Docker para usar o Arsenal. O resto do Omni continua funcionando.
        </Notice>
      )}

      <Card title="Nova varredura" plane>
        {targets.length === 0 ? (
          <p className="text-sm text-suave">
            <Link to="/alvos" className="link">
              Cadastre um alvo
            </Link>{" "}
            para usar o Arsenal.
          </p>
        ) : (
          <div className="space-y-5">
            <TargetPicker id={`alvo-${name}`} />
            {view.Options && target && <view.Options value={options} onChange={setOptions} target={target} />}
            {missing && <p className="text-sm text-aviso">{missing}.</p>}
            <button
              type="button"
              onClick={() => void start()}
              disabled={busy || selected === null || missing !== null || running}
              className="botao"
            >
              {busy ? "Iniciando..." : running ? "Varredura em andamento" : "Iniciar varredura"}
            </button>
            {error && (
              <p role="alert" className="text-sm text-erro">
                {error}
              </p>
            )}
          </div>
        )}
      </Card>

      <Card
        title={view.tagline}
        action={
          shown && (
            <span className="font-mono text-xs text-suave">
              #{shown.id} · {statusLabels[shown.status]} · {when(shown.criada_em)}
            </span>
          )
        }
      >
        {!shown ? (
          <p className="text-sm text-suave">Nenhuma varredura do {view.title} para este alvo.</p>
        ) : running ? (
          <p className="text-sm text-suave">A varredura ainda está rodando; acompanhe a saída abaixo.</p>
        ) : data === null ? (
          <p className="text-sm text-suave">Carregando o relatório...</p>
        ) : (
          // Uma varredura com várias ferramentas pode falhar por causa de
          // outra: o relatório desta, se existir, continua valendo.
          <div className="space-y-4">
            {shown.status === "falhou" && <p className="text-sm text-erro">{shown.erro || "A varredura falhou."}</p>}
            {data.report !== null ? (
              <view.Report report={data.report} findings={data.findings} scan={shown} />
            ) : (
              <>
                {shown.status === "concluida" && <Notice tone="info">Relatório indisponível para esta varredura.</Notice>}
                {(shown.status === "concluida" || data.findings.length > 0) && (
                  <FindingsTable list={data.findings} label="Achados" />
                )}
              </>
            )}
          </div>
        )}
        {shown && !running && (
          <Link to={`/relatorios?varredura=${shown.id}`} className="link mt-5 inline-block text-sm">
            Comparar com a varredura anterior
          </Link>
        )}
      </Card>

      {shown && (
        // A key remonta o painel ao mudar de varredura ou de estado, para ele
        // abrir sozinho só enquanto a varredura roda.
        <details key={`${shown.id}-${running}`} open={running} className="group">
          <summary className="cursor-pointer text-sm font-medium text-texto">Saída da ferramenta</summary>
          <div className="mt-3">
            <ScanConsole scanId={shown.id} finished={!running} />
          </div>
        </details>
      )}

      {scans.length > 1 && (
        <Card title={`Varreduras do ${view.title} neste alvo`} plane>
          <ul className="-mx-2 space-y-px text-sm">
            {scans.slice(0, 10).map((s) => (
              <li key={s.id}>
                <button
                  type="button"
                  onClick={() => setCurrent(s.id)}
                  aria-current={s.id === shown?.id}
                  className="flex w-full flex-wrap justify-between gap-2 rounded-lg px-2 py-2 text-left hover:bg-elevado/70 aria-[current=true]:bg-elevado aria-[current=true]:text-destaque-forte"
                >
                  <span>#{s.id}</span>
                  <span className="font-mono text-xs text-suave">
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
