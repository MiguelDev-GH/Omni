// Chamadas à API. As credenciais ficam no localStorage em Base64 e vão em
// toda requisição como Authorization: Basic (FALHA: qualquer script na
// página, inclusive um XSS, lê a senha do cliente).

const KEY = 'vitrine:credencial'

export const brl = (v) => Number(v).toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })

export function saveCredential(usuario, senha) {
  localStorage.setItem(KEY, btoa(`${usuario}:${senha}`))
}

export function clearCredential() {
  localStorage.removeItem(KEY)
}

export async function api(path, { method = 'GET', body, credential } = {}) {
  const headers = {}
  const cred = credential ?? localStorage.getItem(KEY)
  if (cred) headers.Authorization = `Basic ${cred}`
  if (body) headers['Content-Type'] = 'application/json'
  const res = await fetch(`/api${path}`, { method, headers, body: body && JSON.stringify(body) })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    const err = new Error(data.erro || `erro ${res.status}`)
    err.status = res.status
    err.data = data
    throw err
  }
  return data
}
