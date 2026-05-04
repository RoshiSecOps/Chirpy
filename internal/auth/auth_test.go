package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMakeAndValidateJWT(t *testing.T) {
	userId := uuid.New()
	tokenSecret := "my-test-secret"
	expiresIn := time.Hour
	token, err := MakeJWT(userId, tokenSecret, expiresIn)
	if err != nil {
		t.Fatalf("MakeJWT failed: %v", err)
	}
	id, err := ValidateJWT(token, tokenSecret)
	if err != nil {
		t.Fatalf("ValidateJWT failed: %v", err)
	}
	if id != userId {
		t.Errorf("expected %v, got %v", userId, id)
	}
}

func TestExpiredJWT(t *testing.T) {
	userId := uuid.New()
	tokenSecret := "my-test-secret"
	expiresIn := -time.Hour
	token, err := MakeJWT(userId, tokenSecret, expiresIn)
	if err != nil {
		t.Fatalf("MakeJWT failed: %v", err)
	}
	id, err := ValidateJWT(token, tokenSecret)
	if err == nil {
		t.Errorf("expected error for expired token, got token: %v", id)
	}
}

func TestWrongSecretJWT(t *testing.T) {
	userId := uuid.New()
	tokenSecret := "my-test-secret"
	expiresIn := time.Hour
	token, err := MakeJWT(userId, tokenSecret, expiresIn)
	if err != nil {
		t.Fatalf("MakeJWT failed: %v", err)
	}
	_, err = ValidateJWT(token, "wrong-secret")
	if err == nil {
		t.Errorf("expected error for expired token, got nil")
	}
}
