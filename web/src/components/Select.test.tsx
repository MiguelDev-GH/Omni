import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { expect, test } from "vitest";
import Select from "./Select";

const options = [
  { value: "a", label: "Alfa" },
  { value: "b", label: "Bravo" },
  { value: "c", label: "Charlie" },
];

function Harness() {
  const [v, setV] = useState("a");
  return <Select label="Letra" value={v} onChange={setV} options={options} />;
}

test("picks an option with the mouse", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  const box = screen.getByRole("combobox", { name: "Letra" });
  expect(box).toHaveTextContent("Alfa");
  await user.click(box);
  expect(box).toHaveAttribute("aria-expanded", "true");
  await user.click(screen.getByRole("option", { name: "Charlie" }));
  expect(box).toHaveTextContent("Charlie");
  expect(box).toHaveAttribute("aria-expanded", "false");
});

test("navigates with the keyboard and closes with Escape", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  const box = screen.getByRole("combobox", { name: "Letra" });
  box.focus();
  await user.keyboard("{ArrowDown}{ArrowDown}{Enter}");
  expect(box).toHaveTextContent("Bravo");
  await user.keyboard("c{Enter}");
  expect(box).toHaveTextContent("Charlie");
  await user.keyboard("{ArrowDown}{Escape}");
  expect(box).toHaveAttribute("aria-expanded", "false");
  expect(box).toHaveTextContent("Charlie");
});
