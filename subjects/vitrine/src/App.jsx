import { createContext, useContext, useEffect, useState } from 'react'
import { Link, NavLink, Route, Routes, useNavigate } from 'react-router-dom'
import { api, clearCredential, saveCredential } from './api.js'
import Admin from './pages/Admin.jsx'
import Busca from './pages/Busca.jsx'
import Carrinho from './pages/Carrinho.jsx'
import Entrar from './pages/Entrar.jsx'
import Home from './pages/Home.jsx'
import MeusPedidos from './pages/MeusPedidos.jsx'
import Pedido from './pages/Pedido.jsx'
import Produto from './pages/Produto.jsx'

const Store = createContext(null)
export const useStore = () => useContext(Store)

function load(key, fallback) {
  try {
    return JSON.parse(localStorage.getItem(key)) ?? fallback
  } catch {
    return fallback
  }
}

export default function App() {
  const [user, setUser] = useState(() => load('vitrine:usuario', null))
  const [cart, setCart] = useState(() => load('vitrine:carrinho', []))
  const [query, setQuery] = useState('')
  const navigate = useNavigate()

  useEffect(() => localStorage.setItem('vitrine:carrinho', JSON.stringify(cart)), [cart])

  const store = {
    user,
    cart,
    async login(usuario, senha) {
      const u = await api('/entrar', { method: 'POST', credential: btoa(`${usuario}:${senha}`) })
      saveCredential(usuario, senha)
      localStorage.setItem('vitrine:usuario', JSON.stringify(u))
      setUser(u)
    },
    logout() {
      clearCredential()
      localStorage.removeItem('vitrine:usuario')
      setUser(null)
      navigate('/')
    },
    add(p) {
      setCart((c) => {
        const found = c.find((it) => it.id === p.id)
        if (found) return c.map((it) => (it.id === p.id ? { ...it, quantidade: it.quantidade + 1 } : it))
        return [...c, { id: p.id, nome: p.nome, emoji: p.emoji, preco: Number(p.preco), quantidade: 1 }]
      })
    },
    setQuantity(id, q) {
      setCart((c) => (q <= 0 ? c.filter((it) => it.id !== id) : c.map((it) => (it.id === id ? { ...it, quantidade: q } : it))))
    },
    clearCart: () => setCart([]),
  }

  const count = cart.reduce((s, it) => s + it.quantidade, 0)

  return (
    <Store.Provider value={store}>
      <header className="topo">
        <div className="topo-linha">
          <Link to="/" className="marca">
            <span aria-hidden>🛍️</span> Vitrine
          </Link>
          <form
            className="busca"
            role="search"
            onSubmit={(e) => {
              e.preventDefault()
              navigate(`/busca?q=${encodeURIComponent(query)}`)
            }}
          >
            <input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Buscar produtos" aria-label="Buscar produtos" />
            <button type="submit">Buscar</button>
          </form>
          <nav className="menu">
            {user ? (
              <>
                <NavLink to="/meus-pedidos">Meus pedidos</NavLink>
                {/* FALHA: o menu Admin some para clientes, mas a rota da API não confere nada. */}
                {user.papel === 'admin' && <NavLink to="/admin">Admin</NavLink>}
                <button className="link" onClick={store.logout}>
                  Sair ({user.usuario})
                </button>
              </>
            ) : (
              <NavLink to="/entrar">Entrar</NavLink>
            )}
            <NavLink to="/carrinho" className="carrinho-link">
              🛒 <span className="contador">{count}</span>
            </NavLink>
          </nav>
        </div>
      </header>
      <main className="conteudo">
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/produtos/:id" element={<Produto />} />
          <Route path="/busca" element={<Busca />} />
          <Route path="/carrinho" element={<Carrinho />} />
          <Route path="/entrar" element={<Entrar />} />
          <Route path="/meus-pedidos" element={<MeusPedidos />} />
          <Route path="/pedidos/:id" element={<Pedido />} />
          <Route path="/admin" element={<Admin />} />
          <Route path="*" element={<p className="vazio">Página não encontrada.</p>} />
        </Routes>
      </main>
      <footer className="rodape">Vitrine é uma loja cobaia do Omni, com falhas de propósito. Só roda em 127.0.0.1.</footer>
    </Store.Provider>
  )
}
