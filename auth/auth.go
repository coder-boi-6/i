package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func HashPassword(password string) (string, error) {
	hashed, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	return hashed, err
}

func CheckPasswordHash(password, hash string) (bool, error) {
	validhash, err := argon2id.ComparePasswordAndHash(password, hash)
	return validhash, err
}
func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		Subject:   userID.String(),
	})
	stringhtheory, err := token.SignedString([]byte(tokenSecret))
	return stringhtheory, err
}
func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	user_id_string, err := token.Claims.GetSubject()
	if err != nil {
		return uuid.Nil, err
	}
	user_id, err := uuid.Parse(user_id_string)
	return user_id, err
}
func GetBearerToken(headers http.Header) (string, error) {
	err := errors.New("header doesn't exist")
	token_String := headers.Get("Authorization")
	token_String = strings.TrimSpace(strings.TrimPrefix(token_String, "Bearer"))
	if len(token_String) != 0 {
		err = nil
	}
	return token_String, err
}
func GetAPIKey(headers http.Header) (string, error) {
	err := errors.New("header doesn't exist")
	token_String := headers.Get("Authorization")
	token_String = strings.TrimSpace(strings.TrimPrefix(token_String, "ApiKey"))
	if len(token_String) != 0 {
		err = nil
	}
	return token_String, err
}
func MakeRefreshToken() string {
	byt := make([]byte, 32)
	_, err := rand.Read(byt)
	if err != nil {
		log.Fatal(err)
	}
	randstring := hex.EncodeToString(byt)
	return randstring
}
