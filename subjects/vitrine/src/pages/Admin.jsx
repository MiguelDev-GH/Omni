import { useEffect, useState } from 'react'
import { api, brl } from '../api.js'

export default function Admin() {
  const [users, setUsers] = useState(null)
  const [sales, setSales] = useState(null)
  const [error, setError] = useState(null)

  useEffect(() => {
    api('/admin/usuarios').then(setUsers).catch((e) => setError(e.message))
  }, [])

  return (
    <>
      <h1>Painel administrativo</h1>
      <p className="dica">
        Esta página não confere quem você é: abra /admin sem login e ela carrega igual.
      </p>
      {error && <p className="erro">{error}</p>}
      <h2>Clientes</h2>
      <div className="rolagem">
        <table className="tabela">
          <thead>
            <tr>
              <th>#</th>
              <th>Usuário</th>
              <th>Nome</th>
              <th>E-mail</th>
              <th>CPF</th>
              <th>Papel</th>
              <th>Senha (md5)</th>
            </tr>
          </thead>
          <tbody>
            {users?.map((u) => (
              <tr key={u.id}>
                <td>{u.id}</td>
                <td>{u.usuario}</td>
                <td>{u.nome}</td>
                <td>{u.email}</td>
                <td>{u.cpf}</td>
                <td>{u.papel}</td>
                <td>
                  <code>{u.senha_md5}</code>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <h2>Vendas por dia</h2>
      {!sales ? (
        <button onClick={() => api('/relatorios/vendas').then(setSales).catch((e) => setError(e.message))}>
          Gerar relatório (lento)
        </button>
      ) : (
        <table className="tabela">
          <thead>
            <tr>
              <th>Dia</th>
              <th>Pedidos</th>
              <th>Receita</th>
            </tr>
          </thead>
          <tbody>
            {sales.map((s) => (
              <tr key={s.dia}>
                <td>{new Date(s.dia).toLocaleDateString('pt-BR')}</td>
                <td>{s.pedidos}</td>
                <td>{brl(s.receita)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  )
}
