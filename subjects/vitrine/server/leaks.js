// Arquivos expostos por engano, para que o Nuclei e o ffuf do Omni tenham o
// que encontrar. Os segredos são falsos e só ficam acessíveis porque
// loadConfig impede a vitrine de ouvir fora do loopback.

const envFile = `NODE_ENV=producao
DATABASE_URL=postgres://vitrine:vitrine-cobaia@127.0.0.1:5434/vitrine
JWT_SECRET=FALSO-segredo-da-vitrine
STRIPE_SECRET_KEY=FALSO-sk_live_exemplo
AWS_ACCESS_KEY_ID=FALSO-AKIAVITRINE
AWS_SECRET_ACCESS_KEY=FALSO-segredo-aws-da-vitrine
`

const gitConfig = `[core]
\trepositoryformatversion = 0
\tfilemode = true
\tbare = false
[remote "origin"]
\turl = https://github.com/exemplo/vitrine-cobaia.git
\tfetch = +refs/heads/*:refs/remotes/origin/*
`

const backup = `-- pg_dump da vitrine (FALSO), esquecido na raiz do site
COPY usuarios (id, usuario, email, senha_md5, papel) FROM stdin;
1\tana\tana@exemplo.com\t5390489da3971cbbcd22c159d54d24da\tcliente
6\tadmin\tadmin@vitrine.local\t21232f297a57a5a743894a0e4a801fc3\tadmin
\\.
`

// robots.txt "esconde" justamente o que não devia estar no ar, e acaba
// servindo de mapa para quem varre o site.
const robots = `User-agent: *
Disallow: /admin
Disallow: /backup.sql
Disallow: /api/debug/
`

export function leaks(app, cfg) {
  const text = (body) => (_req, res) => res.type('text/plain').send(body)
  app.get('/.env', text(envFile))
  app.get('/.git/config', text(gitConfig))
  app.get('/.git/HEAD', text('ref: refs/heads/main\n'))
  app.get('/backup.sql', text(backup))
  app.get('/robots.txt', text(robots))
  // FALHA: a configuração e as variáveis de ambiente do processo, abertas.
  app.get('/api/debug/config', (_req, res) => {
    res.json({
      banco: cfg.database,
      node: process.version,
      pid: process.pid,
      cwd: process.cwd(),
      env: Object.fromEntries(Object.entries(process.env).filter(([k]) => /^(VITRINE|NODE)/.test(k))),
    })
  })
}
