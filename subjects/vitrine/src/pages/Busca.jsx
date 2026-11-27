import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api } from '../api.js'
import ProductCard from './ProductCard.jsx'

export default function Busca() {
  const [params] = useSearchParams()
  const q = params.get('q') || ''
  const [products, setProducts] = useState(null)
  const [error, setError] = useState(null)

  useEffect(() => {
    setProducts(null)
    setError(null)
    api(`/busca?q=${encodeURIComponent(q)}`)
      .then(setProducts)
      .catch((e) => setError(e))
  }, [q])

  return (
    <>
      {/* FALHA: XSS refletido. O termo buscado entra na página como HTML;
          experimente buscar <img src=x onerror=alert(localStorage['vitrine:credencial'])> */}
      <h1 dangerouslySetInnerHTML={{ __html: `Resultados para “${q}”` }} />
      {error && (
        <div className="erro">
          <p>Erro na busca: {error.message}</p>
          {/* FALHA: a página mostra o SQL e a pilha que a API devolveu. */}
          {error.data?.sql && <pre>{error.data.sql}</pre>}
        </div>
      )}
      {!products && !error && <p className="vazio">Buscando…</p>}
      {products?.length === 0 && <p className="vazio">Nada encontrado.</p>}
      <div className="grade">{products?.map((p, i) => <ProductCard key={`${p.id}-${i}`} p={p} />)}</div>
    </>
  )
}
