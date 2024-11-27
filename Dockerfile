# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.23.3-alpine AS builder

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with hardening flags
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static"' \
    -tags netgo,osusergo \
    -o vault-migrate \
    cmd/vault-migrations/main.go


# # Runtime stage
# FROM gcr.io/distroless/cc-debian12@sha256:899570acf85a1f1362862a9ea4d9e7b1827cb5c62043ba5b170b21de89618608 AS runtime
FROM builder AS runtime
# FROM gcr.io/distroless/static:nonroot AS runtime

# Copy the binary
COPY --from=builder --chown=nobody:nogroup /app/vault-migrate /bin/vault-migrate
# Copy migrations directory
COPY --from=builder --chown=nobody:nogroup /app/migrations /migrations

USER nobody

# Set environment variables
ENV TZ=Etc/UTC \
    APP_USER=nonroot

# ENTRYPOINT ["/bin/vault-migrate"]
