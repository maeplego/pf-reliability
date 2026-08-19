package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
)

var (
	ErrMissingSignature = errors.New("missing webhook signature")
	ErrBadSignature     = errors.New("invalid webhook signature")
	ErrMissingSecret    = errors.New("webhook secret is empty")
)

const HeaderSignature = "X-Signature-256"

const MaxBodyBytes = 64 * 1024

func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func Verify(secret string, body []byte, header string) error {
	if strings.TrimSpace(secret) == "" {
		return ErrMissingSecret
	}
	got := strings.TrimSpace(header)
	if got == "" {
		return ErrMissingSignature
	}
	got = strings.TrimPrefix(strings.ToLower(got), "sha256=")
	wantHex := strings.TrimPrefix(strings.ToLower(Sign(secret, body)), "sha256=")
	gotBytes, err := hex.DecodeString(got)
	if err != nil {
		return ErrBadSignature
	}
	wantBytes, err := hex.DecodeString(wantHex)
	if err != nil {
		return ErrBadSignature
	}
	if len(gotBytes) != len(wantBytes) {
		return ErrBadSignature
	}
	if subtle.ConstantTimeCompare(gotBytes, wantBytes) != 1 {
		return ErrBadSignature
	}
	return nil
}
