# Dockerfile for the Ubertool Trusted Backend gRPC server (cmd/server only).
# Build context must be the repo root, e.g.:
#   podman build -t ubertool-service -f podman/trusted-group/grpc_service/Dockerfile .
# (install.ps1 in this directory does this for you.)

# Stage 1: build the server binary
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

# Requires api/gen/v1 to already exist (gitignored; run `make proto-gen` first).
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /bin/server ./cmd/server

# Stage 2: minimal runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /bin/server /app/server

RUN chown -R appuser:appuser /app

USER appuser

# gRPC port + mock-storage HTTP port (see internal/config: server.port / storage.base_url)
EXPOSE 50052 50053

ENTRYPOINT ["/app/server"]
CMD ["-config=/app/config/config.precommit.yaml"]
