# whoami

A minimal, high-performance API service written in Go that returns the visitor's IP address and User-Agent as JSON. CLI clients (curl, wget, HTTPie) receive ANSI-colored, column-aligned output.

## Response

**Browser / fetch / programmatic clients** → `application/json`

```json
{
  "ip": "203.17.85.252",
  "ua": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 ..."
}
```

**curl / wget / HTTPie** → `text/plain` with ANSI colors

```
{
  "ip"  : "203.17.85.252",
  "ua"  : "curl/8.7.1"
}
```

## Features

- **Reverse proxy support** — reads `CF-Connecting-IP`, `X-Forwarded-For`, `X-Real-IP` in priority order
- **CORS enabled** — accessible from any origin via browser `fetch()`
- **ANSI colored output** — syntax-highlighted JSON for CLI clients
- **UA sanitization** — strips control characters, ANSI injection, and limits length
- **Zero dependencies** — only Go standard library
- **Docker health check** — built-in `HEALTHCHECK` via `/health` endpoint

## Run

```bash
# Direct
go run main.go

# Custom port
PORT=3000 go run main.go

# Build & run
go build -o whoami .
./whoami
```

## Docker

```bash
docker run -p 8080:8080 ghcr.io/fransallen/whoami
```

Or build locally:

```bash
docker build -t whoami .
docker run -p 8080:8080 whoami
```

Health check status:

```bash
docker inspect --format='{{.State.Health.Status}}' <container>
```

## Reverse Proxy (Nginx)

```nginx
server {
    listen 80;
    server_name whoami.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header Host $host;
    }
}
```

## Test

```bash
# Local
curl http://localhost:8080

# With simulated proxy headers
curl -H "X-Forwarded-For: 203.0.113.50" http://localhost:8080

# CORS preflight
curl -X OPTIONS -H "Origin: https://example.com" -v http://localhost:8080

# Health check
curl http://localhost:8080/health
```

## License

MIT
