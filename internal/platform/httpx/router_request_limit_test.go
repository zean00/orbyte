package httpx

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestShouldLimitRequestBodyExemptsOfflineSync(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/offline/sync", bytes.NewBufferString(`{"items":[]}`))
	req.Header.Set("Content-Type", "application/json")
	if shouldLimitRequestBody(req) {
		t.Fatal("expected offline sync to bypass request body limit")
	}
}

func TestShouldLimitRequestBodyStillLimitsRegularJSONMutations(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/documents", bytes.NewBufferString(`{"title":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	if !shouldLimitRequestBody(req) {
		t.Fatal("expected regular JSON mutation to be limited")
	}
}
