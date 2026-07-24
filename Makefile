.PHONY: build test run clean docker-up docker-down

build:
	go build -o bin/sems ./cmd/sems

test:
	go test ./... -v

run: build
	./bin/sems --config configs/example_station.json --port 8080 --dev

clean:
	rm -rf bin/

docker-up:
	docker-compose up --build -d

docker-down:
	docker-compose down
