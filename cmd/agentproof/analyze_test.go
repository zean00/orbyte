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

func TestVerifyDraftUsesDirectDocumentIDFromAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/documents/doc_123":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"header":{"id":"doc_123","number":"PR-1","status":"draft","version":1,"etag":"etag"},"body":{"payload":{"vendor_name":"North Roast Supply","lines":[{"item_code":"cold-brew","quantity":20},{"item_code":"oat-milk","quantity":16}]}}}`))
		case "/ui/data/documents":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := newAPIClient(server.URL)
	if err != nil {
		t.Fatalf("newAPIClient: %v", err)
	}
	result := promptAnalysisResult{
		Classification: "unacceptable",
		Answer:         "Draft ID: doc_123",
		MissingFacts:   []string{"draft_created", "expected_draft_document"},
	}
	verifyDraft(context.Background(), client, &result, draftExpectation{
		DocumentType:  "purchase_request",
		PayloadChecks: []string{"north roast", "cold brew", "20", "oat milk", "16"},
	})
	if !result.DraftVerified {
		t.Fatalf("expected direct draft id to verify, got %+v", result)
	}
	if result.DraftDocumentID != "doc_123" {
		t.Fatalf("draft document id = %q, want doc_123", result.DraftDocumentID)
	}
	if result.Classification != "exact" {
		t.Fatalf("classification = %q, want exact", result.Classification)
	}
	if len(result.MissingFacts) != 0 {
		t.Fatalf("expected missing facts to clear, got %+v", result.MissingFacts)
	}
}

func TestVerifyArtifactAcceptsSuccessfulDashboardPreviewTraceWhenProviderDropsArtifacts(t *testing.T) {
	result := promptAnalysisResult{Classification: "exact", Answer: "Referenced the relevant widgets."}
	session := sessionTranscript{}
	trace := []sessionTraceEvent{
		{
			ID:   "1",
			Kind: "session_update",
			Payload: map[string]any{
				"update_kind": "tool_call_update",
				"content": map[string]any{
					"status": "completed",
					"rawInput": map[string]any{
						"tool_id": "analytics.dashboard.widgets.preview",
						"payload": map[string]any{
							"widget_keys": []any{
								"analytics.demo.sales.target_attainment",
								"analytics.demo.sales.branch_mix",
								"analytics.demo.sales.daily_trend",
							},
						},
					},
					"rawOutput": map[string]any{
						"output": "Prepared 3 focused dashboard widget previews.",
					},
				},
			},
		},
	}
	verifyArtifact(&result, artifactExpectation{
		Kind:         "dashboard_widget",
		WidgetKeys:   []string{"target_attainment", "branch_mix", "daily_trend"},
		MinArtifacts: 3,
	}, session, trace)
	if !result.ArtifactVerified {
		t.Fatalf("expected artifact verification from preview trace, got %+v", result)
	}
	if !result.RequiredArtifactsPresent {
		t.Fatalf("expected required artifacts to be present, got %+v", result)
	}
	if result.Classification != "exact" {
		t.Fatalf("classification = %q, want exact", result.Classification)
	}
}

func TestVerifyArtifactRejectsBoardPreviewForWidgetExpectation(t *testing.T) {
	result := promptAnalysisResult{Classification: "exact", Answer: "Referenced the board."}
	trace := []sessionTraceEvent{
		{
			ID:   "1",
			Kind: "session_update",
			Payload: map[string]any{
				"update_kind": "tool_call_update",
				"content": map[string]any{
					"status": "completed",
					"rawInput": map[string]any{
						"tool_id": "analytics.dashboard.board.preview",
						"payload": map[string]any{},
					},
					"rawOutput": map[string]any{
						"output": "Prepared dashboard board preview Sales Dashboard. Widget keys: analytics.demo.sales.target_attainment, analytics.demo.sales.branch_mix, analytics.demo.sales.daily_trend.",
					},
				},
			},
		},
	}
	verifyArtifact(&result, artifactExpectation{
		Kind:         "dashboard_widget",
		WidgetKeys:   []string{"target_attainment", "branch_mix", "daily_trend"},
		MinArtifacts: 3,
	}, sessionTranscript{}, trace)
	if result.ArtifactVerified {
		t.Fatalf("expected board preview not to satisfy widget expectation, got %+v", result)
	}
}

func TestVerifyArtifactAcceptsInferredDashboardPreviewFromOutputText(t *testing.T) {
	result := promptAnalysisResult{Classification: "exact", Answer: "Referenced the relevant widgets."}
	trace := []sessionTraceEvent{
		{
			ID:   "1",
			Kind: "session_update",
			Payload: map[string]any{
				"update_kind": "tool_call_update",
				"content": map[string]any{
					"status": "completed",
					"rawInput": map[string]any{
						"tool_id": "analytics.dashboard.widgets.preview",
						"payload": map[string]any{
							"surface": "dashboard",
						},
					},
					"rawOutput": map[string]any{
						"output": "Prepared 3 focused dashboard widget previews using analytics.demo.sales.target_attainment, analytics.demo.sales.branch_mix, analytics.demo.sales.daily_trend. Artifact emission succeeded.",
					},
				},
			},
		},
	}
	verifyArtifact(&result, artifactExpectation{
		Kind:         "dashboard_widget",
		WidgetKeys:   []string{"target_attainment", "branch_mix", "daily_trend"},
		MinArtifacts: 3,
	}, sessionTranscript{}, trace)
	if !result.ArtifactVerified {
		t.Fatalf("expected inferred preview output text to satisfy artifact verification, got %+v", result)
	}
}
