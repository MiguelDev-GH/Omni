import logoClaro from "../assets/logo-claro.png";
import logoEscuro from "../assets/logo-escuro.png";

// Logo mostra a versão da marca que contrasta com o tema: traço escuro no
// tema claro e traço claro no escuro.
export default function Logo({ className }: { className: string }) {
  return (
    <>
      <img src={logoClaro} alt="Omni" className={`${className} dark:hidden`} />
      <img src={logoEscuro} alt="Omni" className={`${className} hidden dark:block`} />
    </>
  );
}
