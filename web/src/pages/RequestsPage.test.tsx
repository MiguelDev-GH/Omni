import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { expect, test } from "vitest";
import type { RouteStats } from "../api/requests";
import { useAppStore } from "../store/app";
import { json, mockFetch } from "../test/fetch";
import RequestsPage, { sortRoutes } from "./RequestsPage";

const routes: RouteStats[] = [
  { metodo: "GET", rota: "/rapida", requisicoes: 900, erros: 0, p50_ms: 1, p95_ms: 2 },
  { metodo: "POST", rota: "/lenta", requisicoes: 10, erros: 3, p50_ms: 300, p95_ms: 1200 },
  { metodo: "GET", rota: "/media/:id", requisicoes: 50, erros: 0, p50_ms: 20, p95_ms: 80 },
];

function renderPage() {
  const calls = mockFetch((c) => {
    if (c.url.startsWith("/api/requisicoes/rotas")) return json(200, routes);
    if (c.url.startsWith("/api/requisicoes")) return json(200, []);
    return json(500, { erro: "inesperado" });
  });
  useAppStore.setState({ token: "t" });
  const router = createMemoryRouter([{ path: "/requisicoes", element: <RequestsPage /> }], { initialEntries: ["/requisicoes"] });
  render(<RouterProvider router={router} />);
  return calls;
}

const routeOrder = () =>
  within(screen.getByRole("table", { name: "Rotas" }))
    .getAllByRole("row")
    .slice(1)
    .map((r) => r.querySelector("td")?.textContent);

test("table lists the slowest routes first and can sort by volume", async () => {
  renderPage();
  await screen.findByText("/lenta");
  expect(routeOrder()).toEqual(["POST/lenta", "GET/media/:id", "GET/rapida"]);
  expect(screen.getByText("1,2 s")).toBeInTheDocument();
  expect(screen.getByText("Requisições em 24 h").nextElementSibling).toHaveTextContent("960");

  await userEvent.click(screen.getByLabelText("Ordenar por"));
  await userEvent.click(screen.getByRole("option", { name: "Mais acessadas" }));
  expect(routeOrder()).toEqual(["GET/rapida", "GET/media/:id", "POST/lenta"]);
});

test("sortRoutes breaks ties by route", () => {
  const a = { ...routes[0]!, rota: "/b", p95_ms: 5 };
  const b = { ...routes[0]!, rota: "/a", p95_ms: 5 };
  expect(sortRoutes([a, b], "p95").map((r) => r.rota)).toEqual(["/a", "/b"]);
});

test("shows an empty state without requests", async () => {
  mockFetch((c) => json(200, c.url.includes("rotas") ? [] : []));
  useAppStore.setState({ token: "t" });
  const router = createMemoryRouter([{ path: "/", element: <RequestsPage /> }]);
  render(<RouterProvider router={router} />);
  expect(await screen.findByText("Nenhuma requisição registrada nas últimas 24 horas.")).toBeInTheDocument();
});
