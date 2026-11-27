// npm run ataque:ddos: centenas de origens pedindo páginas comuns ao mesmo
// tempo. O Omni acusa "Pico de tráfego (possível DDoS)" quando o volume do
// último minuto passa de 3x (amarelo) ou 6x (vermelho) o normal; por isso,
// deixe o npm run trafego rodando 1 a 2 minutos antes.
//   --taxa 150    requisições por segundo
//   --duracao 2m

import { args, between, flood, seconds, send } from '../lib.js'

const o = args({ taxa: 150, duracao: '2m' })
const paths = ['/api/produtos', '/api/categorias', () => `/api/produtos/${between(1, 22)}`, '/']

console.log(`DDoS simulado: ${o.taxa} req/s de IPs 203.0.113.1-254 por ${o.duracao}. Ctrl+C para parar.`)
await flood({
  name: 'ddos',
  rate: o.taxa,
  duration: seconds(o.duracao),
  next: (i) => {
    const p = paths[i % paths.length]
    return send(typeof p === 'function' ? p() : p, { ip: `203.0.113.${between(1, 254)}` })
  },
})
