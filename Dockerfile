ARG TAG=dev

# Development stage for better debugging
FROM golang:1.23.3-alpine AS development
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build stage with minimal dependencies
FROM development AS builder

# Install only necessary build dependencies
RUN apk add --no-cache ca-certificates tzdata gcc musl-dev cosign

# Build the binary with hardening flags
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static" -X main.version=${TAG}' \
    -tags netgo,osusergo \
    -o vault-migrate \
    cmd/vault-migrations/main.go

# Sign the binary if SIGN_BINARY is set
ARG SIGN_BINARY=false
RUN if [ "$SIGN_BINARY" = "true" ]; then \
    --mount=type=secret,id=cosign_private_key \
    --mount=type=secret,id=cosign_public_key \
    COSIGN_PASSWORD="" \
    COSIGN_EXPERIMENTAL=1 cosign sign --key /run/secrets/cosign_private_key /app/vault-migrate; \
    fi

# Runtime stage using distroless
FROM gcr.io/distroless/cc-debian12@sha256:899570acf85a1f1362862a9ea4d9e7b1827cb5c62043ba5b170b21de89618608 AS runtime

# Set environment variables
ENV VAULT_ADDR="" \
    VAULT_TOKEN="" \
    VAULT_NAMESPACE="" \
    VAULT_SKIP_VERIFY="false" \
    LOG_LEVEL="info" \
    LOG_FORMAT="json" \
    TZ=Etc/UTC

# Create necessary directories and set workdir
WORKDIR /app

# Copy binary and assets with proper permissions
COPY --from=builder --chown=nonroot:nonroot /app/vault-migrate /bin/vault-migrate
COPY --from=builder --chown=nonroot:nonroot /app/migrations /migrations

# Copy and verify signature files if binary was signed
ARG SIGN_BINARY=false
RUN if [ "$SIGN_BINARY" = "true" ]; then \
    COPY --from=builder --chown=nonroot:nonroot /app/vault-migrate.sig /app/vault-migrate.sig \
    COPY cosign.pub /etc/vault-migrations/cosign.pub \
    ["/bin/vault-migrate", "verify", "--key", "/etc/vault-migrations/cosign.pub", "/bin/vault-migrate"]; \
    fi

# Use nonroot user for security
USER nonroot:nonroot

# Set the entrypoint and default command
ENTRYPOINT ["/bin/vault-migrate"]
CMD ["--help"]
