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
# distroless/static contains only CA certs, tzdata, and the binary — no shell,
# no package manager, no utilities. An attacker who breaks out of the app
# process has nothing to work with.
# CA certificates are bundled in the image, so no apk/apt install needed.
# The built-in nonroot user (UID 65532) replaces the alpine adduser approach —
# distroless has no shell to run adduser in.
FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=builder /build/go-rag .
# The prompts directory holds runtime config (system prompt, etc.).
# It is not embedded in the binary, so it must be copied explicitly.
COPY --from=builder /build/prompts ./prompts

USER nonroot:nonroot

EXPOSE 8080

CMD ["./go-rag"]
