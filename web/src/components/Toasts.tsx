import { useEffect } from "react";
import { Link } from "react-router";
import { useAppStore, type Toast } from "../store/app";

const TOAST_MS = 10_000;

const toastStyle: Record<Toast["kind"], { bar: string; title: string }> = {
  erro: { bar: "bg-erro", title: "text-erro" },
  aviso: { bar: "bg-aviso", title: "text-aviso" },
  ok: { bar: "bg-destaque", title: "text-destaque-forte" },
};

function ToastItem({ toast }: { toast: Toast }) {
  const dismiss = useAppStore((s) => s.dismissToast);

  useEffect(() => {
    const id = setTimeout(() => dismiss(toast.id), TOAST_MS);
    return () => clearTimeout(id);
  }, [toast.id, dismiss]);

  const isError = toast.kind === "erro";
  const style = toastStyle[toast.kind];
  return (
    <li
      role={isError ? "alert" : "status"}
      className="relative flex animate-entrada items-start gap-3 overflow-hidden rounded-xl bg-elevado py-3 pr-3 pl-4 shadow-[0_16px_40px_-16px_rgb(22_33_30/0.3),0_0_0_1px_rgb(22_33_30/0.05)]"
    >
      <span aria-hidden="true" className={`absolute inset-y-0 left-0 w-1 ${style.bar}`} />
      <div className="min-w-0 flex-1">
        <p className={`text-sm font-medium ${style.title}`}>{toast.title}</p>
        <p className="text-sm break-words text-suave">{toast.body}</p>
        {toast.link && (
          <Link to={toast.link.to} onClick={() => dismiss(toast.id)} className="link text-sm">
            {toast.link.label}
          </Link>
        )}
      </div>
      <button type="button" onClick={() => dismiss(toast.id)} className="botao-leve -my-1 px-2 py-1 text-xs">
        Fechar
      </button>
    </li>
  );
}

export default function Toasts() {
  const toasts = useAppStore((s) => s.toasts);
  return (
    <ul className="fixed right-4 bottom-4 left-4 z-50 flex flex-col gap-2 sm:left-auto sm:w-96">
      {toasts.map((t) => (
        <ToastItem key={t.id} toast={t} />
      ))}
    </ul>
  );
}
