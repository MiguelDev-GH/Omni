import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api, brl } from '../api.js'
import { useStore } from '../App.jsx'

export default function Carrinho() {
  const { cart, setQuantity, clearCart, user } = useStore()
  const [endereco, setEndereco] = useState('')
  const [cartao, setCartao] = useState('')
  const [error, setError] = useState(null)
  const [sending, setSending] = useState(false)
  const navigate = useNavigate()
  const total = cart.reduce((s, it) => s + it.preco * it.quantidade, 0)

  if (cart.length === 0) {
    return (
      <p className="vazio">
        Seu carrinho está vazio. <Link to="/">Ver produtos</Link>
      </p>
    )
  }

  async function finish(e) {
    e.preventDefault()
    setSending(true)
    setError(null)
    try {
      // FALHA: o preço de cada item vai do navegador para a API, que confia nele.
      const order = await api('/pedidos', { method: 'POST', body: { itens: cart, endereco, cartao } })
      clearCart()
      navigate(`/pedidos/${order.id}`)
    } catch (err) {
      setError(err.message)
    } finally {
      setSending(false)
    }
  }

  return (
    <div className="carrinho">
      <section>
        <h1>Carrinho</h1>
        <ul className="itens">
          {cart.map((it) => (
            <li key={it.id}>
              <span className="item-emoji" aria-hidden>
                {it.emoji}
              </span>
              <span className="item-nome">{it.nome}</span>
              <span className="qtd">
                <button onClick={() => setQuantity(it.id, it.quantidade - 1)} aria-label="Diminuir">
                  −
                </button>
                {it.quantidade}
                <button onClick={() => setQuantity(it.id, it.quantidade + 1)} aria-label="Aumentar">
                  +
                </button>
              </span>
              <strong>{brl(it.preco * it.quantidade)}</strong>
            </li>
          ))}
        </ul>
        <p className="total">
          Total <strong>{brl(total)}</strong>
        </p>
      </section>
      <section className="checkout">
        <h2>Finalizar compra</h2>
        {!user ? (
          <p>
            <Link to="/entrar">Entre na sua conta</Link> para finalizar.
          </p>
        ) : (
          <form onSubmit={finish}>
            <label>
              Endereço de entrega
              <input required value={endereco} onChange={(e) => setEndereco(e.target.value)} placeholder="Rua, número, cidade" />
            </label>
            <label>
              Cartão (use um número de teste, como 4111 1111 1111 1111)
              <input required value={cartao} onChange={(e) => setCartao(e.target.value)} inputMode="numeric" />
            </label>
            {error && <p className="erro">{error}</p>}
            <button className="primario" disabled={sending}>
              {sending ? 'Enviando…' : `Pagar ${brl(total)}`}
            </button>
          </form>
        )}
      </section>
    </div>
  )
}
