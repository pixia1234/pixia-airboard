# syntax=docker/dockerfile:1.7

FROM node:20-bookworm-slim AS frontend-build
WORKDIR /src/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

FROM golang:1.22-bookworm AS backend-build
WORKDIR /src

RUN apt-get update \
    && apt-get install -y --no-install-recommends build-essential ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY web ./web

COPY --from=frontend-build /src/web/static/spa ./web/static/spa

RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/airboard ./cmd/airboard

FROM debian:bookworm-slim AS runtime
WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /app/data

COPY --from=backend-build /out/airboard /usr/local/bin/airboard

ENV AIRBOARD_ADDR=:8080
ENV AIRBOARD_DB_PATH=/app/data/airboard.db

EXPOSE 8080
VOLUME ["/app/data"]

CMD ["airboard"]
