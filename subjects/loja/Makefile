.PHONY: subir derrubar derrubar-tudo rodar carga cadastrar demo teste

# subir inicia os serviços do compose e espera ficarem saudáveis.
subir:
	docker compose up -d --wait

# derrubar para os serviços e mantém os dados.
derrubar:
	docker compose down

# derrubar-tudo para os serviços e apaga os volumes.
derrubar-tudo:
	docker compose down -v

# rodar sobe a API da loja em 127.0.0.1:3000.
rodar:
	go run ./cmd/loja

# carga gera tráfego na loja; passe flags em ARGS, como ARGS="-duracao 10s".
carga:
	go run ./cmd/carga $(ARGS)

# cadastrar cria os alvos loja e loja-tls no Omni (precisa de OMNI_TOKEN).
cadastrar:
	go run ./cmd/cadastrar

# demo sobe tudo, cadastra os alvos e gera carga até Ctrl+C.
demo: subir
	go build -o bin/ ./cmd/...
	./bin/loja & pid=$$!; trap "kill $$pid 2>/dev/null" EXIT INT TERM; \
	sleep 1; ./bin/cadastrar && ./bin/carga $(ARGS)

# teste roda lint, vet e testes da loja cobaia.
teste:
	go vet ./...
	golangci-lint run ./...
	go test -race ./...
