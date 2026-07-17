package adminauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// newToken returns a random hex token and the SHA-256 hash of that exact
// string — the same pattern used for installation credentials
// (internal/ingest): only the hash is ever stored, the raw value only
// ever lives in the response/cookie.
func newToken() (raw string, hash []byte) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	raw = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	return raw, sum[:]
}

func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
