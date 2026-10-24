# Multi-stage Dockerfile for production Go API service
#
# Node comparison:
# Node Dockerfiles copy package.json, install node_modules, build TS, and then bundle
# hundreds of megabytes of node runtime + dependencies into a 300MB+ image.
# In Go, the compiler produces a single, self-contained static binary (zero runtime dependencies).
# The final runtime image is typically under 20MB!

# --- Build Stage ---
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Cache dependencies layer
COPY go.mod go.sum ./
RUN go mod download

# Copy application source code
COPY . .

# Build statically linked binary with stripped debug symbols (-w -s)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /bin/api ./cmd/api

# --- Final Production Stage ---
FROM alpine:3.20

# Install ca-certificates for outbound TLS/HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create a non-privileged user and group for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /home/appuser

# Copy static binary from builder stage
COPY --from=builder /bin/api /bin/api

# Switch to non-root user
USER appuser

EXPOSE 8080

ENTRYPOINT ["/bin/api"]
