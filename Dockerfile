# ---- 前端构建 ----
FROM node:20-bookworm-slim AS frontend-builder
WORKDIR /web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- 后端构建 ----
FROM golang:1.26-alpine AS backend-builder
WORKDIR /src
COPY server/go.* ./
RUN go mod download
COPY server/ ./
# 纯 Go SQLite,无需 CGO
ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w" -o /out/taskpanel-server .

# ---- 运行时 ----
FROM alpine:3.20
# py3-pip 供依赖管理(Python/pip3)使用;nodejs 自带 npm
RUN apk add --no-cache ca-certificates tzdata bash python3 py3-pip nodejs git curl \
    && addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=backend-builder /out/taskpanel-server ./
COPY --from=frontend-builder /web/dist ./web
COPY server/config.yaml ./
RUN mkdir -p /app/data/scripts /app/data/logs \
    && chown -R app:app /app
USER app
ENV TZ=Asia/Shanghai \
    PANEL_PORT=5700 \
    SERVER_PORT=5700 \
    DATA_DIR=/app/data \
    DB_PATH=/app/data/taskpanel.db \
    WEB_DIR=/app/web
EXPOSE 5700
HEALTHCHECK --interval=30s --timeout=3s CMD curl -fs http://127.0.0.1:${PANEL_PORT}/api/v1/health || exit 1
ENTRYPOINT ["./taskpanel-server"]
