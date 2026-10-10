package crypto

import (
	"testing"
)

func TestIdentityGenerationAndSigning(t *testing.T) {
	id, err := GenerateIdentity()
	if err != nil {
		t.Fatalf("GenerateIdentity failed: %v", err)
	}

	if id.Address == "" || len(id.PubKeyHex) == 0 {
		t.Errorf("Invalid identity generated: %+v", id)
	}

	payload := []byte("The future of communication is sovereign and decentralized.")
	sig := id.Sign(payload)

	if !VerifySignature(id.PubKeyHex, sig, payload) {
		t.Error("Signature verification failed for valid signature")
	}

	tampered := []byte("Tampered message.")
	if VerifySignature(id.PubKeyHex, sig, tampered) {
		t.Error("Verification unexpectedly succeeded on tampered message")
	}
}

func TestProofOfWorkMiningAndVerification(t *testing.T) {
	payload := "SovereignMesh:board=g:thread=1"
	difficulty := 2 // requires "00" prefix for fast testing

	res, err := Mine(payload, difficulty)
	if err != nil {
		t.Fatalf("Mining failed: %v", err)
	}

	if !VerifyPoW(payload, res.Nonce, difficulty) {
		t.Errorf("VerifyPoW failed for mined nonce %d with hash %s", res.Nonce, res.Hash)
	}

	if VerifyPoW(payload, res.Nonce+9999, difficulty+2) {
		t.Error("VerifyPoW unexpectedly succeeded with invalid nonce/difficulty")
	}
}
