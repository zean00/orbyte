package main

import (
	"os"
	"testing"
	"time"
)

func TestDurationFromEnv(t *testing.T) {
	t.Setenv("APP_HTTP_TEST_TIMEOUT", "")
	if got := durationFromEnv("APP_HTTP_TEST_TIMEOUT", 15*time.Second); got != 15*time.Second {
		t.Fatalf("expected fallback duration, got %s", got)
	}

	t.Setenv("APP_HTTP_TEST_TIMEOUT", "45s")
	if got := durationFromEnv("APP_HTTP_TEST_TIMEOUT", 15*time.Second); got != 45*time.Second {
		t.Fatalf("expected parsed duration, got %s", got)
	}

	t.Setenv("APP_HTTP_TEST_TIMEOUT", "12")
	if got := durationFromEnv("APP_HTTP_TEST_TIMEOUT", 15*time.Second); got != 12*time.Second {
		t.Fatalf("expected numeric seconds duration, got %s", got)
	}

	t.Setenv("APP_HTTP_TEST_TIMEOUT", "-5")
	if got := durationFromEnv("APP_HTTP_TEST_TIMEOUT", 15*time.Second); got != 15*time.Second {
		t.Fatalf("expected fallback on negative seconds, got %s", got)
	}

	t.Setenv("APP_HTTP_TEST_TIMEOUT", "invalid")
	if got := durationFromEnv("APP_HTTP_TEST_TIMEOUT", 15*time.Second); got != 15*time.Second {
		t.Fatalf("expected fallback on invalid value, got %s", got)
	}

	_ = os.Unsetenv("APP_HTTP_TEST_TIMEOUT")
}
