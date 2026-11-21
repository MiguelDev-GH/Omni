import { createBrowserRouter, Navigate } from "react-router";
import Layout from "./components/Layout";
import AlertsPage from "./pages/AlertsPage";
import ArsenalPage from "./pages/ArsenalPage";
import NotFoundPage from "./pages/NotFoundPage";
import OverviewPage from "./pages/OverviewPage";
import QueriesPage from "./pages/QueriesPage";
import ReportsPage from "./pages/ReportsPage";
import RequestsPage from "./pages/RequestsPage";
import TargetsPage from "./pages/TargetsPage";

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
      { path: "/relatorios", element: <ReportsPage /> },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
]);
