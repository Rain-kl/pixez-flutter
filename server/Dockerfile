# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies for CGO (required by sqlite3)
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with CGO enabled
ENV CGO_ENABLED=1
RUN go build -ldflags="-w -s" -o pixez-sync-server .

# Run stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/pixez-sync-server /app/pixez-sync-server

# Create data directory for SQLite database file
RUN mkdir -p /app/data

EXPOSE 8080

ENV GIN_MODE=release
ENV PIXEZ_DB_PATH=/app/data/pixez-sync.db
ENV PIXEZ_LISTEN_ADDR=:8080

CMD ["/app/pixez-sync-server"]
