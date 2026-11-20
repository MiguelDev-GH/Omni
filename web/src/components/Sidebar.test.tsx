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
  const nmap = screen.getByText("Nmap").closest("[aria-disabled]");
  expect(nmap).toHaveAttribute("aria-disabled", "true");
  expect(screen.queryByRole("link", { name: /Nmap/ })).toBeNull();
  expect(screen.getAllByText("em breve").length).toBeGreaterThan(0);
});
