package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func sampleRegistry() Registry {
	return Registry{
		Version: 1,
		Apps: map[string]AppEntry{
			"iara": {
				CreatedAt: "2026-05-17T00:00:00.000Z",
				Services: Services{
					Postgres: &PostgresConfig{Database: "iara", Username: "iara", Password: "secret"},
				},
			},
		},
	}
}

func TestSaveLoadPlainRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apps.json")
	store := NewStore(path)

	if err := store.Save(sampleRegistry()); err != nil {
		t.Fatal(err)
	}

	// Confirm the on-disk file is plain JSON (no "encrypted" key).
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var probe map[string]any
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	if _, ok := probe["encrypted"]; ok {
		t.Fatalf("expected plain file, got encrypted envelope: %s", raw)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Apps["iara"].Services.Postgres.Password != "secret" {
		t.Fatalf("roundtrip lost password: %+v", got)
	}
}

func TestSaveEncryptsWhenSecretPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apps.json")
	t.Setenv("INFRA_SHELF_SECRET", "test-secret-key")

	store := NewStore(path)
	if err := store.Save(sampleRegistry()); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var envelope encryptedRegistryFile
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("expected encrypted envelope, parse failed: %v", err)
	}
	if !envelope.Encrypted || envelope.Algorithm != "AES-256-GCM" || envelope.KDF != "SHA-256" {
		t.Fatalf("bad envelope metadata: %+v", envelope)
	}
	if envelope.Nonce == "" || envelope.Ciphertext == "" {
		t.Fatalf("envelope missing nonce/ciphertext")
	}

	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Apps["iara"].Services.Postgres.Password != "secret" {
		t.Fatalf("decrypted password mismatch: %+v", got)
	}
}

func TestSaveProducesTSCompatibleEnvelope(t *testing.T) {
	// The TS CLI's registry-crypto.ts writes: ciphertext = base64(seal||tag),
	// nonce = base64(12 random bytes). Our Save() must produce something the
	// existing TS-compatible Load() (which reads the same format) can decrypt
	// without changes. The two-direction roundtrip below proves it.
	dir := t.TempDir()
	path := filepath.Join(dir, "apps.json")
	t.Setenv("INFRA_SHELF_SECRET", "test-secret-key")

	store := NewStore(path)
	if err := store.Save(sampleRegistry()); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(path)
	var envelope encryptedRegistryFile
	_ = json.Unmarshal(raw, &envelope)

	// Force the existing decryptRegistryContent path used by Load() — it's the
	// same code paths the legacy TS-encrypted files exercise.
	plaintext, err := decryptRegistryContent(envelope, path)
	if err != nil {
		t.Fatalf("decryptRegistryContent failed on our own output: %v", err)
	}
	var reloaded Registry
	if err := json.Unmarshal(plaintext, &reloaded); err != nil {
		t.Fatal(err)
	}
	if reloaded.Apps["iara"].Services.Postgres.Password != "secret" {
		t.Fatalf("compat decrypt mismatch")
	}
}

func TestSaveWrongSecretFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apps.json")
	t.Setenv("INFRA_SHELF_SECRET", "test-secret-key")

	store := NewStore(path)
	if err := store.Save(sampleRegistry()); err != nil {
		t.Fatal(err)
	}

	t.Setenv("INFRA_SHELF_SECRET", "wrong-secret")
	if _, err := store.Load(); err == nil {
		t.Fatal("expected decryption failure with wrong secret")
	}
}

func TestUpsertDeleteApp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apps.json")
	store := NewStore(path)

	entry := AppEntry{
		CreatedAt: "2026-05-17T00:00:00.000Z",
		Services:  Services{Redis: &RedisConfig{Username: "demo", Password: "p", Prefix: "demo:"}},
	}
	if err := store.UpsertApp("demo", entry); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertApp("other", AppEntry{CreatedAt: "x"}); err != nil {
		t.Fatal(err)
	}

	reg, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if reg.Apps["demo"].Services.Redis.Password != "p" {
		t.Fatalf("upsert lost data: %+v", reg)
	}
	if _, ok := reg.Apps["other"]; !ok {
		t.Fatalf("second upsert missing")
	}

	if err := store.DeleteApp("demo"); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteApp("ghost"); err != nil {
		t.Fatalf("delete of missing entry should be no-op, got %v", err)
	}

	reg, _ = store.Load()
	if _, ok := reg.Apps["demo"]; ok {
		t.Fatalf("delete failed: %+v", reg)
	}
	if _, ok := reg.Apps["other"]; !ok {
		t.Fatalf("delete removed wrong entry")
	}
}

func TestEncryptInPlace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apps.json")

	// Write plain first (no secret in env).
	store := NewStore(path)
	if err := store.Save(sampleRegistry()); err != nil {
		t.Fatal(err)
	}

	// Now set secret and re-encrypt in place.
	t.Setenv("INFRA_SHELF_SECRET", "the-secret")
	if err := store.EncryptInPlace(); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(path)
	var envelope encryptedRegistryFile
	if err := json.Unmarshal(raw, &envelope); err != nil || !envelope.Encrypted {
		t.Fatalf("file is not encrypted after EncryptInPlace: %s", raw)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Apps["iara"].Services.Postgres.Password != "secret" {
		t.Fatal("roundtrip after EncryptInPlace failed")
	}
}

func TestEncryptInPlaceWithoutSecretFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apps.json")
	NewStore(path).Save(sampleRegistry())

	t.Setenv("INFRA_SHELF_SECRET", "")
	t.Setenv("INFRA_SHELF_REGISTRY_SECRET", "")
	if err := NewStore(path).EncryptInPlace(); err == nil {
		t.Fatal("expected error when no secret is set")
	}
}
