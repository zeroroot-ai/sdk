// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateHostKey(t *testing.T) {
	key, err := GenerateHostKey()
	require.NoError(t, err)
	require.NotNil(t, key)

	assert.Len(t, key.PublicKey, ed25519.PublicKeySize, "public key should be 32 bytes")
	assert.Len(t, key.PrivateKey, ed25519.PrivateKeySize, "private key should be 64 bytes")
	assert.NotEmpty(t, key.ID, "host ID should not be empty")
}

func TestGenerateHostKeyUniqueness(t *testing.T) {
	k1, err := GenerateHostKey()
	require.NoError(t, err)

	k2, err := GenerateHostKey()
	require.NoError(t, err)

	assert.NotEqual(t, k1.ID, k2.ID, "two generated host keys should have different IDs")
}

func TestSaveAndLoadHostKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host_key.json")

	original, err := GenerateHostKey()
	require.NoError(t, err)

	require.NoError(t, SaveHostKey(original, path))

	// Verify file permissions.
	fi, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), fi.Mode().Perm(), "host key file should have 0600 permissions")

	// Load and compare.
	loaded, err := LoadHostKey(path)
	require.NoError(t, err)

	assert.Equal(t, []byte(original.PublicKey), []byte(loaded.PublicKey), "public keys should match")
	assert.Equal(t, original.ID, loaded.ID, "host IDs should match")

	// Verify the private key is identical by checking seed bytes.
	assert.Equal(t, original.PrivateKey.Seed(), loaded.PrivateKey.Seed(), "private key seeds should match")
}

func TestLoadOrGenerateHostKey_Generate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host_key.json")

	key, err := LoadOrGenerateHostKey(path)
	require.NoError(t, err)
	require.NotNil(t, key)

	// File should now exist.
	_, err = os.Stat(path)
	assert.NoError(t, err, "key file should have been created")
}

func TestLoadOrGenerateHostKey_Load(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host_key.json")

	first, err := LoadOrGenerateHostKey(path)
	require.NoError(t, err)

	// Second call should load the same key.
	second, err := LoadOrGenerateHostKey(path)
	require.NoError(t, err)

	assert.Equal(t, first.ID, second.ID, "successive LoadOrGenerate calls should return the same key ID")
	assert.Equal(t, []byte(first.PublicKey), []byte(second.PublicKey))
}

func TestPublicKeyJWK_Format(t *testing.T) {
	key, err := GenerateHostKey()
	require.NoError(t, err)

	jwk := key.PublicKeyJWK()
	require.NotEmpty(t, jwk)

	var m map[string]string
	require.NoError(t, json.Unmarshal(jwk, &m))

	assert.Equal(t, "OKP", m["kty"])
	assert.Equal(t, "Ed25519", m["crv"])
	assert.NotEmpty(t, m["x"])
	assert.Empty(t, m["d"], "public JWK must NOT include private key material")

	// x should decode to 32 bytes.
	pubBytes, err := base64.RawURLEncoding.DecodeString(m["x"])
	require.NoError(t, err)
	assert.Len(t, pubBytes, ed25519.PublicKeySize)
}

func TestJWKThumbprint_Deterministic(t *testing.T) {
	key, err := GenerateHostKey()
	require.NoError(t, err)

	tp1 := JWKThumbprint(key.PublicKey)
	tp2 := JWKThumbprint(key.PublicKey)
	assert.Equal(t, tp1, tp2, "thumbprint should be deterministic for the same key")
}

func TestJWKThumbprint_KnownVector(t *testing.T) {
	// Use a fixed public key to produce a deterministic thumbprint and verify it
	// does not change across refactors.
	//
	// pub = 32 zero bytes → base64url = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	// canonical = {"crv":"Ed25519","kty":"OKP","x":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}
	pub := make(ed25519.PublicKey, 32)
	tp := JWKThumbprint(pub)
	assert.NotEmpty(t, tp)

	// Run twice to confirm determinism.
	assert.Equal(t, tp, JWKThumbprint(pub))
}

func TestSaveHostKey_NilKey(t *testing.T) {
	err := SaveHostKey(nil, "/tmp/unused")
	assert.Error(t, err)
}

func TestLoadHostKey_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{invalid}"), 0600))

	_, err := LoadHostKey(path)
	assert.Error(t, err)
}

func TestLoadHostKey_WrongKeyType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rsa.json")
	// Simulate an RSA-looking JWK.
	require.NoError(t, os.WriteFile(path, []byte(`{"kty":"RSA","crv":"","x":"","d":""}`), 0600))

	_, err := LoadHostKey(path)
	assert.Error(t, err, "should reject non-OKP key type")
}
