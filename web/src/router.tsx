import { createBrowserRouter, Navigate } from "react-router";
import Layout from "./components/Layout";
import ArsenalPage from "./pages/ArsenalPage";
import NotFoundPage from "./pages/NotFoundPage";
import OverviewPage from "./pages/OverviewPage";
import QueriesPage from "./pages/QueriesPage";
import RequestsPage from "./pages/RequestsPage";
import TargetsPage from "./pages/TargetsPage";

export const router = createBrowserRouter([
  {
    element: <Layout />,
    children: [
      { index: true, path: "/", element: <Navigate to="/geral" replace /> },
      { path: "/geral", element: <OverviewPage /> },
      { path: "/alvos", element: <TargetsPage /> },
      { path: "/requisicoes", element: <RequestsPage /> },
      { path: "/consultas", element: <QueriesPage /> },
      { path: "/arsenal", element: <ArsenalPage /> },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
]);
