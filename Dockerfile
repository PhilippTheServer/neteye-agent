# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /neteye-agent ./cmd/agent

# ── Runtime stage ─────────────────────────────────────────────────────────────
# The agent needs host networking to read real interface data,
# so it runs with --network=host in docker-compose / k8s daemonset.
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /neteye-agent /neteye-agent

ENTRYPOINT ["/neteye-agent"]
