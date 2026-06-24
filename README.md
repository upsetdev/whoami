# whoami

**Identify what the internet knows about you: IP address, location, network, and browser details.**

`whoami.upset.dev` is a high-performance IP API service written in Go. It returns IP address, geolocation, network, and browser information as JSON. CLI clients (curl, wget, HTTPie) receive ANSI-colored, column-aligned output.

## Response

**Browser / fetch / programmatic clients** → `application/json`

```json
{
  "ip": "203.0.113.50",
  "ua": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 ...",
  "continent": "North America",
  "continent_code": "NA",
  "country": "United States",
  "country_code": "US",
  "city": "New York",
  "timezone": "America/New_York",
  "zip": "10001",
  "lat": 40.7128,
  "lon": -74.006,
  "rad": 20,
  "anycast": false,
  "org": "Example ISP",
  "as": "AS64496",
  "dns": "203-0-113-50.example.com"
}
```

**curl / wget / HTTPie** → `text/plain` with ANSI colors

```
{
  "ip"             : "203.0.113.50",
  "ua"             : "curl/8.7.1",
  "continent"      : "North America",
  "continent_code" : "NA",
  "country"        : "United States",
  "country_code"   : "US",
  "city"           : "New York",
  "timezone"       : "America/New_York",
  "zip"            : "10001",
  "lat"            : 40.7128,
  "lon"            : -74.006,
  "rad"            : 20,
  "anycast"        : false,
  "org"            : "Example ISP",
  "as"             : "AS64496",
  "dns"            : "203-0-113-50.example.com"
}
```

## Fields

| Field            | Description                                            |
| ---------------- | ------------------------------------------------------ |
| `ip`             | Client IP address                                      |
| `ua`             | User-Agent string                                      |
| `continent`      | Continent name                                         |
| `continent_code` | Two-letter continent code (AF, AN, AS, EU, NA, OC, SA) |
| `country`        | Country name                                           |
| `country_code`   | ISO 3166-1 two-letter country code                     |
| `city`           | City name                                              |
| `timezone`       | IANA timezone identifier                               |
| `zip`            | Postal/ZIP code                                        |
| `lat`            | Latitude (WGS84)                                       |
| `lon`            | Longitude (WGS84)                                      |
| `rad`            | Accuracy radius in kilometers                          |
| `anycast`        | Whether the IP is an anycast address                   |
| `org`            | Organization / ISP name (from ASN)                     |
| `as`             | Autonomous System number (e.g. `AS1221`)               |
| `dns`            | Reverse DNS hostname                                   |

Geolocation data is sourced from [MaxMind GeoLite2](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) (City + ASN databases).

## Use Cases

- **Self-hosted IP lookup**: run your own alternative to ipinfo.io or ip-api.com, with no rate limits or third-party dependencies.
- **Network debugging**: quickly verify what IP and headers a server sees, useful when troubleshooting proxies, VPNs, or CDN configurations.
- **Frontend geolocation**: use the CORS-enabled JSON API directly from a browser to localize content, set timezones, or show region-specific UI without a backend.
- **CI/CD diagnostics**: check the egress IP of a runner or deployment environment from a pipeline script.
- **Privacy check**: confirm whether a VPN or proxy is working as expected by comparing the returned IP and ASN against the expected exit node.

## Features

- **IP geolocation**: country, city, coordinates, timezone, ASN via MaxMind GeoLite2.
- **Arbitrary IP lookup**: query any IP via the path (`/8.8.8.8`), not just the caller's.
- **Reverse DNS**: resolves PTR record for the client IP.
- **Reverse proxy support**: reads `CF-Connecting-IP`, `X-Forwarded-For`, `X-Real-IP` in priority order.
- **CORS enabled**: accessible from any origin via browser `fetch()`.
- **ANSI colored output**: syntax-highlighted, column-aligned output for CLI clients.
- **UA sanitization**: strips control characters, ANSI injection, and limits length.
- **Health check**: via `/health` endpoint.

## GeoIP Databases

Geolocation requires the [MaxMind GeoLite2](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) databases. Place them at:

```
./data/GeoLite2-City.mmdb
./data/GeoLite2-ASN.mmdb
```

Override the paths with environment variables:

```bash
GEOIP_CITY_DB=/path/to/GeoLite2-City.mmdb
GEOIP_ASN_DB=/path/to/GeoLite2-ASN.mmdb
```

If a database is missing, the service still runs: geolocation fields are omitted for that source.

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
docker run -p 8080:8080 \
  -v $HOME/geoip:/root/geoip:ro \
  ghcr.io/fransallen/whoami
```

Or build locally:

```bash
docker build -t whoami .
docker run -p 8080:8080 \
  -v $HOME/geoip:/root/geoip:ro \
  whoami
```

Health check status:

```bash
docker inspect --format='{{.State.Health.Status}}' <container>
```

## Endpoints

| Endpoint      | Description                                            |
| ------------- | ----------------------------------------------------- |
| `GET /`       | IP info + geolocation for the client                  |
| `GET /{ip}`   | IP info + geolocation for a specific IP (e.g. `/8.8.8.8`) |
| `GET /health` | Health check: returns `{"status":"ok"}`               |

Passing an invalid IP in the path returns `400 {"error":"invalid ip"}`.

## Test

```bash
# Local
curl http://localhost:8080

# Look up a specific IP
curl http://localhost:8080/8.8.8.8

# With simulated proxy headers
curl -H "X-Forwarded-For: 203.0.113.50" http://localhost:8080

# Health check
curl http://localhost:8080/health
```

## License

MIT
