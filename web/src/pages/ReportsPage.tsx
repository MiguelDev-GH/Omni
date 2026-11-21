import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router";
import { compareScan, listScans, scanFindings, severities, severityLabels, type Comparison, type Finding, type Scan } from "../api/scans";
import { Card } from "../components/Card";
import FindingsDiff from "../components/FindingsDiff";
import FindingsTable, { severityClasses } from "../components/FindingsTable";
import { useAppStore } from "../store/app";
import { statusLabels } from "./ArsenalPage";

const when = (iso: string) => new Date(iso).toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" });

function Counts({ s }: { s: Scan }) {
  const present = severities.filter((sev) => (s.achados[sev] ?? 0) > 0);
  if (present.length === 0) return <span className="text-suave">—</span>;
  return (
    <span className="flex flex-wrap gap-1">
      {present.map((sev) => (
        <span key={sev} className={`rounded border px-1.5 text-[11px] ${severityClasses[sev]}`}>
          {s.achados[sev]} {severityLabels[sev].toLowerCase()}
        </span>
      ))}
    </span>
  );
}

type Detail = { scanId: number; cmp: Comparison; all: Finding[] };

export default function ReportsPage() {
  const targets = useAppStore((s) => s.targets);
  const selected = useAppStore((s) => s.selectedTargetId);
  const selectTarget = useAppStore((s) => s.selectTarget);
  const scanEvents = useAppStore((s) => s.scanEvents);
  const [params, setParams] = useSearchParams();
  const opened = Number(params.get("varredura")) || null;

  const [scans, setScans] = useState<Scan[] | null>(null);
  const [detail, setDetail] = useState<Detail | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    listScans().then(setScans, () => setError("Não foi possível carregar as varreduras."));
  }, [scanEvents]);

  // Uma varredura aberta pelo link passa a valer como alvo selecionado, uma
  // vez por varredura, para não brigar com a troca manual de alvo.
  const openedScan = scans?.find((s) => s.id === opened);
  const synced = useRef<number | null>(null);
  useEffect(() => {
    if (!openedScan || synced.current === openedScan.id) return;
    synced.current = openedScan.id;
    selectTarget(openedScan.alvo_id);
  }, [openedScan, selectTarget]);

  useEffect(() => {
    if (opened === null) return;
    Promise.all([compareScan(opened), scanFindings(opened)]).then(
      ([cmp, all]) => setDetail({ scanId: opened, cmp, all }),
      () => setError("Não foi possível carregar a comparação."),
    );
  }, [opened, scanEvents]);

  const mine = (scans ?? []).filter((s) => s.alvo_id === selected);
  const shown = detail && detail.scanId === opened ? detail : null;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-baseline gap-3">
        <h1 className="text-xl font-semibold">Relatórios</h1>
        <span className="rounded border border-amber-500/40 bg-amber-500/10 px-2 py-0.5 text-xs text-amber-200">
          Visão interna
        </span>
      </div>
      {error && <p className="text-sm text-erro">{error}</p>}

      <Card title="Histórico de varreduras">
        {targets.length > 0 && (
          <div className="mb-3 flex items-center gap-2 text-sm">
            <label htmlFor="alvo-relatorio" className="text-suave">
              Alvo
            </label>
            <select
              id="alvo-relatorio"
              value={selected ?? ""}
              onChange={(e) => {
                selectTarget(Number(e.target.value));
                setParams({});
              }}
              className="rounded border border-borda bg-fundo px-2 py-1"
            >
              {targets.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.nome}
                </option>
              ))}
            </select>
          </div>
        )}
        {scans !== null && mine.length === 0 ? (
          <p className="text-sm text-suave">Nenhuma varredura deste alvo ainda. Use o Arsenal para iniciar uma.</p>
        ) : (
          <div className="overflow-x-auto">
            <table aria-label="Varreduras" className="w-full text-sm">
              <thead className="text-left text-xs text-suave">
                <tr>
                  <th className="pb-2 font-medium">Varredura</th>
                  <th className="pb-2 font-medium">Ferramentas</th>
                  <th className="pb-2 font-medium">Data</th>
                  <th className="pb-2 font-medium">Status</th>
                  <th className="pb-2 font-medium">Achados</th>
                </tr>
              </thead>
              <tbody>
                {mine.map((s) => (
                  <tr key={s.id} className={`border-t border-borda ${s.id === opened ? "bg-zinc-800/60" : ""}`}>
                    <td className="py-1.5">
                      <button
                        type="button"
                        onClick={() => setParams({ varredura: String(s.id) })}
                        className="text-destaque underline"
                      >
                        #{s.id}
                      </button>
                    </td>
                    <td className="py-1.5">{s.ferramentas.join(", ")}</td>
                    <td className="py-1.5 whitespace-nowrap text-suave">{when(s.criada_em)}</td>
                    <td className={`py-1.5 ${s.status === "falhou" ? "text-erro" : ""}`} title={s.erro}>
                      {statusLabels[s.status]}
                    </td>
                    <td className="py-1.5">
                      <Counts s={s} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      {opened !== null && (
        <Card title={`Varredura #${opened}: o que mudou desde a anterior`}>
          {shown ? (
            <div className="space-y-4">
              <FindingsDiff cmp={shown.cmp} />
              <details>
                <summary className="cursor-pointer text-sm text-suave">Todos os achados ({shown.all.length})</summary>
                <div className="mt-2">
                  <FindingsTable list={shown.all} label="Todos os achados" />
                </div>
              </details>
            </div>
          ) : (
            <p className="text-sm text-suave">Carregando...</p>
          )}
        </Card>
      )}
    </div>
  );
}
