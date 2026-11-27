// npm run ataque:derrubar: chama a rota de depuração esquecida, que encerra
// o processo da API. O Poller do Omni gera ALERTA_QUEDA na próxima rodada e
// SERVICO_RESTAURADO quando o npm run dev a sobe de novo (VITRINE_REINICIO,
// 45 s por padrão).

import { send } from '../lib.js'

const r = await send('/api/debug/derrubar', { method: 'POST', ip: '203.0.113.200' })
if (r.status !== 200) {
  console.error(`a API não respondeu como esperado (HTTP ${r.status}): ${JSON.stringify(r.data)}`)
  process.exit(1)
}
console.log('POST /api/debug/derrubar aceito: a API caiu. Esperando ela voltar…')
const down = Date.now()
await new Promise((r) => setTimeout(r, 1500))
for (;;) {
  const ping = await send('/', { agent: 'Vitrine-Ataque/1' })
  const secs = ((Date.now() - down) / 1000).toFixed(0)
  if (ping.status === 200) {
    console.log(`\na API voltou depois de ${secs} s`)
    break
  }
  process.stdout.write(`\r  fora do ar há ${secs} s`)
  await new Promise((r) => setTimeout(r, 1000))
}
