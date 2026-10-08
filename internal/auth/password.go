package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

const (
	passwordAlgorithm = "pbkdf2_sha256"
	passwordIter      = 150000
	passwordSaltBytes = 16
	passwordKeyBytes  = 32
)

// HashPassword returns a salted password hash suitable for storing in MySQL.
func HashPassword(password string) (string, error) {
	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	key := pbkdf2SHA256([]byte(password), salt, passwordIter, passwordKeyBytes)
	return strings.Join([]string{
		passwordAlgorithm,
		strconv.Itoa(passwordIter),
		base64.StdEncoding.EncodeToString(salt),
		base64.StdEncoding.EncodeToString(key),
	}, "$"), nil
}

// VerifyPassword checks a plaintext password against a stored hash.
func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != passwordAlgorithm {
		return false
	}

	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 {
		return false
	}

	salt, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil || len(want) == 0 {
		return false
	}

	got := pbkdf2SHA256([]byte(password), salt, iter, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func pbkdf2SHA256(password, salt []byte, iter, keyLen int) []byte {
	const hLen = sha256.Size

	numBlocks := (keyLen + hLen - 1) / hLen
	dk := make([]byte, 0, numBlocks*hLen)
	var blockIndex [4]byte

	for block := 1; block <= numBlocks; block++ {
		binary.BigEndian.PutUint32(blockIndex[:], uint32(block))

		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write(blockIndex[:])
		u := mac.Sum(nil)

		t := make([]byte, len(u))
		copy(t, u)

		for i := 1; i < iter; i++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}

		dk = append(dk, t...)
	}

	return dk[:keyLen]
}
