.PHONY: proto-gen build run tidy clean test-unit test-integration test-e2e

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
# go build -o bin/server.exe ./cmd/server
	go build ./cmd/server

run-dev:
	go run ./cmd/server -config=config/config.dev.yaml

run-test:
	@echo "Starting server in DEBUG mode for testing..."
	go run ./cmd/server -config=config/config.test.yaml

run-test-debug:
	@echo "Starting server in DEBUG mode with verbose output..."
	set LOG_LEVEL=debug&& go run ./cmd/server -config=config/config.test.yaml

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
