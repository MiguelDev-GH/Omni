import { severityLabels, sortFindings, type Finding, type Severity } from "../api/scans";

export const severityClasses: Record<Severity, string> = {
  critica: "bg-red-500/20 text-red-300 border-red-500/40",
  alta: "bg-orange-500/20 text-orange-300 border-orange-500/40",
  media: "bg-amber-500/15 text-amber-200 border-amber-500/40",
  baixa: "bg-sky-500/15 text-sky-200 border-sky-500/40",
  info: "bg-zinc-700/40 text-zinc-300 border-zinc-600",
};

export function SeverityBadge({ s }: { s: Severity }) {
  return (
    <span className={`inline-block rounded border px-1.5 py-0.5 text-[11px] font-medium ${severityClasses[s]}`}>
      {severityLabels[s]}
    </span>
  );
}

// FindingsTable lista os achados dos mais graves aos menos graves.
export default function FindingsTable({ list, label }: { list: Finding[]; label: string }) {
  if (list.length === 0) return <p className="text-sm text-suave">Nenhum achado.</p>;
  return (
    <div className="overflow-x-auto">
      <table aria-label={label} className="w-full text-sm">
        <thead className="text-left text-xs text-suave">
          <tr>
            <th className="pb-2 font-medium">Severidade</th>
            <th className="pb-2 font-medium">Achado</th>
            <th className="pb-2 font-medium">Ferramenta</th>
            <th className="pb-2 text-right font-medium">Porta</th>
          </tr>
        </thead>
        <tbody>
          {sortFindings(list).map((f) => (
            <tr key={f.assinatura} className="border-t border-borda align-top">
              <td className="py-1.5 pr-2">
                <SeverityBadge s={f.severidade} />
              </td>
              <td className="py-1.5 pr-2">
                <p>{f.titulo}</p>
                {f.evidencia && <p className="text-xs break-all text-suave">{f.evidencia}</p>}
              </td>
              <td className="py-1.5 pr-2 text-suave">{f.ferramenta}</td>
              <td className="py-1.5 text-right tabular-nums">{f.porta || "—"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
