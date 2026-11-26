import { Card, InternalBadge, PageHeader } from "../components/Card";
import ReportPdfForm from "../components/ReportPdfForm";
import Select from "../components/Select";
import { useAppStore } from "../store/app";

// ReportPdfPage é a Exportação em PDF: escolhe o alvo e o intervalo e baixa o
// relatório montado no navegador.
export default function ReportPdfPage() {
  const targets = useAppStore((s) => s.targets);
  const selected = useAppStore((s) => s.selectedTargetId);
  const selectTarget = useAppStore((s) => s.selectTarget);
  const selectedTarget = targets.find((t) => t.id === selected);

  return (
    <div className="space-y-8">
      <PageHeader title="Exportação em PDF" aside={<InternalBadge />} />

      {targets.length === 0 ? (
        <p className="text-sm text-suave">Cadastre um alvo em Alvos para gerar o relatório.</p>
      ) : (
        <div className="flex items-center gap-2 text-sm">
          <label htmlFor="alvo-pdf" className="text-suave">
            Alvo
          </label>
          <Select
            id="alvo-pdf"
            value={selected}
            onChange={selectTarget}
            options={targets.map((t) => ({ value: t.id, label: t.nome, hint: t.host }))}
          />
        </div>
      )}

      {selectedTarget && (
        <Card title="Relatório em PDF" plane>
          <ReportPdfForm target={selectedTarget} />
        </Card>
      )}
    </div>
  );
}
