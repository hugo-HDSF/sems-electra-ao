# Build Stage
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod ./
# If go.sum exists, copy it. For now just go.mod
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o sems ./cmd/sems

# Run Stage
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/sems .
COPY --from=builder /app/configs ./configs
COPY --from=builder /app/swagger ./swagger
EXPOSE 8080
CMD ["./sems", "--config", "configs/example_station.json", "--port", "8080"]
