.PHONY: proto-gen build build-server build-cronjob run tidy clean test-unit test-integration test-e2e test-e2e-rate-limit test-smoke-ec2 cronjob-build cronjob-push deploy-cronjob cronjob-logs cronjob-status cronjob-restart grpc-start grpc-stop grpc-use-precommit grpc-use-uitest grpc-use-manual db-deploy db-teardown db-schema-install db-schema-teardown setup-data-local wipe-db-local reset-db-local ec2-deploy ec2-reinstall-schema ec2-setup-data ec2-wipe-data ec2-reset-data ec2-use-prod ec2-use-uitest my-ip help

# GNU Make on Windows executes recipes via cmd.exe unless it finds a POSIX sh.exe
# on PATH (which Git Bash / WSL put there), in which case it uses that instead.
# `uname` only exists in the POSIX-sh case, so it doubles as a reliable probe for
# which shell will actually run the recipe lines below — this must stay in sync
# with the shell Make picks, not with whichever terminal invoked `make`.
UNAME_S := $(shell uname -s 2>/dev/null)
ifeq ($(UNAME_S),)
MKDIR = if not exist "$(1)" mkdir "$(1)"
RMDIR = if exist "$(1)" rmdir /s /q "$(1)"
SET_DEBUG = set LOG_LEVEL=debug &&
ECHO_BLANK = @echo.
else
MKDIR = mkdir -p "$(1)"
RMDIR = rm -rf "$(1)"
SET_DEBUG = LOG_LEVEL=debug
ECHO_BLANK = @echo
endif

DATAFILE ?= tests/data-setup/user_org.test.yaml

PROTO_SRC_DIR = api/proto
PROTO_DEST_DIR = .
PROTO_FILES = $(wildcard $(PROTO_SRC_DIR)/ubertool_trusted_backend/v1/*.proto)

proto-gen:
	@$(call MKDIR,api/gen/v1)
	protoc --proto_path=$(PROTO_SRC_DIR) \
		--go_out=$(PROTO_DEST_DIR) --go_opt=module=ubertool-backend-trusted \
		--go-grpc_out=$(PROTO_DEST_DIR) --go-grpc_opt=module=ubertool-backend-trusted \
		$(PROTO_FILES)

# build targets
build:
	@$(call MKDIR,bin)
	go build -o bin/server.exe ./cmd/server
	go build -o bin/cronjob.exe ./cmd/cronjob

build-server:
	@$(call MKDIR,bin)
	go build -o bin/server.exe ./cmd/server

build-cronjob:
	@$(call MKDIR,bin)
	go build -o bin/cronjob.exe ./cmd/cronjob

# run targets
run-precommit:
	@echo "Starting server — Scenario A1: pre-commit coding tests (no TLS, no FCM, 2FA bypassed)..."
	$(SET_DEBUG) go run ./cmd/server -config=config/config.precommit.yaml

run-desktop-uitest:
	@echo "Starting server — Scenario A2: desktop UI automated tests (FCM on, 2FA bypassed)..."
	go run ./cmd/server -config=config/config.desktop.uitest.yaml

run-desktop-manual:
	@echo "Starting server — Scenario A3: desktop UI manual tests (FCM on, 2FA live email)..."
	$(SET_DEBUG) go run ./cmd/server -config=config/config.desktop.manual.yaml

run-cronjob-dev:
	@echo "Starting cronjob in DEBUG mode for testing..."
	$(SET_DEBUG) go run ./cmd/cronjob -config=config/config.desktop.manual.yaml

run-cronjob-once:
ifeq ($(JOB),)
	@echo Error: Please specify JOB variable, e.g., make run-cronjob-once JOB=mark-overdue-rentals
else
	go run ./cmd/cronjob -config=config/config.desktop.manual.yaml -run-once=$(JOB)
endif

run-test-cron-debug:
	@echo "Starting cronjob in DEBUG mode with verbose output..."
	$(SET_DEBUG) go run ./cmd/cronjob -config=config/config.precommit.yaml

tidy:
	go mod tidy

clean:
	@$(call RMDIR,bin)
	@$(call RMDIR,api/gen)

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

# Deliberately exhausts the shared per-IP Login/Verify2FA rate limiter (FR-012 in
# specs/001-authentication-legal-consent) — gated behind the "ratelimit" build tag and run in
# isolation so it doesn't starve other Login/Verify2FA e2e tests. Restart the server
# (run-precommit) first so the in-memory buckets start empty.
test-e2e-rate-limit:
	go test -tags ratelimit -v ./tests/e2e -run "TestAuthService_RateLimit_E2E" -config=config/config.precommit.yaml

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
# NOTE: the server has its own image/build path — see grpc-start below
# (podman/trusted-group/grpc_service/). These two build the cronjob image only.
cronjob-build:
	@echo "Building the cronjob image..."
	podman build -f podman/trusted-group/cronjob/Dockerfile -t ubertool-cronjob:latest .

cronjob-push:
	@echo "Pushing cronjob image to registry..."
	podman tag ubertool-cronjob:latest registry.example.com/ubertool-cronjob:latest
	podman push registry.example.com/ubertool-cronjob:latest

# Deployment commands
# NOTE: deploy-cronjob assumes `make db-deploy` has already started ubertool-postgres —
# the cronjob container reaches it externally via host.containers.internal, same as
# grpc-start below (see cronjob/docker-compose.yaml).
deploy-cronjob:
	@echo "Deploying cronjob scheduler..."
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

# Local Podman deployment of the backend server (podman/trusted-group/grpc_service/)
grpc-start:
	@echo "Building and starting the Ubertool backend server container..."
	powershell -NoProfile -ExecutionPolicy Bypass -File podman/trusted-group/grpc_service/install.ps1

grpc-stop:
	@echo "Stopping and removing the Ubertool backend server container..."
	powershell -NoProfile -ExecutionPolicy Bypass -File podman/trusted-group/grpc_service/teardown.ps1

# Switch the already-running grpc-service container to a different config scenario
# (recreates the container from the existing image — no rebuild).
grpc-use-precommit:
	@echo "Switching grpc-service to precommit config (no TLS, no FCM, 2FA bypassed)..."
	powershell -NoProfile -ExecutionPolicy Bypass -File podman/trusted-group/grpc_service/switch-config.ps1 -Mode precommit

grpc-use-uitest:
	@echo "Switching grpc-service to desktop UI-test config (FCM on, 2FA bypassed)..."
	powershell -NoProfile -ExecutionPolicy Bypass -File podman/trusted-group/grpc_service/switch-config.ps1 -Mode uitest

grpc-use-manual:
	@echo "Switching grpc-service to desktop manual-test config (FCM on, live 2FA email)..."
	powershell -NoProfile -ExecutionPolicy Bypass -File podman/trusted-group/grpc_service/switch-config.ps1 -Mode manual

# Local Podman database lifecycle
db-deploy:
	@echo "Building Postgres image and starting container on localhost:5454..."
	powershell -NoProfile -ExecutionPolicy Bypass -File podman/trusted-group/postgres/install.ps1

db-teardown:
	@echo "Stopping Postgres container and removing image and volume..."
	powershell -NoProfile -ExecutionPolicy Bypass -File podman/trusted-group/postgres/teardown.ps1

db-schema-install:
	@echo "Applying SQL schema to running Postgres container..."
	powershell -NoProfile -ExecutionPolicy Bypass -File deploy/local_db/install.ps1

db-schema-teardown:
	@echo "Dropping all tables from local Postgres database..."
	powershell -NoProfile -ExecutionPolicy Bypass -File deploy/local_db/teardown.ps1

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
	powershell -NoProfile -ExecutionPolicy Bypass -File deploy/ec2-mvp/04_deploy.ps1

ec2-reinstall-schema:
	@echo "Wiping and reinstalling RDS schema via EC2..."
	powershell -NoProfile -ExecutionPolicy Bypass -File deploy/ec2-mvp/02_init_db.ps1

ec2-setup-data:
	@echo "Seeding RDS with data from $(DATAFILE)..."
	powershell -NoProfile -ExecutionPolicy Bypass -File deploy/ec2-mvp/05_setup_data.ps1 -DataFile $(DATAFILE)

ec2-wipe-data:
	@echo "Wiping all data from EC2 RDS (schema preserved)..."
	powershell -NoProfile -ExecutionPolicy Bypass -File deploy/ec2-mvp/06_wipe_data.ps1

ec2-reset-data: ec2-wipe-data ec2-setup-data
	@echo "EC2 RDS reset complete."

ec2-use-prod:
	@echo "Switching EC2 to production config (2FA on, real email, S3)..."
	powershell -NoProfile -ExecutionPolicy Bypass -File deploy/ec2-mvp/switch-config.ps1 -Mode prod

ec2-use-uitest:
	@echo "Switching EC2 to UI-test config (2FA bypassed, mock email, S3)..."
	powershell -NoProfile -ExecutionPolicy Bypass -File deploy/ec2-mvp/switch-config.ps1 -Mode uitest

my-ip:
	@powershell -NoProfile -Command "(Invoke-RestMethod https://checkip.amazonaws.com).Trim()"
	@echo "Use the above IP address to whitelist in AWS security groups for EC2 access."
	@echo https://us-west-2.console.aws.amazon.com/ec2/home?region=us-west-2#SecurityGroup:group-id=sg-0604272042828172f

help:
	$(ECHO_BLANK)
	@echo Usage: make [target]
	$(ECHO_BLANK)
	@echo --- Code Generation ---
	@echo   proto-gen                 Generate Go code from .proto files
	$(ECHO_BLANK)
	@echo --- Build ---
	@echo   build                     Build both server and cronjob binaries into bin/
	@echo   build-server              Build the server binary only
	@echo   build-cronjob             Build the cronjob binary only
	@echo   tidy                      Run go mod tidy
	@echo   clean                     Remove bin/ and api/gen/ directories
	$(ECHO_BLANK)
	@echo --- Run microservice [Local Dev] ---
	@echo   run-precommit             Start server for integration and e2e tests [no TLS, no FCM, 2FA bypassed]
	@echo   run-desktop-uitest        Start server for desktop UI automated tests through emulators[FCM on, 2FA bypassed]
	@echo   run-desktop-manual        Start server in DEBUG mode for manual testing from emulators or connected devices
	@echo   run-cronjob-dev           Start cronjob in DEBUG mode
	@echo   run-cronjob-once JOB=name  Run a single named cronjob [e.g. make run-cronjob-once JOB=mark-overdue-rentals]
	@echo   run-test-cron-debug       Start cronjob with pre-commit config and verbose DEBUG logging
	$(ECHO_BLANK)
	@echo --- Tests [local Dev] ---
	@echo   test-unit                 Run unit tests
	@echo   test-integration          Run integration tests [pre-commit config]
	@echo   test-e2e                  Run all e2e tests [pre-commit config]
	@echo   test-e2e-admin-retrieve   Run e2e test: SearchOrganizations admin array check
	@echo   test-e2e-fcm              Run e2e test: push notification service
	@echo   test-precommit            Run full pre-commit suite: unit + integration + e2e
	@echo   test-ext-integration      Run external integration tests against Gmail
	@echo   test-ext-integration-ses  Run external integration tests against AWS SES
	@echo   test-ext-integration-all  Run all external integration tests [Gmail + SES]
	$(ECHO_BLANK)
	@echo --- Tests [EC2] ---
	@echo   test-smoke-ec2            Run smoke tests against live EC2 deployment [TLS and FCM enabled, real 2FA]
	$(ECHO_BLANK)
	@echo --- Local Podman Deployment ---
	@echo   cronjob-build             Build the cronjob image [podman/trusted-group/cronjob/Dockerfile]
	@echo   cronjob-push              Tag and push the cronjob image to the registry
	@echo   deploy-cronjob            Start cronjob scheduler via podman-compose [needs db-deploy first]
	@echo   cronjob-logs              Tail logs from the running cronjob container
	@echo   cronjob-status            Show status of the cronjob container
	@echo   cronjob-restart           Restart the cronjob container
	@echo   grpc-start            Build image and start the backend server container [localhost:50052]
	@echo   grpc-stop             Stop and remove the backend server container and image
	@echo   grpc-use-precommit      Switch grpc-service to precommit config [no rebuild]
	@echo   grpc-use-uitest         Switch grpc-service to desktop UI-test config [no rebuild]
	@echo   grpc-use-manual         Switch grpc-service to desktop manual-test config [no rebuild]
	$(ECHO_BLANK)
	@echo --- Podman Database Lifecycle ---
	@echo   db-deploy                 Build Postgres image and start container on localhost:5454
	@echo   db-teardown               Stop container, remove image and volume
	@echo   db-schema-install         Apply SQL schema to the running Postgres container
	@echo   db-schema-teardown        Drop all tables from the local Postgres database
	$(ECHO_BLANK)
	@echo --- Podman Database Data Setup ---
	@echo   setup-data-local          Populate local test DB from YAML fixture
	@echo   wipe-db-local             Wipe all data from local test DB [schema preserved]
	@echo   reset-db-local            Wipe then repopulate local test DB
	$(ECHO_BLANK)
	@echo --- EC2 Operations ---
	@echo   ec2-deploy                Cross-compile, upload binary, restart service on EC2
	@echo   ec2-use-prod              Switch EC2 to production config and restart service
	@echo   ec2-use-uitest            Switch EC2 to UI-test config and restart service
	$(ECHO_BLANK)
	@echo --- AWS RDS Operations ---
	@echo   ec2-reinstall-schema      Wipe and reinstall the RDS database schema
	@echo   ec2-setup-data            Seed RDS with data [DATAFILE=path, default: user_org.test.yaml]
	@echo   ec2-wipe-data             Wipe all data from EC2 RDS [schema preserved]
	@echo   ec2-reset-data            Wipe then reseed EC2 RDS
	$(ECHO_BLANK)
	@echo --- Utilities ---
	@echo   my-ip                     Print your current public IP [for AWS security group whitelisting]
	@echo   help                      Show this help message
	$(ECHO_BLANK)

