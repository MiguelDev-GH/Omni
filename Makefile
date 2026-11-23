.PHONY: build web dev backend loja loja-cadastrar check

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

# backend compila o binário, libera a captura de pacotes (setcap, pede sudo) e
# sobe o Omni com um OMNI_TOKEN aleatório novo (ou o que já estiver no ambiente),
# imprimindo o token antes de iniciar. Usa o frontend já embutido; rode make build
# para atualizá-lo.
backend:
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
	@: "$${OMNI_TOKEN:?defina OMNI_TOKEN (o que o make backend imprimiu)}"
	@: "$${OMNI_URL:?defina OMNI_URL (o painel= do log do backend, ex.: http://127.0.0.1:39755)}"
	$(MAKE) -C subjects/loja cadastrar

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
