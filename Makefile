.PHONY: build web check

# build compila o frontend, copia para internal/web/static e gera o binário.
build: web
	go build -o omni ./cmd/omni

web:
	cd web && npm ci && npm run build
	find internal/web/static -mindepth 1 ! -name placeholder.html -delete
	cp -R web/dist/. internal/web/static/

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
