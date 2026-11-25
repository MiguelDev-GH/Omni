import type { ConnSnapshot } from "../api/connections";

// connSnapshot monta uma coleta com a porta 3000 da aplicação e a 5432 do banco.
export function connSnapshot(hora: string, app: number): ConnSnapshot {
  return {
    hora,
    portas: [
      { porta: 3000, tipo: "app", estabelecidas: app, estados: { ESTABLISHED: app }, clientes: [] },
      {
        porta: 5432,
        tipo: "banco",
        estabelecidas: 5,
        estados: { ESTABLISHED: 5 },
        clientes: [
          { processo: "loja", conexoes: 4 },
          { processo: "", conexoes: 1 },
        ],
      },
    ],
  };
}
