.PHONY: dev server web test db fmt

dev:
	npm run dev

server:
	cd apps/server && go run ./cmd/server

web:
	cd apps/web && npm run dev

test:
	npm test

db:
	docker compose up -d postgres

fmt:
	cd apps/server && gofmt -w ./cmd ./internal
