package hash

import (
	"crypto/rand"
	"crypto/sha256"

	"golang.org/x/crypto/bcrypt"
)

func GenerateSalt(n int) ([]byte, error) {
	var salt = make([]byte, n)
	_, err := rand.Read(salt[:])
	if err != nil {
		return nil, err
	}
	return salt, nil
}

func Hash(password string, salt []byte) ([]byte, error) {
	return bcrypt.GenerateFromPassword(hashpwsalt(password, salt), 10)
}

func CheckPasswordHash(password string, salt, hash []byte) bool {
	err := bcrypt.CompareHashAndPassword(hash, hashpwsalt(password, salt))
	return err == nil
}

func hashpwsalt(password string, salt []byte) []byte {
	bytes := append([]byte(password), salt...)
	hash := sha256.Sum256(bytes)
	return hash[:]
}
