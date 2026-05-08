// Package auth provides simple password hashing and session token utilities.
// Uses SHA-256 + salt (no external deps) since bcrypt is not available.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const saltLen = 16

// HashPassword returns "sha256:<salt>:<hash>" for the given password.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	saltHex := hex.EncodeToString(salt)
	h := sha256.Sum256([]byte(saltHex + password))
	return fmt.Sprintf("sha256:%s:%s", saltHex, hex.EncodeToString(h[:])), nil
}

// CheckPassword verifies password against a hash produced by HashPassword.
func CheckPassword(hash, password string) bool {
	parts := strings.SplitN(hash, ":", 3)
	if len(parts) != 3 || parts[0] != "sha256" {
		return false
	}
	saltHex := parts[1]
	h := sha256.Sum256([]byte(saltHex + password))
	return hex.EncodeToString(h[:]) == parts[2]
}

// GenerateToken returns a 64-char hex random token.
func GenerateToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
