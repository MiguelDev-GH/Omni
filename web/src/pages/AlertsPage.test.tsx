import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { expect, test } from "vitest";
import type { Alert, AlertsOverview } from "../api/alerts";
import { useAppStore } from "../store/app";
import { json, mockFetch } from "../test/fetch";
import AlertsPage from "./AlertsPage";

const down: Alert = {
  id: 1,
  regra: "disponibilidade",
  servico: "web",
  nome: "Disponibilidade",
  recurso: "loja:3000",
  nivel: "vermelho",
  nivel_anterior: "verde",
  valor: 2,
  limite: 2,
  unidade: "",
  detalhe: "connection refused",
  hora: "2026-11-21T12:00:00Z",
};

const overview: AlertsOverview = {
  estado: [
    { ...down, limite_amarelo: 1, limite_vermelho: 2, desde: down.hora, atualizado: down.hora },
    {
      regra: "sistema_cpu",
      servico: "sistema",
      nome: "Uso de CPU",
      recurso: "",
      nivel: "verde",
      valor: 12.34,
      unidade: "%",
      limite_amarelo: 80,
      limite_vermelho: 95,
      desde: down.hora,
      atualizado: down.hora,
    },
  ],
  historico: [down],
  regras: [{ id: "sistema_cpu", servico: "sistema", nome: "Uso de CPU", unidade: "%", limite_amarelo: 80, limite_vermelho: 95 }],
  email: { ativo: false, destinatarios: 0 },
};

function renderPage(body: AlertsOverview | null = overview) {
  const calls = mockFetch((c) => {
    if (c.url === "/api/alertas" && body) return json(200, body);
    return json(500, { erro: "inesperado" });
  });
  useAppStore.setState({ token: "t" });
  const router = createMemoryRouter([{ path: "/", element: <AlertsPage /> }]);
  render(<RouterProvider router={router} />);
  return calls;
}

test("groups the current level by service", async () => {
  renderPage();
  const web = await screen.findByRole("region", { name: "Web server" });
  await within(web).findByText("fora do ar");
  expect(within(web).getAllByText("Vermelho").length).toBeGreaterThan(0);
  expect(within(web).getByText(/loja:3000/)).toBeInTheDocument();

  const sys = screen.getByRole("region", { name: "Sistema" });
  expect(within(sys).getByText("12.3%")).toBeInTheDocument();
  expect(within(sys).getByText("amarelo ≥ 80% · vermelho ≥ 95%")).toBeInTheDocument();

  expect(within(screen.getByRole("region", { name: "Banco de dados" })).getByText("Sem medições ainda.")).toBeInTheDocument();
  expect(screen.getByText("Vermelho", { selector: "p" }).nextElementSibling).toHaveTextContent("1");
});

test("shows the history and the e-mail state", async () => {
  renderPage();
  const table = await screen.findByRole("table", { name: "Histórico de alertas" });
  const rows = within(table).getAllByRole("row").slice(1);
  expect(rows).toHaveLength(1);
  expect(rows[0]).toHaveTextContent("Disponibilidade");
  expect(rows[0]).toHaveTextContent("connection refused");
  expect(rows[0]).toHaveTextContent("era verde");
  expect(screen.getByRole("note")).toHaveTextContent("Notificação por e-mail desligada");
});

test("live alerts update the page", async () => {
  renderPage();
  await screen.findByRole("table", { name: "Histórico de alertas" });
  act(() =>
    useAppStore.getState().addAlert({ ...down, id: 2, nivel: "verde", nivel_anterior: "vermelho", valor: 0, hora: "2026-11-21T12:01:00Z" }),
  );
  const rows = within(screen.getByRole("table", { name: "Histórico de alertas" })).getAllByRole("row").slice(1);
  expect(rows).toHaveLength(2);
  expect(rows[0]).toHaveTextContent("era vermelho");
  expect(within(screen.getByRole("region", { name: "Web server" })).getByText("no ar")).toBeInTheDocument();
});

test("reports a load error", async () => {
  renderPage(null);
  expect(await screen.findByText("Não foi possível carregar os alertas.")).toBeInTheDocument();
});

test("shows DNS and SMTP services", async () => {
  renderPage({
    ...overview,
    estado: [
      {
        regra: "smtp_sem_starttls",
        servico: "smtp",
        nome: "SMTP sem STARTTLS",
        recurso: "loja:1025",
        nivel: "amarelo",
        valor: 1,
        unidade: "",
        limite_amarelo: 1,
        limite_vermelho: 2,
        desde: down.hora,
        atualizado: down.hora,
      },
    ],
  });
  const smtp = await screen.findByRole("region", { name: "SMTP" });
  await within(smtp).findByText("sem STARTTLS");
  expect(within(smtp).getByText("sem STARTTLS é amarelo")).toBeInTheDocument();
  expect(within(screen.getByRole("region", { name: "DNS" })).getByText("Sem medições ainda.")).toBeInTheDocument();
});

test("shows the security service", async () => {
  renderPage({
    ...overview,
    estado: [
      {
        regra: "seguranca_falhas_login_ip",
        servico: "seguranca",
        nome: "Logins falhos por IP (brute-force)",
        recurso: "IP 203.0.113.66",
        nivel: "vermelho",
        valor: 35,
        unidade: "/min",
        limite_amarelo: 10,
        limite_vermelho: 30,
        desde: down.hora,
        atualizado: down.hora,
      },
      {
        regra: "seguranca_arquivo_alterado",
        servico: "seguranca",
        nome: "Arquivo de configuração alterado",
        recurso: "/etc/caddy/Caddyfile",
        nivel: "amarelo",
        valor: 1,
        unidade: "",
        limite_amarelo: 1,
        limite_vermelho: 2,
        desde: down.hora,
        atualizado: down.hora,
      },
    ],
  });
  const sec = await screen.findByRole("region", { name: "Segurança" });
  await within(sec).findByText("35 /min");
  expect(within(sec).getByText(/IP 203.0.113.66/)).toBeInTheDocument();
  expect(within(sec).getByText("alterado")).toBeInTheDocument();
  expect(within(sec).getByText("alterado é amarelo por 5 min; removido é vermelho")).toBeInTheDocument();
});

function greenStatus(i: number) {
  return {
    regra: "seguranca_taxa_por_ip",
    servico: "seguranca",
    nome: "Requisições por IP",
    recurso: `IP 192.0.2.${i}`,
    nivel: "verde" as const,
    valor: 1600,
    unidade: "/min",
    limite_amarelo: 3000,
    limite_vermelho: 6000,
    desde: down.hora,
    atualizado: down.hora,
  };
}

test("collapses green items beyond five and always shows the alerting ones", async () => {
  const user = userEvent.setup();
  const red = { ...greenStatus(99), nivel: "vermelho" as const, valor: 7000 };
  renderPage({ ...overview, estado: [...Array.from({ length: 8 }, (_, i) => greenStatus(i + 1)), red] });
  const sec = await screen.findByRole("region", { name: "Segurança" });
  await within(sec).findByText(/IP 192.0.2.99/);
  expect(within(sec).getAllByRole("listitem")).toHaveLength(6);
  await user.click(within(sec).getByRole("button", { name: "Mostrar todos (+3 verdes)" }));
  expect(within(sec).getAllByRole("listitem")).toHaveLength(9);
  await user.click(within(sec).getByRole("button", { name: "Mostrar menos" }));
  expect(within(sec).getAllByRole("listitem")).toHaveLength(6);
});

test("pages the history twenty at a time", async () => {
  const user = userEvent.setup();
  const many = Array.from({ length: 25 }, (_, i) => ({ ...down, id: 100 - i }));
  renderPage({ ...overview, historico: many });
  const table = await screen.findByRole("table", { name: "Histórico de alertas" });
  await waitFor(() => expect(within(table).getAllByRole("row")).toHaveLength(21));
  await user.click(screen.getByRole("button", { name: "Mostrar mais (5 restantes)" }));
  expect(within(table).getAllByRole("row")).toHaveLength(26);
  expect(screen.queryByRole("button", { name: /Mostrar mais/ })).not.toBeInTheDocument();
});
