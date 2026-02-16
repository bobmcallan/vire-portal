package auth

import (
	"testing"
)

func TestVerifyPKCE_ValidVerifier(t *testing.T) {
	// RFC 7636 Appendix B test vector
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge := GenerateCodeChallenge(verifier)

	if !VerifyPKCE(verifier, challenge) {
		t.Error("expected PKCE verification to succeed with matching verifier")
	}
}

func TestVerifyPKCE_InvalidVerifier(t *testing.T) {
	verifier := "correct-verifier"
	challenge := GenerateCodeChallenge(verifier)

	if VerifyPKCE("wrong-verifier", challenge) {
		t.Error("expected PKCE verification to fail with wrong verifier")
	}
}

func TestVerifyPKCE_EmptyVerifier(t *testing.T) {
	challenge := GenerateCodeChallenge("some-verifier")

	if VerifyPKCE("", challenge) {
		t.Error("expected PKCE verification to fail with empty verifier")
	}
}

func TestVerifyPKCE_EmptyChallenge(t *testing.T) {
	if VerifyPKCE("some-verifier", "") {
		t.Error("expected PKCE verification to fail with empty challenge")
	}
}

func TestGenerateCodeChallenge_Deterministic(t *testing.T) {
	verifier := "test-verifier-12345"
	c1 := GenerateCodeChallenge(verifier)
	c2 := GenerateCodeChallenge(verifier)
	if c1 != c2 {
		t.Errorf("expected same challenge for same verifier, got %s and %s", c1, c2)
	}
}

func TestGenerateCodeChallenge_DifferentVerifiers(t *testing.T) {
	c1 := GenerateCodeChallenge("verifier-a")
	c2 := GenerateCodeChallenge("verifier-b")
	if c1 == c2 {
		t.Error("expected different challenges for different verifiers")
	}
}

func TestVerifyPKCE_RoundTrip(t *testing.T) {
	verifiers := []string{
		"simple",
		"a-longer-verifier-with-more-entropy-1234567890",
		"special_chars~.-*",
		"unicode-日本語",
	}

	for _, v := range verifiers {
		challenge := GenerateCodeChallenge(v)
		if !VerifyPKCE(v, challenge) {
			t.Errorf("round-trip failed for verifier %q", v)
		}
	}
}
