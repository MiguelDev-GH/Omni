// Banco da vitrine: cria as tabelas e os dados de exemplo na primeira partida.
// Todos os dados pessoais são inventados.

import { createHash } from 'node:crypto'
import pg from 'pg'

// md5 sem sal é uma das falhas de propósito: as senhas que vazam pelo
// /api/admin/usuarios e pela injeção de SQL quebram em segundos.
export const md5 = (s) => createHash('md5').update(s).digest('hex')

const schema = `
CREATE TABLE IF NOT EXISTS usuarios (
  id        serial PRIMARY KEY,
  usuario   text UNIQUE NOT NULL,
  nome      text NOT NULL,
  email     text NOT NULL,
  cpf       text NOT NULL,
  endereco  text NOT NULL,
  senha_md5 text NOT NULL,
  papel     text NOT NULL DEFAULT 'cliente'
);
CREATE TABLE IF NOT EXISTS produtos (
  id        serial PRIMARY KEY,
  nome      text NOT NULL,
  descricao text NOT NULL,
  categoria text NOT NULL,
  emoji     text NOT NULL,
  preco     numeric(10,2) NOT NULL,
  custo     numeric(10,2) NOT NULL,
  estoque   int NOT NULL,
  oculto    boolean NOT NULL DEFAULT false
);
CREATE TABLE IF NOT EXISTS pedidos (
  id           serial PRIMARY KEY,
  usuario_id   int NOT NULL REFERENCES usuarios(id),
  itens        jsonb NOT NULL,
  total        numeric(10,2) NOT NULL,
  status       text NOT NULL,
  endereco     text NOT NULL,
  cartao_final text NOT NULL,
  criado_em    timestamptz NOT NULL DEFAULT now()
);
`

export const usuarios = [
  ['ana', 'Ana Souza', 'ana@exemplo.com', '111.111.111-11', 'Rua das Flores, 10 - Recife/PE', 'ana123', 'cliente'],
  ['bia', 'Beatriz Lima', 'bia@exemplo.com', '222.222.222-22', 'Av. Brasil, 200 - Natal/RN', 'bia123', 'cliente'],
  ['carlos', 'Carlos Pereira', 'carlos@exemplo.com', '333.333.333-33', 'Rua do Sol, 33 - Fortaleza/CE', 'carlos123', 'cliente'],
  ['davi', 'Davi Rocha', 'davi@exemplo.com', '444.444.444-44', 'Rua Nova, 4 - João Pessoa/PB', '123456', 'cliente'],
  ['elis', 'Elis Martins', 'elis@exemplo.com', '555.555.555-55', 'Travessa Azul, 55 - Maceió/AL', 'elis2026', 'cliente'],
  ['admin', 'Administrador', 'admin@vitrine.local', '000.000.000-00', 'Sede da Vitrine', 'admin', 'admin'],
]

// [nome, descrição, categoria, emoji, preço, custo, estoque, oculto]
const produtos = [
  ['Fone Bluetooth Pulse', 'Fone sem fio com 30 h de bateria e cancelamento de ruído.', 'Eletrônicos', '🎧', 249.9, 98, 40, false],
  ['Teclado Mecânico Kilo', 'Switches marrons, ABNT2, iluminação branca.', 'Eletrônicos', '⌨️', 389.0, 160, 25, false],
  ['Mouse Sem Fio Nimbus', 'Sensor de 16 mil DPI e bateria para 70 h.', 'Eletrônicos', '🖱️', 159.9, 55, 60, false],
  ['Caixa de Som Orla', 'Resistente à água, ideal para a praia.', 'Eletrônicos', '🔊', 299.0, 120, 18, false],
  ['Smartwatch Ritmo', 'Monitor cardíaco, GPS e 7 dias de bateria.', 'Eletrônicos', '⌚', 699.0, 310, 12, false],
  ['Câmera Instantânea Flash', 'Fotos impressas na hora, 10 cores de filme.', 'Eletrônicos', '📷', 549.0, 260, 9, false],
  ['Camiseta Algodão Cru', 'Algodão orgânico, modelagem reta.', 'Moda', '👕', 79.9, 22, 120, false],
  ['Tênis Corrida Vento', 'Amortecimento leve para treinos diários.', 'Moda', '👟', 429.0, 180, 30, false],
  ['Boné Aba Curva', 'Ajuste traseiro e tecido respirável.', 'Moda', '🧢', 59.9, 14, 80, false],
  ['Óculos de Sol Mar', 'Lentes polarizadas com proteção UV400.', 'Moda', '🕶️', 189.0, 45, 35, false],
  ['Mochila Urbana 20 L', 'Compartimento acolchoado para notebook de 15".', 'Moda', '🎒', 219.0, 80, 50, false],
  ['Cafeteira Italiana', 'Alumínio, 6 xícaras, para fogão a gás.', 'Casa', '☕', 119.0, 38, 70, false],
  ['Luminária Lua', 'Luz quente regulável, recarregável por USB.', 'Casa', '🌙', 139.9, 41, 44, false],
  ['Kit Facas do Chef', 'Três facas em aço inox com cepo de bambu.', 'Casa', '🔪', 279.0, 95, 20, false],
  ['Planta Jiboia', 'Vaso de cerâmica incluso, fácil de cuidar.', 'Casa', '🪴', 69.9, 18, 65, false],
  ['Manta de Sofá Trama', 'Tricô grosso, 1,5 m x 2 m.', 'Casa', '🧶', 159.0, 52, 28, false],
  ['Livro: Redes para Curiosos', 'Do cabo ao HTTP, com exercícios.', 'Livros', '📘', 89.9, 30, 90, false],
  ['Livro: Segurança na Prática', 'Ataques comuns e como se defender.', 'Livros', '📕', 99.9, 33, 75, false],
  ['Livro: Postgres sem Medo', 'Modelagem, índices e consultas lentas.', 'Livros', '📗', 94.9, 31, 60, false],
  ['Bola de Futebol Arena', 'Tamanho oficial, costura térmica.', 'Esporte', '⚽', 149.0, 52, 40, false],
  ['Garrafa Térmica 1 L', 'Mantém gelado por 24 h.', 'Esporte', '🧊', 99.0, 29, 85, false],
  ['Tapete de Yoga Raiz', 'Borracha natural antiderrapante, 5 mm.', 'Esporte', '🧘', 179.0, 60, 33, false],
  // Produtos ocultos: não aparecem na vitrine, mas a injeção de SQL na busca
  // revela todos, com o custo de cada um.
  ['Protótipo Console X (não lançado)', 'Lançamento previsto para dezembro. Confidencial.', 'Eletrônicos', '🎮', 3999.0, 2100, 3, true],
  ['Cupom interno FUNCIONARIO90', 'Desconto de 90% para a equipe. Não divulgar.', 'Interno', '🎟️', 0.01, 0, 999, true],
]

const status = ['pago', 'enviado', 'entregue', 'entregue', 'entregue', 'cancelado']

export function createPool(connectionString) {
  return new pg.Pool({ connectionString, max: 10 })
}

// setup cria as tabelas e, se estiverem vazias, os dados de exemplo.
export async function setup(pool) {
  await pool.query(schema)
  const { rows } = await pool.query('SELECT count(*)::int AS n FROM usuarios')
  if (rows[0].n > 0) return false

  for (const [usuario, nome, email, cpf, endereco, senha, papel] of usuarios) {
    await pool.query(
      'INSERT INTO usuarios (usuario, nome, email, cpf, endereco, senha_md5, papel) VALUES ($1, $2, $3, $4, $5, $6, $7)',
      [usuario, nome, email, cpf, endereco, md5(senha), papel],
    )
  }
  for (const p of produtos) {
    await pool.query(
      'INSERT INTO produtos (nome, descricao, categoria, emoji, preco, custo, estoque, oculto) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)',
      p,
    )
  }
  // 180 pedidos espalhados entre os clientes, para o ataque de IDOR ter o
  // que vazar. Um gerador fixo deixa os dados iguais em toda instalação.
  let seed = 7
  const rand = (n) => {
    seed = (seed * 1103515245 + 12345) % 2147483648
    return seed % n
  }
  const clientes = usuarios.filter((u) => u[6] === 'cliente')
  const visiveis = produtos.filter((p) => !p[7])
  for (let i = 0; i < 180; i++) {
    const c = rand(clientes.length)
    const itens = []
    for (let k = 0; k <= rand(3); k++) {
      const pi = rand(visiveis.length)
      itens.push({ id: pi + 1, nome: visiveis[pi][0], preco: visiveis[pi][4], quantidade: 1 + rand(2) })
    }
    const total = itens.reduce((s, it) => s + it.preco * it.quantidade, 0)
    await pool.query(
      `INSERT INTO pedidos (usuario_id, itens, total, status, endereco, cartao_final, criado_em)
       VALUES ($1, $2, $3, $4, $5, $6, now() - make_interval(hours => $7))`,
      [c + 1, JSON.stringify(itens), total.toFixed(2), status[rand(status.length)], clientes[c][4], String(1000 + rand(9000)), 1 + rand(24 * 90)],
    )
  }
  return true
}
