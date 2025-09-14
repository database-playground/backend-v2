# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS builder

ENV GOEXPERIMENT=greenteagc

WORKDIR /src

# Copy dependency files first for better layer caching
COPY go.mod go.sum ./

# Download dependencies with cache mount for better performance
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY . .

# Generate GraphQL code with cache mount
RUN --mount=type=cache,target=/go/pkg/mod \
    go generate .

# Build the applications with cache mount for build cache
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o backend ./cmd/backend && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o admin-cli ./cmd/admin-cli

# Final stage with minimal runtime image
FROM alpine:latest AS runtime

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user for security
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

WORKDIR /app

# Copy binaries from builder stage
COPY --from=builder --chown=appuser:appgroup /src/backend /app/backend
COPY --from=builder --chown=appuser:appgroup /src/admin-cli /app/admin-cli

# Switch to non-root user
USER appuser

EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080 || exit 1

CMD ["/app/backend"]
