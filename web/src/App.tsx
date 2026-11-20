import { RouterProvider } from "react-router";
import TokenPage from "./pages/TokenPage";
import { router } from "./router";
import { useAppStore } from "./store/app";

export default function App() {
  const token = useAppStore((s) => s.token);
  return token ? <RouterProvider router={router} /> : <TokenPage />;
}
