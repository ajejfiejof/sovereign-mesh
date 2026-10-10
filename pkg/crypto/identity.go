package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Identity represents a self-sovereign cryptographic persona.
// No centralized authority, registrar, email, or phone required.
type Identity struct {
	PrivateKey ed25519.PrivateKey `json:"-"`
	PublicKey  ed25519.PublicKey  `json:"-"`
	Address    string             `json:"address"`
	PubKeyHex  string             `json:"pubkey"`
	PrivKeyHex string             `json:"privkey,omitempty"`
}

// GenerateIdentity generates a fresh Ed25519 keypair and derives the node address.
func GenerateIdentity() (*Identity, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ed25519 key: %w", err)
	}

	addr := DeriveAddress(pub)
	return &Identity{
		PrivateKey: priv,
		PublicKey:  pub,
		Address:    addr,
		PubKeyHex:  hex.EncodeToString(pub),
		PrivKeyHex: hex.EncodeToString(priv),
	}, nil
}

// DeriveAddress derives a deterministic cryptographic address from an Ed25519 public key.
// Format: mesh:<20-byte-hex-digest>
func DeriveAddress(pub ed25519.PublicKey) string {
	h := sha256.Sum256(pub)
	return fmt.Sprintf("mesh:%s", hex.EncodeToString(h[:20]))
}

// LoadIdentityFromHex loads an existing identity from a private key hex string.
func LoadIdentityFromHex(privHex string) (*Identity, error) {
	privBytes, err := hex.DecodeString(privHex)
	if err != nil {
		return nil, fmt.Errorf("invalid hex encoding: %w", err)
	}
	if len(privBytes) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid private key length")
	}

	priv := ed25519.PrivateKey(privBytes)
	pub := priv.Public().(ed25519.PublicKey)
	addr := DeriveAddress(pub)

	return &Identity{
		PrivateKey: priv,
		PublicKey:  pub,
		Address:    addr,
		PubKeyHex:  hex.EncodeToString(pub),
		PrivKeyHex: privHex,
	}, nil
}

// Sign signs arbitrary message bytes with the identity's private key.
func (id *Identity) Sign(data []byte) string {
	sig := ed25519.Sign(id.PrivateKey, data)
	return hex.EncodeToString(sig)
}

// VerifySignature verifies that a signature was created by the provided public key.
func VerifySignature(pubKeyHex, sigHex string, data []byte) bool {
	pubBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return false
	}
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(pubBytes, data, sigBytes)
}

// SaveToFile writes the identity JSON to disk.
func (id *Identity) SaveToFile(filePath string) error {
	data, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0600)
}

// LoadFromFile loads an identity JSON from disk.
func LoadFromFile(filePath string) (*Identity, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var id Identity
	if err := json.Unmarshal(data, &id); err != nil {
		return nil, err
	}
	return LoadIdentityFromHex(id.PrivKeyHex)
}
