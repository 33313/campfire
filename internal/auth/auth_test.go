package auth

import (
	"testing"
	"time"
)

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := VerifyPassword(hash, "correct horse battery staple")
	if err != nil || !ok {
		t.Fatalf("correct password rejected: ok=%v err=%v", ok, err)
	}
	ok, err = VerifyPassword(hash, "correct horse battery stapl")
	if err != nil || ok {
		t.Fatalf("wrong password accepted: ok=%v err=%v", ok, err)
	}
}

func TestPasswordHashesAreSalted(t *testing.T) {
	a, _ := HashPassword("same")
	b, _ := HashPassword("same")
	if a == b {
		t.Fatal("two hashes of one password are identical: the salt is not random")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	for _, bad := range []string{"", "plaintext", "$argon2id$v=19$m=1$salt"} {
		if _, err := VerifyPassword(bad, "x"); err != ErrBadHash {
			t.Fatalf("VerifyPassword(%q): err = %v, want ErrBadHash", bad, err)
		}
	}
}

// RFC 6238 Appendix B, SHA-1 test vector: the ASCII secret "12345678901234567890".
func TestTOTPMatchesRFCVector(t *testing.T) {
	// base32("12345678901234567890")
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	got, err := TOTPCode(secret, time.Unix(59, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got != "287082" {
		t.Fatalf("TOTPCode at t=59 = %s, want 287082", got)
	}
}

func TestVerifyTOTPAcceptsNeighbouringStepsOnly(t *testing.T) {
	secret, err := NewTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	code, _ := TOTPCode(secret, now)
	if !VerifyTOTP(secret, code, now) {
		t.Fatal("the current code was rejected")
	}
	if !VerifyTOTP(secret, code, now.Add(25*time.Second)) {
		t.Fatal("a code one step old was rejected")
	}
	if VerifyTOTP(secret, code, now.Add(5*time.Minute)) {
		t.Fatal("a five-minute-old code was accepted")
	}
}
