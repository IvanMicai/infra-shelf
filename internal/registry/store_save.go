package registry

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Save persists registry to disk. When INFRA_SHELF_SECRET is present (in env
// or in a .env file resolved from the registry's directory), the payload is
// encrypted using AES-256-GCM with a SHA-256-derived key. The file format is
// bit-compatible with the legacy TypeScript CLI's `registry-crypto.ts`.
//
// Writes are atomic: payload is written to a sibling `<name>.tmp` file and
// renamed into place. Parent directories are created on demand.
func (s *Store) Save(reg Registry) error {
	if reg.Apps == nil {
		reg.Apps = map[string]AppEntry{}
	}
	if reg.Version == 0 {
		reg.Version = 1
	}

	secret, _ := registrySecret(filepath.Dir(s.Path))
	var payload []byte
	if secret != "" {
		encrypted, err := encryptRegistry(reg, secret)
		if err != nil {
			return err
		}
		payload, err = json.MarshalIndent(encrypted, "", "  ")
		if err != nil {
			return err
		}
	} else {
		var err error
		payload, err = json.MarshalIndent(reg, "", "  ")
		if err != nil {
			return err
		}
	}
	payload = append(payload, '\n')

	return atomicWrite(s.Path, payload)
}

// EncryptInPlace re-reads the registry and writes it back forced-encrypted.
// Fails if INFRA_SHELF_SECRET is not set.
func (s *Store) EncryptInPlace() error {
	secret, err := registrySecret(filepath.Dir(s.Path))
	if err != nil || secret == "" {
		return errors.New("INFRA_SHELF_SECRET is required to encrypt the registry")
	}

	reg, err := s.Load()
	if err != nil {
		return err
	}

	encrypted, err := encryptRegistry(reg, secret)
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(encrypted, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')

	return atomicWrite(s.Path, payload)
}

// UpsertApp inserts or replaces an app entry. It performs a read-modify-write
// against the on-disk registry, so callers don't need to manage the full
// Registry struct.
func (s *Store) UpsertApp(name string, entry AppEntry) error {
	reg, err := s.Load()
	if err != nil {
		return err
	}
	reg.Apps[name] = entry
	return s.Save(reg)
}

// DeleteApp removes an app entry. Missing entries are not an error.
func (s *Store) DeleteApp(name string) error {
	reg, err := s.Load()
	if err != nil {
		return err
	}
	if _, ok := reg.Apps[name]; !ok {
		return nil
	}
	delete(reg.Apps, name)
	return s.Save(reg)
}

func encryptRegistry(reg Registry, secret string) (encryptedRegistryFile, error) {
	plaintext, err := json.Marshal(reg)
	if err != nil {
		return encryptedRegistryFile{}, err
	}

	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return encryptedRegistryFile{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return encryptedRegistryFile{}, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return encryptedRegistryFile{}, fmt.Errorf("registry: nonce read: %w", err)
	}

	// gcm.Seal returns ciphertext||tag, matching what the TS code builds via
	// Buffer.concat([encrypted, tag]).
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	return encryptedRegistryFile{
		Version:    2,
		Encrypted:  true,
		Algorithm:  "AES-256-GCM",
		KDF:        "SHA-256",
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

func atomicWrite(path string, payload []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// DefaultPath resolves the registry path the same way config.go does, so the
// CLI binary works without explicit config: prefers INFRA_SHELF_REGISTRY_PATH,
// otherwise falls back to <root>/data/apps.json (with INFRA_SHELF_ROOT picking
// up the workspace root). If nothing resolves, returns the legacy TS location
// for backward compat with existing installations.
func DefaultPath(root string) string {
	if p := strings.TrimSpace(os.Getenv("INFRA_SHELF_REGISTRY_PATH")); p != "" {
		abs, err := filepath.Abs(p)
		if err == nil {
			return abs
		}
		return p
	}
	if root == "" {
		if r := strings.TrimSpace(os.Getenv("INFRA_SHELF_ROOT")); r != "" {
			root = r
		}
	}
	if root == "" {
		// Last-resort fallback: current working directory.
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		}
	}
	// Prefer the legacy path if it already exists (backward compat); otherwise
	// land at the new default under data/.
	legacy := filepath.Join(root, "packages", "cli", "src", "apps.json")
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	return filepath.Join(root, "data", "apps.json")
}
