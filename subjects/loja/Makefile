.PHONY: subir derrubar derrubar-tudo teste

# subir inicia os serviços do compose e espera ficarem saudáveis.
subir:
	docker compose up -d --wait

# derrubar para os serviços e mantém os dados.
derrubar:
	docker compose down

# derrubar-tudo para os serviços e apaga os volumes.
derrubar-tudo:
	docker compose down -v

# teste roda lint, vet e testes da loja cobaia.
teste:
	go vet ./...
	golangci-lint run ./...
	go test -race ./...
