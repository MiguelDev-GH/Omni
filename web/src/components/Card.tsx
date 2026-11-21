export function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-borda bg-superficie p-4">
      <p className="text-xs text-suave">{label}</p>
      <p className="mt-1 text-2xl font-semibold tabular-nums">{value}</p>
    </div>
  );
}

export function Card({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="min-w-0 rounded-lg border border-borda bg-superficie p-4">
      <h2 className="mb-3 text-sm font-medium text-suave">{title}</h2>
      {children}
    </section>
  );
}
