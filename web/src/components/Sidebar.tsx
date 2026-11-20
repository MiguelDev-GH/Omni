import { NavLink } from "react-router";

// Seções do README; os itens chegam nas próximas sprints.
const sections = [
  { title: "Monitoramento", items: ["Geral", "Requisições na API", "Consultas no banco", "Status HTTP", "Sistema"] },
  { title: "Tráfego", items: ["Inspetor de pacotes", "Capturas"] },
  { title: "Arsenal", items: ["Nmap", "Nuclei", "Nikto", "SQLMap", "ffuf", "Amass", "testssl", "Vazamentos"] },
  { title: "Relatórios", items: ["Histórico", "Comparação entre scans", "Exportação em PDF"] },
];

export default function Sidebar() {
  return (
    <nav aria-label="Seções" className="space-y-5 p-4 text-sm">
      <NavLink
        to="/alvos"
        className={({ isActive }) =>
          `block rounded px-2 py-1.5 ${isActive ? "bg-zinc-800 text-texto" : "text-suave hover:text-texto"}`
        }
      >
        Alvos
      </NavLink>
      {sections.map((s) => (
        <section key={s.title} className="space-y-1">
          <h2 className="px-2 text-xs font-semibold tracking-wide text-zinc-500 uppercase">{s.title}</h2>
          <ul>
            {s.items.map((item) => (
              <li key={item}>
                <span aria-disabled="true" className="flex items-center justify-between px-2 py-1 text-zinc-600">
                  {item}
                  <span className="text-[10px] tracking-wide uppercase">em breve</span>
                </span>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </nav>
  );
}
