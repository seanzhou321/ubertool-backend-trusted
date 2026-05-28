.PHONY: proto-gen build build-server build-cronjob run tidy clean test-unit test-integration test-e2e test-smoke-ec2 docker-build docker-push deploy-services deploy-cronjob deploy-all setup-data-local wipe-db-local reset-db-local setup-data-ec2 wipe-db-ec2 reset-db-ec2 my-ip

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
	@echo "Starting server in DEBUG mode for testing..."
	set LOG_LEVEL=debug && go run ./cmd/server -config=config/config.desktop.manual.yaml

run-precommit:
	@echo "Starting server — Scenario A1: pre-commit coding tests (no TLS, no FCM, 2FA bypassed)..."
	go run ./cmd/server -config=config/config.precommit.yaml

run-desktop-uitest:
	@echo "Starting server — Scenario A2: desktop UI automated tests (FCM on, 2FA bypassed)..."
	go run ./cmd/server -config=config/config.desktop.uitest.yaml

run-desktop-manual:
	@echo "Starting server — Scenario A3: desktop UI manual tests (FCM on, 2FA live email)..."
	go run ./cmd/server -config=config/config.desktop.manual.yaml

run-cronjob-dev:
	@echo "Starting cronjob in DEBUG mode for testing..."
	set LOG_LEVEL=debug && go run ./cmd/cronjob -config=config/config.desktop.manual.yaml

run-cronjob-once:
	@if "$(JOB)"=="" (echo Error: Please specify JOB variable, e.g., make run-cronjob-once JOB=mark-overdue-rentals) else (go run ./cmd/cronjob -config=config/config.desktop.manual.yaml -run-once=$(JOB))

run-test:
	@echo "Starting server in DEBUG mode for testing..."
	go run ./cmd/server -config=config/config.precommit.yaml

run-test-debug:
	@echo "Starting server in DEBUG mode with verbose output..."
	set LOG_LEVEL=debug&& go run ./cmd/server -config=config/config.precommit.yaml

run-test-cron-debug:
	@echo "Starting cronjob in DEBUG mode with verbose output..."
	set LOG_LEVEL=debug&& go run ./cmd/cronjob -config=config/config.precommit.yaml

tidy:
	go mod tidy

clean:
	@if exist "bin" rmdir /s /q bin
	@if exist "api\gen" rmdir /s /q api\gen

test-unit:
	go test -v ./tests/unit/...

test-precommit:
	@echo "Running full pre-commit test suite (unit + integration + e2e) — Scenario A1..."
	go test -v ./tests/unit/...
	go test -v ./tests/integration/... -config=config/config.precommit.yaml
	go test -v ./tests/e2e/... -config=config/config.precommit.yaml

test-integration:
	go test -v ./tests/integration/... -config=config/config.precommit.yaml

test-e2e:
	go test -v ./tests/e2e/... -config=config/config.precommit.yaml

test-ext-integration:
	go test -v ./tests/ext-integration/... -run Gmail -config=config/mail_config.test.yaml

test-ext-integration-ses:
	go test -v ./tests/ext-integration/... -run TestSES -config=config/mail_config.test.yaml

test-ext-integration-all:
	go test -v ./tests/ext-integration/... -run "Gmail|TestSES" -config=config/mail_config.test.yaml


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

test-e2e-push-notification:
	go test -v ./tests/e2e -run "TestPushNotificationService_E2E"

setup-data-local:
	@echo "Populating test data from YAML..."
	go run ./tests/data-setup/setup.go -config=config/config.precommit.yaml -setup=tests/data-setup/user_org.test.yaml

wipe-db-local:
	@echo "Wiping all data from local test database (schema is preserved)..."
	go run ./tests/data-setup/setup.go -config=config/config.precommit.yaml -wipe

reset-db-local: wipe-db-local setup-data-local
	@echo "Local test database reset complete."

setup-data-ec2:
	@echo To populate EC2 RDS with initial data, run from a PowerShell terminal:
	@echo   .\deploy\ec2-mvp\05_setup_data.ps1 -DataFile tests\data-setup\user_org.test.yaml

wipe-db-ec2:
	@echo To wipe all data from EC2 RDS, run from a PowerShell terminal:
	@echo   .\deploy\ec2-mvp\06_wipe_data.ps1

reset-db-ec2:
	@echo To reset EC2 RDS to baseline, run from a PowerShell terminal:
	@echo   .\deploy\ec2-mvp\06_wipe_data.ps1
	@echo   .\deploy\ec2-mvp\05_setup_data.ps1 -DataFile tests\data-setup\user_org.test.yaml

# Smoke tests against the live EC2 deployment.
# Ensure config/config.ec2.apitest.yaml exists (see deploy/ec2-mvp/docs/handoff.md Phase 2b)
test-smoke-ec2:
	go test -v -count=1 -timeout 30s ./tests/smoke/ -args -config=config/config.ec2.apitest.yaml

my-ip:
	@powershell -NoProfile -Command "(Invoke-RestMethod https://checkip.amazonaws.com).Trim()"
	@echo "Use the above IP address to whitelist in AWS security groups for EC2 access."
	@echo https://us-west-2.console.aws.amazon.com/ec2/home?region=us-west-2#SecurityGroup:group-id=sg-0604272042828172f

