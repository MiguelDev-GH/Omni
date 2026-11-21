.PHONY: subir derrubar derrubar-tudo rodar carga teste

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

# teste roda lint, vet e testes da loja cobaia.
teste:
	go vet ./...
	golangci-lint run ./...
	go test -race ./...
