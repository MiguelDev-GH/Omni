// Utilitários dos scripts de tráfego e de ataque. Os IPs forjados no
// X-Forwarded-For são dos blocos reservados para documentação (RFC 5737),
// que nunca pertencem a ninguém de verdade. O Omni confia nesse cabeçalho
// porque a conexão vem do loopback, como viria de um proxy reverso local.

export const BASE = process.env.VITRINE_URL || 'http://127.0.0.1:4000'

// args lê opções no formato --nome valor, com os padrões de cada script.
export function args(defaults) {
  const out = { ...defaults }
  const argv = process.argv.slice(2)
  for (let i = 0; i < argv.length; i++) {
    const m = argv[i].match(/^--([\w-]+)(?:=(.*))?$/)
    if (!m) continue
    const value = m[2] ?? argv[++i]
    out[m[1]] = typeof defaults[m[1]] === 'number' ? Number(value) : value
  }
  return out
}

// seconds converte "90", "90s", "2m" ou "1h" em segundos; 0 é sem fim.
export function seconds(v) {
  const m = String(v).match(/^(\d+(?:\.\d+)?)(s|m|h)?$/)
  if (!m) throw new Error(`duração inválida: ${v}`)
  return Number(m[1]) * { s: 1, m: 60, h: 3600 }[m[2] || 's']
}

export const basic = (user, pass) => 'Basic ' + Buffer.from(`${user}:${pass}`).toString('base64')

export async function send(path, { method = 'GET', ip, auth, agent = 'Vitrine-Ataque/1', body } = {}) {
  // Connection: close abre uma conexão TCP nova por requisição. Sem isso, o
  // Node (undici) despeja um ataque de taxa alta numa única conexão reusada,
  // e o sniffer do Omni, que remonta por conexão, captura só uma fração.
  // Uma conexão por requisição também é o que um DDoS real (cada bot) e
  // muitos visitantes simultâneos fazem, então o tráfego fica mais fiel.
  const headers = { 'User-Agent': agent, Connection: 'close' }
  if (ip) headers['X-Forwarded-For'] = ip
  if (auth) headers.Authorization = auth
  if (body) headers['Content-Type'] = 'application/json'
  const started = performance.now()
  try {
    const res = await fetch(BASE + path, { method, headers, body: body && JSON.stringify(body), signal: AbortSignal.timeout(15000) })
    const text = await res.text()
    let data = text
    try {
      data = JSON.parse(text)
    } catch {}
    return { status: res.status, data, ms: performance.now() - started }
  } catch (err) {
    return { status: 0, data: err.cause?.code || err.name, ms: performance.now() - started }
  }
}

// flood dispara next() taxa vezes por segundo, sem esperar as respostas,
// por duracao segundos (0 é até Ctrl+C), e imprime o placar a cada 2 s.
// next devolve uma Promise de { status, ms } ou null para parar.
export async function flood({ name, rate, duration, next, onResult }) {
  const stats = new Map()
  let sent = 0
  let done = 0
  let latency = 0
  let stop = false
  const started = Date.now()
  const pending = new Set()

  const board = () => {
    const parts = [...stats.entries()].sort().map(([s, n]) => `${s === 0 ? 'falha' : s}: ${n}`)
    const secs = ((Date.now() - started) / 1000).toFixed(0)
    const avg = done ? (latency / done).toFixed(0) : '-'
    console.log(`[${name}] ${secs}s  enviadas ${sent}  ${parts.join('  ')}  latência média ${avg} ms`)
  }
  const timer = setInterval(board, 2000)
  const halt = () => {
    stop = true
  }
  process.once('SIGINT', halt)

  const interval = 1000 / rate
  let tick = performance.now()
  while (!stop && (duration === 0 || Date.now() - started < duration * 1000)) {
    const p = next(sent)
    if (!p) break
    sent++
    const tracked = p.then((r) => {
      done++
      latency += r.ms
      stats.set(r.status, (stats.get(r.status) || 0) + 1)
      if (onResult?.(r) === false) stop = true
    })
    pending.add(tracked)
    tracked.finally(() => pending.delete(tracked))
    tick += interval
    const wait = tick - performance.now()
    if (wait > 0) await new Promise((r) => setTimeout(r, wait))
  }
  await Promise.all(pending)
  clearInterval(timer)
  process.removeListener('SIGINT', halt)
  board()
  return stats
}

export const pick = (list) => list[Math.floor(Math.random() * list.length)]
export const between = (lo, hi) => lo + Math.floor(Math.random() * (hi - lo + 1))
