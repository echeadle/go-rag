# ── Stage 1: build ────────────────────────────────────────────────────────────
# golang:1.26-alpine gives us a full Go toolchain on a tiny Alpine base.
# CGO_ENABLED=0 produces a fully static binary — no libc dependency at runtime.
FROM golang:1.26-alpine AS builder

WORKDIR /build

# Download modules first so Docker can cache this layer independently of source.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o go-rag ./cmd/rag/

# ── Stage 2: runtime ──────────────────────────────────────────────────────────
# alpine:3.21 is small (~7 MB) but still has a shell and package manager,
# which is useful for debugging in early development (Steps 1–3).
# Step 4 replaces this with distroless for a minimal attack surface.
FROM alpine:3.21

# ca-certificates is required for any HTTPS call (OpenAI API, remote embedders).
# Without it the Go TLS stack fails with "x509: certificate signed by unknown authority".
RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /build/go-rag .

EXPOSE 8080

CMD ["./go-rag"]
