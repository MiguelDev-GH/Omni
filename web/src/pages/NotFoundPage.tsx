import { Link } from "react-router";

export default function NotFoundPage() {
  return (
    <div className="space-y-2">
      <h1 className="text-xl font-semibold">Página não encontrada</h1>
      <Link to="/alvos" className="text-destaque hover:underline">
        Ir para Alvos
      </Link>
    </div>
  );
}
