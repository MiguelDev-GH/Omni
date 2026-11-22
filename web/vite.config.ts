import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

// Declaração mínima, para não depender de @types/node só por causa do env.
declare const process: { env: Record<string, string | undefined> };

// `make dev` escolhe portas altas livres a cada execução: OMNI_PORTA_WEB é a
// do Vite e OMNI_API o endereço do backend, que só aceita a origem do Vite
// porque o script a coloca em OMNI_ORIGENS.
const api = process.env.OMNI_API;
const webPort = Number(process.env.OMNI_PORTA_WEB) || undefined;

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: webPort,
    strictPort: webPort !== undefined,
    proxy: api
      ? {
          "/api": `http://${api}`,
          "/ws": { target: `ws://${api}`, ws: true },
        }
      : undefined,
  },
  test: {
    environment: "jsdom",
    setupFiles: ["src/test/setup.ts"],
    restoreMocks: true,
  },
});
