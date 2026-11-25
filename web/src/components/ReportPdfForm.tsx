import { useState } from "react";
import { getTargetReport, MAX_REPORT_HOURS } from "../api/targetReport";
import type { Target } from "../api/targets";

type Preset = "dia" | "semana" | "personalizado";

const presets: { id: Preset; label: string; hours?: number }[] = [
  { id: "dia", label: "Último dia", hours: 24 },
  { id: "semana", label: "Última semana", hours: MAX_REPORT_HOURS },
  { id: "personalizado", label: "Personalizado" },
];

// localInput escreve uma data no formato de <input type="datetime-local">.
function localInput(d: Date): string {
  const off = d.getTimezoneOffset() * 60_000;
  return new Date(d.getTime() - off).toISOString().slice(0, 16);
}

// rangeError devolve o problema do intervalo personalizado, ou null.
export function rangeError(from: Date, to: Date): string | null {
  if (Number.isNaN(from.getTime()) || Number.isNaN(to.getTime())) return "Preencha as duas datas.";
  if (from >= to) return "O início precisa ser antes do fim.";
  if (to.getTime() - from.getTime() > MAX_REPORT_HOURS * 3_600_000) return `O intervalo pode ter no máximo ${MAX_REPORT_HOURS} horas (7 dias).`;
  return null;
}

// ReportPdfForm escolhe o intervalo e baixa o PDF do alvo. O módulo do PDF
// (jsPDF) só é carregado no clique, para não pesar na abertura da página.
export default function ReportPdfForm({ target }: { target: Target }) {
  const [preset, setPreset] = useState<Preset>("dia");
  const [custom, setCustom] = useState(() => {
    const now = new Date();
    return { from: localInput(new Date(now.getTime() - 24 * 3_600_000)), to: localInput(now) };
  });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const customFrom = new Date(custom.from);
  const customTo = new Date(custom.to);
  const invalid = preset === "personalizado" ? rangeError(customFrom, customTo) : null;

  async function generate() {
    const hours = presets.find((p) => p.id === preset)?.hours;
    const to = hours ? new Date() : customTo;
    const from = hours ? new Date(to.getTime() - hours * 3_600_000) : customFrom;
    setBusy(true);
    setError(null);
    try {
      const [rep, pdf] = await Promise.all([getTargetReport(target.id, from, to), import("../lib/reportPdf")]);
      pdf.buildReportPdf(rep).save(pdf.reportFileName(rep));
    } catch (err) {
      setError(err instanceof Error && err.message ? `Não foi possível gerar o relatório: ${err.message}.` : "Não foi possível gerar o relatório.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-4">
      <p className="max-w-3xl text-sm text-suave">
        Alertas, disponibilidade das portas, varreduras do Arsenal com os achados e os gráficos de tráfego, HTTP, banco e sistema do
        período. Os dados ficam guardados por até {MAX_REPORT_HOURS / 24} dias.
      </p>
      <div role="radiogroup" aria-label="Intervalo" className="inline-flex flex-wrap gap-1 rounded-lg bg-texto/5 p-1">
        {presets.map((p) => (
          <button
            key={p.id}
            type="button"
            role="radio"
            aria-checked={preset === p.id}
            onClick={() => setPreset(p.id)}
            className={`rounded-md px-3 py-1.5 text-sm ${preset === p.id ? "bg-elevado font-medium text-texto shadow-sm" : "text-suave hover:text-texto"}`}
          >
            {p.label}
          </button>
        ))}
      </div>

      {preset === "personalizado" && (
        <div className="grid max-w-xl gap-3 sm:grid-cols-2">
          <label className="space-y-1 text-sm">
            <span className="text-suave">De</span>
            <input
              type="datetime-local"
              className="campo"
              value={custom.from}
              aria-invalid={invalid !== null}
              onChange={(e) => setCustom((c) => ({ ...c, from: e.target.value }))}
            />
          </label>
          <label className="space-y-1 text-sm">
            <span className="text-suave">Até</span>
            <input
              type="datetime-local"
              className="campo"
              value={custom.to}
              aria-invalid={invalid !== null}
              onChange={(e) => setCustom((c) => ({ ...c, to: e.target.value }))}
            />
          </label>
        </div>
      )}
      {invalid && (
        <p role="alert" className="text-sm text-erro">
          {invalid}
        </p>
      )}

      <div className="flex flex-wrap items-center gap-3">
        <button type="button" className="botao" disabled={busy || invalid !== null} onClick={generate}>
          {busy ? "Gerando..." : "Baixar PDF"}
        </button>
        {error && (
          <p role="alert" className="text-sm text-erro">
            {error}
          </p>
        )}
      </div>
    </div>
  );
}
