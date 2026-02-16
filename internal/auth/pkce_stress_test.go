package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

// =============================================================================
// PKCE Stress Tests
// =============================================================================

// --- PKCE bypass: empty verifier and challenge ---

func TestVerifyPKCE_StressEmptyBothBypass(t *testing.T) {
	// If both verifier and challenge are empty, SHA256("") matches SHA256("").
	// This is a PKCE bypass: an attacker who doesn't know the verifier can
	// send verifier="" and challenge=SHA256("").
	emptyChallenge := GenerateCodeChallenge("")
	if VerifyPKCE("", emptyChallenge) {
		t.Log("FINDING: VerifyPKCE(\"\", SHA256(\"\")) returns true. " +
			"The /authorize handler must reject empty code_challenge, " +
			"and the /token handler must reject empty code_verifier.")
	}
}

// --- PKCE bypass: attacker uses their own verifier ---

func TestVerifyPKCE_StressAttackerSubstitutesVerifier(t *testing.T) {
	// Scenario: Client sends code_challenge=SHA256("legit-verifier") with /authorize.
	// Attacker intercepts the auth code and tries to exchange it with their own verifier.
	// PKCE should prevent this.
	legitimateChallenge := GenerateCodeChallenge("legit-verifier-with-high-entropy")
	attackerVerifier := "attacker-verifier"

	if VerifyPKCE(attackerVerifier, legitimateChallenge) {
		t.Fatal("SECURITY: Attacker's verifier matched legitimate challenge — PKCE is broken")
	}
}

// --- PKCE: verifier length boundaries ---

func TestVerifyPKCE_StressVerifierLengthBoundaries(t *testing.T) {
	// RFC 7636 Section 4.1: code_verifier must be 43-128 characters.
	// The implementation doesn't enforce length — test what happens.
	lengths := []int{0, 1, 42, 43, 128, 129, 256, 1024, 1 << 16}

	for _, length := range lengths {
		verifier := strings.Repeat("a", length)
		challenge := GenerateCodeChallenge(verifier)

		if !VerifyPKCE(verifier, challenge) {
			t.Errorf("round-trip failed for verifier length %d", length)
		}
	}

	t.Log("NOTE: VerifyPKCE does not enforce RFC 7636 verifier length (43-128 chars). " +
		"The /authorize and /token handlers should validate length.")
}

// --- PKCE: verifier character set ---

func TestVerifyPKCE_StressVerifierCharacterSet(t *testing.T) {
	// RFC 7636 Section 4.1: unreserved characters only: [A-Z] / [a-z] / [0-9] / "-" / "." / "_" / "~"
	// The implementation accepts any bytes. Test with invalid characters.
	invalidVerifiers := []struct {
		name     string
		verifier string
	}{
		{"spaces", "verifier with spaces inside it here now"},
		{"unicode", "验证者验证者验证者验证者验证者验证者验证者验证者验"},
		{"null bytes", "verifier\x00with\x00nulls\x00padding"},
		{"newlines", "verifier\nwith\nnewlines\nhere\nnow"},
		{"control chars", "verifier\x01\x02\x03\x04\x05\x06\x07"},
		{"html", "<script>alert(1)</script>+padding+padding"},
	}

	for _, tc := range invalidVerifiers {
		t.Run(tc.name, func(t *testing.T) {
			challenge := GenerateCodeChallenge(tc.verifier)
			if !VerifyPKCE(tc.verifier, challenge) {
				t.Errorf("round-trip failed for verifier with %s", tc.name)
			}
		})
	}

	t.Log("NOTE: VerifyPKCE accepts any byte sequence as verifier. " +
		"RFC 7636 restricts to unreserved URL characters. " +
		"The /authorize handler should validate code_challenge character set, " +
		"and /token should validate code_verifier character set.")
}

// --- PKCE: challenge format manipulation ---

func TestVerifyPKCE_StressChallengeWithPadding(t *testing.T) {
	// base64url should NOT have padding (=). If the client sends
	// a padded challenge, it should not match.
	verifier := "test-verifier-for-padding-check"
	hash := sha256.Sum256([]byte(verifier))

	// Correct (no padding)
	correctChallenge := base64.RawURLEncoding.EncodeToString(hash[:])
	// With padding (wrong per RFC 7636)
	paddedChallenge := base64.URLEncoding.EncodeToString(hash[:])

	if correctChallenge == paddedChallenge {
		t.Skip("challenges are identical (short hash with no padding difference)")
	}

	if VerifyPKCE(verifier, paddedChallenge) {
		t.Log("FINDING: Padded base64 challenge accepted. " +
			"RFC 7636 Section 4.2 requires base64url WITHOUT padding. " +
			"However, since we generate using RawURLEncoding (no padding), " +
			"a padded challenge from a non-compliant client would fail.")
	}
}

func TestVerifyPKCE_StressChallengeStdBase64(t *testing.T) {
	// Standard base64 uses + and / instead of - and _.
	// If a client uses standard base64, the challenge won't match.
	verifier := "test-verifier-for-base64-check"
	hash := sha256.Sum256([]byte(verifier))

	urlEncoded := base64.RawURLEncoding.EncodeToString(hash[:])
	stdEncoded := base64.RawStdEncoding.EncodeToString(hash[:])

	if urlEncoded == stdEncoded {
		t.Skip("encodings are identical for this hash")
	}

	if VerifyPKCE(verifier, stdEncoded) {
		t.Error("SECURITY: Standard base64 challenge accepted — should only accept base64url")
	}
}

// --- PKCE: constant-time comparison verification ---

func TestVerifyPKCE_StressConstantTimeComparison(t *testing.T) {
	// Verify that both correct and incorrect verifiers produce the same
	// type of response (true/false) without timing differences.
	// We can't measure timing precisely in Go tests, but we verify:
	// 1. subtle.ConstantTimeCompare is used (code review)
	// 2. Both paths don't leak information via error messages
	verifier := "correct-verifier-with-enough-entropy"
	challenge := GenerateCodeChallenge(verifier)

	// Correct: should return true
	if !VerifyPKCE(verifier, challenge) {
		t.Fatal("correct verifier should pass")
	}

	// Wrong: should return false (not panic or error)
	if VerifyPKCE("wrong-verifier-with-enough-entropy-too", challenge) {
		t.Fatal("wrong verifier should fail")
	}

	// Almost correct (one char different): should return false
	almostRight := verifier[:len(verifier)-1] + "X"
	if VerifyPKCE(almostRight, challenge) {
		t.Fatal("almost-correct verifier should fail")
	}
}

// --- PKCE: SHA256 collision resistance (sanity check) ---

func TestVerifyPKCE_StressDifferentVerifiersSameLength(t *testing.T) {
	// Two different verifiers of the same length should produce different challenges.
	v1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	v2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	c1 := GenerateCodeChallenge(v1)
	c2 := GenerateCodeChallenge(v2)

	if c1 == c2 {
		t.Error("different verifiers produced same challenge — SHA256 collision?")
	}

	if VerifyPKCE(v1, c2) {
		t.Error("SECURITY: verifier v1 matched challenge from v2")
	}
}

// --- PKCE: code_challenge_method=plain bypass attempt ---

func TestVerifyPKCE_StressPlainMethodBypass(t *testing.T) {
	// If an attacker sends code_challenge_method=plain, the challenge IS the verifier.
	// The server only supports S256. If the attacker sends verifier as the challenge:
	verifier := "the-verifier-is-also-the-challenge"

	// With plain method, challenge == verifier. Does VerifyPKCE accept this?
	if VerifyPKCE(verifier, verifier) {
		t.Error("SECURITY: VerifyPKCE accepted verifier == challenge (plain method bypass). " +
			"This would only happen if SHA256(verifier) == verifier, which is virtually impossible.")
	}
}

// --- PKCE: very large verifier ---

func TestVerifyPKCE_StressVeryLargeVerifier(t *testing.T) {
	// SHA256 handles any input size, but verify no OOM or panic.
	largeVerifier := strings.Repeat("X", 1<<20) // 1MB
	challenge := GenerateCodeChallenge(largeVerifier)

	if !VerifyPKCE(largeVerifier, challenge) {
		t.Error("round-trip failed for 1MB verifier")
	}

	t.Log("NOTE: VerifyPKCE accepts 1MB verifier without limit. " +
		"The /token handler should enforce a max verifier size to prevent DoS.")
}
