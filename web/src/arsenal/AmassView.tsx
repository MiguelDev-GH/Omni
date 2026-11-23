import type { AmassReport, Subdomain } from "../api/reports";
import type { Finding, Scan } from "../api/scans";
import { Notice } from "../components/Card";

export function amassSummary(report: unknown): string {
  const subs = (report as AmassReport).subdominios;
  if (subs.length === 0) return "Nenhum subdomínio encontrado";
  const here = subs.filter((s) => s.neste_servidor).length;
  const total = `${subs.length} ${subs.length === 1 ? "subdomínio" : "subdomínios"}`;
  return here > 0 ? `${total} · ${here} neste servidor` : total;
}

type Node = { label: string; name: string; sub?: Subdomain; children: Node[] };

// tree agrupa os subdomínios pelos rótulos à esquerda do domínio, do mais
// geral para o mais específico: a.api.loja.test fica dentro de api.
export function tree(domain: string, subs: Subdomain[]): Node[] {
  const root: Node = { label: domain, name: domain, children: [] };
  for (const s of subs) {
    if (s.nome === domain) {
      root.sub = s;
      continue;
    }
    const rest = s.nome.endsWith(`.${domain}`) ? s.nome.slice(0, -domain.length - 1) : s.nome;
    let node = root;
    for (const label of rest.split(".").reverse()) {
      const name = `${label}.${node.name}`;
      let child = node.children.find((c) => c.label === label);
      if (!child) {
        child = { label, name, children: [] };
        node.children.push(child);
      }
      node = child;
    }
    node.sub = s;
  }
  const sort = (nodes: Node[]) => {
    nodes.sort((a, b) => a.label.localeCompare(b.label));
    nodes.forEach((n) => sort(n.children));
  };
  sort(root.children);
  return [root];
}

// quiet esconde o "sem endereço" de cada linha quando nenhum subdomínio tem
// endereço: o aviso geral já diz isso uma vez.
function Branch({ nodes, quiet }: { nodes: Node[]; quiet: boolean }) {
  return (
    <ul className="space-y-px">
      {nodes.map((n) => (
        <li key={n.name} aria-label={n.sub ? n.name : undefined}>
          <div className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5 rounded-lg px-2 py-1.5 text-sm hover:bg-elevado/50">
            <span className={`font-mono break-all ${n.sub ? "text-texto" : "text-apagado"}`} title={n.name}>
              {n.label}
            </span>
            {n.sub && (
              <>
                {n.sub.neste_servidor && (
                  <span className="rounded-md bg-destaque-tinta px-1.5 py-0.5 text-[11px] text-destaque-forte">aponta para este servidor</span>
                )}
                {n.sub.enderecos.length > 0 ? (
                  <span className="font-mono text-xs break-all text-suave">{n.sub.enderecos.join(" · ")}</span>
                ) : (
                  !quiet && <span className="text-xs text-apagado">sem endereço</span>
                )}
                {n.sub.fontes.length > 0 && <span className="text-xs text-apagado">via {n.sub.fontes.join(", ")}</span>}
              </>
            )}
          </div>
          {n.children.length > 0 && (
            <div className="ml-4 border-l border-borda pl-2">
              <Branch nodes={n.children} quiet={quiet} />
            </div>
          )}
        </li>
      ))}
    </ul>
  );
}

export default function AmassView({ report }: { report: unknown; findings: Finding[]; scan: Scan }) {
  const rep = report as AmassReport;
  const subs = rep.subdominios;
  const here = subs.filter((s) => s.neste_servidor).length;
  const unresolved = subs.filter((s) => s.enderecos.length === 0).length;
  return (
    <div className="space-y-5">
      <Notice tone="info">Consulta fontes públicas na internet; o alvo não recebe tráfego.</Notice>
      {subs.length === 0 ? (
        <p className="text-sm text-suave">
          O Amass não encontrou nenhum subdomínio de <span className="font-mono text-texto">{rep.dominio}</span> nas fontes públicas.
        </p>
      ) : (
        <>
          <p className="text-sm text-suave">
            {amassSummary(rep)} de <span className="font-mono text-texto">{rep.dominio}</span>
            {here === 0 && <span className="block text-xs">Nenhum endereço conhecido aponta para este servidor.</span>}
            {unresolved > 0 && (
              <span className="block text-xs">
                {unresolved === subs.length ? "Nenhum subdomínio tem endereço" : `${unresolved} vieram sem endereço`}: no modo passivo o
                Amass só repete o que as fontes informam e não resolve os nomes.
              </span>
            )}
          </p>
          <div aria-label="Subdomínios" role="region">
            <Branch nodes={tree(rep.dominio, subs)} quiet={unresolved === subs.length} />
          </div>
        </>
      )}
    </div>
  );
}
