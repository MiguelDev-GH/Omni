import { NavLink } from "react-router";

// Seções do README; os itens sem rota chegam nas próximas sprints.
const routes: Record<string, string> = {
  Geral: "/geral",
  Alertas: "/alertas",
  "Requisições na API": "/requisicoes",
  "Status HTTP": "/requisicoes",
  "Consultas no banco": "/consultas",
  "Visão geral": "/arsenal",
  Nmap: "/arsenal/nmap",
  Nuclei: "/arsenal/nuclei",
  Nikto: "/arsenal/nikto",
  SQLMap: "/arsenal/sqlmap",
  ffuf: "/arsenal/ffuf",
  testssl: "/arsenal/testssl",
  Histórico: "/relatorios",
  "Comparação entre scans": "/relatorios",
};

// Só o primeiro item de cada rota acende como ativo, para a seção atual
// aparecer uma vez só no menu.
const primary = new Set([
  "Alvos",
  ...Object.entries(routes)
    .filter(([, to], i, all) => all.findIndex(([, t]) => t === to) === i)
    .map(([k]) => k),
]);

const linkClass =
  (item: string) =>
  ({ isActive }: { isActive: boolean }) =>
    `relative flex items-center rounded-lg px-2.5 py-1.5 ${
      isActive && primary.has(item)
        ? "bg-elevado/80 font-medium text-texto before:absolute before:top-2 before:bottom-2 before:-left-px before:w-0.5 before:rounded-full before:bg-destaque"
        : "text-suave hover:bg-elevado/50 hover:text-texto"
    }`;

const sections = [
  { title: "Monitoramento", items: ["Geral", "Alertas", "Requisições na API", "Consultas no banco", "Status HTTP", "Sistema"] },
  { title: "Tráfego", items: ["Inspetor de pacotes", "Capturas"] },
  { title: "Arsenal", items: ["Visão geral", "Nmap", "Nuclei", "Nikto", "SQLMap", "ffuf", "Amass", "testssl", "Vazamentos"] },
  { title: "Relatórios", items: ["Histórico", "Comparação entre scans", "Exportação em PDF"] },
];

export default function Sidebar({ onNavigate }: { onNavigate?: () => void }) {
  return (
    <nav aria-label="Seções" className="space-y-6 text-sm">
      <NavLink to="/alvos" onClick={onNavigate} className={linkClass("Alvos")}>
        Alvos
      </NavLink>
      {sections.map((s) => (
        <section key={s.title} className="space-y-1">
          <h2 className="px-2.5 pb-1 text-xs font-medium text-apagado">{s.title}</h2>
          <ul className="space-y-px">
            {s.items.map((item) => (
              <li key={item}>
                {routes[item] ? (
                  <NavLink to={routes[item]} end onClick={onNavigate} className={linkClass(item)}>
                    {item}
                  </NavLink>
                ) : (
                  <span
                    aria-disabled="true"
                    className="flex items-center justify-between gap-2 px-2.5 py-1.5 whitespace-nowrap text-apagado/80"
                  >
                    {item}
                    <span className="font-mono text-[10px] text-apagado/70">em breve</span>
                  </span>
                )}
              </li>
            ))}
          </ul>
        </section>
      ))}
    </nav>
  );
}
