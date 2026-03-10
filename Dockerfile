# ── 1. Build frontend ────────────────────────────────────────────────────────
FROM node:22-alpine AS web
WORKDIR /app

RUN corepack enable

COPY package.json pnpm-workspace.yaml ./
COPY web/package.json web/package.json
RUN pnpm install --no-frozen-lockfile

COPY web ./web
RUN pnpm build:web

# ── 2. Build API ──────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS api
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /aimmod-hub ./cmd/aimmod-hub

# ── 3. Final image ────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=api  /aimmod-hub      ./aimmod-hub
COPY --from=web  /app/web/dist    ./web/dist
COPY docker-entrypoint.sh         /docker-entrypoint.sh

RUN chmod +x /docker-entrypoint.sh

ENV AIMMOD_HUB_STATIC_DIR=/app/web/dist

EXPOSE 8080

ENTRYPOINT ["/docker-entrypoint.sh"]
