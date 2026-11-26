import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { capturePackets, type CapturePage } from "../api/captures";
import { ApiError } from "../api/client";
import { PageHeader, Stat } from "../components/Card";
import PacketTable, { count, type PacketRow } from "../components/PacketTable";
import { linkAcks, type AckIndex } from "../realtime/inspector";

// CaptureInspectorPage é a tabela do Inspetor em modo arquivo: os pacotes de
// uma captura .pcap, de página em página.
export default function CaptureInspectorPage() {
  const id = Number(useParams().id);
  const [page, setPage] = useState(0);
  const [data, setData] = useState<CapturePage | null>(null);
  const [rows, setRows] = useState<PacketRow[]>([]);
  const [selected, setSelected] = useState<PacketRow | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let live = true;
    capturePackets(id, page).then(
      (d) => {
        if (!live) return;
        // Os ids seguem a posição no arquivo, para o Nº bater com o Wireshark.
        const acks: AckIndex = new Map();
        setRows(linkAcks(acks, d.pacotes.map((p, i) => ({ ...p, id: page * d.por_pagina + i }))));
        setData(d);
        setSelected(null);
        setError("");
      },
      (e: unknown) => {
        if (!live) return;
        setError(e instanceof ApiError && e.status !== 500 ? e.message : "Não foi possível ler a captura.");
      },
    );
    return () => {
      live = false;
    };
  }, [id, page]);

  const pages = data ? Math.max(1, Math.ceil(data.total / data.por_pagina)) : 1;

  return (
    <div className="space-y-8">
      <PageHeader
        title={`Captura Nº ${id}`}
        aside={
          <Link to="/capturas" className="botao-leve">
            Voltar às capturas
          </Link>
        }
      >
        Os pacotes do arquivo, na tabela do Inspetor. Como ao vivo, no HTTP aparecem só o método e a rota, e no
        PostgreSQL só o tipo da mensagem; o arquivo completo sai pelo botão Baixar.
      </PageHeader>

      {error && <p className="text-sm text-erro">{error}</p>}

      <div className="grid grid-cols-3 gap-4 sm:gap-6">
        <Stat label="Pacotes no arquivo" value={data ? count(data.total) : "—"} />
        <Stat label="Nesta página" value={count(rows.length)} />
        <Stat label="Página" value={`${page + 1} de ${pages}`} />
      </div>

      <nav aria-label="Páginas" className="flex gap-2">
        <button type="button" className="botao-leve" disabled={page === 0} onClick={() => setPage((p) => p - 1)}>
          Anterior
        </button>
        <button
          type="button"
          className="botao-leve"
          disabled={page + 1 >= pages}
          onClick={() => setPage((p) => p + 1)}
        >
          Próxima
        </button>
      </nav>

      <PacketTable
        rows={rows}
        empty={data ? "A captura não tem pacotes." : "Carregando…"}
        selected={selected}
        onSelect={setSelected}
      />
    </div>
  );
}
