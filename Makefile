.PHONY: proto-gen build build-server build-cronjob run tidy clean test-unit test-integration test-e2e docker-build docker-push deploy-services deploy-cronjob deploy-all

PROTO_SRC_DIR = api/proto
PROTO_DEST_DIR = .
PROTO_FILES = $(wildcard $(PROTO_SRC_DIR)/ubertool_trusted_backend/v1/*.proto)

proto-gen:
	@if not exist "api\gen\v1" mkdir api\gen\v1
	protoc --proto_path=$(PROTO_SRC_DIR) \
		--go_out=$(PROTO_DEST_DIR) --go_opt=module=ubertool-backend-trusted \
		--go-grpc_out=$(PROTO_DEST_DIR) --go-grpc_opt=module=ubertool-backend-trusted \
		$(PROTO_FILES)

build:
	@if not exist "bin" mkdir bin
	go build -o bin/server.exe ./cmd/server
	go build -o bin/cronjob.exe ./cmd/cronjob

build-server:
	@if not exist "bin" mkdir bin
	go build -o bin/server.exe ./cmd/server

build-cronjob:
	@if not exist "bin" mkdir bin
	go build -o bin/cronjob.exe ./cmd/cronjob

run-dev:
	go run ./cmd/server -config=config/config.dev.yaml

run-cronjob-dev:
	go run ./cmd/cronjob -config=config/config.dev.yaml

run-cronjob-once:
	@if "$(JOB)"=="" (echo Error: Please specify JOB variable, e.g., make run-cronjob-once JOB=mark-overdue-rentals) else (go run ./cmd/cronjob -config=config/config.dev.yaml -run-once=$(JOB))

run-test:
	@echo "Starting server in DEBUG mode for testing..."
	go run ./cmd/server -config=config/config.test.yaml

run-test-debug:
	@echo "Starting server in DEBUG mode with verbose output..."
	set LOG_LEVEL=debug&& go run ./cmd/server -config=config/config.test.yaml

run-test-cron-debug:
	@echo "Starting cronjob in DEBUG mode with verbose output..."
	set LOG_LEVEL=debug&& go run ./cmd/cronjob -config=config/config.test.yaml

tidy:
	go mod tidy

clean:
	@if exist "bin" rmdir /s /q bin
	@if exist "api\gen" rmdir /s /q api\gen

test-unit:
	go test -v ./tests/unit/...

test-integration:
	go test -v ./tests/integration/... -config=config/config.test.yaml

test-e2e:
	go test -v ./tests/e2e/... -config=config/config.test.yaml

test-ext-integration:
	go test -v ./tests/ext-integration/... -run Gmail -config=config/config.test.yaml


# Docker commands
docker-build:
	@echo "Building Docker image with both server and cronjob binaries..."
	podman build -f podman/trusted-group/Dockerfile_services_cronjobs -t ubertool-backend:latest .

docker-push:
	@echo "Pushing Docker image to registry..."
	podman tag ubertool-backend:latest registry.example.com/ubertool:latest
	podman push registry.example.com/ubertool:latest

# Deployment commands
deploy-services:
	@echo "Deploying backend services..."
	cd podman/trusted-group/services && podman-compose up -d

deploy-cronjob:
	@echo "Deploying cronjob scheduler..."
	cd podman/trusted-group/cronjob && podman-compose up -d

deploy-all: docker-build
	@echo "Deploying all services..."
	cd podman/trusted-group/services && podman-compose up -d
	cd podman/trusted-group/cronjob && podman-compose up -d

# Cronjob management
cronjob-logs:
	@echo "Showing cronjob logs..."
	podman logs -f ubertool-cronjob

cronjob-status:
	@echo "Checking cronjob status..."
	podman ps -a --filter name=ubertool-cronjob

cronjob-restart:
	@echo "Restarting cronjob container..."
	cd podman/trusted-group/cronjob && podman-compose restart cronjob
test-e2e-admin-retrieve:
	go test -v ./tests/e2e -run "TestOrganizationService_E2E/SearchOrganizations_-_Verify_Admins_Array_Populated"

setup-test-data:
	@echo "Populating test data from YAML..."
	go run ./tests/data-setup/setup.go
