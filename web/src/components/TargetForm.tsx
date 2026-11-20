import { useState, type FormEvent } from "react";
import { ApiError } from "../api/client";
import type { Suggestion } from "../api/discovery";
import { createTarget, parsePorts, updateTarget, type Target, type TargetInput } from "../api/targets";
import { useAppStore } from "../store/app";
import DiscoveredServices from "./DiscoveredServices";

type Field = "nome" | "host" | "portas_app" | "porta_banco" | "dominio";

type Props = { initial?: Target; onDone: () => void; onCancel: () => void };

export default function TargetForm({ initial, onDone, onCancel }: Props) {
  const [values, setValues] = useState<Record<Field, string>>({
    nome: initial?.nome ?? "",
    host: initial?.host ?? "",
    portas_app: initial?.portas_app.join(", ") ?? "",
    porta_banco: initial?.porta_banco ? String(initial.porta_banco) : "",
    dominio: initial?.dominio ?? "",
  });
  const [errors, setErrors] = useState<Partial<Record<Field, string>>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const targets = useAppStore((s) => s.targets);

  // pick preenche o formulário a partir de um serviço detectado na máquina.
  function pick(s: Suggestion) {
    setValues((v) =>
      s.tipo === "banco"
        ? { ...v, porta_banco: String(s.portas[0] ?? "") }
        : {
            ...v,
            nome: v.nome.trim() === "" && s.processo ? s.processo : v.nome,
            host: s.host,
            portas_app: s.portas.join(", "),
          },
    );
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErrors({});
    setFormError(null);
    const ports = parsePorts(values.portas_app);
    if (ports === null) {
      setErrors({ portas_app: "Use só números separados por vírgula." });
      return;
    }
    const dbPort = values.porta_banco.trim();
    if (dbPort !== "" && !/^\d+$/.test(dbPort)) {
      setErrors({ porta_banco: "Use um número de porta." });
      return;
    }
    const input: TargetInput = { nome: values.nome.trim(), host: values.host.trim(), portas_app: ports };
    if (dbPort !== "") input.porta_banco = Number(dbPort);
    if (values.dominio.trim() !== "") input.dominio = values.dominio.trim();

    setSaving(true);
    try {
      if (initial) await updateTarget(initial.id, input);
      else await createTarget(input);
      onDone();
    } catch (err) {
      if (err instanceof ApiError && err.field && err.field in values) {
        setErrors({ [err.field]: err.message });
      } else {
        setFormError(err instanceof Error ? err.message : "Não foi possível salvar.");
      }
    } finally {
      setSaving(false);
    }
  }

  function input(field: Field, label: string, hint?: string) {
    const error = errors[field];
    const describedBy = [hint && `${field}-dica`, error && `${field}-erro`].filter(Boolean).join(" ");
    return (
      <div className="space-y-1">
        <label htmlFor={field} className="block text-sm text-suave">
          {label}
        </label>
        <input
          id={field}
          type="text"
          inputMode={field === "portas_app" || field === "porta_banco" ? "numeric" : undefined}
          value={values[field]}
          onChange={(e) => setValues((v) => ({ ...v, [field]: e.target.value }))}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy || undefined}
          className="w-full rounded border border-borda bg-fundo px-3 py-2 outline-none focus:border-destaque aria-invalid:border-erro"
        />
        {hint && (
          <p id={`${field}-dica`} className="text-xs text-zinc-500">
            {hint}
          </p>
        )}
        {error && (
          <p id={`${field}-erro`} className="text-sm text-erro">
            {error}
          </p>
        )}
      </div>
    );
  }

  return (
    <form onSubmit={onSubmit} className="space-y-4 rounded-lg border border-borda bg-superficie p-4">
      <h2 className="font-semibold">{initial ? `Editar ${initial.nome}` : "Novo alvo"}</h2>
      {formError && (
        <p role="alert" className="text-sm text-erro">
          {formError}
        </p>
      )}
      {!initial && <DiscoveredServices targets={targets} onPick={pick} />}
      <div className="grid gap-4 sm:grid-cols-2">
        {input("nome", "Nome")}
        {input("host", "Host")}
        {input("portas_app", "Portas da aplicação", "Separe por vírgula, ex.: 3000, 8080")}
        {input("porta_banco", "Porta do banco (opcional)")}
        {input("dominio", "Domínio (opcional)")}
      </div>
      <div className="flex gap-2">
        <button
          type="submit"
          disabled={saving}
          className="rounded bg-destaque px-3 py-1.5 font-medium text-zinc-950 disabled:opacity-60"
        >
          Salvar
        </button>
        <button type="button" onClick={onCancel} className="rounded px-3 py-1.5 text-suave hover:text-texto">
          Cancelar
        </button>
      </div>
    </form>
  );
}
