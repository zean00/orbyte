package main

import "testing"

func TestPromptAcceptedRequiresMatchingClientRequestID(t *testing.T) {
	session := acpSession{
		Messages: []sessionMessage{
			{ID: "msg-1", Role: "user", Content: "same prompt", Meta: map[string]any{"client_request_id": "req-1"}},
			{ID: "msg-2", Role: "assistant", Content: "answer"},
		},
	}
	if !promptAccepted(session, "req-1") {
		t.Fatal("expected matching client_request_id to be accepted")
	}
	if promptAccepted(session, "req-2") {
		t.Fatal("did not expect different client_request_id to be accepted")
	}
	if promptAccepted(session, "") {
		t.Fatal("did not expect empty client_request_id to be accepted")
	}
}

func TestPromptAcceptedDoesNotConflateRepeatedPromptText(t *testing.T) {
	session := acpSession{
		Messages: []sessionMessage{
			{ID: "msg-1", Role: "user", Content: "same prompt", Meta: map[string]any{"client_request_id": "req-1"}},
			{ID: "msg-2", Role: "user", Content: "same prompt", Meta: map[string]any{"client_request_id": "req-2"}},
		},
	}
	if !promptAccepted(session, "req-2") {
		t.Fatal("expected latest matching request id to be accepted")
	}
	if !promptAccepted(session, "req-1") {
		t.Fatal("expected older matching request id to be accepted")
	}
	if promptAccepted(session, "req-3") {
		t.Fatal("did not expect unmatched repeated prompt text to be accepted")
	}
}
