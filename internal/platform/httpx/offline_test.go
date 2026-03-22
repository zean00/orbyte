package httpx

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestOfflineBootstrapFiltersCapabilities(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/offline/bootstrap", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var payload struct {
		SchemaVersion          string           `json:"schema_version"`
		PackageManifestVersion string           `json:"package_manifest_version"`
		CacheToken             string           `json:"cache_token"`
		References             []map[string]any `json:"references"`
		Projections            []map[string]any `json:"projections"`
		Documents              []map[string]any `json:"documents"`
		PackageManifest        []map[string]any `json:"package_manifest"`
		SyncCapabilities       map[string]any   `json:"sync_capabilities"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}
	if len(payload.References) == 0 {
		t.Fatal("expected offline references")
	}
	if len(payload.Projections) == 0 {
		t.Fatal("expected offline projections")
	}
	if len(payload.Documents) == 0 {
		t.Fatal("expected offline documents")
	}
	if payload.SchemaVersion == "" || payload.PackageManifestVersion == "" || payload.CacheToken == "" {
		t.Fatalf("expected enriched bootstrap metadata, got %+v", payload)
	}
	if len(payload.PackageManifest) == 0 || payload.SyncCapabilities["queue_model"] == nil {
		t.Fatalf("expected package manifest and sync capabilities, got %+v", payload)
	}
}

func TestOfflineReferencePackage(t *testing.T) {
	h := newTestHarness(t)

	body, _ := json.Marshal(map[string]any{"type_key": "appointment_type", "organization_id": "org_default"})
	rr := h.request(http.MethodPost, "/offline/packages/references", body, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var payload struct {
		ResolvedSet struct {
			TypeKey string           `json:"type_key"`
			Items   []map[string]any `json:"items"`
		} `json:"resolved_set"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ResolvedSet.TypeKey != "appointment_type" {
		t.Fatalf("expected appointment_type, got %s", payload.ResolvedSet.TypeKey)
	}
	if len(payload.ResolvedSet.Items) == 0 {
		t.Fatal("expected reference items")
	}
}

func TestOfflineProjectionPackage(t *testing.T) {
	h := newTestHarness(t)

	created := h.request(http.MethodPost, "/documents", mustJSON(t, map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "Offline Intake"},
	}), true)
	if created.Code != http.StatusCreated {
		t.Fatalf("create document failed: %d body=%s", created.Code, created.Body.String())
	}
	if _, err := h.search.RebuildIndex("documents.requests.search"); err != nil {
		t.Fatalf("rebuild index failed: %v", err)
	}

	rr := h.request(http.MethodPost, "/offline/packages/projections", mustJSON(t, map[string]any{
		"index_key":       "documents.requests.search",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"query":           map[string]any{"query": "Offline Intake", "page_size": 10},
	}), true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var payload struct {
		Result struct {
			Total int `json:"total"`
			Hits  []struct {
				SourceID string `json:"source_id"`
			} `json:"hits"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Result.Total == 0 || len(payload.Result.Hits) == 0 {
		t.Fatal("expected projection hits")
	}
}

func TestOfflineSyncCreateAndConflict(t *testing.T) {
	h := newTestHarness(t)

	create := h.request(http.MethodPost, "/offline/sync", mustJSON(t, map[string]any{
		"items": []map[string]any{{
			"idempotency_key": "sync-create-1",
			"kind":            "document",
			"operation":       "create",
			"document_type":   "generic_request",
			"organization_id": "org_default",
			"location_id":     "loc_hq",
			"payload":         map[string]any{"title": "Offline Draft"},
		}},
	}), true)
	if create.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", create.Code, create.Body.String())
	}

	var createPayload struct {
		BatchID string `json:"batch_id"`
		Items   []struct {
			Status   string `json:"status"`
			TargetID string `json:"target_id"`
			Version  int    `json:"version"`
			ETag     string `json:"etag"`
			BatchID  string `json:"batch_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if len(createPayload.Items) != 1 || createPayload.Items[0].Status != "accepted" {
		t.Fatalf("expected accepted sync result, got %+v", createPayload.Items)
	}
	if createPayload.BatchID == "" || createPayload.Items[0].BatchID == "" {
		t.Fatalf("expected batch metadata, got %+v", createPayload)
	}

	conflict := h.request(http.MethodPost, "/offline/sync", mustJSON(t, map[string]any{
		"items": []map[string]any{{
			"idempotency_key":  "sync-update-conflict-1",
			"kind":             "document",
			"operation":        "update",
			"document_type":    "generic_request",
			"target_id":        createPayload.Items[0].TargetID,
			"expected_version": createPayload.Items[0].Version + 10,
			"expected_etag":    createPayload.Items[0].ETag,
			"payload":          map[string]any{"title": "Stale Update"},
		}},
	}), true)
	if conflict.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", conflict.Code, conflict.Body.String())
	}

	var conflictPayload struct {
		Items []struct {
			Status   string `json:"status"`
			Conflict struct {
				Current           map[string]any `json:"current"`
				Attempted         map[string]any `json:"attempted"`
				ResolutionOptions []string       `json:"resolution_options"`
			} `json:"conflict"`
		} `json:"items"`
	}
	if err := json.Unmarshal(conflict.Body.Bytes(), &conflictPayload); err != nil {
		t.Fatalf("decode conflict response: %v", err)
	}
	if len(conflictPayload.Items) != 1 || conflictPayload.Items[0].Status != "conflict" {
		t.Fatalf("expected conflict sync result, got %+v", conflictPayload.Items)
	}
	if conflictPayload.Items[0].Conflict.Current["id"] == nil {
		t.Fatal("expected conflict metadata")
	}
	if conflictPayload.Items[0].Conflict.Attempted["payload"] == nil || len(conflictPayload.Items[0].Conflict.ResolutionOptions) == 0 {
		t.Fatalf("expected structured conflict details, got %+v", conflictPayload.Items[0].Conflict)
	}
}

func mustJSON(t *testing.T, payload any) []byte {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return body
}
