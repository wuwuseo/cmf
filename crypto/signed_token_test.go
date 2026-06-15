package crypto_test

import (
	"testing"
	"time"

	"github.com/wuwuseo/cmf/crypto"
)

func TestSignAndVerifyExpiringToken(t *testing.T) {
	now := time.Unix(1000, 0)
	token, err := crypto.SignExpiringToken("secret", "attachment:42", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("SignExpiringToken returned error: %v", err)
	}

	subject, err := crypto.VerifyExpiringToken("secret", token, now)
	if err != nil {
		t.Fatalf("VerifyExpiringToken returned error: %v", err)
	}
	if subject != "attachment:42" {
		t.Fatalf("expected attachment:42, got %q", subject)
	}
}

func TestVerifyExpiringTokenRejectsExpired(t *testing.T) {
	now := time.Unix(1000, 0)
	token, err := crypto.SignExpiringToken("secret", "attachment:42", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("SignExpiringToken returned error: %v", err)
	}

	if _, err := crypto.VerifyExpiringToken("secret", token, now.Add(2*time.Minute)); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestVerifyExpiringTokenRejectsTamperedToken(t *testing.T) {
	now := time.Unix(1000, 0)
	token, err := crypto.SignExpiringToken("secret", "attachment:42", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("SignExpiringToken returned error: %v", err)
	}

	tampered := token[:len(token)-1] + "x"
	if _, err := crypto.VerifyExpiringToken("secret", tampered, now); err == nil {
		t.Fatal("expected tampered token to be rejected")
	}
}
