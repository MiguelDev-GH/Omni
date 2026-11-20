import { useEffect } from "react";
import { useAppStore, type Toast } from "../store/app";

const TOAST_MS = 10_000;

function ToastItem({ toast }: { toast: Toast }) {
  const dismiss = useAppStore((s) => s.dismissToast);

  useEffect(() => {
    const id = setTimeout(() => dismiss(toast.id), TOAST_MS);
    return () => clearTimeout(id);
  }, [toast.id, dismiss]);

  const isError = toast.kind === "erro";
  return (
    <li
      role={isError ? "alert" : "status"}
      className={`flex items-start gap-3 rounded-lg border bg-superficie p-3 shadow-lg ${
        isError ? "border-red-500/60" : "border-emerald-500/60"
      }`}
    >
      <div className="min-w-0 flex-1">
        <p className={`font-medium ${isError ? "text-erro" : "text-destaque"}`}>{toast.title}</p>
        <p className="text-sm break-words text-suave">{toast.body}</p>
      </div>
      <button type="button" onClick={() => dismiss(toast.id)} className="text-sm text-suave hover:text-texto">
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
