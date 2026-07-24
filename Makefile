.PHONY: build test run clean docker-up docker-down

# --- Local Native Commands (Requires Go installed on host) ---

build:
	go build -o bin/sems ./cmd/sems

run: build
	./bin/sems --config configs/example_station.json --port 8080 --dev

# --- Dockerized Commands (Recommended) ---

# Tests run in a temporary, throw-away Go container since the final 
# production image (from docker-compose) doesn't have the Go compiler installed.
test:
	docker run --rm -v $${PWD}:/app -w /app golang:1.23-alpine sh -c "go mod download && go test ./... -v"

# Starts the entire application stack via Docker Compose with live logs attached.
# It uses a multi-stage Dockerfile to compile the app and run it in a tiny Alpine image.
dev:
	docker-compose up --build

# --- Utilities ---

clean:
	rm -rf bin/
