.PHONY: build run test swagger docker-build docker-up docker-down clean

# Build local binary
build:
	go build -o pixez-sync-server .

# Run locally with test environment variables
run:
	PIXEZ_AUTH_USER=admin PIXEZ_AUTH_PASS=test12345 PIXEZ_DB_PATH=./pixez-sync.db PIXEZ_LISTEN_ADDR=:8080 go run main.go

# Run tests
test:
	go test -v ./...

# Generate Swagger/OpenAPI documents
swagger:
	go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g main.go -o docs --parseDependency --parseInternal

# Build Docker image
docker-build:
	docker compose build

# Start Docker containers
docker-up:
	docker compose up -d

# Stop Docker containers
docker-down:
	docker compose down

# Clean binaries and DB
clean:
	rm -f pixez-sync-server
	rm -f pixez-sync.db
