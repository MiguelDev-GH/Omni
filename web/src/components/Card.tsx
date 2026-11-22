// Stat é um número grande sobre o fundo, sem caixa: o tique de cor à esquerda
// é o único marcador.
export function Stat({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <div className="relative pl-3.5">
      <span aria-hidden="true" className="absolute top-1 bottom-1 left-0 w-px bg-borda" />
      <span aria-hidden="true" className="absolute top-1 left-0 h-3 w-px bg-destaque" />
      <p className="text-xs text-suave">{label}</p>
      <p className={`mt-1 font-mono text-lg font-medium tracking-tight tabular-nums sm:text-2xl ${tone ?? ""}`}>{value}</p>
    </div>
  );
}

// Card agrupa um título e o conteúdo. Por padrão fica direto sobre o fundo
// (gráficos); "plane" dá um plano levemente mais claro, sem borda, para
// tabelas e formulários.
export function Card({
  title,
  children,
  plane = false,
  action,
}: {
  title: string;
  children: React.ReactNode;
  plane?: boolean;
  action?: React.ReactNode;
}) {
  return (
    <section className={`min-w-0 ${plane ? "rounded-2xl bg-superficie/80 p-5" : "py-1"}`}>
      <div className="mb-3 flex min-h-7 flex-wrap items-center justify-between gap-2">
        <h2 className="text-sm font-medium text-texto">{title}</h2>
        {action}
      </div>
      {children}
    </section>
  );
}

// PageHeader é o título da página com uma descrição curta opcional.
export function PageHeader({
  title,
  children,
  aside,
}: {
  title: string;
  children?: React.ReactNode;
  aside?: React.ReactNode;
}) {
  return (
    <header className="space-y-2">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        {aside}
      </div>
      {children && <div className="max-w-3xl text-sm leading-relaxed text-suave">{children}</div>}
    </header>
  );
}

// Notice é um aviso em faixa suave, sem borda.
export function Notice({ tone = "aviso", children, role }: { tone?: "aviso" | "info"; children: React.ReactNode; role?: string }) {
  const style = tone === "aviso" ? "bg-aviso-tinta text-aviso" : "bg-superficie text-suave";
  return (
    <p role={role} className={`rounded-xl px-4 py-3 text-sm ${style}`}>
      {children}
    </p>
  );
}

// InternalBadge marca as telas que mostram a visão de dentro da máquina.
export function InternalBadge() {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full bg-aviso-tinta px-2.5 py-0.5 text-xs font-medium text-aviso">
      <span aria-hidden="true" className="size-1.5 rounded-full bg-aviso" />
      Visão interna
    </span>
  );
}
