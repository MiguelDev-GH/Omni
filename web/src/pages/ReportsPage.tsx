import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router";
import { compareScan, listScans, scanFindings, severities, severityLabels, type Comparison, type Finding, type Scan } from "../api/scans";
import { Card, InternalBadge, PageHeader } from "../components/Card";
import FindingsDiff from "../components/FindingsDiff";
import FindingsTable, { severityClasses } from "../components/FindingsTable";
import ReportPdfForm from "../components/ReportPdfForm";
import Select from "../components/Select";
import { useAppStore } from "../store/app";
import { statusLabels } from "../arsenal/common";

const when = (iso: string) => new Date(iso).toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" });

function Counts({ s }: { s: Scan }) {
  const present = severities.filter((sev) => (s.achados[sev] ?? 0) > 0);
  if (present.length === 0) return <span className="text-suave">—</span>;
  return (
    <span className="flex flex-wrap gap-1">
      {present.map((sev) => (
        <span key={sev} className={`rounded-md px-1.5 py-0.5 text-[11px] whitespace-nowrap ${severityClasses[sev]}`}>
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

  const selectedTarget = targets.find((t) => t.id === selected);
  const mine = (scans ?? []).filter((s) => s.alvo_id === selected);
  const shown = detail && detail.scanId === opened ? detail : null;

  return (
    <div className="space-y-8">
      <PageHeader title="Relatórios" aside={<InternalBadge />} />
      {error && <p className="text-sm text-erro">{error}</p>}

      {targets.length > 0 && (
        <div className="flex items-center gap-2 text-sm">
          <label htmlFor="alvo-relatorio" className="text-suave">
            Alvo
          </label>
          <Select
            id="alvo-relatorio"
            value={selected}
            onChange={(id) => {
              selectTarget(id);
              setParams({});
            }}
            options={targets.map((t) => ({ value: t.id, label: t.nome, hint: t.host }))}
          />
        </div>
      )}

      {selectedTarget && (
        <Card title="Relatório em PDF" plane>
          <ReportPdfForm target={selectedTarget} />
        </Card>
      )}

      <Card title="Histórico de varreduras" plane>
        {scans !== null && mine.length === 0 ? (
          <p className="text-sm text-suave">Nenhuma varredura deste alvo ainda. Use o Arsenal para iniciar uma.</p>
        ) : (
          <div className="overflow-x-auto">
            <table aria-label="Varreduras" className="tabela">
              <thead>
                <tr>
                  <th className="pr-3">Varredura</th>
                  <th className="pr-3">Ferramentas</th>
                  <th className="pr-3">Data</th>
                  <th className="pr-3">Status</th>
                  <th>Achados</th>
                </tr>
              </thead>
              <tbody>
                {mine.map((s) => (
                  <tr key={s.id} className={s.id === opened ? "bg-destaque-tinta/60" : ""}>
                    <td className="pr-3">
                      <button
                        type="button"
                        onClick={() => setParams({ varredura: String(s.id) })}
                        className="link font-mono text-xs"
                      >
                        #{s.id}
                      </button>
                    </td>
                    <td className="pr-3 font-mono text-xs">{s.ferramentas.join(", ")}</td>
                    <td className="pr-3 font-mono text-xs whitespace-nowrap text-suave">{when(s.criada_em)}</td>
                    <td className={`pr-3 ${s.status === "falhou" ? "text-erro" : ""}`} title={s.erro}>
                      {statusLabels[s.status]}
                    </td>
                    <td>
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
        <Card title={`Varredura #${opened}: o que mudou desde a anterior`} plane>
          {shown ? (
            <div className="space-y-4">
              <FindingsDiff cmp={shown.cmp} />
              <details>
                <summary className="cursor-pointer text-sm text-suave hover:text-texto">Todos os achados ({shown.all.length})</summary>
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
