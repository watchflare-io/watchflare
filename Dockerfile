# syntax=docker/dockerfile:1

# Stage 1: Build frontend (runs natively on the build host, output is arch-independent)
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY frontend/ .
RUN npm run build

# Stage 2: Build the Hub (Go backend + embedded frontend, cross-compiled, no QEMU emulation)
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS backend-builder
ARG TARGETOS TARGETARCH
WORKDIR /app
COPY shared/ ./shared/
COPY backend/go.mod backend/go.sum ./backend/
RUN --mount=type=cache,target=/go/pkg/mod cd backend && go mod download
COPY backend/ ./backend/
COPY --from=frontend-builder /app/frontend/build ./backend/frontend/dist
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    cd backend && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -tags embed_frontend -o watchflare-hub

# Stage 3: Prepare data directories (arch-independent)
FROM --platform=$BUILDPLATFORM alpine AS data-init
RUN mkdir -p /var/lib/watchflare && chmod 750 /var/lib/watchflare

# Stage 4: Runtime (FROM scratch)
FROM scratch
LABEL org.opencontainers.image.source="https://github.com/watchflare-io/watchflare"
LABEL org.opencontainers.image.description="Watchflare Host Monitoring"
COPY --from=backend-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=backend-builder /app/backend/watchflare-hub /watchflare-hub
COPY --from=data-init /var/lib/watchflare /var/lib/watchflare
USER 65532
VOLUME ["/var/lib/watchflare"]
EXPOSE 8080 50051
ENTRYPOINT ["/watchflare-hub"]
