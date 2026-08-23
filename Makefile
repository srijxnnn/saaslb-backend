.PHONY: up down run test tidy

up:
	docker compose up -d

down:
	docker compose down

tidy:
	go mod tidy

test:
	go test ./...

run: up
	go run ./cmd/api
