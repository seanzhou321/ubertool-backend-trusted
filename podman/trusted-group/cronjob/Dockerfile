# Dockerfile for the Ubertool cronjob scheduler (cmd/cronjob only).
# The server has its own dedicated image — see podman/trusted-group/grpc_service/Dockerfile.
# Build context must be the repo root, e.g.:
#   podman build -t ubertool-cronjob -f podman/trusted-group/cronjob/Dockerfile .

# Stage 1: build the cronjob binary
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

# Requires api/gen/v1 to already exist (gitignored; run `make proto-gen` first).
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /bin/cronjob ./cmd/cronjob

# Stage 2: minimal runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /bin/cronjob /app/cronjob

RUN chown -R appuser:appuser /app

USER appuser

ENTRYPOINT ["/app/cronjob"]
CMD ["-config=/app/config/config.yaml"]
