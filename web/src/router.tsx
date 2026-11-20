import { createBrowserRouter, Navigate } from "react-router";
import Layout from "./components/Layout";
import NotFoundPage from "./pages/NotFoundPage";
import TargetsPage from "./pages/TargetsPage";

export const router = createBrowserRouter([
  {
    element: <Layout />,
    children: [
      { index: true, path: "/", element: <Navigate to="/alvos" replace /> },
      { path: "/alvos", element: <TargetsPage /> },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
]);
