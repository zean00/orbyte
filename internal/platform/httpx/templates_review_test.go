package httpx

import (
	"encoding/json"
	"testing"

	"orbyte/internal/platform/templateoutput"
)

func TestDefaultTemplateBodyEscapesFreeFormTitle(t *testing.T) {
	body, err := defaultTemplateBody(templateoutput.Definition{
		Key:        "documents.invoice.special",
		Title:      "Invoice \"Special\"\\nBatch",
		TargetKind: "document",
		TargetKey:  "invoice",
	})
	if err != nil {
		t.Fatalf("default template body failed: %v", err)
	}

	var visual map[string]any
	if err := json.Unmarshal([]byte(body), &visual); err != nil {
		t.Fatalf("expected valid json body, got error: %v\nbody=%s", err, body)
	}
	if got := visual["title"]; got != "Invoice \"Special\"\\nBatch" {
		t.Fatalf("expected escaped title to round-trip, got %#v", got)
	}
}
