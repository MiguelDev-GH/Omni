import { severityLabels, sortFindings, type Finding, type Severity } from "../api/scans";

export const severityClasses: Record<Severity, string> = {
  critica: "bg-red-100 text-red-800",
  alta: "bg-orange-100 text-orange-800",
  media: "bg-amber-100 text-amber-800",
  baixa: "bg-sky-100 text-sky-800",
  info: "bg-texto/5 text-suave",
};

export function SeverityBadge({ s }: { s: Severity }) {
  return (
    <span className={`inline-block rounded-md px-1.5 py-0.5 text-[11px] font-medium whitespace-nowrap ${severityClasses[s]}`}>
      {severityLabels[s]}
    </span>
  );
}

// FindingsTable lista os achados dos mais graves aos menos graves.
export default function FindingsTable({ list, label }: { list: Finding[]; label: string }) {
  if (list.length === 0) return <p className="text-sm text-suave">Nenhum achado.</p>;
  return (
    <div className="overflow-x-auto">
      <table aria-label={label} className="tabela">
        <thead>
          <tr>
            <th>Severidade</th>
            <th>Achado</th>
            <th>Ferramenta</th>
            <th className="text-right">Porta</th>
          </tr>
        </thead>
        <tbody>
          {sortFindings(list).map((f) => (
            <tr key={f.assinatura} className="align-top">
              <td className="pr-3">
                <SeverityBadge s={f.severidade} />
              </td>
              <td className="pr-3">
                <p>{f.titulo}</p>
                {f.evidencia && <p className="mt-0.5 font-mono text-xs break-all text-suave">{f.evidencia}</p>}
              </td>
              <td className="pr-3 text-suave">{f.ferramenta}</td>
              <td className="text-right font-mono text-xs tabular-nums">{f.porta || "—"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
