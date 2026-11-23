import { useAppStore } from "../store/app";

// ThemeSwitch é um interruptor liga/desliga do tema escuro: ligado, o botão
// desliza para a direita e mostra a lua; desligado, fica à esquerda com o sol.
export default function ThemeSwitch() {
  const tema = useAppStore((s) => s.tema);
  const setTema = useAppStore((s) => s.setTema);
  const dark = tema === "escuro";
  return (
    <button
      type="button"
      role="switch"
      aria-checked={dark}
      aria-label="Tema escuro"
      title={dark ? "Tema escuro ligado" : "Tema escuro desligado"}
      onClick={() => setTema(dark ? "claro" : "escuro")}
      className="group inline-flex items-center rounded-full p-2"
    >
      <span
        aria-hidden="true"
        className={`flex h-6 w-11 items-center rounded-full p-0.5 transition-colors duration-300 ease-(--ease-suave) ${dark ? "bg-destaque" : "bg-borda group-hover:bg-texto/15"}`}
      >
        <span
          className={`flex size-5 items-center justify-center rounded-full bg-elevado text-suave shadow-sm transition-transform duration-300 ease-(--ease-suave) ${dark ? "translate-x-5 text-destaque" : ""}`}
        >
          <svg viewBox="0 0 16 16" className="size-3" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
            {dark ? (
              <path d="M13.5 9.5A5.5 5.5 0 0 1 6.5 2.5a5.5 5.5 0 1 0 7 7Z" />
            ) : (
              <>
                <circle cx="8" cy="8" r="2.75" />
                <path d="M8 1.5v1.25M8 13.25v1.25M1.5 8h1.25M13.25 8h1.25M3.4 3.4l.9.9M11.7 11.7l.9.9M12.6 3.4l-.9.9M4.3 11.7l-.9.9" />
              </>
            )}
          </svg>
        </span>
      </span>
    </button>
  );
}
