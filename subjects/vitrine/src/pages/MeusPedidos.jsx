import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, brl } from '../api.js'
import { useStore } from '../App.jsx'

export default function MeusPedidos() {
  const { user } = useStore()
  const [orders, setOrders] = useState(null)
  const [error, setError] = useState(null)

  useEffect(() => {
    if (user) api('/meus-pedidos').then(setOrders).catch((e) => setError(e.message))
  }, [user])

  if (!user) return <p className="vazio">Entre na sua conta para ver seus pedidos.</p>
  if (error) return <p className="erro">{error}</p>
  if (!orders) return <p className="vazio">Carregando…</p>
  return (
    <>
      <h1>Meus pedidos</h1>
      {orders.length === 0 && <p className="vazio">Nenhum pedido ainda.</p>}
      <table className="tabela">
        <thead>
          <tr>
            <th>Pedido</th>
            <th>Data</th>
            <th>Itens</th>
            <th>Status</th>
            <th>Total</th>
          </tr>
        </thead>
        <tbody>
          {orders.map((o) => (
            <tr key={o.id}>
              <td>
                <Link to={`/pedidos/${o.id}`}>#{o.id}</Link>
              </td>
              <td>{new Date(o.criado_em).toLocaleDateString('pt-BR')}</td>
              <td>{o.itens.length}</td>
              <td>
                <span className={`status status-${o.status}`}>{o.status}</span>
              </td>
              <td>{brl(o.total)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  )
}
