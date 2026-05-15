# syntax=docker/dockerfile:1

# ─── Stage 1: Builder ────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

ARG VERSION=dev

# git is required by some go module downloads.
RUN apk add --no-cache git

WORKDIR /src

# Cache module downloads separately from source so they survive source changes.
COPY go.mod go.sum ./
RUN go mod download

# Copy the full source tree.
COPY . .

# Produce a fully static binary. CGO is not needed because the project uses
# modernc.org/sqlite (pure Go). -trimpath strips local paths from the binary;
# the version string is injected via ldflags.
RUN CGO_ENABLED=0 go build \
      -trimpath \
      -ldflags "-s -w -X main.version=${VERSION}" \
      -o /ping-itor \
      ./cmd/ping-itor

# ─── Stage 2: Runtime ────────────────────────────────────────────────────────
FROM alpine:3.21

# ca-certificates is required for outbound HTTPS checks and SMTP TLS.
RUN apk add --no-cache ca-certificates && \
    addgroup -S appgroup && \
    adduser  -S -G appgroup appuser

# Persist SQLite database outside the container.
VOLUME ["/data"]

COPY --from=builder /ping-itor /ping-itor

# Drop privileges before starting the process.
USER appuser

EXPOSE 8080

CMD ["/ping-itor"]
