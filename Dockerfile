FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o app .

FROM alpine:3.20
RUN apk add --no-cache curl
COPY --from=builder /app/app /app
COPY public/ /public/
COPY data/ /data/
EXPOSE 8080
HEALTHCHECK \
    --interval=5m \
    --timeout=10s \
    --retries=3 \
    --start-period=5s \
    CMD curl --fail "http://localhost:8080/health" || exit 1
ENTRYPOINT ["/app"]
