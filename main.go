package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/oschwald/geoip2-golang"
)

// ANSI color codes
const (
	reset   = "\033[39m"
	cyan    = "\033[96m" // keys
	green   = "\033[92m" // string values
	magenta = "\033[95m" // number values
	blue    = "\033[94m" // bool values
)

var (
	cityDB *geoip2.Reader
	asnDB  *geoip2.Reader
)

// field represents a key-value pair for aligned JSON output.
type field struct {
	key string
	val string
	typ string // "string", "number", "bool"
}

// isCLI returns true if the User-Agent indicates a CLI client
// (curl, wget, HTTPie) that should receive text/plain with ANSI colors.
func isCLI(ua string) bool {
	lower := strings.ToLower(ua)
	return strings.HasPrefix(lower, "curl/") ||
		strings.HasPrefix(lower, "wget/") ||
		strings.HasPrefix(lower, "httpie/")
}

// formatColorAligned produces column-aligned JSON with ANSI colors for CLI clients.
func formatColorAligned(fields []field) string {
	maxLen := 0
	for _, f := range fields {
		if len(f.key) > maxLen {
			maxLen = len(f.key)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s{\n", reset)

	for i, f := range fields {
		padding := strings.Repeat(" ", maxLen-len(f.key))
		comma := ","
		if i == len(fields)-1 {
			comma = ""
		}

		valColor := green
		switch f.typ {
		case "number":
			valColor = magenta
		case "bool":
			valColor = blue
		}
		fmt.Fprintf(&b, "  %s\"%s\"%s%s : %s%s%s%s\n",
			cyan, f.key, reset, padding,
			valColor, f.val, reset, comma)
	}

	b.WriteString("}\n")
	return b.String()
}

type IPResponse struct {
	IP            string
	UserAgent     string
	Continent     string
	ContinentCode string
	Country       string
	CountryCode   string
	City          string
	Timezone      string
	Zip           string
	Lat           float64
	Lon           float64
	Rad           uint16
	Anycast       bool
	Org           string
	AS            string
	DNS           string
}

type jsonResponse struct {
	IP            string  `json:"ip"`
	UserAgent     string  `json:"ua"`
	Continent     string  `json:"continent"`
	ContinentCode string  `json:"continent_code"`
	Country       string  `json:"country"`
	CountryCode   string  `json:"country_code"`
	City          string  `json:"city"`
	Timezone      string  `json:"timezone"`
	Zip           string  `json:"zip"`
	Lat           float64 `json:"lat"`
	Lon           float64 `json:"lon"`
	Rad           uint16  `json:"rad"`
	Anycast       bool    `json:"anycast"`
	Org           string  `json:"org"`
	AS            string  `json:"as"`
	DNS           string  `json:"dns"`
}

func formatJSON(resp IPResponse) ([]byte, error) {
	data := jsonResponse{
		IP:            resp.IP,
		UserAgent:     resp.UserAgent,
		Continent:     resp.Continent,
		ContinentCode: resp.ContinentCode,
		Country:       resp.Country,
		CountryCode:   resp.CountryCode,
		City:          resp.City,
		Timezone:      resp.Timezone,
		Zip:           resp.Zip,
		Lat:           resp.Lat,
		Lon:           resp.Lon,
		Rad:           resp.Rad,
		Anycast:       resp.Anycast,
		Org:           resp.Org,
		AS:            resp.AS,
		DNS:           resp.DNS,
	}
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func buildFields(resp IPResponse) []field {
	return []field{
		{"ip", fmt.Sprintf("%q", resp.IP), "string"},
		{"ua", fmt.Sprintf("%q", resp.UserAgent), "string"},
		{"continent", fmt.Sprintf("%q", resp.Continent), "string"},
		{"continent_code", fmt.Sprintf("%q", resp.ContinentCode), "string"},
		{"country", fmt.Sprintf("%q", resp.Country), "string"},
		{"country_code", fmt.Sprintf("%q", resp.CountryCode), "string"},
		{"city", fmt.Sprintf("%q", resp.City), "string"},
		{"timezone", fmt.Sprintf("%q", resp.Timezone), "string"},
		{"zip", fmt.Sprintf("%q", resp.Zip), "string"},
		{"lat", fmt.Sprintf("%g", resp.Lat), "number"},
		{"lon", fmt.Sprintf("%g", resp.Lon), "number"},
		{"rad", fmt.Sprintf("%d", resp.Rad), "number"},
		{"anycast", fmt.Sprintf("%t", resp.Anycast), "bool"},
		{"org", fmt.Sprintf("%q", resp.Org), "string"},
		{"as", fmt.Sprintf("%q", resp.AS), "string"},
		{"dns", fmt.Sprintf("%q", resp.DNS), "string"},
	}
}

// getClientIP extracts the real client IP from reverse proxy headers,
// falling back to the direct connection address.
// Priority: CF-Connecting-IP → X-Forwarded-For → X-Real-IP → RemoteAddr
func getClientIP(r *http.Request) string {
	// Cloudflare
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return sanitizeIP(ip)
	}

	// X-Forwarded-For: client, proxy1, proxy2
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i != -1 {
			return sanitizeIP(xff[:i])
		}
		return sanitizeIP(xff)
	}

	// Nginx typically sets this
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return sanitizeIP(ip)
	}

	// Direct connection fallback — strip port
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func sanitizeIP(ip string) string {
	return strings.TrimSpace(ip)
}

// sanitizeUA strips control characters, ANSI escape sequences, and
// truncates to a safe length to prevent injection or abuse.
func sanitizeUA(ua string) string {
	const maxLen = 512

	clean := strings.Map(func(r rune) rune {
		if r == '\033' {
			return -1
		}
		if r < 0x20 || r == 0x7F {
			return -1
		}
		return r
	}, ua)

	for {
		idx := strings.Index(clean, "[")
		if idx == -1 {
			break
		}
		end := idx + 1
		for end < len(clean) && (clean[end] >= '0' && clean[end] <= '9' || clean[end] == ';') {
			end++
		}
		if end < len(clean) && end > idx+1 && clean[end] >= 'A' && clean[end] <= 'z' {
			clean = clean[:idx] + clean[end+1:]
		} else {
			break
		}
	}

	if len(clean) > maxLen {
		clean = clean[:maxLen]
	}

	return strings.TrimSpace(clean)
}

type geoResult struct {
	Continent     string
	ContinentCode string
	Country       string
	CountryCode   string
	City          string
	Timezone      string
	Zip           string
	Lat           float64
	Lon           float64
	Rad           uint16
	Anycast       bool
	Org           string
	AS            string
	DNS           string
}

func lookupGeo(ipStr string) geoResult {
	var g geoResult

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return g
	}

	if cityDB != nil {
		if record, err := cityDB.City(ip); err == nil {
			if name, ok := record.Continent.Names["en"]; ok {
				g.Continent = name
			}
			g.ContinentCode = record.Continent.Code
			if name, ok := record.Country.Names["en"]; ok {
				g.Country = name
			}
			g.CountryCode = record.Country.IsoCode
			if name, ok := record.City.Names["en"]; ok {
				g.City = name
			}
			g.Timezone = record.Location.TimeZone
			g.Zip = record.Postal.Code
			g.Lat = record.Location.Latitude
			g.Lon = record.Location.Longitude
			g.Rad = record.Location.AccuracyRadius
			g.Anycast = record.Traits.IsAnycast
		}
	}

	if asnDB != nil {
		if record, err := asnDB.ASN(ip); err == nil {
			g.AS = fmt.Sprintf("AS%d", record.AutonomousSystemNumber)
			g.Org = record.AutonomousSystemOrganization
		}
	}

	if names, err := net.LookupAddr(ipStr); err == nil && len(names) > 0 {
		g.DNS = strings.TrimSuffix(names[0], ".")
	}

	return g
}

// corsMiddleware adds CORS headers to every response.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func handleIP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	ipStr := getClientIP(r)
	geo := lookupGeo(ipStr)

	resp := IPResponse{
		IP:            ipStr,
		UserAgent:     sanitizeUA(r.UserAgent()),
		Continent:     geo.Continent,
		ContinentCode: geo.ContinentCode,
		Country:       geo.Country,
		CountryCode:   geo.CountryCode,
		City:          geo.City,
		Timezone:      geo.Timezone,
		Zip:           geo.Zip,
		Lat:           geo.Lat,
		Lon:           geo.Lon,
		Rad:           geo.Rad,
		Anycast:       geo.Anycast,
		Org:           geo.Org,
		AS:            geo.AS,
		DNS:           geo.DNS,
	}

	w.Header().Set("Cache-Control", "private, max-age=0, no-store, no-cache, must-revalidate")

	if isCLI(r.UserAgent()) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fields := buildFields(resp)
		w.Write([]byte(formatColorAligned(fields)))
	} else {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		data, err := formatJSON(resp)
		if err != nil {
			log.Printf("encode error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		w.Write(data)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write([]byte(`{"status":"ok"}`))
}

func handleFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, "public/favicon.ico")
}

func openDB(envKey, defaultPath string) *geoip2.Reader {
	path := os.Getenv(envKey)
	if path == "" {
		path = defaultPath
	}
	db, err := geoip2.Open(path)
	if err != nil {
		log.Printf("geoip: skipping %s (%s): %v", envKey, path, err)
		return nil
	}
	log.Printf("geoip: loaded %s from %s", envKey, path)
	return db
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	cityDB = openDB("GEOIP_CITY_DB", "data/GeoLite2-City.mmdb")
	asnDB = openDB("GEOIP_ASN_DB", "data/GeoLite2-ASN.mmdb")

	if cityDB != nil {
		defer cityDB.Close()
	}
	if asnDB != nil {
		defer asnDB.Close()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIP)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/favicon.ico", handleFavicon)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Printf("whoami listening on http://localhost:%s", port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
