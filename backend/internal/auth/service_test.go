package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("valid password did not verify")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Fatal("wrong password verified")
	}
}

func TestAccessTokenRoundTrip(t *testing.T) {
	service := New("01234567890123456789012345678901", time.Minute, time.Hour)
	userID, sessionID := uuid.New(), uuid.New()
	access, _, _, _, err := service.NewSessionTokens(userID, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	gotUser, gotSession, err := service.ParseAccessToken(access)
	if err != nil || gotUser != userID || gotSession != sessionID {
		t.Fatalf("unexpected token parse: %s %s %v", gotUser, gotSession, err)
	}
}
