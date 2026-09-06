// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// HostKey holds a persistent Ed25519 keypair that identifies a host.
// The host ID is the RFC 7638 JWK thumbprint of the public key and is stable
// across process restarts as long as the same key file is used.
type HostKey struct {
	// PrivateKey is the Ed25519 private key (64-byte seed + public key).
	PrivateKey ed25519.PrivateKey

	// PublicKey is the Ed25519 public key (32 bytes).
	PublicKey ed25519.PublicKey

	// ID is the RFC 7638 JWK thumbprint computed over the public key.
	// Use this value as the host identifier when communicating with the platform.
	ID string
}

// hostKeyFile is the on-disk JSON representation of a host keypair.
// Only fields defined in RFC 8037 (OKP keys) are included.
type hostKeyFile struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"` // base64url-encoded public key
	D   string `json:"d"` // base64url-encoded private key seed
}

// GenerateHostKey creates a new Ed25519 keypair and computes the host ID.
func GenerateHostKey() (*HostKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("capabilitygrant: generate ed25519 keypair: %w", err)
	}
	return &HostKey{
		PrivateKey: priv,
		PublicKey:  pub,
		ID:         JWKThumbprint(pub),
	}, nil
}

// SaveHostKey writes the keypair to path in JWK JSON format with 0600 permissions.
// The parent directory must already exist. If the file already exists it is
// overwritten atomically (write to temp file, rename).
func SaveHostKey(key *HostKey, path string) error {
	if key == nil {
		return errors.New("capabilitygrant: SaveHostKey: key is nil")
	}

	// ed25519 private key is 64 bytes: first 32 bytes are the seed.
	seed := key.PrivateKey.Seed()

	kf := hostKeyFile{
		Kty: "OKP",
		Crv: "Ed25519",
		X:   base64.RawURLEncoding.EncodeToString(key.PublicKey),
		D:   base64.RawURLEncoding.EncodeToString(seed),
	}

	data, err := json.Marshal(kf)
	if err != nil {
		return fmt.Errorf("capabilitygrant: marshal host key: %w", err)
	}

	// Write to a temp file in the same directory, then rename to avoid partial writes.
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".hostkey-*.tmp")
	if err != nil {
		return fmt.Errorf("capabilitygrant: create temp file for host key: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("capabilitygrant: write host key: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("capabilitygrant: close host key temp file: %w", err)
	}

	// Set permissions before rename so the file is never readable by others.
	if err := os.Chmod(tmpName, 0600); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("capabilitygrant: chmod host key: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("capabilitygrant: rename host key into place: %w", err)
	}

	return nil
}

// LoadHostKey reads a keypair from a JWK JSON file at path.
func LoadHostKey(path string) (*HostKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("capabilitygrant: read host key file: %w", err)
	}

	var kf hostKeyFile
	if err := json.Unmarshal(data, &kf); err != nil {
		return nil, fmt.Errorf("capabilitygrant: parse host key file: %w", err)
	}

	if kf.Kty != "OKP" || kf.Crv != "Ed25519" {
		return nil, fmt.Errorf("capabilitygrant: host key file has unexpected kty/crv: %s/%s", kf.Kty, kf.Crv)
	}

	pubBytes, err := base64.RawURLEncoding.DecodeString(kf.X)
	if err != nil {
		return nil, fmt.Errorf("capabilitygrant: decode host key public key: %w", err)
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("capabilitygrant: host key public key wrong size: %d", len(pubBytes))
	}

	seedBytes, err := base64.RawURLEncoding.DecodeString(kf.D)
	if err != nil {
		return nil, fmt.Errorf("capabilitygrant: decode host key seed: %w", err)
	}
	if len(seedBytes) != ed25519.SeedSize {
		return nil, fmt.Errorf("capabilitygrant: host key seed wrong size: %d", len(seedBytes))
	}

	priv := ed25519.NewKeyFromSeed(seedBytes)
	pub := ed25519.PublicKey(pubBytes)

	return &HostKey{
		PrivateKey: priv,
		PublicKey:  pub,
		ID:         JWKThumbprint(pub),
	}, nil
}

// LoadOrGenerateHostKey loads the keypair from path if the file exists.
// If the file does not exist, a new keypair is generated and saved to path.
// The parent directory must already exist.
func LoadOrGenerateHostKey(path string) (*HostKey, error) {
	if _, err := os.Stat(path); err == nil {
		return LoadHostKey(path)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("capabilitygrant: stat host key file: %w", err)
	}

	key, err := GenerateHostKey()
	if err != nil {
		return nil, err
	}

	if err := SaveHostKey(key, path); err != nil {
		return nil, err
	}

	return key, nil
}

// PublicKeyJWK returns the public portion of the host key in JWK format.
// The "d" (private key seed) field is NOT included.
func (k *HostKey) PublicKeyJWK() json.RawMessage {
	return marshalPublicJWK(k.PublicKey)
}

// JWKThumbprint computes the RFC 7638 thumbprint of an Ed25519 public key.
//
// The thumbprint is the base64url-encoded SHA-256 hash of the canonical JSON
// representation of the JWK. For an OKP Ed25519 key the canonical member set
// is: crv, kty, x (lexicographic order, no whitespace).
func JWKThumbprint(pub ed25519.PublicKey) string {
	// RFC 7638 §3.3: canonical JSON for OKP = {"crv":"Ed25519","kty":"OKP","x":"<base64url>"}
	canonical := fmt.Sprintf(
		`{"crv":"Ed25519","kty":"OKP","x":"%s"}`,
		base64.RawURLEncoding.EncodeToString(pub),
	)
	hash := sha256.Sum256([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// marshalPublicJWK builds a minimal public-only OKP JWK JSON value.
func marshalPublicJWK(pub ed25519.PublicKey) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(
		`{"kty":"OKP","crv":"Ed25519","x":"%s"}`,
		base64.RawURLEncoding.EncodeToString(pub),
	))
}
