import { useEffect, useState } from "react";
import type { NmapPort, NmapReport } from "../api/reports";
import { compareScan, type Finding, type Scan } from "../api/scans";

// O estado aparece sempre em texto; a cor só reforça.
const states: Record<string, { label: string; mark: string; text: string }> = {
  open: { label: "Aberta", mark: "bg-destaque", text: "text-destaque-forte" },
  closed: { label: "Fechada", mark: "bg-apagado", text: "text-suave" },
  filtered: { label: "Filtrada", mark: "bg-aviso", text: "text-aviso" },
};
const stateOf = (s: string) => states[s] ?? { label: s, mark: "bg-apagado", text: "text-suave" };

export function nmapSummary(report: unknown): string {
  const ports = (report as NmapReport).hosts.flatMap((h) => h.portas);
  const open = ports.filter((p) => p.estado === "open").length;
  return `${open} ${open === 1 ? "porta aberta" : "portas abertas"} de ${ports.length}`;
}

// changes compara os achados do Nmap com a varredura anterior: achado novo é
// porta que abriu, achado resolvido é porta que fechou.
function useChanges(scan: Scan): Map<number, string> | null {
  const [changes, setChanges] = useState<{ scanId: number; map: Map<number, string> } | null>(null);
  useEffect(() => {
    let live = true;
    compareScan(scan.id).then(
      (cmp) => {
        if (!live) return;
        const map = new Map<number, string>();
        if (cmp.anteriores.nmap) {
          const ofNmap = (f: Finding) => f.ferramenta === "nmap" && f.porta > 0;
          cmp.novos.filter(ofNmap).forEach((f) => map.set(f.porta, "abriu desde a anterior"));
          cmp.resolvidos.filter(ofNmap).forEach((f) => map.set(f.porta, "fechou desde a anterior"));
        }
        setChanges({ scanId: scan.id, map });
      },
      () => live && setChanges({ scanId: scan.id, map: new Map() }),
    );
    return () => {
      live = false;
    };
  }, [scan.id]);
  return changes?.scanId === scan.id ? changes.map : null;
}

export default function NmapView({ report, scan }: { report: unknown; findings: Finding[]; scan: Scan }) {
  const rep = report as NmapReport;
  const changes = useChanges(scan);
  const ports: (NmapPort & { endereco: string })[] = rep.hosts.flatMap((h) =>
    h.portas.map((p) => ({ ...p, endereco: h.endereco })),
  );
  if (ports.length === 0) return <p className="text-sm text-suave">O Nmap não relatou nenhuma porta.</p>;
  const multiHost = rep.hosts.length > 1;

  return (
    <div className="space-y-6">
      <ul aria-label="Faixa das portas" className="flex flex-wrap gap-x-4 gap-y-3">
        {ports.map((p) => {
          const st = stateOf(p.estado);
          return (
            <li key={`${p.endereco}-${p.protocolo}-${p.porta}`} className="flex flex-col items-center gap-1">
              <span aria-hidden="true" className={`h-6 w-1.5 rounded-full ${st.mark}`} />
              <span className="font-mono text-[11px] tabular-nums text-suave">
                {p.porta}
                <span className="sr-only"> {st.label}</span>
              </span>
            </li>
          );
        })}
      </ul>

      <div className="overflow-x-auto">
        <table aria-label="Mapa de portas" className="tabela">
          <thead>
            <tr>
              <th>Porta</th>
              <th>Estado</th>
              <th>Serviço</th>
              <th>Produto e versão</th>
              {multiHost && <th>Endereço</th>}
            </tr>
          </thead>
          <tbody>
            {ports.map((p) => {
              const st = stateOf(p.estado);
              const change = changes?.get(p.porta);
              const product = [p.produto, p.versao].filter(Boolean).join(" ");
              return (
                <tr key={`${p.endereco}-${p.protocolo}-${p.porta}`} className="align-top">
                  <td className="pr-3 font-mono text-xs tabular-nums">
                    {p.porta}/{p.protocolo}
                  </td>
                  <td className="pr-3">
                    <span className={`inline-flex items-center gap-1.5 font-medium ${st.text}`}>
                      <span aria-hidden="true" className={`size-1.5 rounded-full ${st.mark}`} />
                      {st.label}
                    </span>
                    {change && <span className="mt-0.5 block text-xs text-aviso">{change}</span>}
                  </td>
                  <td className="pr-3 font-mono text-xs">{p.servico || "—"}</td>
                  <td className="pr-3">
                    {product || <span className="text-apagado">—</span>}
                    {p.extra && <p className="mt-0.5 text-xs text-suave">{p.extra}</p>}
                  </td>
                  {multiHost && <td className="font-mono text-xs">{p.endereco}</td>}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
