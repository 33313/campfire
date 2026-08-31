package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP, RFC 6238: HMAC-SHA1 over a 30-second counter, six digits. This is
// what every authenticator application implements, and it is short enough
// that a dependency would be the larger thing to maintain.

const totpStep = 30 * time.Second

// NewTOTPSecret returns a base32 secret suitable for an authenticator app.
func NewTOTPSecret() (string, error) {
	raw := make([]byte, 20) // 160 bits, the RFC 4226 recommendation for SHA-1
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

// TOTPCode returns the six-digit code for secret at t.
func TOTPCode(secret string, t time.Time) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).
		DecodeString(strings.ToUpper(strings.ReplaceAll(secret, " ", "")))
	if err != nil {
		return "", fmt.Errorf("totp secret: %w", err)
	}
	counter := uint64(t.Unix()) / uint64(totpStep.Seconds())
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", value%1_000_000), nil
}

// VerifyTOTP reports whether code is valid for secret now, accepting the
// neighbouring steps so that a clock a few seconds out still works.
func VerifyTOTP(secret, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	for _, skew := range []time.Duration{-totpStep, 0, totpStep} {
		want, err := TOTPCode(secret, now.Add(skew))
		if err != nil {
			return false
		}
		if hmac.Equal([]byte(want), []byte(code)) {
			return true
		}
	}
	return false
}

// TOTPURI returns the otpauth:// URI an authenticator app scans.
func TOTPURI(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{"secret": {secret}, "issuer": {issuer}}
	return "otpauth://totp/" + label + "?" + q.Encode()
}
