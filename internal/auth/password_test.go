package auth

import "testing"

func TestPasswordHashRoundTrip(t *testing.T) {
	hash := HashPassword("correct horse battery staple")
	secondHash := HashPassword("correct horse battery staple")
	if hash == secondHash {
		t.Fatal("password hashes reused a salt")
	}
	if !CheckPassword(hash, "correct horse battery staple") {
		t.Fatal("expected password to verify")
	}
	if CheckPassword(hash, "wrong password") {
		t.Fatal("wrong password verified")
	}
	if CheckPassword("argon2id$broken", "correct horse battery staple") {
		t.Fatal("malformed hash verified")
	}
}
