import { useState, type FormEvent } from "react";
import { ApiError } from "../api/client";
import type { Suggestion } from "../api/discovery";
import { createTarget, parsePorts, updateTarget, type Target, type TargetInput } from "../api/targets";
import { useAppStore } from "../store/app";
import DiscoveredServices from "./DiscoveredServices";

type Field = "nome" | "host" | "portas_app" | "porta_banco" | "dominio" | "porta_dns" | "porta_smtp" | "repositorio";

const singlePorts = ["porta_banco", "porta_dns", "porta_smtp"] as const;

type Props = { initial?: Target; onDone: () => void; onCancel: () => void };

export default function TargetForm({ initial, onDone, onCancel }: Props) {
  const [values, setValues] = useState<Record<Field, string>>({
    nome: initial?.nome ?? "",
    host: initial?.host ?? "",
    portas_app: initial?.portas_app.join(", ") ?? "",
    porta_banco: initial?.porta_banco ? String(initial.porta_banco) : "",
    dominio: initial?.dominio ?? "",
    porta_dns: initial?.porta_dns ? String(initial.porta_dns) : "",
    porta_smtp: initial?.porta_smtp ? String(initial.porta_smtp) : "",
    repositorio: initial?.repositorio ?? "",
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
    const input: TargetInput = { nome: values.nome.trim(), host: values.host.trim(), portas_app: ports };
    for (const field of singlePorts) {
      const v = values[field].trim();
      if (v === "") continue;
      if (!/^\d+$/.test(v)) {
        setErrors({ [field]: "Use um número de porta." });
        return;
      }
      input[field] = Number(v);
    }
    if (values.dominio.trim() !== "") input.dominio = values.dominio.trim();
    if (values.repositorio.trim() !== "") input.repositorio = values.repositorio.trim();

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
      <div className="space-y-1.5">
        <label htmlFor={field} className="block text-xs font-medium text-suave">
          {label}
        </label>
        <input
          id={field}
          type="text"
          inputMode={field === "portas_app" || (singlePorts as readonly string[]).includes(field) ? "numeric" : undefined}
          value={values[field]}
          onChange={(e) => setValues((v) => ({ ...v, [field]: e.target.value }))}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy || undefined}
          className="campo"
        />
        {hint && (
          <p id={`${field}-dica`} className="text-xs text-apagado">
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
    <form onSubmit={onSubmit} className="animate-entrada space-y-5 rounded-2xl bg-superficie/80 p-5">
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
        {input("dominio", "Domínio (opcional)", "A sonda de DNS resolve este nome e a de SMTP testa postmaster@domínio")}
        {input("porta_dns", "Porta DNS (opcional)", "Servidor DNS no mesmo host, ex.: 53")}
        {input("porta_smtp", "Porta SMTP (opcional)", "Servidor de e-mail no mesmo host, ex.: 25 ou 587")}
        {input(
          "repositorio",
          "Código-fonte (opcional)",
          "Caminho absoluto da pasta neste servidor, ex.: /srv/loja. O Gitleaks a lê como somente leitura",
        )}
      </div>
      <div className="flex gap-2">
        <button
          type="submit"
          disabled={saving}
          className="botao"
        >
          Salvar
        </button>
        <button type="button" onClick={onCancel} className="botao-leve">
          Cancelar
        </button>
      </div>
    </form>
  );
}
