.PHONY: all build fmt vet lint test test-migrations demo-segment4 demo-segments-1-2 test-stage test-e2e test-postman test-postman-identity test-postman-admin test-postman-authoring test-postman-media seed seed-clean seed-reset seed-status check clean run-api run-worker docker-up docker-down

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

demo-segments-1-2:
	@echo "==> Running Segments 1 & 2 Live Interactive Demo (Identity & Authoring)..."
	@./scripts/demo_segments_1_and_2.sh

test-e2e:
	@echo "==> Running Full E2E Test Suite (Identity, Admin, Authoring) & Harvesting Evidence..."
	@./scripts/run_e2e_identity_authoring.sh

test-stage:
	@echo "==> Running Stage Automated Test & Validation Suite..."
	@./scripts/validate_stage.sh

test-postman-identity:
	@echo "==> Running Identity Postman/Newman automated integration tests..."
	@docker compose exec -T redis redis-cli EVAL "for _,k in ipairs(redis.call('keys','ratelimit:*')) do redis.call('del',k) end" 0 >/dev/null 2>&1 || true
	@MSYS_NO_PATHCONV=1 docker run --rm --network plataforma-mooc_default \
		-v "$$(pwd)/docs/postman":/etc/newman postman/newman:alpine \
		run /etc/newman/collection_api.postman_collection.json \
		-e /etc/newman/mooc_docker.postman_environment.json

test-postman-admin:
	@echo "==> Running Administration Postman/Newman automated integration tests..."
	@docker compose exec -T redis redis-cli EVAL "for _,k in ipairs(redis.call('keys','ratelimit:*')) do redis.call('del',k) end" 0 >/dev/null 2>&1 || true
	@MSYS_NO_PATHCONV=1 docker run --rm --network plataforma-mooc_default \
		-v "$$(pwd)/docs/postman":/etc/newman postman/newman:alpine \
		run /etc/newman/collection_admin.postman_collection.json \
		-e /etc/newman/mooc_docker.postman_environment.json

test-postman-authoring:
	@echo "==> Running Course Authoring Postman/Newman automated integration tests..."
	@docker compose exec -T redis redis-cli EVAL "for _,k in ipairs(redis.call('keys','ratelimit:*')) do redis.call('del',k) end" 0 >/dev/null 2>&1 || true
	@MSYS_NO_PATHCONV=1 docker run --rm --network plataforma-mooc_default \
		-v "$$(pwd)/docs/postman":/etc/newman postman/newman:alpine \
		run /etc/newman/collection_authoring.postman_collection.json \
		-e /etc/newman/mooc_docker.postman_environment.json

test-postman-media:
	@echo "==> Running Media direct-upload Postman/Newman automated integration tests..."
	@docker compose exec -T redis redis-cli EVAL "for _,k in ipairs(redis.call('keys','ratelimit:*')) do redis.call('del',k) end" 0 >/dev/null 2>&1 || true
	@MSYS_NO_PATHCONV=1 docker run --rm --network plataforma-mooc_default \
		-v "$$(pwd)/docs/postman":/etc/newman postman/newman:alpine \
		run /etc/newman/collection_media.postman_collection.json \
		-e /etc/newman/mooc_docker.postman_environment.json

test-postman: test-postman-identity test-postman-admin test-postman-authoring test-postman-media

seed:
	@./scripts/seed.sh --load

seed-clean:
	@./scripts/seed.sh --clean

seed-reset:
	@./scripts/seed.sh --reset

seed-status:
	@./scripts/seed.sh --status

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
