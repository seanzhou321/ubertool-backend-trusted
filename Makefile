.PHONY: proto-gen build run tidy clean

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

run:
	go run ./cmd/server

tidy:
	go mod tidy

clean:
	@if exist "bin" rmdir /s /q bin
	@if exist "api\gen" rmdir /s /q api\gen
