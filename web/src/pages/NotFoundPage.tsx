import { Link } from "react-router";

export default function NotFoundPage() {
  return (
    <div className="space-y-3 py-12">
      <p className="font-mono text-sm text-apagado">404</p>
      <h1 className="text-2xl font-semibold tracking-tight">Página não encontrada</h1>
      <Link to="/alvos" className="link">
        Ir para Alvos
      </Link>
    </div>
  );
}
