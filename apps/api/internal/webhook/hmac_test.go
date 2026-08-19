package webhook_test

import (
	"strings"
	"testing"

	"github.com/portfolio/pf-reliability/apps/api/internal/webhook"
)

func TestSignAndVerify(t *testing.T) {
	body := []byte(`{"dedup_key":"commerce-inventory-5xx"}`)
	sig := webhook.Sign("dev-secret", body)
	if err := webhook.Verify("dev-secret", body, sig); err != nil {
		t.Fatal(err)
	}
	if err := webhook.Verify("dev-secret", body, strings.TrimPrefix(sig, "sha256=")); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyRejectsWrongSecretAndTamper(t *testing.T) {
	body := []byte(`{"summary":"5xx"}`)
	sig := webhook.Sign("dev-secret", body)
	if err := webhook.Verify("other", body, sig); err != webhook.ErrBadSignature {
		t.Fatalf("wrong secret: %v", err)
	}
	if err := webhook.Verify("dev-secret", []byte(`{"summary":"tampered"}`), sig); err != webhook.ErrBadSignature {
		t.Fatalf("tamper: %v", err)
	}
}

func TestVerifyMissing(t *testing.T) {
	if err := webhook.Verify("dev-secret", []byte(`{}`), ""); err != webhook.ErrMissingSignature {
		t.Fatalf("got %v", err)
	}
	if err := webhook.Verify("", []byte(`{}`), "sha256=00"); err != webhook.ErrMissingSecret {
		t.Fatalf("got %v", err)
	}
}

func TestVerifyMalformedHeader(t *testing.T) {
	if err := webhook.Verify("dev-secret", []byte(`{}`), "sha256=zzzz"); err != webhook.ErrBadSignature {
		t.Fatalf("got %v", err)
	}
	if err := webhook.Verify("dev-secret", []byte(`{}`), "sha256=ab"); err != webhook.ErrBadSignature {
		t.Fatalf("short hex: %v", err)
	}
}
