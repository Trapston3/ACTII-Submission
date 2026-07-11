# Stage 1: Build the statically-linked Go binary
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Cache module resolution
COPY go.mod go.sum ./
RUN go mod download

# Copy source files
COPY . .

# Compile statically-linked linux binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /agent ./cmd/agent

# Stage 2: Ultra-minimal production scratch image
FROM scratch

# Crucial: copy CA root certificates from builder stage to prevent TLS handshake failures
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy binary
COPY --from=builder /agent /agent

ENTRYPOINT ["/agent"]
