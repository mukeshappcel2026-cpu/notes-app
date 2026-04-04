.PHONY: up down build logs test

# Start all services
up:
	docker compose up -d

# Stop all services
down:
	docker compose down

# Build and start
build:
	docker compose up -d --build

# View logs
logs:
	docker compose logs -f canopy-server

# Run Go tests
test-backend:
	cd backend && go test ./...

# Install Python SDK locally
install-sdk:
	cd sdk/python && pip install -e ".[dev,litellm]"

# Run Python tests
test-sdk:
	cd sdk/python && pytest tests/

# Full test
test: test-backend test-sdk

# Reset databases (destructive)
reset:
	docker compose down -v
	docker compose up -d
