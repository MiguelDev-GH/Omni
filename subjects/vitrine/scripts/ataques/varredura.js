// npm run ataque:varredura: procura arquivos e rotas que não deviam estar no
// ar, como faria um ffuf ou dirb. No Omni aparece como uma rajada de 404
// vindos de um mesmo IP; as rotas que respondem 200 são o que vazou.
//   --taxa 25
//   --voltas 3    quantas vezes passa pela lista

import { args, flood, send } from '../lib.js'

const o = args({ taxa: 25, voltas: 3 })
const ip = '192.0.2.50'
const lista = [
  '/.env', '/.env.local', '/.env.production', '/.git/config', '/.git/HEAD', '/.gitignore', '/.svn/entries',
  '/.aws/credentials', '/.ssh/id_rsa', '/.DS_Store', '/backup.sql', '/backup.zip', '/dump.sql', '/db.sqlite',
  '/robots.txt', '/sitemap.xml', '/admin', '/administrator', '/wp-login.php', '/wp-admin/', '/phpmyadmin/',
  '/server-status', '/config.json', '/config.php', '/web.config', '/package.json', '/.npmrc', '/Dockerfile',
  '/docker-compose.yml', '/api', '/api/admin', '/api/admin/usuarios', '/api/usuarios', '/api/debug', '/api/debug/config',
  '/api/config', '/api/swagger.json', '/swagger-ui/', '/actuator/health', '/actuator/env', '/graphql', '/console',
  '/login', '/api/pedidos/1', '/api/relatorios/vendas?ms=0', '/uploads/', '/static/', '/old/', '/test/', '/tmp/',
]
const found = new Map()

console.log(`varredura: ${lista.length} caminhos x ${o.voltas} voltas, de ${ip}, a ${o.taxa}/s.`)
await flood({
  name: 'varredura',
  rate: o.taxa,
  duration: 0,
  next: (i) => (i < lista.length * o.voltas ? send(lista[i % lista.length], { ip }).then((r) => ({ ...r, path: lista[i % lista.length] })) : null),
  onResult: (r) => {
    if (r.status !== 404 && r.status !== 0) found.set(r.path, r.status)
  },
})
console.log('\ncaminhos que responderam (o que não devia estar exposto):')
for (const [path, status] of found) console.log(`  ${status}  ${path}`)
