import { createBrowserRouter, Navigate } from "react-router";
import Layout from "./components/Layout";
import AlertsPage from "./pages/AlertsPage";
import InspectorPage from "./pages/InspectorPage";
import ArsenalPage from "./pages/ArsenalPage";
import NotFoundPage from "./pages/NotFoundPage";
import OverviewPage from "./pages/OverviewPage";
import QueriesPage from "./pages/QueriesPage";
import ReportPdfPage from "./pages/ReportPdfPage";
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
      { path: "/relatorios/pdf", element: <ReportPdfPage /> },
      { path: "/dns", element: <ProbeServicePage kind="dns" /> },
      { path: "/smtp", element: <ProbeServicePage kind="smtp" /> },
      { path: "/sistema", element: <SystemPage /> },
      { path: "/inspetor", element: <InspectorPage /> },
      // As gravações .pcap ficam no Inspetor; o endereço antigo leva até ele.
      { path: "/capturas", element: <Navigate to="/inspetor" replace /> },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
]);
