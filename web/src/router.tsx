import { createBrowserRouter, Navigate } from "react-router";
import Layout from "./components/Layout";
import NotFoundPage from "./pages/NotFoundPage";
import OverviewPage from "./pages/OverviewPage";
import TargetsPage from "./pages/TargetsPage";

export const router = createBrowserRouter([
  {
    element: <Layout />,
    children: [
      { index: true, path: "/", element: <Navigate to="/geral" replace /> },
      { path: "/geral", element: <OverviewPage /> },
      { path: "/alvos", element: <TargetsPage /> },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
]);
