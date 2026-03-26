package httpx

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/idempotency"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/logging"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/offline"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/shared"
)

type offlineReferencePackageRequest struct {
	TypeKey        string `json:"type_key"`
	OrganizationID string `json:"organization_id,omitempty"`
	LocationID     string `json:"location_id,omitempty"`
}

type offlineProjectionPackageRequest struct {
	IndexKey       string `json:"index_key"`
	OrganizationID string `json:"organization_id,omitempty"`
	LocationID     string `json:"location_id,omitempty"`
}

type offlineSyncRequest struct {
	Items []offlineSyncItem `json:"items"`
}

type offlineSyncItem struct {
	IdempotencyKey  string                           `json:"idempotency_key,omitempty"`
	Kind            string                           `json:"kind"`
	Operation       string                           `json:"operation"`
	DocumentType    string                           `json:"document_type,omitempty"`
	ModelKey        string                           `json:"model_key,omitempty"`
	OrganizationID  string                           `json:"organization_id,omitempty"`
	LocationID      string                           `json:"location_id,omitempty"`
	TargetID        string                           `json:"target_id,omitempty"`
	ExpectedVersion int                              `json:"expected_version,omitempty"`
	ExpectedETag    string                           `json:"expected_etag,omitempty"`
	Payload         map[string]any                   `json:"payload,omitempty"`
	Values          map[string]any                   `json:"values,omitempty"`
	Relations       map[string][]model.ChildMutation `json:"relations,omitempty"`
}

func registerOfflineRoutes(mux *http.ServeMux, ident *identity.Service, modules *module.Service, offlineSvc *offline.Service, docs *document.Service, docActions *application.DocumentActions, models *model.Service, modelActions *application.ModelActions, searchSvc *search.Service, fieldSecurity *securityfields.Service, idempotencySvc *idempotency.Service) {
	mux.HandleFunc("GET /offline/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthenticatedPrincipal(w, r)
		if !ok {
			return
		}
		bootstrap := offlineSvc.Bootstrap()
		allow := func(required []string) bool {
			return principalAllowsAll(ident, p, required)
		}
		bootstrap.References = offline.FilterReferenceCapabilities(bootstrap.References, allow)
		bootstrap.Projections = offline.FilterProjectionCapabilities(bootstrap.Projections, allow)
		bootstrap.Documents = offline.FilterDocumentCapabilities(bootstrap.Documents, allow)
		bootstrap.Models = offline.FilterModelCapabilities(bootstrap.Models, allow)
		bootstrap.PackageManifest = offlineSvc.BuildPackageManifest("", effectiveOfflineLocation("", p), bootstrap.References, bootstrap.Projections)
		bootstrap.CacheToken = offlineBootstrapToken(bootstrap)
		respondJSON(w, http.StatusOK, bootstrap)
	})

	mux.HandleFunc("POST /offline/packages/references", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthenticatedPrincipal(w, r)
		if !ok {
			return
		}
		var req offlineReferencePackageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid offline reference package request"))
			return
		}
		def, ok := modules.OfflineReference(strings.TrimSpace(req.TypeKey))
		if !ok {
			respondError(w, shared.NotFound("offline reference package is not registered"))
			return
		}
		if !principalAllowsAll(ident, p, def.RequiredPermissions) {
			respondError(w, shared.Forbidden("access denied"))
			return
		}
		locationID := effectiveOfflineLocation(req.LocationID, p)
		pkg, err := offlineSvc.ReferencePackage(req.TypeKey, req.OrganizationID, locationID, time.Now().UTC())
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, pkg)
	})

	mux.HandleFunc("POST /offline/packages/projections", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthenticatedPrincipal(w, r)
		if !ok {
			return
		}
		var raw struct {
			IndexKey       string         `json:"index_key"`
			OrganizationID string         `json:"organization_id,omitempty"`
			LocationID     string         `json:"location_id,omitempty"`
			Query          map[string]any `json:"query,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			respondError(w, shared.Validation("invalid offline projection package request"))
			return
		}
		def, ok := modules.OfflineProjection(strings.TrimSpace(raw.IndexKey))
		if !ok {
			respondError(w, shared.NotFound("offline projection package is not registered"))
			return
		}
		if !principalAllowsAll(ident, p, def.RequiredPermissions) {
			respondError(w, shared.Forbidden("access denied"))
			return
		}
		req := decodeOfflineProjectionQuery(raw.Query)
		locationID := effectiveOfflineLocation(raw.LocationID, p)
		pkg, err := offlineSvc.ProjectionPackage(raw.IndexKey, raw.OrganizationID, locationID, req)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, pkg)
	})

	mux.HandleFunc("POST /offline/sync", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthenticatedPrincipal(w, r)
		if !ok {
			return
		}
		var req offlineSyncRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid offline sync request"))
			return
		}
		batch := offlineSvc.StartBatch(logging.CorrelationID(r.Context()), principalEffectiveUserID(p), strings.TrimSpace(r.Header.Get("X-Offline-Device-ID")), len(req.Items))
		results := make([]offline.SyncResultItem, 0, len(req.Items))
		for _, item := range req.Items {
			outcome, err := idempotencySvc.Execute("offline.sync:"+strings.TrimSpace(item.Kind)+":"+strings.TrimSpace(item.Operation), item.IdempotencyKey, principalActorID(p), item, func() (idempotency.Outcome, error) {
				result := applyOfflineSyncItem(ident, p, modules, docs, docActions, models, modelActions, searchSvc, fieldSecurity, item)
				return idempotency.Outcome{StatusCode: http.StatusOK, Response: map[string]any{"item": result}}, nil
			})
			if err != nil {
				respondError(w, err)
				return
			}
			var result offline.SyncResultItem
			encoded, _ := json.Marshal(outcome.Response["item"])
			_ = json.Unmarshal(encoded, &result)
			result.BatchID = batch.ID
			result.CorrelationID = batch.CorrelationID
			result.DeviceID = batch.DeviceID
			if result.ProcessedAt.IsZero() {
				result.ProcessedAt = time.Now().UTC()
			}
			if result.AttemptCount <= 0 {
				result.AttemptCount = 1
			}
			offlineSvc.RecordOutcome(&batch, result)
			results = append(results, result)
		}
		respondJSON(w, http.StatusOK, map[string]any{"batch_id": batch.ID, "items": results})
	})
}

func applyOfflineSyncItem(ident *identity.Service, p principal, modules *module.Service, docs *document.Service, docActions *application.DocumentActions, models *model.Service, modelActions *application.ModelActions, searchSvc *search.Service, fieldSecurity *securityfields.Service, item offlineSyncItem) offline.SyncResultItem {
	result := offline.SyncResultItem{
		IdempotencyKey: item.IdempotencyKey,
		Kind:           strings.TrimSpace(item.Kind),
		Operation:      strings.TrimSpace(item.Operation),
		TargetID:       strings.TrimSpace(item.TargetID),
		Status:         offline.StatusFailedTerminal,
		AttemptCount:   1,
		ProcessedAt:    time.Now().UTC(),
	}
	switch result.Kind {
	case "document":
		return applyOfflineDocumentSync(ident, p, modules, docs, docActions, searchSvc, fieldSecurity, item, result)
	case "model":
		return applyOfflineModelSync(ident, p, modules, models, modelActions, fieldSecurity, item, result)
	default:
		result.Error = "unsupported offline kind"
		result.ErrorCode = "unsupported_kind"
		return result
	}
}

func applyOfflineDocumentSync(ident *identity.Service, p principal, modules *module.Service, docs *document.Service, docActions *application.DocumentActions, searchSvc *search.Service, fieldSecurity *securityfields.Service, item offlineSyncItem, result offline.SyncResultItem) offline.SyncResultItem {
	def, ok := modules.OfflineDocument(strings.TrimSpace(item.DocumentType))
	if !ok {
		result.Error = "offline document type is not registered"
		return result
	}
	switch strings.TrimSpace(item.Operation) {
	case "create":
		required := append([]string(nil), def.RequiredPermissions...)
		if def.CreatePermissionKey != "" {
			required = append(required, def.CreatePermissionKey)
		}
		if !principalAllowsAll(ident, p, required) {
			result.Status = offline.StatusForbidden
			result.Error = "access denied"
			result.ErrorCode = "forbidden"
			return result
		}
		if !principalAllowsDocumentType(p, item.DocumentType) {
			result.Status = offline.StatusForbidden
			result.Error = "delegation grant does not allow this document type"
			result.ErrorCode = "delegation_forbidden"
			return result
		}
		locationID := effectiveOfflineLocation(item.LocationID, p)
		candidate := document.Record{
			Header: document.Header{
				Type:           item.DocumentType,
				Status:         "draft",
				OrganizationID: item.OrganizationID,
				LocationID:     locationID,
			},
			Body: document.Body{Payload: item.Payload},
		}
		if err := validateDocumentWrite(fieldSecurity, ident, p, candidate, item.Payload, "", "api"); err != nil {
			result.Error = err.Error()
			result.ErrorCode = "validation_error"
			return result
		}
		if err := validateDocumentPayloadForType(modules, item.DocumentType, item.Payload); err != nil {
			result.Error = err.Error()
			result.ErrorCode = "validation_error"
			return result
		}
		record, err := docs.Create(item.DocumentType, item.OrganizationID, locationID, principalEffectiveUserID(p), item.Payload)
		if err != nil {
			result = offlineFailureFromError(err, result)
			return result
		}
		refreshDocumentSearch(searchSvc, record)
		result.Status = offline.StatusAccepted
		result.TargetID = record.Header.ID
		result.Version = record.Header.Version
		result.ETag = record.Header.ETag
		return result
	case "update":
		required := append([]string(nil), def.RequiredPermissions...)
		if def.UpdatePermissionKey != "" {
			required = append(required, def.UpdatePermissionKey)
		}
		if !principalAllowsAll(ident, p, required) {
			result.Status = offline.StatusForbidden
			result.Error = "access denied"
			result.ErrorCode = "forbidden"
			return result
		}
		current, err := docs.Get(item.TargetID)
		if err != nil {
			result = offlineFailureFromError(err, result)
			return result
		}
		if err := validateDocumentWrite(fieldSecurity, ident, p, current, item.Payload, "", "api"); err != nil {
			result.Error = err.Error()
			result.ErrorCode = "validation_error"
			return result
		}
		if err := validateDocumentPayloadForType(modules, current.Header.Type, item.Payload); err != nil {
			result.Error = err.Error()
			result.ErrorCode = "validation_error"
			return result
		}
		var record document.Record
		if docActions != nil {
			record, err = docActions.UpdateDraft(item.TargetID, principalActingContext(p), item.Payload, item.ExpectedVersion, item.ExpectedETag)
		} else {
			result.Error = "document actions are not configured"
			result.ErrorCode = "missing_dependency"
			return result
		}
		if err != nil {
			result = offlineConflictFromError(err, result, map[string]any{
				"id":      current.Header.ID,
				"version": current.Header.Version,
				"etag":    current.Header.ETag,
				"status":  current.Header.Status,
				"payload": current.Body.Payload,
			}, map[string]any{
				"payload": item.Payload,
			})
			return result
		}
		result.Status = offline.StatusAccepted
		result.TargetID = record.Header.ID
		result.Version = record.Header.Version
		result.ETag = record.Header.ETag
		return result
	default:
		result.Error = "unsupported offline document operation"
		result.ErrorCode = "unsupported_operation"
		return result
	}
}

func applyOfflineModelSync(ident *identity.Service, p principal, modules *module.Service, models *model.Service, modelActions *application.ModelActions, fieldSecurity *securityfields.Service, item offlineSyncItem, result offline.SyncResultItem) offline.SyncResultItem {
	def, ok := modules.OfflineModel(strings.TrimSpace(item.ModelKey))
	if !ok {
		result.Error = "offline model is not registered"
		return result
	}
	modelDef, ok := models.Definition(item.ModelKey)
	if !ok {
		result.Error = "model definition not found"
		return result
	}
	switch strings.TrimSpace(item.Operation) {
	case "create":
		required := append([]string(nil), def.RequiredPermissions...)
		if def.CreatePermissionKey != "" {
			required = append(required, def.CreatePermissionKey)
		}
		if !principalAllowsAll(ident, p, required) {
			result.Status = offline.StatusForbidden
			result.Error = "access denied"
			result.ErrorCode = "forbidden"
			return result
		}
		if err := validateModelWriteAccess(fieldSecurity, ident, p, modelDef, item.Values, "api"); err != nil {
			result.Error = err.Error()
			result.ErrorCode = "validation_error"
			return result
		}
		var record model.Record
		var err error
		if modelActions != nil {
			record, _, err = modelActions.CreateComposite(item.ModelKey, principalActingContext(p), model.CompositeMutation{Values: item.Values, Relations: item.Relations})
		} else {
			record, _, err = models.CreateComposite(item.ModelKey, principalEffectiveUserID(p), model.CompositeMutation{Values: item.Values, Relations: item.Relations})
		}
		if err != nil {
			result = offlineFailureFromError(err, result)
			return result
		}
		result.Status = offline.StatusAccepted
		result.TargetID = record.ID
		result.Version = record.Version
		return result
	case "update":
		required := append([]string(nil), def.RequiredPermissions...)
		if def.UpdatePermissionKey != "" {
			required = append(required, def.UpdatePermissionKey)
		}
		if !principalAllowsAll(ident, p, required) {
			result.Status = offline.StatusForbidden
			result.Error = "access denied"
			result.ErrorCode = "forbidden"
			return result
		}
		current, err := models.Get(item.ModelKey, item.TargetID)
		if err != nil {
			result = offlineFailureFromError(err, result)
			return result
		}
		if err := validateModelWriteAccess(fieldSecurity, ident, p, modelDef, item.Values, "api"); err != nil {
			result.Error = err.Error()
			result.ErrorCode = "validation_error"
			return result
		}
		var record model.Record
		mutation := model.CompositeMutation{
			ExpectedVersion: item.ExpectedVersion,
			Values:          item.Values,
			Relations:       item.Relations,
		}
		if modelActions != nil {
			record, _, err = modelActions.UpdateComposite(item.ModelKey, item.TargetID, principalActingContext(p), mutation)
		} else {
			record, _, err = models.UpdateComposite(item.ModelKey, item.TargetID, principalEffectiveUserID(p), mutation)
		}
		if err != nil {
			result = offlineConflictFromError(err, result, map[string]any{
				"id":      current.ID,
				"version": current.Version,
				"values":  current.Values,
			}, map[string]any{
				"values": item.Values,
			})
			return result
		}
		result.Status = offline.StatusAccepted
		result.TargetID = record.ID
		result.Version = record.Version
		return result
	default:
		result.Error = "unsupported offline model operation"
		result.ErrorCode = "unsupported_operation"
		return result
	}
}

func requireAuthenticatedPrincipal(w http.ResponseWriter, r *http.Request) (principal, bool) {
	if err := authError(r); err != nil {
		respondError(w, err)
		return principal{}, false
	}
	p, ok := currentPrincipal(r)
	if !ok {
		respondError(w, shared.Unauthorized("authentication required"))
		return principal{}, false
	}
	return p, true
}

func effectiveOfflineLocation(locationID string, p principal) string {
	if strings.TrimSpace(locationID) != "" {
		return strings.TrimSpace(locationID)
	}
	return strings.TrimSpace(p.currentLocationID)
}

func offlineConflictFromError(err error, result offline.SyncResultItem, current, attempted map[string]any) offline.SyncResultItem {
	var platformErr shared.Error
	if errors.As(err, &platformErr) {
		if platformErr.Kind == shared.KindConflict {
			result.Status = offline.StatusConflict
			result.Error = platformErr.Message
			result.ErrorCode = "version_conflict"
			result.Conflict = offline.SyncConflict{
				Current:           current,
				Attempted:         attempted,
				ResolutionOptions: []string{"reload", "retry_with_latest", "duplicate_as_new"},
			}
			return result
		}
		result = offlineFailureFromPlatformError(platformErr, result)
		return result
	}
	result.Status = offline.StatusFailedRetryable
	result.Error = err.Error()
	result.ErrorCode = "sync_retryable"
	result.AttemptCount++
	result.RetryAfter = time.Now().UTC().Add(5 * time.Second).Format(time.RFC3339)
	return result
}

func offlineFailureFromError(err error, result offline.SyncResultItem) offline.SyncResultItem {
	var platformErr shared.Error
	if errors.As(err, &platformErr) {
		return offlineFailureFromPlatformError(platformErr, result)
	}
	result.Status = offline.StatusFailedRetryable
	result.Error = err.Error()
	result.ErrorCode = "sync_retryable"
	result.AttemptCount++
	result.RetryAfter = time.Now().UTC().Add(5 * time.Second).Format(time.RFC3339)
	return result
}

func offlineFailureFromPlatformError(err shared.Error, result offline.SyncResultItem) offline.SyncResultItem {
	result.Error = err.Message
	switch err.Kind {
	case shared.KindForbidden:
		result.Status = offline.StatusForbidden
		result.ErrorCode = "forbidden"
	case shared.KindValidation:
		result.Status = offline.StatusFailedTerminal
		result.ErrorCode = "validation_error"
	case shared.KindNotFound:
		result.Status = offline.StatusFailedTerminal
		result.ErrorCode = "not_found"
	default:
		result.Status = offline.StatusFailedRetryable
		result.ErrorCode = "sync_retryable"
		result.AttemptCount++
		result.RetryAfter = time.Now().UTC().Add(5 * time.Second).Format(time.RFC3339)
	}
	return result
}

func offlineBootstrapToken(bootstrap offline.Bootstrap) string {
	payload := map[string]any{
		"schema_version": bootstrap.SchemaVersion,
		"references":     bootstrap.References,
		"projections":    bootstrap.Projections,
		"documents":      bootstrap.Documents,
		"models":         bootstrap.Models,
		"manifest":       bootstrap.PackageManifest,
	}
	encoded, _ := json.Marshal(payload)
	hash := sha256.Sum256(encoded)
	sum := hex.EncodeToString(hash[:])
	if len(sum) > 16 {
		return sum[:16]
	}
	return sum
}

func decodeOfflineProjectionQuery(raw map[string]any) search.QueryRequest {
	encoded, _ := json.Marshal(raw)
	var req search.QueryRequest
	_ = json.Unmarshal(encoded, &req)
	return req
}
