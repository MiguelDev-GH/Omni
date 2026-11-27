// npm run trafego: clientes navegando normalmente na loja, até Ctrl+C. Rode
// antes do ataque:ddos, para o Omni aprender o que é o volume normal.
//   --taxa 5      requisições por segundo
//   --duracao 0   segundos, ou 2m, 1h; 0 é até Ctrl+C

import { args, basic, between, flood, pick, seconds, send } from './lib.js'

const o = args({ taxa: 5, duracao: '0' })
const agent = 'Vitrine-Cliente/1'
const termos = ['fone', 'livro', 'camiseta', 'garrafa', 'teclado', 'tênis', 'planta']
const clientes = [
  ['ana', 'ana123'],
  ['bia', 'bia123'],
  ['carlos', 'carlos123'],
]

const visitas = [
  [30, () => send('/api/produtos', { agent })],
  [15, () => send('/api/categorias', { agent })],
  [25, () => send(`/api/produtos/${between(1, 22)}`, { agent })],
  [10, () => send(`/api/busca?q=${encodeURIComponent(pick(termos))}`, { agent })],
  [8, () => {
    const [u, s] = pick(clientes)
    return send('/api/entrar', { method: 'POST', agent, auth: basic(u, s) })
  }],
  [8, () => {
    const [u, s] = pick(clientes)
    return send('/api/meus-pedidos', { agent, auth: basic(u, s) })
  }],
  [2, () => send(`/api/produtos/${between(900, 999)}`, { agent })], // link quebrado: 404
  [2, () => send('/', { agent })],
]
const peso = visitas.reduce((s, [w]) => s + w, 0)

function visita() {
  let r = Math.random() * peso
  for (const [w, fn] of visitas) {
    if ((r -= w) < 0) return fn()
  }
  return visitas[0][1]()
}

console.log(`tráfego normal: ${o.taxa} req/s. Ctrl+C para parar.`)
await flood({ name: 'trafego', rate: o.taxa, duration: seconds(o.duracao), next: visita })
