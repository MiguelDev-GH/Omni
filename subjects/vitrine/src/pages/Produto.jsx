import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, brl } from '../api.js'
import { useStore } from '../App.jsx'

export default function Produto() {
  const { id } = useParams()
  const { add } = useStore()
  const [p, setP] = useState(null)
  const [error, setError] = useState(null)
  const [added, setAdded] = useState(false)

  useEffect(() => {
    api(`/produtos/${id}`).then(setP).catch((e) => setError(e.message))
  }, [id])

  if (error) return <p className="erro">{error}</p>
  if (!p) return <p className="vazio">Carregando…</p>
  return (
    <div className="produto">
      <div className="produto-img" aria-hidden>
        {p.emoji}
      </div>
      <div>
        <Link to="/" className="voltar">
          ← Voltar para a vitrine
        </Link>
        <small className="categoria">{p.categoria}</small>
        <h1>{p.nome}</h1>
        <p>{p.descricao}</p>
        <p className="preco">{brl(p.preco)}</p>
        <p className="estoque">{p.estoque > 0 ? `${p.estoque} em estoque` : 'Esgotado'}</p>
        <button
          className="primario"
          onClick={() => {
            add(p)
            setAdded(true)
          }}
        >
          Adicionar ao carrinho
        </button>
        {added && (
          <p className="ok">
            Adicionado. <Link to="/carrinho">Ver carrinho</Link>
          </p>
        )}
      </div>
    </div>
  )
}
