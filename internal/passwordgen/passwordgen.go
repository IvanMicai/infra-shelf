// Package passwordgen produces random URL-safe passwords for provisioned
// service credentials.
package passwordgen

import (
	"crypto/rand"
	"encoding/base64"
)

// Generate returns a base64url-encoded password of exactly length characters.
// Matches the TS lib's `generatePassword(24)` (randomBytes → base64url → slice).
func Generate(length int) string {
	if length <= 0 {
		length = 24
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand.Read can only fail on systems without a working entropy
		// source; treat as fatal — we'd rather crash than emit a weak password.
		panic("passwordgen: rand.Read failed: " + err.Error())
	}
	encoded := base64.RawURLEncoding.EncodeToString(buf)
	if len(encoded) > length {
		encoded = encoded[:length]
	}
	return encoded
}
