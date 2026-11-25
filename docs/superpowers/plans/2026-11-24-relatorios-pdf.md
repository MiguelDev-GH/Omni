# Relatórios em PDF por intervalo

## Decisões (24/11/2026)

- **Intervalo:** presets de 1 dia e 1 semana, mais um intervalo personalizado (de/até). Máximo de 168 h, igual à retenção padrão das verificações (`OMNI_RETENCAO_VERIFICACOES`). Relatório de mês fica para depois.
- **Alertas:** passam a ser gravados no SQLite (`010_alerts.sql`) a cada mudança de nível, apagados junto com a retenção das verificações. Alertas anteriores a essa mudança não existem no banco.
- **PDF:** gerado no navegador com jsPDF. Os gráficos são desenhados em vetor direto no PDF, em vez de capturar a tela: não dependem do tema nem do tamanho da janela, e o texto fica selecionável.

## Conteúdo do relatório

1. Cabeçalho: alvo (nome, host, portas), intervalo, hora de geração.
2. Resumo de alertas: contagem de verde, amarelo e vermelho no intervalo e a lista de mudanças de nível.
3. Arsenal: cada varredura do alvo no intervalo — ferramenta, status, contagem de achados por severidade e os achados.
4. Gráficos atuais da aplicação (disponibilidade/latência, tráfego, HTTP, banco, sistema) recalculados para o intervalo.

## Etapas

1. [x] Persistir alertas no SQLite e expor `GET /api/alertas/historico?desde=&ate=`.
2. [x] Tráfego, HTTP, banco e sistema seguem `OMNI_RETENCAO_VERIFICACOES` (antes, 24 h fixas).
3. [x] `GET /api/alvos/{id}/relatorio?desde=&ate=` devolve tudo de uma vez: alvo, contagem e lista de alertas, disponibilidade por porta, séries agrupadas em até 300 pontos (passo múltiplo de 10 s) e as varreduras criadas no intervalo com os achados. Tráfego, HTTP, banco e sistema são do servidor todo. Os p50/p95 agrupados são média ponderada dos percentis, uma aproximação.
4. [x] Cartão "Relatório em PDF" na página Relatórios: alvo, intervalo (último dia, última semana, personalizado) e validação do limite de 168 h.
5. [x] `web/src/lib/reportPdf.ts` monta o PDF (jsPDF + jspdf-autotable), carregado só no clique. A rota `GET /api/alvos/{id}/relatorio` foi registrada em `cmd/omni/run.go`, que faltava.

## Ajuste depois da detecção de segurança (25/11/2026)

Os detectores de segurança geram alertas por IP e por conta, então um ataque pode gravar milhares de mudanças no intervalo. O relatório passou a:

- contar as mudanças no banco, por regra e por nível, sem limite (`resumo_alertas` e `contagem_alertas`);
- listar só as 100 mudanças mais graves (vermelho, depois amarelo, depois verde; no mesmo nível, as mais novas), e o PDF avisa quando a lista está cortada.
