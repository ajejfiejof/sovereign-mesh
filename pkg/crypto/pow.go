package crypto

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	// DefaultPoWSalt is the protocol domain separator for anti-replay protection.
	DefaultPoWSalt = "SovereignMeshAntiSpamV1"

	// Argon2id parameters: memory-hard, highly resistant to GPU/ASIC botnet farms.
	PoWTimeCost   = 1
	PoWMemoryCost = 32 * 1024 // 32 MB of RAM per hash attempt
	PoWThreads    = 2
	PoWKeyLen     = 32
)

// PoWResult holds the mined nonce, resulting hash, and execution duration.
type PoWResult struct {
	Nonce    uint64        `json:"nonce"`
	Hash     string        `json:"hash"`
	Duration time.Duration `json:"duration_ms"`
}

// ComputeHash computes a single Argon2id hash for the given payload and nonce.
func ComputeHash(payload string, nonce uint64) string {
	header := fmt.Sprintf("%s:%d", payload, nonce)
	hash := argon2.IDKey([]byte(header), []byte(DefaultPoWSalt), PoWTimeCost, PoWMemoryCost, PoWThreads, PoWKeyLen)
	return hex.EncodeToString(hash)
}

// Mine runs the Proof-of-Work mining loop until a hash satisfies the required difficulty prefix.
// Difficulty indicates the number of leading hex zeros required (e.g. difficulty=2 requires "00", difficulty=4 requires "0000").
func Mine(payload string, difficulty int) (*PoWResult, error) {
	if difficulty <= 0 {
		return &PoWResult{Nonce: 0, Hash: ComputeHash(payload, 0), Duration: 0}, nil
	}

	targetPrefix := strings.Repeat("0", difficulty)
	start := time.Now()
	var nonce uint64 = 0

	for {
		h := ComputeHash(payload, nonce)
		if strings.HasPrefix(h, targetPrefix) {
			return &PoWResult{
				Nonce:    nonce,
				Hash:     h,
				Duration: time.Since(start),
			}, nil
		}
		nonce++
	}
}

// VerifyPoW verifies that a given payload and nonce satisfy the required difficulty.
func VerifyPoW(payload string, nonce uint64, difficulty int) bool {
	if difficulty <= 0 {
		return true
	}
	targetPrefix := strings.Repeat("0", difficulty)
	h := ComputeHash(payload, nonce)
	return strings.HasPrefix(h, targetPrefix)
}
