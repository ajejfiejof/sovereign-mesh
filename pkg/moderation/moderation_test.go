package moderation

import (
	"testing"
)

func TestHateContentScanning(t *testing.T) {
	flaggedCases := []string{
		"We must secure the 14 words for our people",
		"Join atomwaffen division today",
		"Blood and soil is our creed",
		"Sieg heil to the movement",
		"Praising combat 18 militants",
		"Secret symbol: \u534D hidden inside text",
		"Runic symbol: \u16CB\u16CB SS guard",
		// Leetspeak obfuscation
		"b100d and s0i1",
		"14 88 forever",
		"@t0mw@ffen division",
	}

	for _, text := range flaggedCases {
		allowed, reason := ScanContent(text)
		if allowed {
			t.Errorf("Expected content to be blocked, but was allowed: %q (reason: %s)", text, reason)
		}
	}

	cleanCases := []string{
		"Discussing decentralized P2P network architectures and cryptography.",
		"What are the best open-source overlay routing protocols in Go?",
		"The quick brown fox jumps over the lazy dog.",
		"Studying historical events and sociology in university.",
		"Free speech and self-sovereign identity over encrypted meshes.",
	}

	for _, text := range cleanCases {
		allowed, reason := ScanContent(text)
		if !allowed {
			t.Errorf("Expected clean content to be allowed, but was blocked: %q (reason: %s)", text, reason)
		}
	}
}

func TestDefederationManager(t *testing.T) {
	dm := NewDefederationManager()

	authorAddr := "mesh:hostile123456789"
	authorPub := "deadbeefhostilekey"

	// 1. Initial clean post passes
	cleanPost := "Hello network, testing p2p communications."
	allowed, _ := dm.ValidatePost(cleanPost, authorAddr, authorPub)
	if !allowed {
		t.Error("Expected clean post to pass validation")
	}

	// 2. Hate post triggers immediate taint & sever
	hatePost := "We will rise with atomwaffen division"
	allowedHate, reason := dm.ValidatePost(hatePost, authorAddr, authorPub)
	if allowedHate {
		t.Error("Expected hate post to be rejected")
	}
	if reason == "" {
		t.Error("Expected violation reason to be populated")
	}

	// 3. Author is now permanently tainted and shunned
	if !dm.IsBanned(authorAddr, authorPub, "") {
		t.Error("Author should be marked as permanently banned after violation")
	}

	// Subsequent clean post from same author must be blocked
	allowedSubsequent, subReason := dm.ValidatePost(cleanPost, authorAddr, authorPub)
	if allowedSubsequent {
		t.Error("Subsequent post from banned author must be blocked")
	}
	if subReason != "Author identity is on defederation blacklist" {
		t.Errorf("Unexpected rejection reason: %s", subReason)
	}

	// 4. Test community blocklist import
	blocklistJSON := []byte(`{
		"version": "1.0",
		"banned_addresses": ["mesh:badactor1", "mesh:badactor2"],
		"banned_pubkeys": ["badkey123"],
		"banned_endpoints": ["198.51.100.1:9000"],
		"default_reason": "Known violent neo-Nazi network node"
	}`)

	count, err := dm.ImportBlocklist(blocklistJSON)
	if err != nil {
		t.Fatalf("ImportBlocklist failed: %v", err)
	}
	if count != 4 {
		t.Errorf("Expected 4 imported banned entities, got %d", count)
	}

	if !dm.IsBanned("mesh:badactor1", "", "") {
		t.Error("Imported banned address should be recognized as banned")
	}
	if !dm.IsBanned("", "badkey123", "") {
		t.Error("Imported banned pubkey should be recognized as banned")
	}
	if !dm.IsBanned("", "", "198.51.100.1:9000") {
		t.Error("Imported banned endpoint should be recognized as banned")
	}
}
