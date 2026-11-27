import { useEffect, useState } from 'react'
import { api } from '../api.js'
import ProductCard from './ProductCard.jsx'

export default function Home() {
  const [categories, setCategories] = useState([])
  const [category, setCategory] = useState(null)
  const [products, setProducts] = useState(null)
  const [error, setError] = useState(null)

  useEffect(() => {
    api('/categorias').then(setCategories).catch(() => {})
  }, [])

  useEffect(() => {
    setProducts(null)
    api(`/produtos${category ? `?categoria=${encodeURIComponent(category)}` : ''}`)
      .then(setProducts)
      .catch((e) => setError(e.message))
  }, [category])

  return (
    <>
      <section className="destaque">
        <h1>Ofertas da semana</h1>
        <p>Frete grátis acima de R$ 199 e até 10x sem juros.</p>
      </section>
      <div className="filtros" role="tablist">
        <button className={!category ? 'ativo' : ''} onClick={() => setCategory(null)}>
          Tudo
        </button>
        {categories.map((c) => (
          <button key={c.categoria} className={category === c.categoria ? 'ativo' : ''} onClick={() => setCategory(c.categoria)}>
            {c.categoria} <small>{c.total}</small>
          </button>
        ))}
      </div>
      {error && <p className="erro">Não foi possível carregar os produtos: {error}</p>}
      {!products && !error && <p className="vazio">Carregando…</p>}
      <div className="grade">{products?.map((p) => <ProductCard key={p.id} p={p} />)}</div>
    </>
  )
}
