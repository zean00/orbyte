package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTraceWindowForPromptCountsOnlyCurrentTurnToolCalls(t *testing.T) {
	trace := []sessionTraceEvent{
		{ID: "1", Kind: "session_started"},
		{ID: "2", Kind: "user_message", Payload: map[string]any{"content": "first question"}},
		{ID: "3", Kind: "tool_call"},
		{ID: "4", Kind: "turn_completed"},
		{ID: "5", Kind: "user_message", Payload: map[string]any{"content": "second question"}},
		{ID: "6", Kind: "session_update", Payload: map[string]any{"update_kind": "tool_call"}},
		{ID: "7", Kind: "session_update", Payload: map[string]any{"update_kind": "agent_message_chunk"}},
		{ID: "8", Kind: "turn_completed"},
	}

	firstWindow, nextCursor := traceWindowForPrompt("first question", trace, 0)
	if got := countToolCalls(firstWindow); got != 1 {
		t.Fatalf("first prompt tool count = %d, want 1", got)
	}
	secondWindow, _ := traceWindowForPrompt("second question", trace, nextCursor)
	if got := countToolCalls(secondWindow); got != 1 {
		t.Fatalf("second prompt tool count = %d, want 1", got)
	}
}

func TestTraceWindowForPromptMissKeepsCursor(t *testing.T) {
	trace := []sessionTraceEvent{{ID: "1", Kind: "user_message", Payload: map[string]any{"content": "known"}}}
	window, cursor := traceWindowForPrompt("missing", trace, 0)
	if len(window) != 0 {
		t.Fatalf("unexpected window length %d", len(window))
	}
	if cursor != 0 {
		t.Fatalf("cursor = %d, want 0", cursor)
	}
}

func TestVerifyDraftMarksResultWhenExpectedDraftExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ui/data/documents" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"header":{"id":"doc-1","type":"generic_request","status":"draft","number":"REQ-1"},"body":{"payload":{"title":"Promotion Plan 20260404-120000","summary":"Bundle espresso and croissant for gold members. Replace Beans Boost."}}}]}`))
	}))
	defer server.Close()

	client, err := newAPIClient(server.URL)
	if err != nil {
		t.Fatalf("newAPIClient: %v", err)
	}
	result := promptAnalysisResult{Classification: "exact"}
	verifyDraft(context.Background(), client, &result, draftExpectation{
		DocumentType:  "generic_request",
		TitleChecks:   []string{"Promotion Plan 20260404-120000"},
		PayloadChecks: []string{"espresso", "croissant", "gold", "beans boost"},
	})
	if !result.DraftVerified {
		t.Fatalf("expected draft to verify, got %+v", result)
	}
	if result.DraftDocumentID != "doc-1" {
		t.Fatalf("draft document id = %q, want doc-1", result.DraftDocumentID)
	}
}

func TestVerifyDraftMarksResultUnacceptableWhenExpectedDraftMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	client, err := newAPIClient(server.URL)
	if err != nil {
		t.Fatalf("newAPIClient: %v", err)
	}
	result := promptAnalysisResult{Classification: "exact"}
	verifyDraft(context.Background(), client, &result, draftExpectation{
		DocumentType: "generic_request",
		TitleChecks:  []string{"Promotion Plan 20260404-120000"},
	})
	if result.Classification != "unacceptable" {
		t.Fatalf("classification = %q, want unacceptable", result.Classification)
	}
	if result.DraftVerified {
		t.Fatalf("expected draft verification to fail, got %+v", result)
	}
}

func TestContainsCheckMatchesRunSuffixedNamesByBaseName(t *testing.T) {
	answer := "Run a bundle campaign for Butter Croissant + Espresso Double targeting gold members."
	if !containsCheck(answer, "Espresso Double 20260404-001343") {
		t.Fatal("expected run-suffixed product name to match base product mention")
	}
	if !containsCheck(answer, "Butter Croissant 20260404-001343") {
		t.Fatal("expected run-suffixed croissant name to match base product mention")
	}
}

func TestContainsCheckMatchesWordNumbers(t *testing.T) {
	answer := "Beans Boost had only 1 redemption and should be replaced."
	if !containsCheck(answer, "one") {
		t.Fatal("expected numeric digit to match word-number check")
	}
}

func TestVerifyDraftClearsDraftTitleRequirementWhenArtifactVerified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"header":{"id":"doc-1","type":"generic_request","status":"draft","number":"REQ-1"},"body":{"payload":{"title":"Promotion Plan 20260404-120000","summary":"Bundle espresso and croissant for gold members. Replace Beans Boost."}}}]}`))
	}))
	defer server.Close()

	client, err := newAPIClient(server.URL)
	if err != nil {
		t.Fatalf("newAPIClient: %v", err)
	}
	result := promptAnalysisResult{
		Classification: "unacceptable",
		MissingFacts:   []string{"draft_title"},
	}
	verifyDraft(context.Background(), client, &result, draftExpectation{
		DocumentType:  "generic_request",
		TitleChecks:   []string{"Promotion Plan 20260404-120000"},
		PayloadChecks: []string{"espresso", "croissant", "gold", "beans boost"},
	})
	if result.Classification != "exact" {
		t.Fatalf("classification = %q, want exact", result.Classification)
	}
	if len(result.MissingFacts) != 0 {
		t.Fatalf("expected no missing facts after verified draft, got %+v", result.MissingFacts)
	}
}
