# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a statically-linked binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server .

# ── Stage 2: Runtime ─────────────────────────────────────────────────────────
FROM alpine:3.21

# Add CA certs (needed for outbound HTTPS calls to the API)
RUN apk add --no-cache ca-certificates

# Non-root user for security
RUN adduser -D -h /app appuser
WORKDIR /app

# Copy the binary and runtime assets from the builder
COPY --from=builder /app/server .
COPY templates/ ./templates/
COPY static/     ./static/
COPY prev-website/ ./prev-website/

# Switch to non-root user
USER appuser

EXPOSE 8080

ENV GIN_MODE=release
ENV PORT=8080

ENTRYPOINT ["./server"]
