package httpx

import (
	"net/http"
	"os"
	"sort"
	"strings"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
)

func registerDocsRoutes(mux *http.ServeMux, cfg *config.Service, modules *module.Service, models *model.Service, docs *document.Service, searchSvc *search.Service) {
	if !devDocsEnabled() {
		return
	}
	mux.HandleFunc("GET /dev/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, buildOpenAPIDocument(cfg, modules, models, docs, searchSvc))
	})
	mux.HandleFunc("GET /dev/swagger", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(swaggerUIHTML))
	})
}

func devDocsEnabled() bool {
	if strings.EqualFold(os.Getenv("APP_AUTH_DEV_MODE"), "true") {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))) {
	case "development", "dev", "test":
		return true
	default:
		return false
	}
}

func buildOpenAPIDocument(cfg *config.Service, modules *module.Service, models *model.Service, docs *document.Service, searchSvc *search.Service) map[string]any {
	modelKeys := sortedModelKeys(models)
	documentTypes := sortedDocumentTypes(docs)
	searchIndexKeys := sortedSearchIndexKeys(searchSvc)
	userViewKeys := sortedViewKeys(modules, module.UISurfaceUser)
	userPaths := sortedActionPaths(modules, module.UISurfaceUser)
	offlineReferenceKeys := sortedOfflineReferenceKeys(modules)
	offlineProjectionKeys := sortedOfflineProjectionKeys(modules)
	moduleKeys := sortedModuleKeys(modules)

	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":       "Orbyte Platform API",
			"version":     "dev",
			"description": "Development-only OpenAPI document for the runtime generic platform APIs. Registered modules become available through the generic model, document, UI contract, search, and offline endpoints. Custom module-specific controller namespaces are not auto-generated.",
		},
		"servers": []map[string]any{{"url": "/"}},
		"tags": []map[string]any{
			{"name": "Auth", "description": "Authentication and session endpoints."},
			{"name": "Platform", "description": "Core platform health and context endpoints."},
			{"name": "Models", "description": "Generic manifest-driven model APIs."},
			{"name": "Documents", "description": "Generic manifest-driven document APIs."},
			{"name": "Search", "description": "Search and projection query APIs."},
			{"name": "Offline", "description": "Offline bootstrap, package, and sync APIs."},
			{"name": "UI Contracts", "description": "Supported headless UI contract endpoints for external clients."},
			{"name": "Admin", "description": "Admin-only and internal administration APIs."},
		},
		"x-orbyte-runtime": map[string]any{
			"registered_modules": moduleKeys,
			"model_keys":         modelKeys,
			"document_types":     documentTypes,
			"search_indexes":     searchIndexKeys,
			"user_view_keys":     userViewKeys,
			"user_route_paths":   userPaths,
			"config_keys":        configKeys(cfg),
		},
		"paths": map[string]any{
			"/auth/options": opPath("get", "Auth", "public-headless", "Get auth options", "Returns login surface options for shell or external client sign-in UIs.", nil, nil, map[string]any{"200": jsonResponse("Auth options", "#/components/schemas/AuthOptions")}),
			"/auth/login":   opPath("post", "Auth", "public-headless", "Login with password", "Creates a session and returns session metadata.", requestBodyRef("#/components/schemas/LoginRequest"), nil, map[string]any{"200": jsonResponse("Session created", "#/components/schemas/LoginResponse")}),
			"/auth/logout":  opPath("post", "Auth", "public-headless", "Logout", "Revokes the current session.", nil, nil, map[string]any{"200": jsonResponse("Logged out", "#/components/schemas/MessageResponse")}),
			"/healthz":      opPath("get", "Platform", "public-headless", "Health check", "Liveness endpoint.", nil, nil, map[string]any{"200": jsonResponse("Healthy", "#/components/schemas/HealthResponse")}),
			"/readyz":       opPath("get", "Platform", "public-headless", "Readiness check", "Readiness endpoint with dependency state.", nil, nil, map[string]any{"200": jsonResponse("Ready", "#/components/schemas/ReadinessResponse")}),
			"/platform/context": opPath("get", "Platform", "public-headless", "Get platform context", "Returns organization, roles, document types, workflows, and config keys for the current principal.", nil, nil, map[string]any{
				"200": jsonResponse("Platform context", "#/components/schemas/PlatformContextResponse"),
			}),
			"/models/{modelKey}": map[string]any{
				"get":  operation("Models", "public-headless", "List model records", "Lists records for the given manifest-registered model key.", nil, []map[string]any{pathEnumParam("modelKey", "Registered model key.", modelKeys)}, map[string]any{"200": jsonResponse("Model list", "#/components/schemas/ModelListResponse")}),
				"post": operation("Models", "public-headless", "Create model record", "Creates a record through the generic model API for the given model key.", requestBodyRef("#/components/schemas/ModelMutationRequest"), []map[string]any{pathEnumParam("modelKey", "Registered model key.", modelKeys)}, map[string]any{"201": jsonResponse("Model created", "#/components/schemas/ModelMutationResponse")}),
			},
			"/models/{modelKey}/{recordID}": map[string]any{
				"get": operation("Models", "public-headless", "Get model record", "Returns a record, its definition, and related definitions.", nil, []map[string]any{pathEnumParam("modelKey", "Registered model key.", modelKeys), pathStringParam("recordID", "Model record ID.")}, map[string]any{"200": jsonResponse("Model detail", "#/components/schemas/ModelDetailResponse")}),
				"put": operation("Models", "public-headless", "Update model record", "Updates a record through the generic model API.", requestBodyRef("#/components/schemas/ModelMutationRequest"), []map[string]any{pathEnumParam("modelKey", "Registered model key.", modelKeys), pathStringParam("recordID", "Model record ID.")}, map[string]any{"200": jsonResponse("Model updated", "#/components/schemas/ModelMutationResponse")}),
			},
			"/documents": map[string]any{
				"get":  operation("Documents", "public-headless", "List documents", "Lists documents visible to the current principal.", nil, nil, map[string]any{"200": jsonResponse("Document list", "#/components/schemas/DocumentListResponse")}),
				"post": operation("Documents", "public-headless", "Create document", "Creates a draft document using a registered document type.", requestBodyRef("#/components/schemas/CreateDocumentRequest"), nil, map[string]any{"201": jsonResponse("Document created", "#/components/schemas/DocumentRecord")}),
			},
			"/documents/{documentID}": map[string]any{
				"get": operation("Documents", "public-headless", "Get document", "Returns a document record in normal, expanded, or raw view mode.", nil, []map[string]any{pathStringParam("documentID", "Document ID.")}, map[string]any{"200": jsonResponse("Document detail", "#/components/schemas/DocumentRecord")}),
				"put": operation("Documents", "public-headless", "Update document draft", "Updates a draft document using optimistic concurrency.", requestBodyRef("#/components/schemas/UpdateDocumentRequest"), []map[string]any{pathStringParam("documentID", "Document ID.")}, map[string]any{"200": jsonResponse("Document updated", "#/components/schemas/DocumentRecord")}),
			},
			"/documents/{documentID}/actions": map[string]any{
				"post": operation("Documents", "public-headless", "Run document action", "Runs submit, approve, reject, reopen, or cancel through the generic document action API.", requestBodyRef("#/components/schemas/DocumentActionRequest"), []map[string]any{pathStringParam("documentID", "Document ID.")}, map[string]any{"200": jsonResponse("Document action result", "#/components/schemas/DocumentRecord")}),
			},
			"/search/indexes": opPath("get", "Search", "public-headless", "List search indexes", "Lists search index definitions visible to the current principal.", nil, nil, map[string]any{"200": jsonResponse("Search indexes", "#/components/schemas/SearchIndexListResponse")}),
			"/search/indexes/{indexKey}": map[string]any{
				"get": operation("Search", "public-headless", "Get search index", "Returns one search index definition.", nil, []map[string]any{pathEnumParam("indexKey", "Registered search index key.", searchIndexKeys)}, map[string]any{"200": jsonResponse("Search index", "#/components/schemas/SearchIndexDefinition")}),
			},
			"/search/indexes/{indexKey}/query": map[string]any{
				"post": operation("Search", "public-headless", "Keyword query", "Runs a keyword query against the index.", requestBodyRef("#/components/schemas/SearchQueryRequest"), []map[string]any{pathEnumParam("indexKey", "Registered search index key.", searchIndexKeys)}, map[string]any{"200": jsonResponse("Query result", "#/components/schemas/SearchQueryResponse")}),
			},
			"/search/indexes/{indexKey}/query/vector": map[string]any{
				"post": operation("Search", "public-headless", "Vector query", "Runs a vector query against the index.", requestBodyRef("#/components/schemas/SearchQueryRequest"), []map[string]any{pathEnumParam("indexKey", "Registered search index key.", searchIndexKeys)}, map[string]any{"200": jsonResponse("Query result", "#/components/schemas/SearchQueryResponse")}),
			},
			"/search/indexes/{indexKey}/query/hybrid": map[string]any{
				"post": operation("Search", "public-headless", "Hybrid query", "Runs a hybrid query against the index.", requestBodyRef("#/components/schemas/SearchQueryRequest"), []map[string]any{pathEnumParam("indexKey", "Registered search index key.", searchIndexKeys)}, map[string]any{"200": jsonResponse("Query result", "#/components/schemas/SearchQueryResponse")}),
			},
			"/offline/bootstrap": opPath("get", "Offline", "public-headless", "Get offline bootstrap", "Returns the offline capabilities visible to the current principal.", nil, nil, map[string]any{"200": jsonResponse("Offline bootstrap", "#/components/schemas/OfflineBootstrap")}),
			"/offline/packages/references": opPath("post", "Offline", "public-headless", "Get offline reference package", "Returns an offline reference package for a registered offline reference key.", requestBodyRefWithExample(map[string]any{
				"type":       "object",
				"properties": map[string]any{"type_key": map[string]any{"type": "string", "enum": offlineReferenceKeys}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}},
			}, map[string]any{"type_key": firstOrEmpty(offlineReferenceKeys)}), nil, map[string]any{"200": jsonResponse("Reference package", "#/components/schemas/ReferencePackageResponse")}),
			"/offline/packages/projections": opPath("post", "Offline", "public-headless", "Get offline projection package", "Returns an offline projection package for a registered offline projection key.", requestBodyRefWithExample(map[string]any{
				"type":       "object",
				"properties": map[string]any{"index_key": map[string]any{"type": "string", "enum": offlineProjectionKeys}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "query": map[string]any{"type": "object"}},
			}, map[string]any{"index_key": firstOrEmpty(offlineProjectionKeys)}), nil, map[string]any{"200": jsonResponse("Projection package", "#/components/schemas/ProjectionPackageResponse")}),
			"/offline/sync":      opPath("post", "Offline", "public-headless", "Sync offline mutations", "Accepts draft-safe offline create and update mutations in batch.", requestBodyRef("#/components/schemas/OfflineSyncRequest"), nil, map[string]any{"200": jsonResponse("Sync results", "#/components/schemas/OfflineSyncResponse")}),
			"/ui/bootstrap":      opPath("get", "UI Contracts", "public-headless", "Get UI bootstrap", "Returns the user workspace menus, actions, views, locale, and default route path.", nil, nil, map[string]any{"200": jsonResponse("UI bootstrap", "#/components/schemas/UIBootstrapResponse")}),
			"/ui/routes/resolve": opPath("get", "UI Contracts", "public-headless", "Resolve UI route", "Resolves a hash-route path into an action, view, or custom entry.", nil, []map[string]any{queryEnumParam("path", "User route path.", userPaths)}, map[string]any{"200": jsonResponse("Route resolution", "#/components/schemas/UIRouteResolution")}),
			"/ui/views/{viewKey}": map[string]any{
				"get": operation("UI Contracts", "public-headless", "Get UI view", "Returns one registered user-surface view definition.", nil, []map[string]any{pathEnumParam("viewKey", "Registered user view key.", userViewKeys)}, map[string]any{"200": jsonResponse("View definition", "#/components/schemas/UIViewDefinition")}),
			},
			"/ui/data/documents":         opPath("get", "UI Contracts", "public-headless", "Get document summaries for UI", "Returns the document summary payload used by the generic user workspace.", nil, nil, map[string]any{"200": jsonResponse("UI document data", "#/components/schemas/UIDocumentsResponse")}),
			"/ui/data/models":            opPath("get", "UI Contracts", "public-headless", "Get model list for UI", "Returns model data for the generic user workspace.", nil, nil, map[string]any{"200": jsonResponse("UI model data", "#/components/schemas/UIModelDataResponse")}),
			"/admin/api/bootstrap":       opPath("get", "Admin", "admin", "Get admin bootstrap", "Returns admin workspace navigation and chrome state.", nil, nil, map[string]any{"200": jsonResponse("Admin bootstrap", "#/components/schemas/AdminBootstrapResponse")}),
			"/admin/api/modules":         opPath("get", "Admin", "admin", "List modules", "Returns installed module details for administration.", nil, nil, map[string]any{"200": jsonResponse("Module list", "#/components/schemas/ModuleListResponse")}),
			"/admin/api/config/validate": opPath("get", "Admin", "admin", "Validate configuration", "Runs configuration validation for the selected scope.", nil, nil, map[string]any{"200": jsonResponse("Config validation", "#/components/schemas/GenericObjectResponse")}),
		},
		"components": map[string]any{
			"schemas": map[string]any{
				"ErrorResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"error": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"kind":    map[string]any{"type": "string"},
								"message": map[string]any{"type": "string"},
							},
						},
					},
				},
				"MessageResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"message": map[string]any{"type": "string"},
					},
				},
				"GenericObjectResponse": genericObjectSchema("Generic object response."),
				"AuthOptions": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"password_enabled":    map[string]any{"type": "boolean"},
						"google_enabled":      map[string]any{"type": "boolean"},
						"login_title":         map[string]any{"type": "string"},
						"login_subtitle":      map[string]any{"type": "string"},
						"google_button_label": map[string]any{"type": "string"},
					},
				},
				"LoginRequest": map[string]any{
					"type":     "object",
					"required": []string{"username", "password"},
					"properties": map[string]any{
						"username":    map[string]any{"type": "string"},
						"password":    map[string]any{"type": "string"},
						"location_id": map[string]any{"type": "string"},
					},
				},
				"LoginResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"session": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id":                  map[string]any{"type": "string"},
								"user_id":             map[string]any{"type": "string"},
								"status":              map[string]any{"type": "string"},
								"issued_at":           map[string]any{"type": "string", "format": "date-time"},
								"expires_at":          map[string]any{"type": "string", "format": "date-time"},
								"last_seen_at":        map[string]any{"type": "string", "format": "date-time"},
								"current_location_id": map[string]any{"type": "string"},
							},
						},
					},
				},
				"HealthResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"status": map[string]any{"type": "string"},
						"time":   map[string]any{"type": "string", "format": "date-time"},
					},
				},
				"ReadinessResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"live":          map[string]any{"type": "boolean"},
						"ready":         map[string]any{"type": "boolean"},
						"dependency_ok": map[string]any{"type": "boolean"},
					},
					"additionalProperties": true,
				},
				"PlatformContextResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"organization":    genericObjectSchema("Organization root."),
						"config_keys":     stringArraySchema(),
						"reference_types": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/ReferenceTypeDefinition"}},
						"roles":           map[string]any{"type": "array", "items": genericObjectSchema("Role")},
						"document_types":  stringArraySchema(),
						"workflows":       stringArraySchema(),
					},
				},
				"ModelMutationRequest": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"values":    map[string]any{"type": "object", "additionalProperties": true},
						"relations": map[string]any{"type": "object", "additionalProperties": true},
					},
				},
				"ModelMutationResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"record":  map[string]any{"$ref": "#/components/schemas/ModelRecord"},
						"related": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/ModelRecord"}}},
					},
				},
				"ModelListResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"items":               map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/ModelRecord"}},
						"total":               map[string]any{"type": "integer"},
						"definition":          map[string]any{"$ref": "#/components/schemas/ModelDefinition"},
						"related_definitions": map[string]any{"type": "object", "additionalProperties": map[string]any{"$ref": "#/components/schemas/ModelDefinition"}},
					},
				},
				"ModelDetailResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"record":              map[string]any{"$ref": "#/components/schemas/ModelRecord"},
						"definition":          map[string]any{"$ref": "#/components/schemas/ModelDefinition"},
						"timeline":            map[string]any{"type": "array", "items": genericObjectSchema("Activity timeline item")},
						"related":             map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/ModelRecord"}}},
						"related_definitions": map[string]any{"type": "object", "additionalProperties": map[string]any{"$ref": "#/components/schemas/ModelDefinition"}},
						"model_definitions":   map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/ModelDefinition"}},
					},
				},
				"ModelDefinition": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"key":                   map[string]any{"type": "string"},
						"display_name":          map[string]any{"type": "string"},
						"display_name_i18n":     map[string]any{"$ref": "#/components/schemas/LocalizedText"},
						"owner_module_key":      map[string]any{"type": "string"},
						"version":               map[string]any{"type": "string"},
						"create_permission_key": map[string]any{"type": "string"},
						"list_permission_key":   map[string]any{"type": "string"},
						"read_permission_key":   map[string]any{"type": "string"},
						"update_permission_key": map[string]any{"type": "string"},
						"default_sort":          map[string]any{"type": "string"},
						"fields":                map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/ModelFieldDefinition"}},
						"relations":             map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/ModelRelationDefinition"}},
					},
				},
				"ModelFieldDefinition": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"key":                  map[string]any{"type": "string"},
						"label":                map[string]any{"type": "string"},
						"label_i18n":           map[string]any{"$ref": "#/components/schemas/LocalizedText"},
						"type":                 map[string]any{"type": "string"},
						"required":             map[string]any{"type": "boolean"},
						"read_only":            map[string]any{"type": "boolean"},
						"indexed":              map[string]any{"type": "boolean"},
						"sensitive":            map[string]any{"type": "boolean"},
						"security_class":       map[string]any{"type": "string"},
						"default_mask":         map[string]any{"type": "string"},
						"search_visible":       map[string]any{"type": "boolean"},
						"export_visible":       map[string]any{"type": "boolean"},
						"read_permission_key":  map[string]any{"type": "string"},
						"write_permission_key": map[string]any{"type": "string"},
						"default_value":        map[string]any{},
					},
					"additionalProperties": true,
				},
				"ModelRelationDefinition": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"key":              map[string]any{"type": "string"},
						"type":             map[string]any{"type": "string"},
						"target_model_key": map[string]any{"type": "string"},
						"foreign_key":      map[string]any{"type": "string"},
					},
				},
				"ModelRecord": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"model_key":  map[string]any{"type": "string"},
						"id":         map[string]any{"type": "string"},
						"version":    map[string]any{"type": "integer"},
						"values":     map[string]any{"type": "object", "additionalProperties": true},
						"created_by": map[string]any{"type": "string"},
						"created_at": map[string]any{"type": "string", "format": "date-time"},
						"updated_by": map[string]any{"type": "string"},
						"updated_at": map[string]any{"type": "string", "format": "date-time"},
					},
				},
				"CreateDocumentRequest": map[string]any{
					"type":     "object",
					"required": []string{"type", "organization_id", "payload"},
					"properties": map[string]any{
						"type":            map[string]any{"type": "string", "enum": documentTypes},
						"organization_id": map[string]any{"type": "string"},
						"location_id":     map[string]any{"type": "string"},
						"payload":         map[string]any{"type": "object", "additionalProperties": true},
					},
				},
				"UpdateDocumentRequest": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"expected_version": map[string]any{"type": "integer"},
						"expected_etag":    map[string]any{"type": "string"},
						"payload":          map[string]any{"type": "object", "additionalProperties": true},
					},
				},
				"DocumentActionRequest": map[string]any{
					"type":     "object",
					"required": []string{"action"},
					"properties": map[string]any{
						"action":           map[string]any{"type": "string", "enum": []string{"submit", "approve", "reject", "reopen", "cancel"}},
						"expected_version": map[string]any{"type": "integer"},
						"expected_etag":    map[string]any{"type": "string"},
					},
				},
				"DocumentRecord": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"header":      map[string]any{"$ref": "#/components/schemas/DocumentHeader"},
						"body":        map[string]any{"$ref": "#/components/schemas/DocumentBody"},
						"lines":       map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/DocumentLine"}},
						"links":       map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/DocumentLink"}},
						"attachments": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/DocumentAttachment"}},
					},
				},
				"DocumentHeader": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":              map[string]any{"type": "string"},
						"type":            map[string]any{"type": "string"},
						"status":          map[string]any{"type": "string"},
						"version":         map[string]any{"type": "integer"},
						"etag":            map[string]any{"type": "string"},
						"organization_id": map[string]any{"type": "string"},
						"location_id":     map[string]any{"type": "string"},
						"number":          map[string]any{"type": "string"},
						"created_by":      map[string]any{"type": "string"},
						"created_at":      map[string]any{"type": "string", "format": "date-time"},
						"updated_by":      map[string]any{"type": "string"},
						"updated_at":      map[string]any{"type": "string", "format": "date-time"},
						"submitted_by":    map[string]any{"type": "string"},
						"submitted_at":    map[string]any{"type": "string", "format": "date-time"},
						"total_amount":    genericObjectSchema("Money"),
						"metadata":        map[string]any{"type": "object", "additionalProperties": true},
					},
				},
				"DocumentBody": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"document_id":    map[string]any{"type": "string"},
						"schema_version": map[string]any{"type": "string"},
						"payload":        map[string]any{"type": "object", "additionalProperties": true},
						"content_hash":   map[string]any{"type": "string"},
					},
				},
				"DocumentLine": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":          map[string]any{"type": "string"},
						"document_id": map[string]any{"type": "string"},
						"line_no":     map[string]any{"type": "integer"},
						"line_type":   map[string]any{"type": "string"},
						"schema_ref":  map[string]any{"type": "string"},
						"payload":     map[string]any{"type": "object", "additionalProperties": true},
						"amount":      genericObjectSchema("Money"),
					},
				},
				"DocumentLink": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":                 map[string]any{"type": "string"},
						"document_id":        map[string]any{"type": "string"},
						"linked_document_id": map[string]any{"type": "string"},
						"link_type":          map[string]any{"type": "string"},
						"metadata":           map[string]any{"type": "object", "additionalProperties": true},
						"created_at":         map[string]any{"type": "string", "format": "date-time"},
					},
				},
				"DocumentAttachment": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":              map[string]any{"type": "string"},
						"document_id":     map[string]any{"type": "string"},
						"attachment_type": map[string]any{"type": "string"},
						"file_name":       map[string]any{"type": "string"},
						"content_type":    map[string]any{"type": "string"},
						"storage_key":     map[string]any{"type": "string"},
						"size_bytes":      map[string]any{"type": "integer"},
						"created_at":      map[string]any{"type": "string", "format": "date-time"},
					},
				},
				"DocumentListResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/DocumentRecord"}},
					},
				},
				"SearchIndexDefinition": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"key":                  map[string]any{"type": "string"},
						"title":                map[string]any{"type": "string"},
						"title_i18n":           map[string]any{"$ref": "#/components/schemas/LocalizedText"},
						"source_kind":          map[string]any{"type": "string"},
						"document_type":        map[string]any{"type": "string"},
						"model_key":            map[string]any{"type": "string"},
						"projection_key":       map[string]any{"type": "string"},
						"required_permissions": stringArraySchema(),
						"query_filter_fields":  stringArraySchema(),
						"query_sort_fields":    stringArraySchema(),
						"modes":                stringArraySchema(),
					},
					"additionalProperties": true,
				},
				"SearchIndexListResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/SearchIndexDefinition"}},
					},
				},
				"SearchQueryRequest": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"term":      map[string]any{"type": "string"},
						"filters":   map[string]any{"type": "object", "additionalProperties": true},
						"sort_key":  map[string]any{"type": "string"},
						"desc":      map[string]any{"type": "boolean"},
						"page":      map[string]any{"type": "integer"},
						"page_size": map[string]any{"type": "integer"},
						"mode":      map[string]any{"type": "string"},
					},
					"additionalProperties": true,
				},
				"SearchQueryResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"items":     map[string]any{"type": "array", "items": genericObjectSchema("Search result item")},
						"total":     map[string]any{"type": "integer"},
						"index_key": map[string]any{"type": "string"},
						"mode":      map[string]any{"type": "string"},
						"page":      map[string]any{"type": "integer"},
						"page_size": map[string]any{"type": "integer"},
					},
					"additionalProperties": true,
				},
				"OfflineBootstrap": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"references":  map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/OfflineReferenceCapability"}},
						"projections": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/OfflineProjectionCapability"}},
						"documents":   map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/OfflineDocumentCapability"}},
						"models":      map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/OfflineModelCapability"}},
					},
				},
				"OfflineReferenceCapability": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"type_key":             map[string]any{"type": "string"},
						"title":                map[string]any{"type": "string"},
						"title_i18n":           map[string]any{"$ref": "#/components/schemas/LocalizedText"},
						"required_permissions": stringArraySchema(),
					},
				},
				"OfflineProjectionCapability": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"index_key":              map[string]any{"type": "string"},
						"title":                  map[string]any{"type": "string"},
						"title_i18n":             map[string]any{"$ref": "#/components/schemas/LocalizedText"},
						"required_permissions":   stringArraySchema(),
						"default_filters":        stringArraySchema(),
						"default_include_fields": stringArraySchema(),
					},
				},
				"OfflineDocumentCapability": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"type":                  map[string]any{"type": "string"},
						"title":                 map[string]any{"type": "string"},
						"title_i18n":            map[string]any{"$ref": "#/components/schemas/LocalizedText"},
						"create_permission_key": map[string]any{"type": "string"},
						"update_permission_key": map[string]any{"type": "string"},
						"required_permissions":  stringArraySchema(),
					},
				},
				"OfflineModelCapability": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"model_key":             map[string]any{"type": "string"},
						"title":                 map[string]any{"type": "string"},
						"title_i18n":            map[string]any{"$ref": "#/components/schemas/LocalizedText"},
						"create_permission_key": map[string]any{"type": "string"},
						"update_permission_key": map[string]any{"type": "string"},
						"required_permissions":  stringArraySchema(),
					},
				},
				"ReferencePackageResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"type_key":        map[string]any{"type": "string"},
						"version":         map[string]any{"type": "string"},
						"checksum":        map[string]any{"type": "string"},
						"generated_at":    map[string]any{"type": "string", "format": "date-time"},
						"organization_id": map[string]any{"type": "string"},
						"location_id":     map[string]any{"type": "string"},
						"items":           map[string]any{"type": "array", "items": genericObjectSchema("Reference package item")},
					},
					"additionalProperties": true,
				},
				"ProjectionPackageResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"index_key":       map[string]any{"type": "string"},
						"version":         map[string]any{"type": "string"},
						"checksum":        map[string]any{"type": "string"},
						"generated_at":    map[string]any{"type": "string", "format": "date-time"},
						"organization_id": map[string]any{"type": "string"},
						"location_id":     map[string]any{"type": "string"},
						"items":           map[string]any{"type": "array", "items": genericObjectSchema("Projection package item")},
						"query":           map[string]any{"type": "object", "additionalProperties": true},
					},
					"additionalProperties": true,
				},
				"OfflineSyncRequest": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"items": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"idempotency_key":  map[string]any{"type": "string"},
									"kind":             map[string]any{"type": "string", "enum": []string{"document", "model"}},
									"operation":        map[string]any{"type": "string", "enum": []string{"create", "update"}},
									"document_type":    map[string]any{"type": "string", "enum": documentTypes},
									"model_key":        map[string]any{"type": "string", "enum": modelKeys},
									"organization_id":  map[string]any{"type": "string"},
									"location_id":      map[string]any{"type": "string"},
									"target_id":        map[string]any{"type": "string"},
									"expected_version": map[string]any{"type": "integer"},
									"expected_etag":    map[string]any{"type": "string"},
									"payload":          map[string]any{"type": "object", "additionalProperties": true},
									"values":           map[string]any{"type": "object", "additionalProperties": true},
									"relations":        map[string]any{"type": "object", "additionalProperties": true},
								},
							},
						},
					},
				},
				"OfflineSyncResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/OfflineSyncResultItem"}},
					},
				},
				"OfflineSyncResultItem": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"idempotency_key": map[string]any{"type": "string"},
						"kind":            map[string]any{"type": "string"},
						"operation":       map[string]any{"type": "string"},
						"target_id":       map[string]any{"type": "string"},
						"status":          map[string]any{"type": "string"},
						"version":         map[string]any{"type": "integer"},
						"etag":            map[string]any{"type": "string"},
						"error":           map[string]any{"type": "string"},
						"conflict":        genericObjectSchema("Conflict payload"),
					},
					"additionalProperties": true,
				},
				"UIBootstrapResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"menus":             map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/UIMenuDefinition"}},
						"actions":           map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/UIActionDefinition"}},
						"views":             map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/UIViewDefinition"}},
						"default_path":      map[string]any{"type": "string"},
						"admin_access":      map[string]any{"type": "boolean"},
						"admin_path":        map[string]any{"type": "string"},
						"locale":            map[string]any{"type": "string"},
						"supported_locales": stringArraySchema(),
					},
				},
				"UIRouteResolution": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action":       map[string]any{"$ref": "#/components/schemas/UIActionDefinition"},
						"view":         map[string]any{"$ref": "#/components/schemas/UIViewDefinition"},
						"custom_entry": map[string]any{"$ref": "#/components/schemas/UICustomEntryDefinition"},
					},
				},
				"UIViewDefinition": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"key":                  map[string]any{"type": "string"},
						"title":                map[string]any{"type": "string"},
						"title_i18n":           map[string]any{"$ref": "#/components/schemas/LocalizedText"},
						"kind":                 map[string]any{"type": "string"},
						"document_type":        map[string]any{"type": "string"},
						"model_key":            map[string]any{"type": "string"},
						"projection_key":       map[string]any{"type": "string"},
						"required_permissions": stringArraySchema(),
						"allowed_actions":      stringArraySchema(),
						"columns":              map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/UIViewColumn"}},
						"cards":                map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/UICardDefinition"}},
						"tabs":                 map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/UITabDefinition"}},
					},
					"additionalProperties": true,
				},
				"UIMenuDefinition": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"key":                  map[string]any{"type": "string"},
						"label":                map[string]any{"type": "string"},
						"label_i18n":           map[string]any{"$ref": "#/components/schemas/LocalizedText"},
						"action_key":           map[string]any{"type": "string"},
						"order":                map[string]any{"type": "integer"},
						"required_permissions": stringArraySchema(),
					},
				},
				"UIActionDefinition": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"key":                  map[string]any{"type": "string"},
						"label":                map[string]any{"type": "string"},
						"label_i18n":           map[string]any{"$ref": "#/components/schemas/LocalizedText"},
						"kind":                 map[string]any{"type": "string"},
						"route_path":           map[string]any{"type": "string"},
						"view_key":             map[string]any{"type": "string"},
						"custom_entry_key":     map[string]any{"type": "string"},
						"render_mode":          map[string]any{"type": "string"},
						"required_permissions": stringArraySchema(),
					},
					"additionalProperties": true,
				},
				"UICustomEntryDefinition": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"key":                  map[string]any{"type": "string"},
						"title":                map[string]any{"type": "string"},
						"title_i18n":           map[string]any{"$ref": "#/components/schemas/LocalizedText"},
						"route_path":           map[string]any{"type": "string"},
						"bundle_key":           map[string]any{"type": "string"},
						"component_export":     map[string]any{"type": "string"},
						"required_permissions": stringArraySchema(),
					},
				},
				"UIViewColumn": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"key":        map[string]any{"type": "string"},
						"label":      map[string]any{"type": "string"},
						"label_i18n": map[string]any{"$ref": "#/components/schemas/LocalizedText"},
						"path":       map[string]any{"type": "string"},
						"type":       map[string]any{"type": "string"},
					},
					"additionalProperties": true,
				},
				"UICardDefinition": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"key":        map[string]any{"type": "string"},
						"label":      map[string]any{"type": "string"},
						"label_i18n": map[string]any{"$ref": "#/components/schemas/LocalizedText"},
						"path":       map[string]any{"type": "string"},
					},
					"additionalProperties": true,
				},
				"UITabDefinition": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"key":        map[string]any{"type": "string"},
						"title":      map[string]any{"type": "string"},
						"title_i18n": map[string]any{"$ref": "#/components/schemas/LocalizedText"},
						"sections":   map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/UISectionDefinition"}},
					},
				},
				"UISectionDefinition": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"key":        map[string]any{"type": "string"},
						"title":      map[string]any{"type": "string"},
						"title_i18n": map[string]any{"$ref": "#/components/schemas/LocalizedText"},
						"fields":     map[string]any{"type": "array", "items": genericObjectSchema("Section field")},
					},
					"additionalProperties": true,
				},
				"UIDocumentsResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"items": map[string]any{"type": "array", "items": genericObjectSchema("UI document item")},
					},
					"additionalProperties": true,
				},
				"UIModelDataResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"items": map[string]any{"type": "array", "items": genericObjectSchema("UI model item")},
					},
					"additionalProperties": true,
				},
				"AdminBootstrapResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"organization":      genericObjectSchema("Organization root."),
						"locations":         map[string]any{"type": "array", "items": genericObjectSchema("Location")},
						"roles":             map[string]any{"type": "array", "items": genericObjectSchema("Role")},
						"menus":             map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/UIMenuDefinition"}},
						"actions":           map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/UIActionDefinition"}},
						"views":             map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/UIViewDefinition"}},
						"custom_entries":    map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/UICustomEntryDefinition"}},
						"default_path":      map[string]any{"type": "string"},
						"ui_access":         map[string]any{"type": "boolean"},
						"ui_path":           map[string]any{"type": "string"},
						"locale":            map[string]any{"type": "string"},
						"supported_locales": stringArraySchema(),
					},
				},
				"ModuleListResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/ModuleDetail"}},
					},
				},
				"ModuleDetail": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"manifest":               genericObjectSchema("Module manifest"),
						"installed":              genericObjectSchema("Installed module state"),
						"dependency_state":       map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "boolean"}},
						"dependency_diagnostics": map[string]any{"type": "array", "items": genericObjectSchema("Dependency diagnostic")},
						"lifecycle_state":        map[string]any{"type": "string"},
					},
				},
				"ReferenceTypeDefinition": genericObjectSchema("Reference type definition."),
				"LocalizedText": map[string]any{
					"type":                 "object",
					"additionalProperties": map[string]any{"type": "string"},
				},
			},
		},
	}
}

func configKeys(cfg *config.Service) []string {
	if cfg == nil {
		return nil
	}
	keys := cfg.Keys()
	sort.Strings(keys)
	return keys
}

func operation(tag, supportLevel, summary, description string, requestBody any, parameters []map[string]any, responses map[string]any) map[string]any {
	op := map[string]any{
		"tags":                   []string{tag},
		"summary":                summary,
		"description":            description,
		"x-orbyte-support-level": supportLevel,
		"responses":              withDefaultErrors(responses),
	}
	if len(parameters) > 0 {
		op["parameters"] = parameters
	}
	if requestBody != nil {
		op["requestBody"] = requestBody
	}
	return op
}

func opPath(method, tag, supportLevel, summary, description string, requestBody any, parameters []map[string]any, responses map[string]any) map[string]any {
	return map[string]any{
		method: operation(tag, supportLevel, summary, description, requestBody, parameters, responses),
	}
}

func requestBodyRef(ref string) map[string]any {
	return map[string]any{
		"required": true,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{"$ref": ref},
			},
		},
	}
}

func requestBodyRefWithExample(schema any, example any) map[string]any {
	return map[string]any{
		"required": true,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema":  schema,
				"example": example,
			},
		},
	}
}

func jsonResponse(description, ref string) map[string]any {
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{"$ref": ref},
			},
		},
	}
}

func withDefaultErrors(responses map[string]any) map[string]any {
	if responses == nil {
		responses = map[string]any{}
	}
	for code, description := range map[string]string{
		"400": "Bad request",
		"401": "Unauthorized",
		"403": "Forbidden",
		"404": "Not found",
		"409": "Conflict",
	} {
		if _, exists := responses[code]; !exists {
			responses[code] = jsonResponse(description, "#/components/schemas/ErrorResponse")
		}
	}
	return responses
}

func pathEnumParam(name, description string, values []string) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "path",
		"required":    true,
		"description": description,
		"schema":      map[string]any{"type": "string", "enum": values},
	}
}

func pathStringParam(name, description string) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "path",
		"required":    true,
		"description": description,
		"schema":      map[string]any{"type": "string"},
	}
}

func queryEnumParam(name, description string, values []string) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "query",
		"required":    true,
		"description": description,
		"schema":      map[string]any{"type": "string", "enum": values},
	}
}

func genericObjectSchema(description string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          description,
		"additionalProperties": true,
	}
}

func stringArraySchema() map[string]any {
	return map[string]any{
		"type":  "array",
		"items": map[string]any{"type": "string"},
	}
}

func sortedModelKeys(models *model.Service) []string {
	if models == nil {
		return nil
	}
	defs := models.Definitions()
	keys := make([]string, 0, len(defs))
	for _, def := range defs {
		keys = append(keys, def.Key)
	}
	sort.Strings(keys)
	return keys
}

func sortedDocumentTypes(docs *document.Service) []string {
	if docs == nil {
		return nil
	}
	items := append([]string(nil), docs.DocumentTypes()...)
	sort.Strings(items)
	return items
}

func sortedSearchIndexKeys(searchSvc *search.Service) []string {
	if searchSvc == nil {
		return nil
	}
	defs := searchSvc.IndexDefinitions()
	keys := make([]string, 0, len(defs))
	for _, def := range defs {
		keys = append(keys, def.Key)
	}
	sort.Strings(keys)
	return keys
}

func sortedViewKeys(modules *module.Service, surface module.UISurface) []string {
	if modules == nil {
		return nil
	}
	defs := modules.ViewsForSurface(surface)
	keys := make([]string, 0, len(defs))
	for _, def := range defs {
		keys = append(keys, def.Key)
	}
	sort.Strings(keys)
	return keys
}

func sortedActionPaths(modules *module.Service, surface module.UISurface) []string {
	if modules == nil {
		return nil
	}
	defs := modules.ActionsForSurface(surface)
	keys := make([]string, 0, len(defs))
	for _, def := range defs {
		if strings.TrimSpace(def.RoutePath) != "" {
			keys = append(keys, def.RoutePath)
		}
	}
	sort.Strings(keys)
	return keys
}

func sortedOfflineReferenceKeys(modules *module.Service) []string {
	if modules == nil {
		return nil
	}
	defs := modules.OfflineReferences()
	keys := make([]string, 0, len(defs))
	for _, def := range defs {
		keys = append(keys, def.TypeKey)
	}
	sort.Strings(keys)
	return keys
}

func sortedOfflineProjectionKeys(modules *module.Service) []string {
	if modules == nil {
		return nil
	}
	defs := modules.OfflineProjections()
	keys := make([]string, 0, len(defs))
	for _, def := range defs {
		keys = append(keys, def.IndexKey)
	}
	sort.Strings(keys)
	return keys
}

func sortedModuleKeys(modules *module.Service) []string {
	if modules == nil {
		return nil
	}
	items := modules.List()
	keys := make([]string, 0, len(items))
	for _, item := range items {
		keys = append(keys, item.Manifest.Key)
	}
	sort.Strings(keys)
	return keys
}

func firstOrEmpty(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Orbyte API Docs</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
    <style>
      body { margin: 0; background: #f5f6fa; }
      .topbar { display: none; }
    </style>
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
      window.ui = SwaggerUIBundle({
        url: '/dev/openapi.json',
        dom_id: '#swagger-ui',
        docExpansion: 'list',
        deepLinking: true,
        persistAuthorization: true,
        displayRequestDuration: true
      });
    </script>
  </body>
</html>`
