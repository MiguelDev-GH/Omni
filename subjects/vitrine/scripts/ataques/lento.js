// npm run ataque:lento: martela o relatório de vendas, que espera dentro do
// banco (pg_sleep). Com o pool de 10 conexões esgotado, a loja inteira fica
// lenta. No Omni: p95 alto em /api/relatorios/vendas e SELECT pg_sleep($1)
// entre as consultas mais lentas.
//   --taxa 8
//   --ms 2000     quanto cada relatório espera no banco
//   --duracao 1m

import { args, flood, seconds, send } from '../lib.js'

const o = args({ taxa: 8, ms: 2000, duracao: '1m' })
const ip = '192.0.2.120'

console.log(`consultas lentas: ${o.taxa}/s de ${o.ms} ms cada por ${o.duracao}. Abra a loja no navegador e veja ela travar.`)
await flood({ name: 'lento', rate: o.taxa, duration: seconds(o.duracao), next: () => send(`/api/relatorios/vendas?ms=${o.ms}`, { ip }) })
