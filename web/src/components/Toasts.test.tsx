import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import { useAppStore } from "../store/app";
import Toasts from "./Toasts";

test("close button removes toast", async () => {
  useAppStore.getState().pushToast({ kind: "ok", title: "Serviço restaurado", body: "api, porta 3000" });
  render(<Toasts />);
  expect(screen.getByRole("status")).toHaveTextContent("Serviço restaurado");
  await userEvent.setup().click(screen.getByRole("button", { name: "Fechar" }));
  expect(screen.queryByText("Serviço restaurado")).toBeNull();
});

test("toast disappears after 10 s", () => {
  vi.useFakeTimers();
  useAppStore.getState().pushToast({ kind: "erro", title: "Serviço fora do ar", body: "api, porta 3000: x" });
  render(<Toasts />);
  expect(screen.getByRole("alert")).toBeInTheDocument();
  act(() => vi.advanceTimersByTime(9_999));
  expect(screen.getByRole("alert")).toBeInTheDocument();
  act(() => vi.advanceTimersByTime(1));
  expect(screen.queryByRole("alert")).toBeNull();
});
