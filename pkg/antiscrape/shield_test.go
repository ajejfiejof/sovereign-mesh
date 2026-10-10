package antiscrape

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUserAgentDetection(t *testing.T) {
	scrapers := []string{
		"python-requests/2.31.0",
		"curl/8.2.1",
		"Scrapy/2.11.0 (+https://scrapy.org)",
		"Go-http-client/1.1",
		"aiohttp/3.8.5",
		"",
	}

	for _, ua := range scrapers {
		if !IsKnownScraperAgent(ua) {
			t.Errorf("Expected UA %q to be flagged as scraper", ua)
		}
	}

	legitBrowsers := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14.1; rv:120.0) Gecko/20100101 Firefox/120.0",
	}

	for _, ua := range legitBrowsers {
		if IsKnownScraperAgent(ua) {
			t.Errorf("Expected legit browser %q not to be flagged as scraper", ua)
		}
	}
}

func TestShieldCanaryAndRateLimiting(t *testing.T) {
	shield := NewShield(5.0, 5.0) // 5 tokens/sec, capacity 5
	canaryPath := shield.GetRandomCanaryPath()

	// 1. Canary hit by crawler
	reqCanary := httptest.NewRequest("GET", canaryPath, nil)
	reqCanary.RemoteAddr = "192.0.2.1:12345"
	reqCanary.Header.Set("User-Agent", "Mozilla/5.0 (CustomCrawler)")

	allowed, tarpit, _ := shield.CheckRequest(reqCanary)
	if allowed || !tarpit {
		t.Errorf("Expected canary request to be rejected and tarpitted: allowed=%v, tarpit=%v", allowed, tarpit)
	}

	// Immediate subsequent request from same IP should still be banned
	reqFollowup := httptest.NewRequest("GET", "/api/threads", nil)
	reqFollowup.RemoteAddr = "192.0.2.1:12345"
	allowed2, tarpit2, _ := shield.CheckRequest(reqFollowup)
	if allowed2 || !tarpit2 {
		t.Error("Expected IP banned by canary to remain blocked on normal endpoints")
	}

	// 2. Normal browser request from different IP
	reqNormal := httptest.NewRequest("GET", "/api/threads", nil)
	reqNormal.RemoteAddr = "192.0.2.2:54321"
	reqNormal.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) Firefox/120.0")

	allowedN, tarpitN, _ := shield.CheckRequest(reqNormal)
	if !allowedN || tarpitN {
		t.Errorf("Expected normal browser request to pass: allowed=%v, tarpit=%v", allowedN, tarpitN)
	}

	// 3. Rate-limit burst exhaustion
	reqBurst := httptest.NewRequest("GET", "/api/threads", nil)
	reqBurst.RemoteAddr = "192.0.2.3:11111"
	reqBurst.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) Firefox/120.0")

	// Exhaust 5 tokens
	for range 5 {
		_, _, _ = shield.CheckRequest(reqBurst)
	}

	// 6th immediate request should fail rate-limiting
	allowedB, tarpitB, _ := shield.CheckRequest(reqBurst)
	if allowedB {
		t.Error("Expected 6th request to be blocked by token bucket rate limiter")
	}
	if tarpitB {
		t.Error("Rate limit exceeded should challenge, not tarpit yet")
	}
}

func TestZeroWidthWatermarking(t *testing.T) {
	orig := "Decentralized mesh networks resist censorship and surveillance."
	key := "mesh:node123456"

	watermarked := WatermarkText(orig, key)
	if watermarked == orig {
		t.Error("Expected watermarked text to differ in byte content")
	}

	if !strings.Contains(watermarked, ZWSP) && !strings.Contains(watermarked, ZWNJ) {
		t.Error("Watermarked text must contain zero-width Unicode characters")
	}

	if len(watermarked) <= len(orig) {
		t.Errorf("Watermarked text length (%d) should be greater than original (%d)", len(watermarked), len(orig))
	}
}

func TestTarpitResponse(t *testing.T) {
	shield := NewShield(10.0, 10.0)
	shield.tarpitDelay = 1 * time.Millisecond // fast for testing

	rec := httptest.NewRecorder()
	shield.TarpitResponse(rec)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429 Too Many Requests, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(".\n")) {
		t.Error("Tarpit response should output dots")
	}
}
