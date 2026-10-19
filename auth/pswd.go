package auth

import (
	"github.com/alexedwards/argon2id"
)

func HashPassword(password string) (string, error) {
	hashed, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	return hashed, err
}

func CheckPasswordHash(password, hash string) (bool, error) {
	validhash, err := argon2id.ComparePasswordAndHash(password, hash)
	return validhash, err
}
