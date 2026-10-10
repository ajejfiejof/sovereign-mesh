package antiscrape

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Zero-width Unicode markers for invisible text watermarking
const (
	ZWSP = "\u200B" // Zero-Width Space
	ZWNJ = "\u200C" // Zero-Width Non-Joiner
	ZWJ  = "\u200D" // Zero-Width Joiner
)

type ClientState struct {
	Tokens      float64
	LastRefill  time.Time
	IsBot       bool
	BanUntil    time.Time
	CanaryHits  int
	RequestHits int
}

type Shield struct {
	mu           sync.Mutex
	clients      map[string]*ClientState
	canaryTokens map[string]bool
	rateLimit    float64 // tokens per second
	bucketCap    float64 // max token capacity
	tarpitDelay  time.Duration
}

func NewShield(ratePerSec, bucketCapacity float64) *Shield {
	s := &Shield{
		clients:      make(map[string]*ClientState),
		canaryTokens: make(map[string]bool),
		rateLimit:    ratePerSec,
		bucketCap:    bucketCapacity,
		tarpitDelay:  100 * time.Millisecond,
	}

	// Generate initial canary honeypot tokens
	for range 3 {
		s.generateCanaryToken()
	}

	return s
}

func (s *Shield) generateCanaryToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	tok := hex.EncodeToString(b)
	s.canaryTokens[tok] = true
	return tok
}

func (s *Shield) GetRandomCanaryPath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for tok := range s.canaryTokens {
		return fmt.Sprintf("/api/canary/%s", tok)
	}
	return "/api/canary/" + s.generateCanaryToken()
}

// ExtractIP extracts the client IP address from remote address or headers.
func ExtractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// IsKnownScraperAgent checks for automated scraping tools, libraries, and headless crawlers.
func IsKnownScraperAgent(ua string) bool {
	uaLower := strings.ToLower(ua)
	if uaLower == "" {
		return true // Missing user-agent is almost always automated
	}

	// Always permit the official Sovereign Mesh CLI
	if strings.Contains(uaLower, "sovereignmesh") {
		return false
	}

	scraperSignatures := []string{
		"python-requests", "python-urllib", "aiohttp", "scrapy", "curl/", "wget/",
		"go-http-client", "httpx", "colly", "mechanize", "postmanruntime",
		"headlesschrome", "phantomjs", "selenium", "puppeteer",
	}

	for _, sig := range scraperSignatures {
		if strings.Contains(uaLower, sig) {
			return true
		}
	}
	return false
}

// CheckRequest evaluates whether an inbound request should be allowed, challenged, or tarpitted.
func (s *Shield) CheckRequest(r *http.Request) (allowed bool, tarpit bool, reason string) {
	ip := ExtractIP(r)
	ua := r.UserAgent()
	path := r.URL.Path

	s.mu.Lock()
	defer s.mu.Unlock()

	client, exists := s.clients[ip]
	now := time.Now()

	if !exists {
		client = &ClientState{
			Tokens:     s.bucketCap,
			LastRefill: now,
		}
		s.clients[ip] = client
	}

	// 1. Check if currently banned
	if now.Before(client.BanUntil) {
		return false, true, "IP banned due to honeypot trap activation or volumetric abuse"
	}

	// 2. Check Canary Honeypot Hit
	if strings.HasPrefix(path, "/api/canary/") {
		token := strings.TrimPrefix(path, "/api/canary/")
		if s.canaryTokens[token] {
			client.IsBot = true
			client.CanaryHits++
			client.BanUntil = now.Add(1 * time.Hour) // 1-hour automated blacklist
			return false, true, "Honeypot canary accessed by automated crawler"
		}
	}

	// 3. User-Agent Heuristic Check
	if IsKnownScraperAgent(ua) {
		client.IsBot = true
		client.BanUntil = now.Add(15 * time.Minute)
		return false, true, "Known automated scraping agent fingerprint detected"
	}

	// 4. Token-Bucket Rate Limiter
	elapsed := now.Sub(client.LastRefill).Seconds()
	client.LastRefill = now
	client.Tokens += elapsed * s.rateLimit
	if client.Tokens > s.bucketCap {
		client.Tokens = s.bucketCap
	}

	if client.Tokens < 1.0 {
		// Burst limit exceeded
		return false, false, "Read rate limit exceeded; challenge required"
	}

	client.Tokens -= 1.0
	client.RequestHits++
	return true, false, ""
}

// TarpitResponse stalls automated bots by slowly writing garbage bytes, wasting crawler resources.
func (s *Shield) TarpitResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusTooManyRequests)

	flusher, ok := w.(http.Flusher)
	for range 5 {
		_, _ = w.Write([]byte(".\n"))
		if ok {
			flusher.Flush()
		}
		time.Sleep(s.tarpitDelay)
	}
}

// WatermarkText injects invisible zero-width Unicode characters into text to poison scrapers
// and mark proprietary/origin data without affecting human readability in any modern font.
func WatermarkText(text string, originKey string) string {
	if text == "" {
		return ""
	}

	h := sha256.Sum256([]byte(originKey))
	binarySeq := fmt.Sprintf("%08b%08b", h[0], h[1])

	var wm strings.Builder
	for _, bit := range binarySeq {
		if bit == '1' {
			wm.WriteString(ZWSP)
		} else {
			wm.WriteString(ZWNJ)
		}
	}

	// Insert invisible watermark after the first word
	words := strings.SplitN(text, " ", 2)
	if len(words) == 2 {
		return words[0] + wm.String() + " " + words[1]
	}
	return text + wm.String()
}
