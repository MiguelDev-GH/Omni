import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, brl } from '../api.js'
import { useStore } from '../App.jsx'

export default function Pedido() {
  const { id } = useParams()
  const { user } = useStore()
  const [order, setOrder] = useState(null)
  const [error, setError] = useState(null)

  useEffect(() => {
    setOrder(null)
    setError(null)
    if (user) api(`/pedidos/${id}`).then(setOrder).catch((e) => setError(e.message))
  }, [id, user])

  if (!user) return <p className="vazio">Entre na sua conta para ver o pedido.</p>
  if (error) return <p className="erro">{error}</p>
  if (!order) return <p className="vazio">Carregando…</p>
  return (
    <div className="pedido">
      {/* FALHA: troque o número na URL (/pedidos/1, /pedidos/2…) e veja o pedido de outra pessoa. */}
      <nav className="pedido-nav">
        <Link to={`/pedidos/${Number(id) - 1}`}>← #{Number(id) - 1}</Link>
        <Link to="/meus-pedidos">Meus pedidos</Link>
        <Link to={`/pedidos/${Number(id) + 1}`}>#{Number(id) + 1} →</Link>
      </nav>
      <h1>
        Pedido #{order.id} <span className={`status status-${order.status}`}>{order.status}</span>
      </h1>
      <dl className="dados">
        <dt>Cliente</dt>
        <dd>{order.cliente}</dd>
        <dt>E-mail</dt>
        <dd>{order.email}</dd>
        <dt>CPF</dt>
        <dd>{order.cpf}</dd>
        <dt>Entrega</dt>
        <dd>{order.endereco}</dd>
        <dt>Cartão</dt>
        <dd>final {order.cartao_final}</dd>
        <dt>Data</dt>
        <dd>{new Date(order.criado_em).toLocaleString('pt-BR')}</dd>
      </dl>
      <ul className="itens">
        {order.itens.map((it, i) => (
          <li key={i}>
            <span className="item-nome">
              {it.quantidade}× {it.nome}
            </span>
            <strong>{brl(it.preco * it.quantidade)}</strong>
          </li>
        ))}
      </ul>
      <p className="total">
        Total <strong>{brl(order.total)}</strong>
      </p>
    </div>
  )
}
