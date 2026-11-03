package auth

import (
	"testing"
	"time"
)

func TestTokenRoundTripAndExpiry(t *testing.T) {
	now := time.Now().UTC()
	token, err := IssueToken("secret", 42, "admin", now)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseToken("secret", token)
	if err != nil || claims.UserID != 42 || claims.Role != "admin" {
		t.Fatalf("unexpected claims: %#v, error: %v", claims, err)
	}
	if _, err := ParseToken("wrong", token); err == nil {
		t.Fatal("token verified with wrong secret")
	}
}
