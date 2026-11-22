import { useEffect, useState } from "react";
import type { FfufReport, FfufResult } from "../api/reports";
import { listRoutes } from "../api/requests";
import type { Finding, Scan } from "../api/scans";

export function ffufSummary(report: unknown): string {
  const n = (report as FfufReport).resultados.length;
  if (n === 0) return "Nenhum caminho encontrado";
  return `${n} ${n === 1 ? "caminho encontrado" : "caminhos encontrados"}`;
}

type StatusClass = "2xx" | "3xx" | "4xx" | "5xx";
const classes: StatusClass[] = ["2xx", "3xx", "4xx", "5xx"];
const classLabels: Record<StatusClass, string> = { "2xx": "Sucesso", "3xx": "Redirecionamento", "4xx": "Erro do cliente", "5xx": "Erro do servidor" };
const classTone: Record<StatusClass, string> = {
  "2xx": "text-destaque-forte",
  "3xx": "text-suave",
  "4xx": "text-aviso",
  "5xx": "text-erro",
};

const statusClass = (status: number): StatusClass => classes[Math.min(3, Math.max(0, Math.floor(status / 100) - 2))]!;

// seenInTraffic diz se o caminho bate com uma rota que o sniffer viu; as
// rotas vêm normalizadas, com ":id" no lugar dos identificadores.
export function seenInTraffic(path: string, routes: string[]): boolean {
  const segs = path.split("/").filter(Boolean);
  return routes.some((route) => {
    const r = route.split("/").filter(Boolean);
    return r.length === segs.length && r.every((s, i) => s === ":id" || s === segs[i]);
  });
}

type Node = { name: string; path: string; result?: FfufResult; children: Node[] };

// tree agrupa os caminhos por segmento: /admin e /admin/login ficam juntos.
export function tree(list: FfufResult[]): Node[] {
  const root: Node = { name: "", path: "", children: [] };
  for (const r of list) {
    let node = root;
    for (const seg of r.caminho.split("/").filter(Boolean)) {
      const path = `${node.path}/${seg}`;
      let child = node.children.find((c) => c.name === seg);
      if (!child) {
        child = { name: seg, path, children: [] };
        node.children.push(child);
      }
      node = child;
    }
    if (node !== root) node.result = r;
  }
  const sort = (nodes: Node[]) => {
    nodes.sort((a, b) => a.name.localeCompare(b.name));
    nodes.forEach((n) => sort(n.children));
  };
  sort(root.children);
  return root.children;
}

const bytes = (n: number) => (n < 1024 ? `${n} B` : `${(n / 1024).toFixed(1).replace(".", ",")} KiB`);

function Branch({ nodes, routes }: { nodes: Node[]; routes: string[] | null }) {
  return (
    <ul className="space-y-px">
      {nodes.map((n) => (
        <li key={n.path}>
          <div className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5 rounded-lg px-2 py-1.5 text-sm hover:bg-elevado/50">
            <span className={`font-mono break-all ${n.result ? "text-texto" : "text-apagado"}`}>/{n.name}</span>
            {n.result && (
              <>
                <span className={`font-mono text-xs font-medium ${classTone[statusClass(n.result.status)]}`}>
                  {n.result.status} <span className="sr-only">{classLabels[statusClass(n.result.status)]}</span>
                </span>
                <span className="font-mono text-xs text-suave">{bytes(n.result.tamanho)}</span>
                {n.result.redirecionamento && (
                  <span className="font-mono text-xs break-all text-suave">→ {n.result.redirecionamento}</span>
                )}
                {routes && !seenInTraffic(n.path, routes) && (
                  <span className="rounded-md bg-aviso-tinta px-1.5 py-0.5 text-[11px] text-aviso">não aparece no tráfego real</span>
                )}
              </>
            )}
          </div>
          {n.children.length > 0 && (
            <div className="ml-4 border-l border-borda pl-2">
              <Branch nodes={n.children} routes={routes} />
            </div>
          )}
        </li>
      ))}
    </ul>
  );
}

export default function FfufView({ report }: { report: unknown; findings: Finding[]; scan: Scan }) {
  const rep = report as FfufReport;
  const [hidden, setHidden] = useState<StatusClass[]>([]);
  // routes fica null enquanto carrega ou se falhar, e vazio quando o
  // sniffer não viu tráfego HTTP: nos dois casos, nenhum caminho é marcado
  // como escondido, porque não há com o que comparar.
  const [routes, setRoutes] = useState<string[] | null>(null);

  useEffect(() => {
    let live = true;
    listRoutes().then(
      (list) => live && setRoutes(list.map((r) => r.rota)),
      () => {},
    );
    return () => {
      live = false;
    };
  }, []);

  const list = rep.resultados;
  if (list.length === 0) return <p className="text-sm text-suave">O ffuf não encontrou nenhum caminho em {rep.base_url}.</p>;

  const present = classes.filter((c) => list.some((r) => statusClass(r.status) === c));
  const shown = list.filter((r) => !hidden.includes(statusClass(r.status)));
  const known = routes !== null && routes.length > 0 ? routes : null;
  const unseen = known ? list.filter((r) => !seenInTraffic(r.caminho, known)).length : null;
  const toggle = (c: StatusClass) => setHidden((h) => (h.includes(c) ? h.filter((x) => x !== c) : [...h, c]));

  return (
    <div className="space-y-5">
      <p className="text-sm text-suave">
        Caminhos encontrados em <span className="font-mono text-texto">{rep.base_url}</span>
        {unseen !== null && (
          <>
            {" "}
            · <span className={unseen > 0 ? "text-aviso" : ""}>{unseen} fora do tráfego real</span>
          </>
        )}
        {routes !== null && routes.length === 0 && (
          <span className="block text-xs">Sem tráfego HTTP registrado nas últimas 24 horas: não dá para apontar as rotas escondidas.</span>
        )}
      </p>
      <fieldset className="flex flex-wrap items-center gap-1.5 text-sm">
        <legend className="sr-only">Classe de status</legend>
        {present.map((c) => (
          <label
            key={c}
            className="flex cursor-pointer items-center gap-1.5 rounded-lg px-2 py-1 hover:bg-elevado/70 has-checked:bg-elevado"
          >
            <input type="checkbox" checked={!hidden.includes(c)} onChange={() => toggle(c)} className="size-3.5" />
            <span className={`font-mono ${classTone[c]}`}>{c}</span>
            {classLabels[c]}
            <span className="font-mono text-xs text-suave">{list.filter((r) => statusClass(r.status) === c).length}</span>
          </label>
        ))}
      </fieldset>
      {shown.length === 0 ? (
        <p className="text-sm text-suave">Nenhum caminho com estes filtros.</p>
      ) : (
        <div aria-label="Árvore de rotas" role="region">
          <Branch nodes={tree(shown)} routes={known} />
        </div>
      )}
    </div>
  );
}
