import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { expect, test } from "vitest";
import Sidebar from "./Sidebar";

test("shows sections and disables future items", () => {
  render(
    <MemoryRouter>
      <Sidebar />
    </MemoryRouter>,
  );
  for (const name of ["Monitoramento", "Tráfego", "Arsenal", "Relatórios"]) {
    expect(screen.getByRole("heading", { level: 2, name })).toBeInTheDocument();
  }
  expect(screen.getByRole("link", { name: "Alvos" })).toHaveAttribute("href", "/alvos");
  expect(screen.getByRole("link", { name: "Geral" })).toHaveAttribute("href", "/geral");
  expect(screen.getByRole("link", { name: "Alertas" })).toHaveAttribute("href", "/alertas");
  expect(screen.getByRole("link", { name: "Visão geral" })).toHaveAttribute("href", "/arsenal");
  expect(screen.getByRole("link", { name: "Nmap" })).toHaveAttribute("href", "/arsenal/nmap");
  expect(screen.getByRole("link", { name: "Nuclei" })).toHaveAttribute("href", "/arsenal/nuclei");
  expect(screen.getByRole("link", { name: "testssl" })).toHaveAttribute("href", "/arsenal/testssl");
  expect(screen.getByRole("link", { name: "Nikto" })).toHaveAttribute("href", "/arsenal/nikto");
  expect(screen.getByRole("link", { name: "ffuf" })).toHaveAttribute("href", "/arsenal/ffuf");
  const sqlmap = screen.getByText("SQLMap").closest("[aria-disabled]");
  expect(sqlmap).toHaveAttribute("aria-disabled", "true");
  expect(screen.queryByRole("link", { name: /SQLMap/ })).toBeNull();
  expect(screen.getAllByText("em breve").length).toBeGreaterThan(0);
});
