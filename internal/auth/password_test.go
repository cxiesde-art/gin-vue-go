package auth

import "testing"

func TestPasswordHashRoundTrip(t *testing.T) {
	hash := HashPassword("correct horse battery staple")
	if !CheckPassword(hash, "correct horse battery staple") {
		t.Fatal("expected password to verify")
	}
	if CheckPassword(hash, "wrong password") {
		t.Fatal("wrong password verified")
	}
}
