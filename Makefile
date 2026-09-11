.PHONY: build run tidy clean docker-build docker-up docker-down docker-logs docker-restart docker-push

build:
	go build -o bin/investment-bot ./cmd/bot

run: build
	./bin/investment-bot

tidy:
	go mod tidy

clean:
	rm -rf bin/ data/

docker-build:
	docker compose build

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f bot

docker-restart:
	docker compose down && docker compose up -d --build
