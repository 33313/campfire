package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// NewToken returns a fresh opaque credential and its storage hash.
//
// Session cookies, invitation links, password resets and agent tokens are all
// this shape: 32 random bytes shown once, and only the hash kept. A token is
// a bearer credential with no structure to parse and nothing to leak from the
// row it is stored in.
func NewToken() (token, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashToken(token), nil
}

// HashToken returns the storage form of a token.
//
// SHA-256 rather than a password hash: a token is 256 bits of uniform
// randomness, so there is no dictionary to slow an attacker down through, and
// every request carries one.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
