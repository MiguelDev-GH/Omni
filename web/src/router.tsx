import { createBrowserRouter, Navigate } from "react-router";
import Layout from "./components/Layout";
import AlertsPage from "./pages/AlertsPage";
import CaptureInspectorPage from "./pages/CaptureInspectorPage";
import CapturesPage from "./pages/CapturesPage";
import InspectorPage from "./pages/InspectorPage";
import ArsenalPage from "./pages/ArsenalPage";
import NotFoundPage from "./pages/NotFoundPage";
import OverviewPage from "./pages/OverviewPage";
import QueriesPage from "./pages/QueriesPage";
import ReportsPage from "./pages/ReportsPage";
import ProbeServicePage from "./pages/ProbeServicePage";
import SystemPage from "./pages/SystemPage";
import RequestsPage from "./pages/RequestsPage";
import TargetsPage from "./pages/TargetsPage";
import ToolPage from "./pages/ToolPage";

export const router = createBrowserRouter([
  {
    element: <Layout />,
    children: [
      { index: true, path: "/", element: <Navigate to="/geral" replace /> },
      { path: "/geral", element: <OverviewPage /> },
      { path: "/alertas", element: <AlertsPage /> },
      { path: "/alvos", element: <TargetsPage /> },
      { path: "/requisicoes", element: <RequestsPage /> },
      { path: "/consultas", element: <QueriesPage /> },
      { path: "/arsenal", element: <ArsenalPage /> },
      { path: "/arsenal/:ferramenta", element: <ToolPage /> },
      { path: "/relatorios", element: <ReportsPage /> },
      { path: "/dns", element: <ProbeServicePage kind="dns" /> },
      { path: "/smtp", element: <ProbeServicePage kind="smtp" /> },
      { path: "/sistema", element: <SystemPage /> },
      { path: "/inspetor", element: <InspectorPage /> },
      { path: "/inspetor/captura/:id", element: <CaptureInspectorPage /> },
      { path: "/capturas", element: <CapturesPage /> },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
]);
