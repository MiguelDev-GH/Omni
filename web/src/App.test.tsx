import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import App from "./App";

test("renders title", () => {
  render(<App />);
  expect(screen.getByRole("heading", { name: "Omni" })).toBeInTheDocument();
});
