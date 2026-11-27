// npm run cadastrar: cria o alvo "vitrine" no Omni (API na porta 4000 e banco
// na 5434), se ainda não existir. Precisa de OMNI_URL e OMNI_TOKEN, que saem
// no terminal do make init-reset.

const url = process.env.OMNI_URL
const token = process.env.OMNI_TOKEN
if (!url || !token) {
  console.error('defina OMNI_URL (o painel= do log do Omni) e OMNI_TOKEN (o que o make init-reset imprimiu)')
  process.exit(1)
}

const alvo = { nome: 'vitrine', host: '127.0.0.1', portas_app: [4000], porta_banco: 5434 }
const headers = { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }

const res = await fetch(`${url}/api/alvos`, { headers })
if (res.status === 401) {
  console.error('o Omni recusou o token (401)')
  process.exit(1)
}
if (!res.ok) {
  console.error(`GET /api/alvos respondeu ${res.status}`)
  process.exit(1)
}
const alvos = await res.json()
if (alvos.some((a) => a.nome === alvo.nome)) {
  console.log('o alvo vitrine já existe no Omni')
  process.exit(0)
}
const created = await fetch(`${url}/api/alvos`, { method: 'POST', headers, body: JSON.stringify(alvo) })
if (created.status !== 201) {
  console.error(`POST /api/alvos respondeu ${created.status}: ${await created.text()}`)
  process.exit(1)
}
console.log('alvo vitrine criado no Omni: 127.0.0.1, porta 4000, banco 5434')
