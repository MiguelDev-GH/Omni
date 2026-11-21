import { useEffect } from "react";
import { Outlet } from "react-router";
import { loadTargets } from "../api/targets";
import { useOmniSocket } from "../realtime/useOmniSocket";
import Sidebar from "./Sidebar";
import Toasts from "./Toasts";
import Topbar from "./Topbar";

export default function Layout() {
  useOmniSocket();
  // A barra superior e a tela Geral precisam dos alvos em qualquer página;
  // a página de Alvos recarrega a lista e mostra os próprios erros.
  useEffect(() => {
    loadTargets().catch(() => {});
  }, []);
  return (
    <div className="flex min-h-screen flex-col">
      <Topbar />
      <div className="flex flex-1 flex-col md:flex-row">
        <aside className="border-b border-borda md:w-60 md:shrink-0 md:border-r md:border-b-0">
          <Sidebar />
        </aside>
        <main className="min-w-0 flex-1 p-4 md:p-6">
          <Outlet />
        </main>
      </div>
      <Toasts />
    </div>
  );
}
