// Package adminauth implements dashboard operator authentication (section
// 10.3): Argon2id password hashing, session tokens, CSRF, login
// throttling, and role/project scoping.
package adminauth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters match the Go package's own documented recommendation
// for interactive login (golang.org/x/crypto/argon2 doc comment): fast
// enough for a login request, expensive enough to resist offline attack.
const (
	argonTime    = 1
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
)

// HashPassword returns a self-describing PHC-style string:
// $argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyPassword reports whether password matches encoded, comparing in
// constant time. A malformed encoded string is treated as a mismatch, not
// an error — callers should never distinguish "corrupt hash" from "wrong
// password" in their response.
func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	var mem uint32
	var time uint8
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &time, &threads); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	got := argon2.IDKey([]byte(password), salt, uint32(time), mem, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
