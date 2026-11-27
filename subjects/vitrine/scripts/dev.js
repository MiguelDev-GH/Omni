// npm run dev: sobe a API (porta 4000) e o Vite (porta 5180) até Ctrl+C.
// Se a API cair (ataque:derrubar), ela volta sozinha depois de
// VITRINE_REINICIO segundos, para o Omni ver a queda e a volta.

import { spawn } from 'node:child_process'

const restartAfter = Number(process.env.VITRINE_REINICIO || 45)
const children = new Set()
let stopping = false

function run(name, args, onExit) {
  const child = spawn(process.execPath, args, { stdio: ['ignore', 'pipe', 'pipe'] })
  children.add(child)
  const prefix = (stream, out) =>
    stream.on('data', (d) => {
      for (const line of d.toString().split('\n')) if (line) out.write(`[${name}] ${line}\n`)
    })
  prefix(child.stdout, process.stdout)
  prefix(child.stderr, process.stderr)
  child.on('exit', (code) => {
    children.delete(child)
    if (!stopping) onExit?.(code)
  })
  return child
}

function api() {
  run('api', ['server/index.js'], (code) => {
    console.log(`[dev] a API caiu (código ${code}); volta em ${restartAfter} s (VITRINE_REINICIO)`)
    setTimeout(api, restartAfter * 1000)
  })
}

api()
run('web', ['node_modules/vite/bin/vite.js'], () => {
  console.log('[dev] o Vite parou; encerrando')
  stop()
})

function stop() {
  stopping = true
  for (const c of children) c.kill('SIGTERM')
  process.exit(0)
}
process.on('SIGINT', stop)
process.on('SIGTERM', stop)
