import { useEffect, useState } from "react";
import { capturePackets, describeFilter, type Capture, type CapturePage } from "../api/captures";
import { ApiError } from "../api/client";
import { linkAcks, type AckIndex } from "../realtime/inspector";
import { Stat } from "./Card";
import PacketTable, { count, type PacketRow } from "./PacketTable";

// CaptureFile mostra os pacotes de uma gravação na tabela do Inspetor, de
// página em página.
export default function CaptureFile({ capture, onClose }: { capture: Capture; onClose: () => void }) {
  const [page, setPage] = useState(0);
  const [data, setData] = useState<CapturePage | null>(null);
  const [rows, setRows] = useState<PacketRow[]>([]);
  const [selected, setSelected] = useState<PacketRow | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let live = true;
    capturePackets(capture.id, page).then(
      (d) => {
        if (!live) return;
        // Os ids seguem a posição no arquivo, para o Nº bater com o Wireshark.
        const acks: AckIndex = new Map();
        setRows(
          linkAcks(
            acks,
            d.pacotes.map((p, i) => ({ ...p, id: page * d.por_pagina + i })),
          ),
        );
        setData(d);
        setSelected(null);
        setError("");
      },
      (e: unknown) => {
        if (!live) return;
        setError(e instanceof ApiError && e.status !== 500 ? e.message : "Não foi possível ler a gravação.");
      },
    );
    return () => {
      live = false;
    };
  }, [capture.id, page]);

  const pages = data ? Math.max(1, Math.ceil(data.total / data.por_pagina)) : 1;

  return (
    <section aria-label={`Gravação Nº ${capture.id}`} className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl bg-superficie px-4 py-3 text-sm">
        <p className="text-suave">
          Mostrando a gravação <span className="text-texto">Nº {capture.id}</span> · filtro{" "}
          <span className="font-mono text-texto">{describeFilter(capture)}</span>. O fluxo ao vivo está parado.
        </p>
        <button type="button" className="botao" onClick={onClose}>
          Voltar ao vivo
        </button>
      </div>

      {error && <p className="text-sm text-erro">{error}</p>}

      <div className="grid grid-cols-3 gap-4 sm:gap-6">
        <Stat label="Pacotes no arquivo" value={data ? count(data.total) : "—"} />
        <Stat label="Nesta página" value={count(rows.length)} />
        <Stat label="Página" value={`${page + 1} de ${pages}`} />
      </div>

      <nav aria-label="Páginas da gravação" className="flex gap-2">
        <button type="button" className="botao-leve" disabled={page === 0} onClick={() => setPage((p) => p - 1)}>
          Anterior
        </button>
        <button type="button" className="botao-leve" disabled={page + 1 >= pages} onClick={() => setPage((p) => p + 1)}>
          Próxima
        </button>
      </nav>

      <PacketTable
        rows={rows}
        empty={data ? "A gravação não tem pacotes." : "Carregando…"}
        selected={selected}
        onSelect={setSelected}
      />
    </section>
  );
}
