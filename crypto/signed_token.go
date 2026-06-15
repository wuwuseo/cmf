package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func SignExpiringToken(secret, subject string, expiresAt time.Time) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("secret is required")
	}
	if subject == "" {
		return "", fmt.Errorf("subject is required")
	}

	encodedSubject := base64.RawURLEncoding.EncodeToString([]byte(subject))
	expires := strconv.FormatInt(expiresAt.Unix(), 10)
	message := encodedSubject + "." + expires
	signature := signTokenMessage(secret, message)
	return message + "." + signature, nil
}

func VerifyExpiringToken(secret, token string, now time.Time) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("secret is required")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid token")
	}

	message := parts[0] + "." + parts[1]
	want := signTokenMessage(secret, message)
	if subtle.ConstantTimeCompare([]byte(parts[2]), []byte(want)) != 1 {
		return "", fmt.Errorf("invalid token signature")
	}

	expires, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid token expiry: %w", err)
	}
	if now.Unix() > expires {
		return "", fmt.Errorf("token expired")
	}

	subjectBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid token subject: %w", err)
	}
	return string(subjectBytes), nil
}

func signTokenMessage(secret, message string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
