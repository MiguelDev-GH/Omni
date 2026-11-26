import { useEffect, useRef, useState, type FormEvent } from "react";
import type { Capture } from "../api/captures";
import CaptureFile from "../components/CaptureFile";
import CaptureRecorder from "../components/CaptureRecorder";
import { Notice, PageHeader, Stat } from "../components/Card";
import PacketTable, { count, type PacketRow } from "../components/PacketTable";
import Select from "../components/Select";
import {
  appendRows,
  emptyFilter,
  linkAcks,
  MAX_ROWS,
  onPackets,
  pruneAcks,
  setInspector,
  type AckIndex,
  type InspectorFilter,
} from "../realtime/inspector";
import { useAppStore } from "../store/app";

type Row = PacketRow;

const protocols = ["HTTP", "HTTPS", "PostgreSQL", "MySQL", "DNS", "SMTP", "TCP", "UDP", "ICMP"];

const protocolOptions = [{ value: "", label: "Todos os protocolos" }, ...protocols.map((p) => ({ value: p, label: p }))];

// validFilter confere o formulário e devolve o filtro ou a mensagem de erro.
function validFilter(ip: string, port: string, protocolo: string): InspectorFilter | string {
  const p = port.trim() === "" ? 0 : Number(port);
  if (!Number.isInteger(p) || p < 0 || p > 65535) return "A porta precisa ser um número de 1 a 65535.";
  const addr = ip.trim();
  if (addr !== "" && !/^[0-9a-fA-F.:]+$/.test(addr)) return "O IP precisa ser um endereço IPv4 ou IPv6.";
  return { ip: addr, porta: p, protocolo };
}

export default function InspectorPage() {
  const sniffer = useAppStore((s) => s.engines.sniffer);
  const [rows, setRows] = useState<Row[]>([]);
  const [dropped, setDropped] = useState(0);
  const [paused, setPaused] = useState(false);
  const [filter, setFilter] = useState<InspectorFilter>(emptyFilter);
  const [ipField, setIpField] = useState("");
  const [portField, setPortField] = useState("");
  const [protoField, setProtoField] = useState("");
  const [filterError, setFilterError] = useState("");
  const [selected, setSelected] = useState<Row | null>(null);
  // opened é a gravação aberta na tabela; com ela, o fluxo ao vivo para.
  const [opened, setOpened] = useState<Capture | null>(null);
  const nextId = useRef(0);
  const acks = useRef<AckIndex>(new Map());

  // Assina enquanto a página está aberta, não pausada e sem gravação aberta;
  // sair da página, pausar ou abrir uma gravação manda ativo:false.
  const live = !paused && opened === null;
  useEffect(() => {
    if (!live) return;
    setInspector(true, filter);
    const off = onPackets((b) => {
      const batch = linkAcks(
        acks.current,
        b.pacotes.map((p) => ({ ...p, id: nextId.current++ })),
      );
      setRows((prev) => {
        const next = appendRows(prev, batch);
        if (acks.current.size > 2 * MAX_ROWS) pruneAcks(acks.current, next[0]!.id);
        return next;
      });
      if (b.descartados) setDropped((d) => d + b.descartados);
    });
    return () => {
      off();
      setInspector(false, filter);
    };
  }, [live, filter]);

  function applyFilter(e: FormEvent) {
    e.preventDefault();
    const f = validFilter(ipField, portField, protoField);
    if (typeof f === "string") {
      setFilterError(f);
      return;
    }
    setFilterError("");
    acks.current.clear();
    setRows([]);
    setSelected(null);
    setFilter(f);
  }

  function clear() {
    acks.current.clear();
    setRows([]);
    setDropped(0);
    setSelected(null);
  }

  return (
    <div className="space-y-8">
      <PageHeader
        title="Inspetor de pacotes"
        aside={
          opened === null && (
            <div className="flex gap-2">
              <button type="button" className="botao-leve" onClick={clear}>
                Limpar
              </button>
              <button type="button" className="botao" aria-pressed={paused} onClick={() => setPaused((p) => !p)}>
                {paused ? "Retomar" : "Pausar"}
              </button>
            </div>
          )
        }
      >
        Os pacotes das portas dos alvos, ao vivo, um por linha. No HTTP aparecem só o método e a rota, e no PostgreSQL só o
        tipo da mensagem: cabeçalhos, corpos e consultas nunca saem do servidor. A tabela guarda os últimos{" "}
        {count(MAX_ROWS)} pacotes. Para guardar o tráfego, grave em .pcap com o filtro aplicado, mais abaixo.
      </PageHeader>

      {sniffer !== "ativo" && (
        <Notice>
          {sniffer === "sem-permissao"
            ? "O sniffer está sem permissão de captura, então nenhum pacote vai chegar. Veja no README como liberar a captura."
            : "O sniffer não está capturando agora. Cadastre um alvo com portas para ele começar."}
        </Notice>
      )}

      <form onSubmit={applyFilter} aria-label="Filtros" className="flex flex-wrap items-end gap-3">
        <div className="space-y-1.5">
          <label htmlFor="filtro-ip" className="block text-xs font-medium text-suave">
            IP
          </label>
          <input
            id="filtro-ip"
            className="campo w-48 font-mono"
            placeholder="qualquer"
            value={ipField}
            onChange={(e) => setIpField(e.target.value)}
          />
        </div>
        <div className="space-y-1.5">
          <label htmlFor="filtro-porta" className="block text-xs font-medium text-suave">
            Porta
          </label>
          <input
            id="filtro-porta"
            inputMode="numeric"
            className="campo w-28 font-mono"
            placeholder="qualquer"
            value={portField}
            onChange={(e) => setPortField(e.target.value)}
          />
        </div>
        <div className="space-y-1.5">
          <span className="block text-xs font-medium text-suave">Protocolo</span>
          <Select label="Protocolo" value={protoField} options={protocolOptions} onChange={setProtoField} />
        </div>
        <button type="submit" className="botao">
          Filtrar
        </button>
        {filterError && (
          <p role="alert" className="w-full text-sm text-erro">
            {filterError}
          </p>
        )}
      </form>

      {opened ? (
        <CaptureFile key={opened.id} capture={opened} onClose={() => setOpened(null)} />
      ) : (
        <>
          <div className="grid grid-cols-3 gap-4 sm:gap-6">
            <Stat label="Na tabela" value={count(rows.length)} />
            <Stat label="Descartados" value={count(dropped)} tone={dropped > 0 ? "text-aviso" : undefined} />
            <Stat label="Fluxo" value={paused ? "pausado" : "ao vivo"} tone={paused ? "text-suave" : "text-destaque"} />
          </div>

          <PacketTable
            rows={rows}
            empty={paused ? "Pausado. Retome para ver os pacotes novos." : "Esperando pacotes…"}
            selected={selected}
            onSelect={setSelected}
          />
        </>
      )}

      <CaptureRecorder
        filter={filter}
        canRecord={sniffer === "ativo"}
        openedId={opened?.id ?? null}
        onOpen={(c) => {
          setOpened(c);
          window.scrollTo({ top: 0, behavior: "smooth" });
        }}
        onDeleted={(id) => setOpened((o) => (o?.id === id ? null : o))}
      />
    </div>
  );
}
