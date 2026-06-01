# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.23-alpine AS builder

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG GOPROXY=https://goproxy.cn,https://proxy.golang.org,direct
ENV HTTP_PROXY=${HTTP_PROXY} HTTPS_PROXY=${HTTPS_PROXY} GOPROXY=${GOPROXY}

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown

RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/posthub-api ./cmd/api

RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/posthub-worker ./cmd/worker

# ---- Runtime stage ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 1000 app

COPY --from=builder /out/posthub-api    /usr/local/bin/posthub-api
COPY --from=builder /out/posthub-worker /usr/local/bin/posthub-worker

USER app

EXPOSE 8080

ENTRYPOINT ["posthub-api"]
