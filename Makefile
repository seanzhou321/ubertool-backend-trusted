.PHONY: proto-gen build build-server build-cronjob run tidy clean test-unit test-integration test-e2e test-smoke-ec2 podman-build podman-push deploy-services deploy-cronjob deploy-all db-deploy db-teardown db-schema-install db-schema-teardown setup-data-local wipe-db-local reset-db-local ec2-deploy ec2-reinstall-schema ec2-setup-data ec2-wipe-data ec2-reset-data ec2-use-prod ec2-use-uitest my-ip help

DATAFILE ?= tests\data-setup\user_org.test.yaml

PROTO_SRC_DIR = api/proto
PROTO_DEST_DIR = .
PROTO_FILES = $(wildcard $(PROTO_SRC_DIR)/ubertool_trusted_backend/v1/*.proto)

proto-gen:
	@if not exist "api\gen\v1" mkdir api\gen\v1
	protoc --proto_path=$(PROTO_SRC_DIR) \
		--go_out=$(PROTO_DEST_DIR) --go_opt=module=ubertool-backend-trusted \
		--go-grpc_out=$(PROTO_DEST_DIR) --go-grpc_opt=module=ubertool-backend-trusted \
		$(PROTO_FILES)

# build targets
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

# run targets
run-precommit:
	@echo "Starting server — Scenario A1: pre-commit coding tests (no TLS, no FCM, 2FA bypassed)..."
	set LOG_LEVEL=debug && go run ./cmd/server -config=config/config.precommit.yaml

run-desktop-uitest:
	@echo "Starting server — Scenario A2: desktop UI automated tests (FCM on, 2FA bypassed)..."
	go run ./cmd/server -config=config/config.desktop.uitest.yaml

run-desktop-manual:
	@echo "Starting server — Scenario A3: desktop UI manual tests (FCM on, 2FA live email)..."
	set LOG_LEVEL=debug && go run ./cmd/server -config=config/config.desktop.manual.yaml

run-cronjob-dev:
	@echo "Starting cronjob in DEBUG mode for testing..."
	set LOG_LEVEL=debug && go run ./cmd/cronjob -config=config/config.desktop.manual.yaml

run-cronjob-once:
	@if "$(JOB)"=="" (echo Error: Please specify JOB variable, e.g., make run-cronjob-once JOB=mark-overdue-rentals) else (go run ./cmd/cronjob -config=config/config.desktop.manual.yaml -run-once=$(JOB))

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

test-integration:
	go test -v ./tests/integration/... -config=config/config.precommit.yaml

test-e2e:
	go test -v ./tests/e2e/... -config=config/config.precommit.yaml

test-e2e-admin-retrieve:
	go test -v ./tests/e2e -run "TestOrganizationService_E2E/SearchOrganizations_-_Verify_Admins_Array_Populated"

test-e2e-fcm:
	go test -v ./tests/e2e -run "TestPushNotificationService_E2E"

test-precommit:
	@echo "Running full pre-commit test suite (unit + integration + e2e) — Scenario A1..."
	go test -v ./tests/unit/...
	go test -v ./tests/integration/... -config=config/config.precommit.yaml
	go test -v ./tests/e2e/... -config=config/config.precommit.yaml

test-ext-integration:
	go test -v ./tests/ext-integration/... -run Gmail -config=config/mail_config.test.yaml

test-ext-integration-ses:
	go test -v ./tests/ext-integration/... -run TestSES -config=config/mail_config.test.yaml

test-ext-integration-all:
	go test -v ./tests/ext-integration/... -run "Gmail|TestSES" -config=config/mail_config.test.yaml

# Smoke tests against the live EC2 deployment. 
# Only test the open API endpoints that don't require authentication, since we don't want to hardcode any credentials in the Makefile.
# Ensure config/config.ec2.apitest.yaml exists (see deploy/ec2-mvp/docs/handoff.md Phase 2b)
test-smoke-ec2:
	go test -v -count=1 -timeout 30s ./tests/smoke/ -args -config=config/config.ec2.apitest.yaml


# Docker commands
podman-build:
	@echo "Building Docker image with both server and cronjob binaries..."
	podman build -f podman/trusted-group/Dockerfile_services_cronjobs -t ubertool-backend:latest .

podman-push:
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

deploy-all: podman-build
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

# Local Podman database lifecycle
db-deploy:
	@echo "Building Postgres image and starting container on localhost:5454..."
	powershell -NoProfile -ExecutionPolicy Bypass -File podman\trusted-group\postgres\install.ps1

db-teardown:
	@echo "Stopping Postgres container and removing image and volume..."
	powershell -NoProfile -ExecutionPolicy Bypass -File podman\trusted-group\postgres\teardown.ps1

db-schema-install:
	@echo "Applying SQL schema to running Postgres container..."
	powershell -NoProfile -ExecutionPolicy Bypass -File deploy\local_db\install.ps1

db-schema-teardown:
	@echo "Dropping all tables from local Postgres database..."
	powershell -NoProfile -ExecutionPolicy Bypass -File deploy\local_db\teardown.ps1

setup-data-local:
	@echo "Populating test data from YAML..."
	go run ./tests/data-setup/setup.go -config=config/config.precommit.yaml -setup=tests/data-setup/user_org.test.yaml

wipe-db-local:
	@echo "Wiping all data from local test database (schema is preserved)..."
	go run ./tests/data-setup/setup.go -config=config/config.precommit.yaml -wipe

reset-db-local: wipe-db-local setup-data-local
	@echo "Local test database reset complete."

# EC2 + RDS operations
ec2-deploy:
	@echo "Redeploying: cross-compiling, uploading binary, and restarting service on EC2..."
	powershell -NoProfile -ExecutionPolicy Bypass -File deploy\ec2-mvp\04_deploy.ps1

ec2-reinstall-schema:
	@echo "Wiping and reinstalling RDS schema via EC2..."
	powershell -NoProfile -ExecutionPolicy Bypass -File deploy\ec2-mvp\02_init_db.ps1

ec2-setup-data:
	@echo "Seeding RDS with data from $(DATAFILE)..."
	powershell -NoProfile -ExecutionPolicy Bypass -File deploy\ec2-mvp\05_setup_data.ps1 -DataFile $(DATAFILE)

ec2-wipe-data:
	@echo "Wiping all data from EC2 RDS (schema preserved)..."
	powershell -NoProfile -ExecutionPolicy Bypass -File deploy\ec2-mvp\06_wipe_data.ps1

ec2-reset-data: ec2-wipe-data ec2-setup-data
	@echo "EC2 RDS reset complete."

ec2-use-prod:
	@echo "Switching EC2 to production config (2FA on, real email, S3)..."
	powershell -NoProfile -ExecutionPolicy Bypass -File deploy\ec2-mvp\switch-config.ps1 -Mode prod

ec2-use-uitest:
	@echo "Switching EC2 to UI-test config (2FA bypassed, mock email, S3)..."
	powershell -NoProfile -ExecutionPolicy Bypass -File deploy\ec2-mvp\switch-config.ps1 -Mode uitest

my-ip:
	@powershell -NoProfile -Command "(Invoke-RestMethod https://checkip.amazonaws.com).Trim()"
	@echo "Use the above IP address to whitelist in AWS security groups for EC2 access."
	@echo https://us-west-2.console.aws.amazon.com/ec2/home?region=us-west-2#SecurityGroup:group-id=sg-0604272042828172f

help:
	@echo.
	@echo Usage: make [target]
	@echo.
	@echo --- Code Generation ---
	@echo   proto-gen                 Generate Go code from .proto files
	@echo.
	@echo --- Build ---
	@echo   build                     Build both server and cronjob binaries into bin/
	@echo   build-server              Build the server binary only
	@echo   build-cronjob             Build the cronjob binary only
	@echo   tidy                      Run go mod tidy
	@echo   clean                     Remove bin/ and api/gen/ directories
	@echo.
	@echo --- Run microservice (Local Dev) ---
	@echo   run-precommit             Start server for integration and e2e tests (no TLS, no FCM, 2FA bypassed)
	@echo   run-desktop-uitest        Start server for desktop UI automated tests through emulators(FCM on, 2FA bypassed)
	@echo   run-desktop-manual        Start server in DEBUG mode for manual testing from emulators or connected devices
	@echo   run-cronjob-dev           Start cronjob in DEBUG mode
	@echo   run-cronjob-once JOB=^<name^>  Run a single named cronjob (e.g. make run-cronjob-once JOB=mark-overdue-rentals)
	@echo   run-test-cron-debug       Start cronjob with pre-commit config and verbose DEBUG logging
	@echo.
	@echo --- Tests (local Dev) ---
	@echo   test-unit                 Run unit tests
	@echo   test-integration          Run integration tests (pre-commit config)
	@echo   test-e2e                  Run all e2e tests (pre-commit config)
	@echo   test-e2e-admin-retrieve   Run e2e test: SearchOrganizations admin array check
	@echo   test-e2e-fcm              Run e2e test: push notification service
	@echo   test-precommit            Run full pre-commit suite: unit + integration + e2e
	@echo   test-ext-integration      Run external integration tests against Gmail
	@echo   test-ext-integration-ses  Run external integration tests against AWS SES
	@echo   test-ext-integration-all  Run all external integration tests (Gmail + SES)
	@echo.
	@echo --- Tests (EC2) ---
	@echo   test-smoke-ec2            Run smoke tests against live EC2 deployment (TLS and FCM enabled, real 2FA)
	@echo.
	@echo --- Local Podman Deployment (in progress)---
	@echo   podman-build              Build the Podman image (server + cronjob)
	@echo   podman-push               Tag and push the image to the registry
	@echo   deploy-services           Start backend services via podman-compose
	@echo   deploy-cronjob            Start cronjob scheduler via podman-compose
	@echo   deploy-all                Build image and start all services
	@echo   cronjob-logs              Tail logs from the running cronjob container
	@echo   cronjob-status            Show status of the cronjob container
	@echo   cronjob-restart           Restart the cronjob container
	@echo.
	@echo --- Podman Database Lifecycle ---
	@echo   db-deploy                 Build Postgres image and start container on localhost:5454
	@echo   db-teardown               Stop container, remove image and volume
	@echo   db-schema-install         Apply SQL schema to the running Postgres container
	@echo   db-schema-teardown        Drop all tables from the local Postgres database
	@echo.
	@echo --- Podman Database Data Setup ---
	@echo   setup-data-local          Populate local test DB from YAML fixture
	@echo   wipe-db-local             Wipe all data from local test DB (schema preserved)
	@echo   reset-db-local            Wipe then repopulate local test DB
	@echo.
	@echo --- EC2 Operations ---
	@echo   ec2-deploy                Cross-compile, upload binary, restart service on EC2
	@echo   ec2-use-prod              Switch EC2 to production config and restart service
	@echo   ec2-use-uitest            Switch EC2 to UI-test config and restart service
	@echo.
	@echo --- AWS RDS Operations ---
	@echo   ec2-reinstall-schema      Wipe and reinstall the RDS database schema
	@echo   ec2-setup-data            Seed RDS with data (DATAFILE=path, default: user_org.test.yaml)
	@echo   ec2-wipe-data             Wipe all data from EC2 RDS (schema preserved)
	@echo   ec2-reset-data            Wipe then reseed EC2 RDS
	@echo.
	@echo --- Utilities ---
	@echo   my-ip                     Print your current public IP (for AWS security group whitelisting)
	@echo   help                      Show this help message
	@echo.

