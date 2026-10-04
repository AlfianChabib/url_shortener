# Justfile command manager for URL Shortener

set shell := ["powershell.exe", "-NoProfile", "-Command"]

# Show available commands
default:
    @just --list

# Run application directly
run:
    go run ./cmd/api

# Run application in development mode with Air hot-reload
dev:
    air

# Build the application binary to bin/app.exe
build:
    go build -o ./bin/app.exe ./cmd/api

# Run the compiled binary
start:
    ./bin/app.exe

# Run all unit tests
test:
    go test -v ./...

# Run performance benchmarks with memory allocations
bench:
    go test -bench="Benchmark" -benchmem -run="^$" ./cmd/api

# Run k6 redirect load test (10k RPS target)
loadtest-redirect:
    k6 run scripts/k6/redirect_10k_rps.js

# Run k6 link creation load test (1k RPS target)
loadtest-create:
    k6 run scripts/k6/create_link_1k_rps.js

# Run full system enterprise stress test
loadtest-all:
    k6 run scripts/k6/enterprise_stress_test.js

# Generate Wire dependency injection code
wire:
    wire ./cmd/api

# Clean and tidy go modules
tidy:
    go mod tidy

# Start docker-compose services (PostgreSQL & Redis)
docker-up:
    docker compose up -d

# Stop docker-compose services
docker-down:
    docker compose down

# View logs from docker-compose services
docker-logs:
    docker compose logs -f

# Database migrations
migrate-up:
    migrate -path db/migrations -database "postgres://postgres:postgres@localhost:5432/url_shortener_db?sslmode=disable" up

migrate-down:
    migrate -path db/migrations -database "postgres://postgres:postgres@localhost:5432/url_shortener_db?sslmode=disable" down 1

# Seed database with initial data
db-seed:
    Get-Content db/seed/seed.sql | docker exec -i url_shortener_postgres psql -U postgres -d url_shortener_db

# Clean build artifacts
clean:
    if (Test-Path tmp) { Remove-Item -Recurse -Force tmp }
    if (Test-Path bin\app.exe) { Remove-Item -Force bin\app.exe }

