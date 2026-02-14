# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Version build arguments
ARG VERSION=dev
ARG BUILD=unknown
ARG GIT_COMMIT=unknown

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build portal binary with version injection
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w \
    -X 'github.com/bobmcallan/vire-portal/internal/config.Version=${VERSION}' \
    -X 'github.com/bobmcallan/vire-portal/internal/config.Build=${BUILD}' \
    -X 'github.com/bobmcallan/vire-portal/internal/config.GitCommit=${GIT_COMMIT}'" \
    -o vire-portal ./cmd/vire-portal

# Runtime stage
FROM alpine:3.21

LABEL org.opencontainers.image.source="https://github.com/bobmcallan/vire-portal"

WORKDIR /app

# Install ca-certificates for HTTPS requests and wget for healthcheck
RUN apk --no-cache add ca-certificates wget

# Copy binary from builder
COPY --from=builder /build/vire-portal .

# Copy templates, config, seed data, and version file
COPY --from=builder /build/pages ./pages
COPY --from=builder /build/docker/vire-portal.toml .
COPY --from=builder /build/data ./seed
COPY .version .

# Create data and logs directories
RUN mkdir -p /app/data /app/logs

# Default to prod mode — prevents accidental dev mode in containers
ENV VIRE_ENV=prod

EXPOSE 4241

HEALTHCHECK NONE

ENTRYPOINT ["./vire-portal"]
