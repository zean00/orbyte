package communication

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSendPlainTextEmailUsesDefaultSubjectWhenBlank(t *testing.T) {
	outbox := t.TempDir()
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_FROM", "sender@example.com")
	t.Setenv("WORKFLOW_EMAIL_OUTBOX_DIR", outbox)

	delivery, err := SendPlainTextEmail(" \n ", "hello fallback", "recipient@example.com")
	if err != nil {
		t.Fatalf("SendPlainTextEmail failed: %v", err)
	}
	content, err := os.ReadFile(delivery.Path)
	if err != nil {
		t.Fatalf("read outbox email: %v", err)
	}
	if !strings.Contains(string(content), "Subject: Workflow notification") {
		t.Fatalf("expected default subject in email, got %s", string(content))
	}
}

func TestSendPlainTextEmailFailsWhenOutboxPathIsAFile(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(filePath, []byte("occupied"), 0o644); err != nil {
		t.Fatalf("seed outbox file failed: %v", err)
	}

	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_FROM", "sender@example.com")
	t.Setenv("WORKFLOW_EMAIL_OUTBOX_DIR", filePath)

	if _, err := SendPlainTextEmail("subject", "body", "recipient@example.com"); err == nil {
		t.Fatal("expected outbox directory creation failure")
	}
}

func TestSendPlainTextEmailSMTPFailures(t *testing.T) {
	t.Setenv("SMTP_FROM", "sender@example.com")
	t.Setenv("SMTP_HOST", "127.0.0.1")
	t.Setenv("SMTP_PORT", "1")
	t.Setenv("SMTP_USERNAME", "")
	t.Setenv("SMTP_PASSWORD", "")
	t.Setenv("WORKFLOW_EMAIL_OUTBOX_DIR", "")

	t.Setenv("SMTP_TLS", "false")
	if _, err := SendPlainTextEmail("subject", "body", "recipient@example.com"); err == nil {
		t.Fatal("expected smtp send failure")
	}

	t.Setenv("SMTP_TLS", "true")
	if _, err := SendPlainTextEmail("subject", "body", "recipient@example.com"); err == nil {
		t.Fatal("expected smtp tls send failure")
	}
}

func TestSendMailTLSFailsFastOnUnreachableServer(t *testing.T) {
	if err := sendMailTLS("127.0.0.1:1", "127.0.0.1", nil, "sender@example.com", []string{"recipient@example.com"}, []byte("body")); err == nil {
		t.Fatal("expected tls dial failure")
	}
}
