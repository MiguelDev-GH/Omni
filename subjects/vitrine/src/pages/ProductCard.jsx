import { Link } from 'react-router-dom'
import { brl } from '../api.js'
import { useStore } from '../App.jsx'

export default function ProductCard({ p }) {
  const { add } = useStore()
  return (
    <article className="cartao">
      <Link to={`/produtos/${p.id}`} className="cartao-img" aria-label={p.nome}>
        <span aria-hidden>{p.emoji}</span>
      </Link>
      <div className="cartao-corpo">
        <small className="categoria">{p.categoria}</small>
        <Link to={`/produtos/${p.id}`} className="cartao-nome">
          {p.nome}
        </Link>
        <div className="cartao-pe">
          <strong>{brl(p.preco)}</strong>
          <button onClick={() => add(p)}>Adicionar</button>
        </div>
      </div>
    </article>
  )
}
