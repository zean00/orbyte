package communication

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSendPlainTextEmailValidatesRecipient(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	if _, err := SendPlainTextEmail("subject", "body", " "); err == nil {
		t.Fatal("expected missing recipient validation error")
	}
	if _, err := SendPlainTextEmail("subject", "body", "not-an-email"); err == nil {
		t.Fatal("expected invalid recipient validation error")
	}
}

func TestSendPlainTextEmailWritesOutboxMessage(t *testing.T) {
	outbox := t.TempDir()
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_FROM", "sender@example.com")
	t.Setenv("WORKFLOW_EMAIL_OUTBOX_DIR", outbox)

	delivery, err := SendPlainTextEmail("  Workflow Notice  ", "hello world", "recipient@example.com")
	if err != nil {
		t.Fatalf("SendPlainTextEmail failed: %v", err)
	}
	if delivery.Mode != "outbox" || delivery.Path == "" {
		t.Fatalf("expected outbox delivery, got %+v", delivery)
	}
	if !strings.HasPrefix(delivery.Path, outbox+string(os.PathSeparator)) {
		t.Fatalf("expected delivery path inside outbox, got %q", delivery.Path)
	}
	content, err := os.ReadFile(delivery.Path)
	if err != nil {
		t.Fatalf("read outbox email: %v", err)
	}
	body := string(content)
	for _, expected := range []string{
		"From: sender@example.com",
		"To: recipient@example.com",
		"Subject: Workflow Notice",
		"hello world",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected email body to contain %q, got %s", expected, body)
		}
	}
}

func TestBuildPlainTextEmailAndHelpers(t *testing.T) {
	message, err := buildPlainTextEmail("sender@example.com", "recipient@example.com", "subject", "body")
	if err != nil {
		t.Fatalf("buildPlainTextEmail failed: %v", err)
	}
	rendered := string(message)
	if !strings.Contains(rendered, "Content-Type: multipart/alternative; boundary=") {
		t.Fatalf("expected multipart content type, got %s", rendered)
	}
	if !strings.Contains(rendered, "text/plain; charset=utf-8") {
		t.Fatalf("expected plain text part, got %s", rendered)
	}
	if got := firstNonEmpty(" ", "\n", " value "); got != "value" {
		t.Fatalf("expected trimmed first value, got %q", got)
	}
	if got := firstNonEmpty("", " "); got != "" {
		t.Fatalf("expected empty fallback, got %q", got)
	}
}

func TestSendPlainTextEmailUsesTemporaryOutboxWhenUnset(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_FROM", "sender@example.com")
	t.Setenv("WORKFLOW_EMAIL_OUTBOX_DIR", "")

	delivery, err := SendPlainTextEmail("subject", "body", "recipient@example.com")
	if err != nil {
		t.Fatalf("SendPlainTextEmail failed: %v", err)
	}
	if delivery.Mode != "outbox" {
		t.Fatalf("expected outbox mode, got %+v", delivery)
	}
	if filepath.Ext(delivery.Path) != ".eml" {
		t.Fatalf("expected .eml output, got %q", delivery.Path)
	}
}
