.PHONY: all build fmt vet lint test test-migrations demo-segment4 check clean run-api run-worker docker-up docker-down

all: check

build:
	@echo "==> Building API binary..."
	@mkdir -p bin
	@go build -o bin/api ./cmd/api
	@echo "==> Building Worker binary..."
	@go build -o bin/worker ./cmd/worker
	@echo "==> Build completed successfully! Binaries located in ./bin/"

fmt:
	@echo "==> Formatting Go code..."
	@go fmt ./...

vet:
	@echo "==> Running go vet..."
	@go vet ./...

lint:
	@./scripts/lint.sh

test:
	@echo "==> Running unit tests..."
	@go test -v ./...

test-migrations:
	@echo "==> Testing database migrations..."
	@go test -v ./migrations/...

demo-segment4:
	@echo "==> Running Segment 4 Demo (Fault Tolerance & Idempotency)..."
	@./scripts/demo_segment4_idempotency.sh

check: fmt vet lint test test-migrations build
	@echo "==> All checks (fmt, vet, lint, test, test-migrations, build) passed cleanly!"


clean:
	@echo "==> Cleaning build artifacts..."
	@rm -rf bin/

run-api:
	@FREE_PORT=$$(./scripts/find_free_port.sh 8080); \
	echo "==> Starting API server on available port $$FREE_PORT..."; \
	PORT=$$FREE_PORT go run ./cmd/api

run-worker:
	@go run ./cmd/worker

docker-up:
	@API_PORT=$$(./scripts/find_free_port.sh 8080); \
	POSTGRES_PORT=$$(./scripts/find_free_port.sh 5432); \
	REDIS_PORT=$$(./scripts/find_free_port.sh 6379); \
	MINIO_PORT=$$(./scripts/find_free_port.sh 9000); \
	MINIO_CONSOLE_PORT=$$(./scripts/find_free_port.sh $$((MINIO_PORT + 1))); \
	MAILPIT_PORT=$$(./scripts/find_free_port.sh 8025); \
	MAILPIT_SMTP_PORT=$$(./scripts/find_free_port.sh 1025); \
	echo "==> Starting Docker Compose with auto-selected free host ports:"; \
	echo "    API:                $$API_PORT"; \
	echo "    PostgreSQL:         $$POSTGRES_PORT"; \
	echo "    Redis:              $$REDIS_PORT"; \
	echo "    MinIO S3:           $$MINIO_PORT"; \
	echo "    MinIO Console:      $$MINIO_CONSOLE_PORT"; \
	echo "    Mailpit UI:         $$MAILPIT_PORT"; \
	echo "    Mailpit SMTP:       $$MAILPIT_SMTP_PORT"; \
	API_PORT=$$API_PORT \
	POSTGRES_PORT=$$POSTGRES_PORT \
	REDIS_PORT=$$REDIS_PORT \
	MINIO_PORT=$$MINIO_PORT \
	MINIO_CONSOLE_PORT=$$MINIO_CONSOLE_PORT \
	MAILPIT_PORT=$$MAILPIT_PORT \
	MAILPIT_SMTP_PORT=$$MAILPIT_SMTP_PORT \
	docker compose up --build

docker-down:
	@docker compose down
