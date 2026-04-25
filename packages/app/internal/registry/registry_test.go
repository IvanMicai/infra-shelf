package registry

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreLoadPlainRegistry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apps.json")
	content := `{"version":1,"apps":{"demo":{"createdAt":"2026-04-25T00:00:00Z","services":{"redis":{"username":"demo","password":"secret","prefix":"demo:"}}}}}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	registry, err := NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}

	if registry.Apps["demo"].Services.Redis.Password != "secret" {
		t.Fatalf("expected decrypted redis password")
	}
}

func TestStoreLoadEncryptedRegistry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apps.json")
	t.Setenv("INFRA_SHELF_SECRET", "test-secret")

	plaintext := []byte(`{"version":1,"apps":{"demo":{"createdAt":"2026-04-25T00:00:00Z","services":{"postgres":{"database":"demo","username":"demo","password":"secret"}}}}}`)
	envelope := encryptForTest(t, "test-secret", plaintext)
	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}

	registry, err := NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}

	if registry.Apps["demo"].Services.Postgres.Password != "secret" {
		t.Fatalf("expected decrypted postgres password")
	}
}

func encryptForTest(t *testing.T, secret string, plaintext []byte) encryptedRegistryFile {
	t.Helper()

	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := []byte("123456789012")
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	return encryptedRegistryFile{
		Version:    2,
		Encrypted:  true,
		Algorithm:  "AES-256-GCM",
		KDF:        "SHA-256",
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}
}
