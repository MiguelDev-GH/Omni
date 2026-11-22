import { useEffect, useState } from "react";
import { Outlet, useLocation } from "react-router";
import { loadTargets } from "../api/targets";
import { useOmniSocket } from "../realtime/useOmniSocket";
import Sidebar from "./Sidebar";
import Toasts from "./Toasts";
import Topbar from "./Topbar";

export default function Layout() {
  useOmniSocket();
  const { pathname } = useLocation();
  const [menuOpen, setMenuOpen] = useState(false);
  // A barra superior e a tela Geral precisam dos alvos em qualquer página;
  // a página de Alvos recarrega a lista e mostra os próprios erros.
  useEffect(() => {
    loadTargets().catch(() => {});
  }, []);
  return (
    <div className="flex min-h-screen flex-col">
      <Topbar menuOpen={menuOpen} onToggleMenu={() => setMenuOpen((o) => !o)} />
      <div className="mx-auto flex w-full max-w-[1500px] flex-1 flex-col md:flex-row">
        <aside
          id="menu"
          data-open={menuOpen}
          className="hidden px-4 pb-4 data-[open=true]:block md:sticky md:top-16 md:block md:h-[calc(100vh-4rem)] md:w-56 md:shrink-0 md:overflow-y-auto md:py-6 md:pr-2 md:[scrollbar-width:none] md:hover:[scrollbar-width:thin]"
        >
          <Sidebar onNavigate={() => setMenuOpen(false)} />
        </aside>
        {/* A chave reinicia a entrada suave a cada troca de página. */}
        <main key={pathname} className="min-w-0 flex-1 animate-entrada px-4 pt-4 pb-16 md:px-8 md:pt-6">
          <Outlet />
        </main>
      </div>
      <Toasts />
    </div>
  );
}
