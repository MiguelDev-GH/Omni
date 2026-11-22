import { useEffect, useId, useRef, useState, type KeyboardEvent } from "react";

export type Option<T extends string | number> = { value: T; label: string; hint?: string };

type Props<T extends string | number> = {
  id?: string;
  label?: string; // nome acessível quando não há <label htmlFor>
  value: T | null;
  options: Option<T>[];
  onChange: (value: T) => void;
  placeholder?: string;
  disabled?: boolean;
  className?: string;
};

// Select é uma lista suspensa própria, com o comportamento de teclado de um
// combobox: setas movem o destaque, Enter escolhe, Esc fecha e letras pulam
// para a primeira opção que começa com elas.
export default function Select<T extends string | number>({
  id,
  label,
  value,
  options,
  onChange,
  placeholder = "Selecione",
  disabled = false,
  className = "",
}: Props<T>) {
  const autoId = useId();
  const buttonId = id ?? `${autoId}-botao`;
  const listId = `${autoId}-lista`;
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const root = useRef<HTMLDivElement>(null);
  const list = useRef<HTMLUListElement>(null);

  const selectedIndex = options.findIndex((o) => o.value === value);
  const selected = options[selectedIndex];
  const isDisabled = disabled || options.length === 0;

  // Fecha ao clicar fora.
  useEffect(() => {
    if (!open) return;
    const onDown = (e: PointerEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", onDown);
    return () => document.removeEventListener("pointerdown", onDown);
  }, [open]);

  // Mantém a opção destacada visível quando a lista rola.
  useEffect(() => {
    if (!open) return;
    list.current?.children[active]?.scrollIntoView?.({ block: "nearest" });
  }, [open, active]);

  function show() {
    setActive(Math.max(selectedIndex, 0));
    setOpen(true);
  }

  function choose(i: number) {
    const o = options[i];
    if (o) onChange(o.value);
    setOpen(false);
  }

  function onKeyDown(e: KeyboardEvent<HTMLButtonElement>) {
    const last = options.length - 1;
    const move = (i: number) => {
      e.preventDefault();
      if (!open) show();
      setActive(Math.min(Math.max(i, 0), last));
    };
    switch (e.key) {
      case "ArrowDown":
        return open ? move(active + 1) : move(Math.max(selectedIndex, 0));
      case "ArrowUp":
        return open ? move(active - 1) : move(Math.max(selectedIndex, 0));
      case "Home":
        return move(0);
      case "End":
        return move(last);
      case "Enter":
      case " ":
        e.preventDefault();
        return open ? choose(active) : show();
      case "Escape":
        if (open) {
          e.preventDefault();
          setOpen(false);
        }
        return;
      case "Tab":
        setOpen(false);
        return;
    }
    if (e.key.length === 1) {
      const k = e.key.toLowerCase();
      const from = open ? active + 1 : 0;
      const order = [...options.slice(from), ...options.slice(0, from)];
      const hit = order.find((o) => o.label.toLowerCase().startsWith(k));
      if (hit) move(options.indexOf(hit));
    }
  }

  return (
    <div ref={root} className={`relative inline-block ${className}`}>
      <button
        id={buttonId}
        type="button"
        role="combobox"
        aria-label={label}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={listId}
        aria-activedescendant={open ? `${listId}-${active}` : undefined}
        disabled={isDisabled}
        onClick={() => (open ? setOpen(false) : show())}
        onKeyDown={onKeyDown}
        className="group flex w-full min-w-36 items-center justify-between gap-3 rounded-lg bg-elevado/70 py-1.5 pr-2.5 pl-3 text-left text-sm ring-1 ring-borda/70 ring-inset hover:bg-elevado disabled:cursor-not-allowed disabled:text-apagado aria-expanded:bg-elevado aria-expanded:ring-destaque/60"
      >
        <span className="truncate">{selected?.label ?? placeholder}</span>
        <svg
          aria-hidden="true"
          viewBox="0 0 12 12"
          className="size-3 shrink-0 text-apagado transition-transform duration-200 ease-(--ease-suave) group-aria-expanded:rotate-180 group-aria-expanded:text-destaque"
        >
          <path d="M2.5 4.5 6 8l3.5-3.5" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </svg>
      </button>
      <ul
        ref={list}
        id={listId}
        role="listbox"
        aria-labelledby={buttonId}
        data-open={open}
        className="invisible absolute top-full left-0 z-40 mt-1.5 max-h-72 min-w-full origin-top -translate-y-1 scale-[0.98] overflow-auto rounded-xl bg-elevado p-1 text-sm opacity-0 shadow-[0_12px_32px_-12px_rgb(22_33_30/0.25),0_0_0_1px_rgb(22_33_30/0.06)] transition-[opacity,transform,visibility] duration-200 ease-(--ease-suave) data-[open=true]:visible data-[open=true]:translate-y-0 data-[open=true]:scale-100 data-[open=true]:opacity-100"
      >
        {options.map((o, i) => (
          <li
            key={String(o.value)}
            id={`${listId}-${i}`}
            role="option"
            aria-selected={o.value === value}
            data-active={i === active}
            onPointerMove={() => setActive(i)}
            onClick={() => choose(i)}
            className="flex cursor-pointer items-center gap-2 rounded-lg px-2.5 py-1.5 whitespace-nowrap data-[active=true]:bg-fundo"
          >
            <span
              aria-hidden="true"
              className={`size-1.5 shrink-0 rounded-full ${o.value === value ? "bg-destaque" : "bg-transparent"}`}
            />
            <span className={o.value === value ? "font-medium" : ""}>{o.label}</span>
            {o.hint && <span className="ml-auto pl-4 font-mono text-xs text-apagado">{o.hint}</span>}
          </li>
        ))}
      </ul>
    </div>
  );
}
