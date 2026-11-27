.PHONY: build web dev init-reset loja loja-cadastrar vitrine vitrine-cadastrar vitrine-ataque check

# build compila o frontend, copia para internal/web/static e gera o binário.
build: web
	go build -o omni ./cmd/omni

web:
	cd web && npm ci && npm run build
	find internal/web/static -mindepth 1 ! -name placeholder.html -delete
	cp -R web/dist/. internal/web/static/

# dev sobe o backend e o Vite com recarga automática, em portas altas livres.
dev:
	./scripts/dev.sh

# init-reset compila o binário, libera a captura de pacotes (setcap, pede sudo) e
# sobe o Omni com um OMNI_TOKEN aleatório novo (ou o que já estiver no ambiente),
# imprimindo o token antes de iniciar. Usa o frontend já embutido; rode make build
# para atualizá-lo.
init-reset:
	go build -o omni ./cmd/omni
	sudo setcap cap_net_raw,cap_net_admin=eip ./omni
	@export OMNI_TOKEN="$${OMNI_TOKEN:-$$(openssl rand -hex 32)}"; \
	echo "OMNI_TOKEN=$$OMNI_TOKEN"; \
	exec ./omni

# loja sobe a loja cobaia (containers, API e carga) até Ctrl+C, na porta de
# port= (padrão 3000). Não precisa de credenciais: ela não conhece o Omni.
port ?= 3000
loja:
	$(MAKE) -C subjects/loja app PORTA=$(port)

# loja-cadastrar registra os alvos da loja no Omni. Precisa de OMNI_TOKEN e,
# como o backend usa porta aleatória, de OMNI_URL.
loja-cadastrar:
	@: "$${OMNI_TOKEN:?defina OMNI_TOKEN (o que o make init-reset imprimiu)}"
	@: "$${OMNI_URL:?defina OMNI_URL (o painel= do log do backend, ex.: http://127.0.0.1:39755)}"
	$(MAKE) -C subjects/loja cadastrar

# vitrine sobe a loja com interface (banco no Docker, API em 127.0.0.1:4000 e
# site em http://127.0.0.1:5180) até Ctrl+C. Se a API cair, volta sozinha. Com
# OMNI_TOKEN e OMNI_URL no ambiente, cadastra o alvo no Omni antes de subir.
vitrine:
	cd subjects/vitrine && npm install --no-audit --no-fund && npm run banco
	@if [ -n "$$OMNI_TOKEN" ] || [ -n "$$OMNI_URL" ]; then \
		: "$${OMNI_TOKEN:?defina também OMNI_TOKEN para cadastrar no Omni}"; \
		: "$${OMNI_URL:?defina também OMNI_URL para cadastrar no Omni}"; \
		cd subjects/vitrine && npm run -s cadastrar; \
	else \
		echo "sem OMNI_TOKEN/OMNI_URL: pulei o cadastro (rode make vitrine-cadastrar depois ou use a página de Alvos)"; \
	fi
	cd subjects/vitrine && npm run dev

# vitrine-cadastrar cria o alvo vitrine no Omni (portas 4000 e 5434).
vitrine-cadastrar:
	@: "$${OMNI_TOKEN:?defina OMNI_TOKEN (o que o make init-reset imprimiu)}"
	@: "$${OMNI_URL:?defina OMNI_URL (o painel= do log do Omni, ex.: http://127.0.0.1:39755)}"
	cd subjects/vitrine && npm run -s cadastrar

# vitrine-ataque roda um ataque simulado contra a vitrine, como em
# make vitrine-ataque a=ddos. Opções do script vão em ARGS="--taxa 300".
vitrine-ataque:
	@: "$${a:?escolha o ataque: a=ddos, forca-bruta, spraying, varredura, idor, sqli, lento ou derrubar}"
	cd subjects/vitrine && npm run -s ataque:$(a) -- $(ARGS)

# check roda as verificações da skill convencoes-omni, parando na primeira falha.
check:
	test -z "$$(gofmt -l .)"
	go vet ./...
	golangci-lint run ./...
	go build ./...
	go test -race ./...
	cd web && npm run lint
	cd web && npm run typecheck
	cd web && npm test
	cd subjects/loja && $(MAKE) teste
