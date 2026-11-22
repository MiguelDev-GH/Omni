#!/usr/bin/env bash
# dev.sh sobe o backend e o Vite, cada um numa porta alta livre escolhida pelo
# sistema a cada execução. Exige OMNI_TOKEN no ambiente.
set -euo pipefail
cd "$(dirname "$0")/.."

: "${OMNI_TOKEN:?defina OMNI_TOKEN}"

# porta_livre pede ao kernel uma porta efêmera livre no loopback.
porta_livre() {
	node -e 'const s = require("net").createServer(); s.listen(0, "127.0.0.1", () => { console.log(s.address().port); s.close(); });'
}

api=$(porta_livre)
web=$(porta_livre)
while [ "$web" = "$api" ]; do web=$(porta_livre); done

tmp=$(mktemp -d)
go build -o "$tmp/omni" ./cmd/omni

export OMNI_ADDR="127.0.0.1:$api"
export OMNI_ORIGENS="http://localhost:$web,http://127.0.0.1:$web${OMNI_ORIGENS:+,$OMNI_ORIGENS}"
"$tmp/omni" &
backend=$!
trap 'kill "$backend" 2>/dev/null; wait "$backend" 2>/dev/null; rm -rf "$tmp"' EXIT

echo "omni: backend em http://127.0.0.1:$api, painel em http://localhost:$web"
OMNI_API="127.0.0.1:$api" OMNI_PORTA_WEB="$web" npm --prefix web run dev
