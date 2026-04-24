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
)

// ANSI color codes
const (
	reset   = "\033[39m"
	cyan    = "\033[96m" // keys
	green   = "\033[92m" // string values
	magenta = "\033[95m" // number values
	blue    = "\033[94m" // bool values
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
	// Find the longest key for padding
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

// formatJSON produces standard pretty-printed JSON for non-CLI clients.
func formatJSON(resp IPResponse) ([]byte, error) {
	data := map[string]string{
		"ip": resp.IP,
		"ua": resp.UserAgent,
	}
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// buildFields constructs the response fields from the request.
func buildFields(resp IPResponse) []field {
	return []field{
		{"ip", fmt.Sprintf("%q", resp.IP), "string"},
		{"ua", fmt.Sprintf("%q", resp.UserAgent), "string"},
	}
}

type IPResponse struct {
	IP        string
	UserAgent string
}

// getClientIP extracts the real client IP from reverse proxy headers,
// falling back to the direct connection address.
// Priority: X-Forwarded-For → X-Real-IP → CF-Connecting-IP → RemoteAddr
func getClientIP(r *http.Request) string {
	// Cloudflare
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return sanitizeIP(ip)
	}

	// X-Forwarded-For: client, proxy1, proxy2
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First IP in the chain is the original client
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

	// Strip ANSI escape sequences (e.g. \033[31m)
	clean := strings.Map(func(r rune) rune {
		if r == '\033' {
			return -1 // drop ESC
		}
		// Drop all other control chars (0x00-0x1F, 0x7F) except space
		if r < 0x20 || r == 0x7F {
			return -1
		}
		return r
	}, ua)

	// Remove any leftover ANSI CSI fragments like [31m
	for {
		idx := strings.Index(clean, "[")
		if idx == -1 {
			break
		}
		// Look for pattern: [ + digits/semicolons + letter
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

// corsMiddleware adds CORS headers to every response.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Handle preflight
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

	resp := IPResponse{
		IP:        getClientIP(r),
		UserAgent: sanitizeUA(r.UserAgent()),
	}

	w.Header().Set("Cache-Control", "no-store")

	if isCLI(r.UserAgent()) {
		// CLI clients: ANSI colored, column-aligned text/plain
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fields := buildFields(resp)
		w.Write([]byte(formatColorAligned(fields)))
	} else {
		// Everything else: standard pretty-printed JSON
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

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIP)
	mux.HandleFunc("/health", handleHealth)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Printf("whoami listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
