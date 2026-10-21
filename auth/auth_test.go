package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWTRoundTrip(t *testing.T) {
	nuser_id := uuid.New()
	token, err := MakeJWT(nuser_id, "test-secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	user_id, err := ValidateJWT(token, "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	if nuser_id != user_id {
		t.Fatalf("got %v, want %v", user_id, nuser_id)
	}
}
func TestJWTExpired(t *testing.T) {
	nuser_id := uuid.New()
	token, err := MakeJWT(nuser_id, "test-secret", -time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ValidateJWT(token, "test-secret")
	if err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}
func TestJWTWrongSecret(t *testing.T) {
	nuser_id := uuid.New()
	token, err := MakeJWT(nuser_id, "test-secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ValidateJWT(token, "secret-test")
	if err == nil {
		t.Fatal("expected wrong token secret to be rejected")
	}
}