import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { useAppStore } from "../store/app";
import ReportPdfPage from "./ReportPdfPage";

test("shows the PDF form for the selected target", () => {
  useAppStore.setState({
    token: "t",
    targets: [
      { id: 1, nome: "api", host: "127.0.0.1", portas_app: [3000] },
      { id: 2, nome: "site", host: "127.0.0.1", portas_app: [443] },
    ],
    selectedTargetId: 2,
  });
  render(<ReportPdfPage />);
  expect(screen.getByRole("heading", { name: "Exportação em PDF" })).toBeInTheDocument();
  expect(screen.getByText("Relatório em PDF")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Baixar PDF" })).toBeInTheDocument();
});

test("without targets it points to the Targets page", () => {
  useAppStore.setState({ token: "t", targets: [], selectedTargetId: null });
  render(<ReportPdfPage />);
  expect(screen.getByText(/Cadastre um alvo/)).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Baixar PDF" })).not.toBeInTheDocument();
});
