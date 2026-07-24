.PHONY: build test run clean down

# --- Dockerized Commands (Recommended) ---

# Tests run in a temporary, throw-away Go container since the final 
# production image (from docker-compose) doesn't have the Go compiler installed.
test:
	docker run --rm -v $${PWD}:/app -w /app golang:1.23-alpine sh -c "go mod download && go test ./... -v"

# Starts the entire application stack via Docker Compose with live logs attached.
# It uses a multi-stage Dockerfile to compile the app and run it in a tiny Alpine image.
build:
	docker-compose up --build

# Stops and removes the Docker containers.
down:
	docker-compose down

# --- Utilities ---

clean:
	rm -rf bin/
