package main

import (
	"testing"
	"time"
)

func TestDurationFromEnv(t *testing.T) {
	t.Run("uses fallback when unset", func(t *testing.T) {
		t.Setenv("APP_HTTP_TEST_TIMEOUT", "")
		if got := durationFromEnv("APP_HTTP_TEST_TIMEOUT", 7*time.Second); got != 7*time.Second {
			t.Fatalf("expected fallback, got %v", got)
		}
	})

	t.Run("parses duration strings", func(t *testing.T) {
		t.Setenv("APP_HTTP_TEST_TIMEOUT", "45s")
		if got := durationFromEnv("APP_HTTP_TEST_TIMEOUT", time.Second); got != 45*time.Second {
			t.Fatalf("expected parsed duration, got %v", got)
		}
	})

	t.Run("parses integer seconds", func(t *testing.T) {
		t.Setenv("APP_HTTP_TEST_TIMEOUT", "12")
		if got := durationFromEnv("APP_HTTP_TEST_TIMEOUT", time.Second); got != 12*time.Second {
			t.Fatalf("expected integer second duration, got %v", got)
		}
	})

	t.Run("falls back on invalid values", func(t *testing.T) {
		t.Setenv("APP_HTTP_TEST_TIMEOUT", "-5")
		if got := durationFromEnv("APP_HTTP_TEST_TIMEOUT", 9*time.Second); got != 9*time.Second {
			t.Fatalf("expected fallback for invalid value, got %v", got)
		}
		t.Setenv("APP_HTTP_TEST_TIMEOUT", "nonsense")
		if got := durationFromEnv("APP_HTTP_TEST_TIMEOUT", 9*time.Second); got != 9*time.Second {
			t.Fatalf("expected fallback for invalid string, got %v", got)
		}
	})
}
