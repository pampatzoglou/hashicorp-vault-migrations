# Specify the Go version
ARG GO_VERSION=1.23.3
FROM golang:${GO_VERSION}-alpine AS development

WORKDIR /app
COPY . .

# Download dependencies
RUN go mod download

# Command to run the application in development mode
CMD ["go", "run", "cmd/vault-migrations/main.go"]

FROM development AS build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o vault-migrate cmd/vault-migrations/main.go

# FROM gcr.io/distroless/cc-debian12@sha256:899570acf85a1f1362862a9ea4d9e7b1827cb5c62043ba5b170b21de89618608 AS runtime
FROM golang:${GO_VERSION}-alpine AS runtime

WORKDIR /app
COPY --from=build --chown=nobody:nogroup /app/vault-migrate /bin/vault-migrate
COPY --from=development --chown=nobody:nogroup /app/migrations migrations/

# Use a non-root user for security
USER nobody

# CMD ["vault-migrate"]