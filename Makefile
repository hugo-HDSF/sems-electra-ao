.PHONY: build test run clean docker-up docker-down

build:
	go build -o bin/sems ./cmd/sems

test:
	docker run --rm -v $${PWD}:/app -w /app golang:1.23-alpine sh -c "go mod download && go test ./... -v"

run: build
	./bin/sems --config configs/example_station.json --port 8080 --dev

clean:
	rm -rf bin/

dev:
	docker-compose up --build
