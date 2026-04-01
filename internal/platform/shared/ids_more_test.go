package shared

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
)

type failingEntropy struct{}

func (failingEntropy) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func TestIDServiceFormattingAndDefaults(t *testing.T) {
	now := func() time.Time { return time.Unix(1700000000, 0).UTC() }
	svc := NewIDServiceWithEntropy(now, strings.NewReader(strings.Repeat("a", 64)))
	if svc == nil {
		t.Fatal("expected id service")
	}

	plain := svc.New("")
	if _, err := ulid.Parse(plain); err != nil {
		t.Fatalf("expected bare ulid, got %q: %v", plain, err)
	}

	withColon := svc.New(" user ")
	if !strings.HasPrefix(withColon, "user:") {
		t.Fatalf("expected colon-prefixed id, got %q", withColon)
	}
	if _, err := ulid.Parse(strings.TrimPrefix(withColon, "user:")); err != nil {
		t.Fatalf("expected ulid suffix, got %q: %v", withColon, err)
	}

	withUnderscore := svc.New("evt_")
	if !strings.HasPrefix(withUnderscore, "evt_") {
		t.Fatalf("expected underscore-prefixed id, got %q", withUnderscore)
	}
	if _, err := ulid.Parse(strings.TrimPrefix(withUnderscore, "evt_")); err != nil {
		t.Fatalf("expected ulid suffix, got %q: %v", withUnderscore, err)
	}

	child := svc.Child(":root:", " section ", "", ":item:")
	parts := strings.Split(child, ":")
	if len(parts) != 4 {
		t.Fatalf("expected root, section, item, ulid parts; got %q", child)
	}
	if parts[0] != "root" || parts[1] != "section" || parts[2] != "item" {
		t.Fatalf("unexpected child id format: %q", child)
	}
	if _, err := ulid.Parse(parts[3]); err != nil {
		t.Fatalf("expected ulid suffix, got %q: %v", child, err)
	}
}

func TestIDServicePackageHelpers(t *testing.T) {
	if DefaultIDService() == nil {
		t.Fatal("expected default id service")
	}
	if got := NewID("session"); !strings.HasPrefix(got, "session:") {
		t.Fatalf("expected prefixed package-level id, got %q", got)
	}
	if got := ChildID("session", "child"); !strings.HasPrefix(got, "session:child:") {
		t.Fatalf("expected package-level child id, got %q", got)
	}
}

func TestNewIDServiceWithEntropyFallbacks(t *testing.T) {
	svc := NewIDServiceWithEntropy(nil, nil)
	if svc == nil || svc.now == nil || svc.entropy == nil {
		t.Fatal("expected fallback dependencies to be installed")
	}
}

func TestIDServicePanicsWhenEntropyFails(t *testing.T) {
	svc := NewIDServiceWithEntropy(func() time.Time { return time.Now().UTC() }, failingEntropy{})
	defer func() {
		if recover() == nil {
			t.Fatal("expected ulid generation panic when entropy fails")
		}
	}()
	_ = svc.New("panic")
}
