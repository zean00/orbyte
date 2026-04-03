package main

import "testing"

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
