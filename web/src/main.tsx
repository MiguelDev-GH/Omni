import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import { useAppStore } from "./store/app";
import { applyTema } from "./lib/tema";
import "./index.css";

const root = document.getElementById("root");
if (!root) throw new Error("elemento #root ausente");
applyTema(useAppStore.getState().tema);
createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
