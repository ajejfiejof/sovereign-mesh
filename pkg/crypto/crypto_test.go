package crypto

import (
	"path/filepath"
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

	// Invalid signature or pubkey hex
	if VerifySignature("invalidhex", sig, payload) {
		t.Error("Verification unexpectedly succeeded on invalid pubkey hex")
	}
	if VerifySignature(id.PubKeyHex, "invalidhex", payload) {
		t.Error("Verification unexpectedly succeeded on invalid sig hex")
	}
}

func TestIdentityFilePersistenceAndHex(t *testing.T) {
	tempDir := t.TempDir()
	keyFile := filepath.Join(tempDir, "identity.json")

	id, err := GenerateIdentity()
	if err != nil {
		t.Fatalf("GenerateIdentity failed: %v", err)
	}

	if err := id.SaveToFile(keyFile); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	loadedID, err := LoadFromFile(keyFile)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if loadedID.Address != id.Address {
		t.Errorf("Address mismatch: got %s, expected %s", loadedID.Address, id.Address)
	}
	if loadedID.PubKeyHex != id.PubKeyHex {
		t.Errorf("PubKeyHex mismatch: got %s, expected %s", loadedID.PubKeyHex, id.PubKeyHex)
	}

	// Test LoadIdentityFromHex errors
	if _, err := LoadIdentityFromHex("nothex"); err == nil {
		t.Error("Expected error on invalid hex string")
	}
	if _, err := LoadIdentityFromHex("1234"); err == nil {
		t.Error("Expected error on wrong key length")
	}

	// Test LoadFromFile errors
	if _, err := LoadFromFile(filepath.Join(tempDir, "nonexistent.json")); err == nil {
		t.Error("Expected error loading nonexistent file")
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

	// Zero/negative difficulty tests
	zeroRes, err := Mine(payload, 0)
	if err != nil || zeroRes.Nonce != 0 {
		t.Errorf("Expected zero nonce for difficulty 0, got %v (err=%v)", zeroRes, err)
	}
	if !VerifyPoW(payload, 0, 0) {
		t.Error("VerifyPoW should succeed for difficulty 0")
	}
}
