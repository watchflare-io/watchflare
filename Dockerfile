# Stage 1: Build frontend
FROM node:24-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# Stage 2: Build backend (with embedded frontend)
FROM golang:1.26-alpine AS backend-builder
WORKDIR /app
COPY shared/ ./shared/
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download
COPY backend/ ./backend/
COPY --from=frontend-builder /app/frontend/build ./backend/frontend/dist
RUN cd backend && CGO_ENABLED=0 go build -tags embed_frontend -o watchflare-app

# Stage 3: Prepare data directories
FROM alpine AS data-init
RUN mkdir -p /var/lib/watchflare && chmod 750 /var/lib/watchflare

# Stage 4: Runtime (FROM scratch)
FROM scratch
LABEL org.opencontainers.image.source="https://github.com/watchflare-io/watchflare"
LABEL org.opencontainers.image.description="Watchflare Host Monitoring"
COPY --from=backend-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=backend-builder /app/backend/watchflare-app /watchflare-app
COPY --from=data-init /var/lib/watchflare /var/lib/watchflare
USER 65532
VOLUME ["/var/lib/watchflare"]
EXPOSE 8080 50051
ENTRYPOINT ["/watchflare-app"]
