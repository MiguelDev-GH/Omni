# Relatórios em PDF por intervalo

## Decisões (24/11/2026)

- **Intervalo:** presets de 1 dia e 1 semana, mais um intervalo personalizado (de/até). Máximo de 168 h, igual à retenção padrão das verificações (`OMNI_RETENCAO_VERIFICACOES`). Relatório de mês fica para depois.
- **Alertas:** passam a ser gravados no SQLite (`010_alerts.sql`) a cada mudança de nível, apagados junto com a retenção das verificações. Alertas anteriores a essa mudança não existem no banco.
- **PDF:** gerado no navegador com jsPDF; os gráficos entram como imagem.

## Conteúdo do relatório

1. Cabeçalho: alvo (nome, host, portas), intervalo, hora de geração.
2. Resumo de alertas: contagem de verde, amarelo e vermelho no intervalo e a lista de mudanças de nível.
3. Arsenal: cada varredura do alvo no intervalo — ferramenta, status, contagem de achados por severidade e os achados.
4. Gráficos atuais da aplicação (disponibilidade/latência, tráfego, HTTP, banco, sistema) recalculados para o intervalo.

## Etapas

1. [x] Persistir alertas no SQLite e expor `GET /api/alertas/historico?desde=&ate=`.
2. [x] Tráfego, HTTP, banco e sistema seguem `OMNI_RETENCAO_VERIFICACOES` (antes, 24 h fixas).
3. [x] `GET /api/alvos/{id}/relatorio?desde=&ate=` devolve tudo de uma vez: alvo, contagem e lista de alertas, disponibilidade por porta, séries agrupadas em até 300 pontos (passo múltiplo de 10 s) e as varreduras criadas no intervalo com os achados. Tráfego, HTTP, banco e sistema são do servidor todo. Os p50/p95 agrupados são média ponderada dos percentis, uma aproximação.
4. [ ] Tela "Gerar relatório" no frontend: escolher alvo e intervalo.
5. [ ] Montar o PDF com jsPDF (texto, tabelas, gráficos capturados como imagem).
