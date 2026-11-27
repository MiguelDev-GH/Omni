// API da vitrine cobaia. Cada falha marcada com FALHA existe de propósito,
// para ser vista pelo Omni ou mostrada numa apresentação. Não copie nada
// daqui para uma aplicação de verdade.

import express from 'express'
import { loadConfig } from './config.js'
import { createPool, md5, setup } from './db.js'
import { leaks } from './leaks.js'

const cfg = loadConfig()
const pool = createPool(cfg.database)
const app = express()
// O Express já manda "X-Powered-By: Express" (FALHA: revela a tecnologia).
app.use(express.json())

app.use((_req, res, next) => {
  // FALHA: CORS aberto para qualquer origem e nenhum cabeçalho de segurança
  // (CSP, X-Frame-Options, HSTS).
  res.set('Access-Control-Allow-Origin', '*')
  res.set('Server', 'Vitrine/1.4.2 (Node ' + process.version + ')')
  next()
})

const wrap = (fn) => (req, res, next) => fn(req, res).catch(next)

// GET / é o que o Poller do Omni consulta. Responde 200 mesmo sem banco.
app.get('/', (_req, res) => res.json({ servico: 'vitrine-api', ok: true }))
app.get('/api/saude', (_req, res) => res.json({ ok: true }))

leaks(app, cfg)

// basic lê o cabeçalho Authorization: Basic. O Omni vê o usuário de cada
// requisição por ele, e é assim que conta logins falhos por conta.
function basic(req) {
  const h = req.get('authorization') || ''
  if (!h.startsWith('Basic ')) return null
  const raw = Buffer.from(h.slice(6), 'base64').toString()
  const i = raw.indexOf(':')
  return i < 0 ? null : { usuario: raw.slice(0, i), senha: raw.slice(i + 1) }
}

async function authenticate(req, res) {
  const cred = basic(req)
  if (!cred) {
    res.status(401).json({ erro: 'faça login' })
    return null
  }
  const { rows } = await pool.query('SELECT id, usuario, nome, email, papel, senha_md5 FROM usuarios WHERE usuario = $1', [cred.usuario])
  // FALHA: mensagens diferentes para "não existe" e "senha errada" deixam
  // um atacante descobrir quais contas existem.
  if (rows.length === 0) {
    res.status(401).json({ erro: `usuário ${cred.usuario} não existe` })
    return null
  }
  if (rows[0].senha_md5 !== md5(cred.senha)) {
    res.status(401).json({ erro: 'senha incorreta' })
    return null
  }
  const { senha_md5: _, ...user } = rows[0]
  return user
}

// FALHA: nenhum limite de tentativas. É o que deixa a força bruta e o
// password spraying rodarem à vontade.
app.post('/api/entrar', wrap(async (req, res) => {
  const user = await authenticate(req, res)
  if (user) res.json(user)
}))

app.get('/api/categorias', wrap(async (_req, res) => {
  const { rows } = await pool.query('SELECT categoria, count(*)::int AS total FROM produtos WHERE NOT oculto GROUP BY categoria ORDER BY categoria')
  res.json(rows)
}))

app.get('/api/produtos', wrap(async (req, res) => {
  const categoria = req.query.categoria || null
  const { rows } = await pool.query(
    'SELECT id, nome, categoria, emoji, preco, estoque FROM produtos WHERE NOT oculto AND ($1::text IS NULL OR categoria = $1) ORDER BY id',
    [categoria],
  )
  res.json(rows)
}))

app.get('/api/produtos/:id', wrap(async (req, res) => {
  const id = Number(req.params.id)
  if (!Number.isInteger(id)) return res.status(400).json({ erro: 'id inválido' })
  const { rows } = await pool.query('SELECT id, nome, descricao, categoria, emoji, preco, estoque FROM produtos WHERE id = $1 AND NOT oculto', [id])
  if (rows.length === 0) return res.status(404).json({ erro: 'produto não encontrado' })
  res.json(rows[0])
}))

// FALHA: injeção de SQL. O termo da busca vai direto para dentro do texto da
// consulta. "' OR 1=1--" mostra os produtos ocultos, e um UNION lê a tabela
// de usuários. Sem parâmetros, o pg usa o protocolo simples do PostgreSQL,
// e o Omni mostra a consulta com o literal trocado por "?".
app.get('/api/busca', wrap(async (req, res) => {
  const q = String(req.query.q || '')
  const sql = `SELECT id, nome, categoria, emoji, preco FROM produtos WHERE NOT oculto AND nome ILIKE '%${q}%' ORDER BY nome`
  try {
    const { rows } = await pool.query(sql)
    res.json(rows)
  } catch (err) {
    err.sql = sql
    throw err
  }
}))

app.get('/api/meus-pedidos', wrap(async (req, res) => {
  const user = await authenticate(req, res)
  if (!user) return
  const { rows } = await pool.query('SELECT id, total, status, criado_em, itens FROM pedidos WHERE usuario_id = $1 ORDER BY criado_em DESC', [user.id])
  res.json(rows)
}))

// FALHA: IDOR. Qualquer cliente logado lê o pedido de qualquer outro, com
// nome, CPF, endereço e final do cartão: o código não confere o dono.
app.get('/api/pedidos/:id', wrap(async (req, res) => {
  const user = await authenticate(req, res)
  if (!user) return
  const id = Number(req.params.id)
  if (!Number.isInteger(id)) return res.status(400).json({ erro: 'id inválido' })
  const { rows } = await pool.query(
    `SELECT p.id, p.itens, p.total, p.status, p.endereco, p.cartao_final, p.criado_em,
            u.nome AS cliente, u.email, u.cpf
       FROM pedidos p JOIN usuarios u ON u.id = p.usuario_id WHERE p.id = $1`,
    [id],
  )
  if (rows.length === 0) return res.status(404).json({ erro: 'pedido não encontrado' })
  res.json(rows[0])
}))

// FALHA: o preço vem do navegador. Mudar "preco" no corpo da requisição
// compra qualquer produto por R$ 0,01.
app.post('/api/pedidos', wrap(async (req, res) => {
  const user = await authenticate(req, res)
  if (!user) return
  const { itens, endereco, cartao } = req.body || {}
  if (!Array.isArray(itens) || itens.length === 0) return res.status(400).json({ erro: 'carrinho vazio' })
  if (!endereco || !cartao) return res.status(400).json({ erro: 'informe endereço e cartão' })
  const total = itens.reduce((s, it) => s + Number(it.preco) * Number(it.quantidade), 0)
  const { rows } = await pool.query(
    'INSERT INTO pedidos (usuario_id, itens, total, status, endereco, cartao_final) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, total, status',
    [user.id, JSON.stringify(itens), total.toFixed(2), 'pago', endereco, String(cartao).replace(/\D/g, '').slice(-4)],
  )
  // FALHA: o número completo do cartão vai parar no log do servidor.
  console.log(`pedido ${rows[0].id} de ${user.usuario}: cartão ${cartao}, total R$ ${total.toFixed(2)}`)
  res.status(201).json(rows[0])
}))

// FALHA: controle de acesso só na interface. O menu Admin some para quem não
// é admin, mas a rota responde para qualquer um, sem login nenhum.
app.get('/api/admin/usuarios', wrap(async (_req, res) => {
  const { rows } = await pool.query('SELECT id, usuario, nome, email, cpf, endereco, papel, senha_md5 FROM usuarios ORDER BY id')
  res.json(rows)
}))

// Relatório lento: espera ms milissegundos dentro do banco (pg_sleep) antes
// de somar as vendas. É o alvo do ataque:lento.
app.get('/api/relatorios/vendas', wrap(async (req, res) => {
  const ms = Math.min(Math.max(Number(req.query.ms) || 800, 0), 5000)
  await pool.query('SELECT pg_sleep($1)', [ms / 1000])
  const { rows } = await pool.query(
    "SELECT date_trunc('day', criado_em) AS dia, count(*)::int AS pedidos, sum(total) AS receita FROM pedidos GROUP BY 1 ORDER BY 1 DESC LIMIT 30",
  )
  res.json(rows)
}))

// FALHA: rota de depuração esquecida em produção, sem autenticação, que
// encerra o processo. É o ataque:derrubar.
app.post('/api/debug/derrubar', (_req, res) => {
  res.json({ ok: true, mensagem: 'encerrando o servidor' })
  console.log('POST /api/debug/derrubar: encerrando o processo')
  setTimeout(() => process.exit(1), 100)
})

app.use((req, res) => res.status(404).json({ erro: `rota ${req.method} ${req.path} não existe` }))

// FALHA: o erro volta inteiro para o cliente, com a pilha e o SQL.
app.use((err, _req, res, _next) => {
  console.error(err.message)
  res.status(500).json({ erro: err.message, sql: err.sql, pilha: err.stack })
})

async function start() {
  for (;;) {
    try {
      if (await setup(pool)) console.log('banco criado com os dados de exemplo')
      break
    } catch (err) {
      console.error(`banco indisponível (${err.message}); rode npm run banco. Nova tentativa em 3 s`)
      await new Promise((r) => setTimeout(r, 3000))
    }
  }
}

app.listen(cfg.port, cfg.host, () => {
  console.log(`vitrine-api em http://${cfg.host}:${cfg.port}`)
  start()
})
