// Configuração da API da vitrine. Ela tem falhas de propósito, então só pode
// ouvir no loopback: um endereço fora dele derruba o processo na partida.

const loopback = new Set(['127.0.0.1', 'localhost', '::1', '[::1]'])

export function loadConfig(env = process.env) {
  const addr = env.VITRINE_ADDR || '127.0.0.1:4000'
  const i = addr.lastIndexOf(':')
  const host = addr.slice(0, i)
  const port = Number(addr.slice(i + 1))
  if (!loopback.has(host)) {
    throw new Error(`VITRINE_ADDR=${addr} não é loopback; a vitrine tem falhas de propósito e só roda em 127.0.0.1`)
  }
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error(`VITRINE_ADDR=${addr} tem uma porta inválida`)
  }
  return {
    host: host.replace(/^\[|\]$/g, ''),
    port,
    database: env.VITRINE_BANCO || 'postgres://vitrine:vitrine-cobaia@127.0.0.1:5434/vitrine?sslmode=disable',
  }
}
