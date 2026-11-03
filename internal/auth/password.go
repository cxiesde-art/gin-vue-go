package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/argon2"
	"strconv"
	"strings"
)

func HashPassword(password string) string {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic(fmt.Sprintf("generate password salt: %v", err))
	}
	return encodePassword(password, salt)
}

func CheckPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[3])
	expected, err2 := base64.RawStdEncoding.DecodeString(parts[4])
	if err1 != nil || err2 != nil {
		return false
	}
	params := strings.Split(parts[2], ",")
	memory, iterations, threads, ok := parseParams(params)
	if !ok || len(salt) < 8 || len(expected) == 0 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func encodePassword(password string, salt []byte) string {
	const memory = 64 * 1024
	const iterations = 1
	const threads = 4
	hash := argon2.IDKey([]byte(password), salt, iterations, memory, threads, 32)
	return fmt.Sprintf("argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", memory, iterations, threads, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash))
}

func parseParams(values []string) (memory uint32, iterations uint32, threads uint8, ok bool) {
	if len(values) != 3 {
		return 0, 0, 0, false
	}
	parsed := make(map[string]uint64, len(values))
	for _, value := range values {
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 {
			return 0, 0, 0, false
		}
		number, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			return 0, 0, 0, false
		}
		parsed[parts[0]] = number
	}
	if parsed["m"] < 8*1024 || parsed["t"] == 0 || parsed["p"] == 0 || parsed["p"] > 32 {
		return 0, 0, 0, false
	}
	return uint32(parsed["m"]), uint32(parsed["t"]), uint8(parsed["p"]), true
}
