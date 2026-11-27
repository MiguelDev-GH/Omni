import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useStore } from '../App.jsx'

export default function Entrar() {
  const { login } = useStore()
  const [usuario, setUsuario] = useState('')
  const [senha, setSenha] = useState('')
  const [error, setError] = useState(null)
  const navigate = useNavigate()

  async function submit(e) {
    e.preventDefault()
    setError(null)
    try {
      await login(usuario, senha)
      navigate('/')
    } catch (err) {
      // FALHA: a mensagem diz se a conta existe ou se só a senha está errada.
      setError(err.message)
    }
  }

  return (
    <form className="entrar" onSubmit={submit}>
      <h1>Entrar</h1>
      <label>
        Usuário
        <input required autoComplete="username" value={usuario} onChange={(e) => setUsuario(e.target.value)} />
      </label>
      <label>
        Senha
        <input required type="password" autoComplete="current-password" value={senha} onChange={(e) => setSenha(e.target.value)} />
      </label>
      {error && <p className="erro">{error}</p>}
      <button className="primario">Entrar</button>
      <p className="dica">Contas de teste: ana / ana123, bia / bia123, carlos / carlos123, admin / admin</p>
    </form>
  )
}
