package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (s *Server) listTools(actor ActorContext) []ToolDescriptor {
	items := make([]ToolDescriptor, 0)
	if s == nil || s.modules == nil {
		return items
	}
	items = append(items, s.listBuiltInTools(actor)...)
	for _, detail := range s.modules.List() {
		if !detail.Installed.Enabled {
			continue
		}
		scope := scopeForModule(detail.Manifest.Key)
		if !scopeMatches(actor.EndpointScope, scope) {
			continue
		}
		for _, def := range detail.Manifest.MCP.Tools {
			if !allowsAll(actor.PermissionChecker, def.RequiredPermissions) {
				continue
			}
			items = append(items, ToolDescriptor{
				Name:        def.Key,
				Title:       def.Title,
				Description: def.Description,
				Scope:       scope,
				InputSchema: cloneMap(def.InputSchema),
				Contract: contractDescriptorFromModule(
					def.Contract,
					def.RequiredPermissions,
					defaultToolSideEffectClass(def.Key, def.Operation),
					defaultToolIdempotency(def.Key, def.Operation),
					"mcp.tool."+strings.TrimSpace(def.Key),
				),
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if strings.TrimSpace(items[i].Name) == strings.TrimSpace(items[j].Name) {
			return strings.TrimSpace(items[i].Scope) < strings.TrimSpace(items[j].Scope)
		}
		return strings.TrimSpace(items[i].Name) < strings.TrimSpace(items[j].Name)
	})
	return items
}

func (s *Server) listResources(actor ActorContext) []ResourceDescriptor {
	items := make([]ResourceDescriptor, 0)
	if s == nil || s.modules == nil {
		return items
	}
	items = append(items, s.listBuiltInResources(actor)...)
	for _, detail := range s.modules.List() {
		if !detail.Installed.Enabled {
			continue
		}
		scope := scopeForModule(detail.Manifest.Key)
		if !scopeMatches(actor.EndpointScope, scope) {
			continue
		}
		for _, def := range detail.Manifest.MCP.Resources {
			if !allowsAll(actor.PermissionChecker, def.RequiredPermissions) {
				continue
			}
			items = append(items, ResourceDescriptor{
				URI:         def.URI,
				Name:        def.Title,
				Description: def.Description,
				Scope:       scope,
				MIMEType:    def.MIMEType,
				Contract: contractDescriptorFromModule(
					def.Contract,
					def.RequiredPermissions,
					"read",
					"read-only",
					"mcp.resource."+strings.TrimSpace(def.Key),
				),
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if strings.TrimSpace(items[i].URI) == strings.TrimSpace(items[j].URI) {
			return strings.TrimSpace(items[i].Scope) < strings.TrimSpace(items[j].Scope)
		}
		return strings.TrimSpace(items[i].URI) < strings.TrimSpace(items[j].URI)
	})
	return items
}

func (s *Server) readResource(actor ActorContext, uri string) ([]ResourceContent, error) {
	if s == nil || s.modules == nil {
		return nil, fmt.Errorf("mcp resources are unavailable")
	}
	if contents, ok, err := s.readBuiltInResource(actor, uri); ok {
		return contents, err
	}
	def, ok := s.lookupResourceByURI(actor.EndpointScope, uri)
	if !ok {
		if _, found := s.lookupResourceByURI(EndpointScopeAll, uri); found {
			return nil, fmt.Errorf("resource is not available on this endpoint")
		}
		return nil, fmt.Errorf("resource not found")
	}
	if !allowsAll(actor.PermissionChecker, def.RequiredPermissions) {
		return nil, fmt.Errorf("resource is not allowed")
	}
	switch def.Provider {
	case "analytics.snapshot.current":
		payload, err := s.analyticsSnapshotPayload(actor)
		if err != nil {
			return nil, err
		}
		body, _ := json.Marshal(payload)
		return []ResourceContent{{URI: def.URI, MIMEType: firstNonEmpty(def.MIMEType, "application/json"), Text: string(body)}}, nil
	case "mcp.app":
		appDef, ok := s.lookupApp(actor.EndpointScope, def.AppKey)
		if !ok {
			return nil, fmt.Errorf("app not found")
		}
		html, err := s.renderApp(actor, appDef)
		if err != nil {
			return nil, err
		}
		return []ResourceContent{{URI: def.URI, MIMEType: firstNonEmpty(def.MIMEType, "text/html"), Text: html}}, nil
	default:
		return nil, fmt.Errorf("unsupported resource provider")
	}
}

func (s *Server) callTool(ctx context.Context, actor ActorContext, name string, arguments map[string]any) (map[string]any, error) {
	ctx, span := s.startToolSpan(ctx, actor, name)
	if span != nil {
		defer span.End()
	}

	start := time.Now()
	result, err := s.executeTool(ctx, actor, name, arguments)
	duration := time.Since(start)

	status := "success"
	if err != nil {
		status = "error"
		if span != nil {
			span.SetStatus(codes.Error, err.Error())
		}
	}

	s.recordToolMetrics(name, status, duration)

	if span != nil {
		span.SetAttributes(attribute.String("tool.status", status))
	}

	return result, err
}

func (s *Server) startToolSpan(ctx context.Context, actor ActorContext, name string) (context.Context, trace.Span) {
	if s.otelTracer == nil {
		return ctx, nil
	}
	return s.otelTracer.Start(ctx, "mcp.tool.call",
		trace.WithAttributes(
			attribute.String("tool.name", name),
			attribute.String("actor.id", actor.ActorID),
		),
	)
}

func (s *Server) executeTool(ctx context.Context, actor ActorContext, name string, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.modules == nil {
		return nil, fmt.Errorf("mcp tools are unavailable")
	}
	if result, ok, err := s.callBuiltInTool(actor, strings.TrimSpace(name), arguments); ok {
		return result, err
	}
	def, ok := s.lookupTool(actor.EndpointScope, strings.TrimSpace(name))
	if !ok {
		if _, found := s.lookupTool(EndpointScopeAll, strings.TrimSpace(name)); found {
			return nil, fmt.Errorf("tool is not available on this endpoint")
		}
		return nil, fmt.Errorf("tool not found")
	}
	if !allowsAll(actor.PermissionChecker, def.RequiredPermissions) {
		return nil, fmt.Errorf("tool is not allowed")
	}
	switch def.Operation {
	case "analytics.snapshot.get":
		payload, err := s.analyticsSnapshotPayload(actor)
		if err != nil {
			return nil, err
		}
		result := map[string]any{
			"content": []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Analytics snapshot generated at %s with %d submitted documents.", payload.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"), payload.Documents.Submitted),
			}},
			"structuredContent": payload,
		}
		if def.AppKey != "" {
			if appDef, ok := s.lookupApp(actor.EndpointScope, def.AppKey); ok {
				if resource, ok := s.lookupResourceByKey(actor.EndpointScope, appDef.ResourceKey); ok {
					result["_meta"] = map[string]any{
						"orbyte/app": map[string]any{
							"key":          appDef.Key,
							"title":        appDef.Title,
							"resource_uri": resource.URI,
							"stream_uri":   s.preferredAnalyticsStreamPath(),
						},
					}
				}
			}
		}
		return result, nil
	default:
		_ = arguments
		return nil, fmt.Errorf("unsupported tool operation")
	}
}

func (s *Server) recordToolMetrics(name, status string, duration time.Duration) {
	if s.observability == nil {
		return
	}
	labels := map[string]string{"tool_name": name, "status": status}
	_ = s.observability.RecordMetric("mcp.tool.calls.total", labels, 1)
	s.observability.ObserveHistogram("mcp.tool.call.duration.ms", float64(duration.Milliseconds()), map[string]string{"tool_name": name})
}
