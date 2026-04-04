package templateoutput

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	xhtml "golang.org/x/net/html"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/reporting"
	"orbyte/internal/platform/shared"
)

type Service struct {
	repo      Repository
	documents *document.Service
	reporting *reporting.Service
	defs      map[string]Definition
}

func NewService(documents *document.Service, reportingSvc *reporting.Service) *Service {
	return &Service{
		repo:      NewMemoryRepository(),
		documents: documents,
		reporting: reportingSvc,
		defs:      map[string]Definition{},
	}
}

func NewServiceWithRepository(repo Repository, documents *document.Service, reportingSvc *reporting.Service) *Service {
	if repo == nil {
		repo = NewMemoryRepository()
	}
	return &Service{repo: repo, documents: documents, reporting: reportingSvc, defs: map[string]Definition{}}
}

func FromModule(def module.TemplateDefinition, moduleKey string) Definition {
	return Definition{
		Key:                 def.Key,
		Title:               def.Title,
		TitleI18n:           def.TitleI18n,
		Description:         def.Description,
		DescriptionI18n:     def.DescriptionI18n,
		OwnerModuleKey:      moduleKey,
		TargetKind:          def.TargetKind,
		TargetKey:           def.TargetKey,
		RendererKind:        def.RendererKind,
		DefaultFormat:       def.DefaultFormat,
		Formats:             append([]string(nil), def.Formats...),
		Purpose:             def.Purpose,
		Channel:             def.Channel,
		RelatedSources:      nil,
		AllowedScopes:       append([]string(nil), def.AllowedScopes...),
		RequiredPermissions: append([]string(nil), def.RequiredPermissions...),
		DefaultBody:         def.DefaultBody,
		DefaultStyle:        def.DefaultStyle,
	}
}

func (s *Service) RegisterDefinition(def Definition) error {
	if strings.TrimSpace(def.Key) == "" || strings.TrimSpace(def.Title) == "" || strings.TrimSpace(def.TargetKind) == "" || strings.TrimSpace(def.TargetKey) == "" {
		return shared.Validation("template key, title, target_kind, and target_key are required")
	}
	if err := validateDefinition(def); err != nil {
		return err
	}
	def.RendererKind = normalizeRenderer(def.RendererKind)
	if def.RendererKind == "" {
		return shared.Validation("template renderer_kind is invalid")
	}
	if def.DefaultFormat == "" {
		def.DefaultFormat = "html"
	}
	if len(def.Formats) == 0 {
		def.Formats = []string{def.DefaultFormat}
	}
	if _, exists := s.repo.GetDefinition(def.Key); exists {
		return nil
	}
	if _, exists := s.defs[def.Key]; exists {
		return nil
	}
	s.defs[def.Key] = def
	return s.repo.SaveDefinition(def)
}

func (s *Service) SaveDefinition(def Definition) (Definition, error) {
	if strings.TrimSpace(def.Key) == "" || strings.TrimSpace(def.Title) == "" || strings.TrimSpace(def.TargetKind) == "" || strings.TrimSpace(def.TargetKey) == "" {
		return Definition{}, shared.Validation("template key, title, target_kind, and target_key are required")
	}
	if err := validateDefinition(def); err != nil {
		return Definition{}, err
	}
	def.RendererKind = normalizeRenderer(def.RendererKind)
	if def.RendererKind == "" {
		return Definition{}, shared.Validation("template renderer_kind is invalid")
	}
	if def.DefaultFormat == "" {
		def.DefaultFormat = "html"
	}
	if len(def.Formats) == 0 {
		def.Formats = []string{def.DefaultFormat}
	}
	if current, ok := s.Definition(def.Key); ok {
		if strings.TrimSpace(def.DefaultBody) == "" {
			def.DefaultBody = current.DefaultBody
		}
		if strings.TrimSpace(def.DefaultStyle) == "" {
			def.DefaultStyle = current.DefaultStyle
		}
		if len(def.AllowedScopes) == 0 {
			def.AllowedScopes = append([]string(nil), current.AllowedScopes...)
		}
		if len(def.RequiredPermissions) == 0 {
			def.RequiredPermissions = append([]string(nil), current.RequiredPermissions...)
		}
	}
	s.defs[def.Key] = def
	if err := s.repo.SaveDefinition(def); err != nil {
		return Definition{}, err
	}
	return def, nil
}

func (s *Service) Definitions() []Definition {
	itemsByKey := map[string]Definition{}
	for _, item := range s.defs {
		itemsByKey[item.Key] = item
	}
	for _, item := range s.repo.Definitions() {
		itemsByKey[item.Key] = item
	}
	items := make([]Definition, 0, len(itemsByKey))
	for _, item := range itemsByKey {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) DeleteDefinition(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return shared.Validation("template key is required")
	}
	if _, ok := s.Definition(key); !ok {
		return shared.NotFound("template definition not found")
	}
	delete(s.defs, key)
	if err := s.repo.DeleteDefinition(key); err != nil {
		return err
	}
	if err := s.repo.DeleteVersions(key); err != nil {
		return err
	}
	if err := s.repo.DeleteFixtures(key); err != nil {
		return err
	}
	for _, binding := range s.Bindings() {
		if binding.TemplateKey == key {
			if err := s.repo.DeleteBinding(binding.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) Definition(key string) (Definition, bool) {
	key = strings.TrimSpace(key)
	if item, ok := s.repo.GetDefinition(key); ok {
		return item, true
	}
	item, ok := s.defs[key]
	return item, ok
}

func (s *Service) Versions(templateKey string) []Version {
	items := s.repo.Versions(strings.TrimSpace(templateKey))
	sort.Slice(items, func(i, j int) bool { return items[i].Version < items[j].Version })
	return items
}

func (s *Service) Bindings() []Binding {
	return s.repo.Bindings()
}

func (s *Service) Fixtures(templateKey, targetKind string) []TemplateFixture {
	return s.repo.Fixtures(strings.TrimSpace(templateKey), strings.TrimSpace(targetKind))
}

func (s *Service) Resolve(req RenderRequest) (Definition, Version, error) {
	return s.resolveTemplate(req)
}

func (s *Service) ResolveBindingDebug(req RenderRequest) (BindingResolutionDebug, error) {
	debug := BindingResolutionDebug{
		RequestedTargetKind: strings.TrimSpace(req.TargetKind),
		RequestedTargetKey:  strings.TrimSpace(req.TargetKey),
		RequestedPurpose:    strings.TrimSpace(req.Purpose),
		RequestedChannel:    strings.TrimSpace(req.Channel),
		Mode:                "published",
	}
	if req.Draft {
		debug.Mode = "draft"
	}
	scopes := resolveScopes(req)
	bindings := s.Bindings()
	for _, candidate := range scopes {
		for _, item := range bindings {
			if item.TargetKind != debug.RequestedTargetKind || item.TargetKey != debug.RequestedTargetKey {
				continue
			}
			if debug.RequestedPurpose != "" && item.Purpose != "" && item.Purpose != debug.RequestedPurpose {
				continue
			}
			if debug.RequestedChannel != "" && item.Channel != "" && item.Channel != debug.RequestedChannel {
				continue
			}
			if item.ScopeType != candidate.ScopeType || item.ScopeID != candidate.ScopeID {
				continue
			}
			matched := item
			debug.MatchedBinding = &matched
			debug.ScopePath = append(debug.ScopePath, Binding{ScopeType: candidate.ScopeType, ScopeID: candidate.ScopeID})
			def, ok := s.Definition(item.TemplateKey)
			if ok {
				version := s.activeVersion(def, req.Draft)
				debug.DefinitionKey = def.Key
				debug.Version = version.Version
			}
			return debug, nil
		}
		debug.ScopePath = append(debug.ScopePath, Binding{ScopeType: candidate.ScopeType, ScopeID: candidate.ScopeID})
	}
	def, version, err := s.resolveTemplate(req)
	if err != nil {
		return debug, err
	}
	debug.DefinitionKey = def.Key
	debug.Version = version.Version
	return debug, nil
}

func (s *Service) HasTemplate(targetKind, targetKey, purpose, channel, scopeType, scopeID string) bool {
	_, _, err := s.resolveTemplate(RenderRequest{
		TargetKind: targetKind,
		TargetKey:  targetKey,
		Purpose:    purpose,
		Channel:    channel,
		ScopeType:  scopeType,
		ScopeID:    scopeID,
	})
	return err == nil
}

func (s *Service) SaveDraft(templateKey, body, style, actorID string) (Version, error) {
	return s.SaveDraftWithOptions(templateKey, body, style, actorID, "", 0)
}

func (s *Service) SaveDraftWithOptions(templateKey, body, style, actorID, changeNote string, clonedFromVersion int) (Version, error) {
	def, ok := s.Definition(templateKey)
	if !ok {
		return Version{}, shared.NotFound("template definition not found")
	}
	versions := s.Versions(templateKey)
	var draft *Version
	maxVersion := 0
	for i := range versions {
		if versions[i].Version > maxVersion {
			maxVersion = versions[i].Version
		}
		if versions[i].Status == "draft" {
			draft = &versions[i]
		}
	}
	now := time.Now().UTC()
	version := Version{
		TemplateKey:       templateKey,
		Version:           maxVersion + 1,
		Status:            "draft",
		RendererKind:      def.RendererKind,
		Body:              body,
		Style:             style,
		ChangeNote:        strings.TrimSpace(changeNote),
		ClonedFromVersion: clonedFromVersion,
		UpdatedAt:         now,
		UpdatedBy:         actorID,
	}
	if draft != nil {
		version.Version = draft.Version
		version.LastPreviewedAt = draft.LastPreviewedAt
		version.LastRenderStatus = draft.LastRenderStatus
		version.LastRenderError = draft.LastRenderError
		version.LastRenderedAt = draft.LastRenderedAt
		if version.ChangeNote == "" {
			version.ChangeNote = draft.ChangeNote
		}
		if version.ClonedFromVersion == 0 {
			version.ClonedFromVersion = draft.ClonedFromVersion
		}
	}
	if strings.TrimSpace(version.Body) == "" {
		version.Body = def.DefaultBody
	}
	if strings.TrimSpace(version.Style) == "" {
		version.Style = def.DefaultStyle
	}
	if issues := s.validateVersion(def, version); len(filterIssues(issues, "error")) > 0 {
		return Version{}, shared.Validation(joinIssueMessages(issues))
	}
	if err := s.repo.SaveVersion(version); err != nil {
		return Version{}, err
	}
	return version, nil
}

func (s *Service) DuplicateDraft(templateKey string, fromVersion int, actorID string) (Version, error) {
	def, ok := s.Definition(templateKey)
	if !ok {
		return Version{}, shared.NotFound("template definition not found")
	}
	var source Version
	found := false
	for _, item := range s.Versions(templateKey) {
		if item.Version == fromVersion {
			source = item
			found = true
			break
		}
	}
	if !found {
		source = s.activeVersion(def, false)
		fromVersion = source.Version
	}
	return s.SaveDraftWithOptions(templateKey, source.Body, source.Style, actorID, "Duplicated from v"+fmt.Sprintf("%d", fromVersion), fromVersion)
}

func (s *Service) ResetDraftToPublished(templateKey, actorID string) (Version, error) {
	def, ok := s.Definition(templateKey)
	if !ok {
		return Version{}, shared.NotFound("template definition not found")
	}
	published := s.activeVersion(def, false)
	return s.SaveDraftWithOptions(templateKey, published.Body, published.Style, actorID, "Reset to published v"+fmt.Sprintf("%d", published.Version), published.Version)
}

func (s *Service) CompareVersions(templateKey string, leftVersionNo, rightVersionNo int) (VersionCompare, error) {
	versions := s.Versions(templateKey)
	var left Version
	var right Version
	foundLeft := false
	foundRight := false
	for _, item := range versions {
		if item.Version == leftVersionNo {
			left = item
			foundLeft = true
		}
		if item.Version == rightVersionNo {
			right = item
			foundRight = true
		}
	}
	if !foundLeft || !foundRight {
		return VersionCompare{}, shared.NotFound("template version not found")
	}
	changed := make([]string, 0)
	if left.Body != right.Body {
		changed = append(changed, "body")
	}
	if left.Style != right.Style {
		changed = append(changed, "style")
	}
	if left.RendererKind != right.RendererKind {
		changed = append(changed, "renderer_kind")
	}
	if left.ChangeNote != right.ChangeNote {
		changed = append(changed, "change_note")
	}
	return VersionCompare{
		TemplateKey:    templateKey,
		LeftVersion:    left,
		RightVersion:   right,
		ChangedFields:  changed,
		HasDifferences: len(changed) > 0,
	}, nil
}

func (s *Service) Publish(templateKey string, versionNo int, actorID string) (Version, error) {
	versions := s.Versions(templateKey)
	found := false
	var target Version
	for _, item := range versions {
		if item.Version == versionNo {
			target = item
			found = true
			continue
		}
		if item.Status == "published" {
			item.Status = "archived"
			if err := s.repo.SaveVersion(item); err != nil {
				return Version{}, err
			}
		}
	}
	if !found {
		return Version{}, shared.NotFound("template version not found")
	}
	target.Status = "published"
	target.PublishedAt = time.Now().UTC()
	target.PublishedBy = actorID
	target.UpdatedAt = target.PublishedAt
	target.UpdatedBy = actorID
	if err := s.repo.SaveVersion(target); err != nil {
		return Version{}, err
	}
	return target, nil
}

func (s *Service) SaveBinding(binding Binding) (Binding, error) {
	if _, ok := s.Definition(binding.TemplateKey); !ok {
		return Binding{}, shared.NotFound("template definition not found")
	}
	if strings.TrimSpace(binding.ScopeType) == "" {
		binding.ScopeType = "deployment"
	}
	if strings.TrimSpace(binding.TargetKind) == "" || strings.TrimSpace(binding.TargetKey) == "" {
		def, _ := s.Definition(binding.TemplateKey)
		binding.TargetKind = def.TargetKind
		binding.TargetKey = def.TargetKey
	}
	for _, current := range s.Bindings() {
		if current.ScopeType != binding.ScopeType || current.ScopeID != binding.ScopeID {
			continue
		}
		if current.TargetKind != binding.TargetKind || current.TargetKey != binding.TargetKey {
			continue
		}
		if current.Purpose != binding.Purpose || current.Channel != binding.Channel {
			continue
		}
		binding.ID = current.ID
		break
	}
	if strings.TrimSpace(binding.ID) == "" {
		binding.ID = shared.NewID("tmplbind")
	}
	binding.UpdatedAt = time.Now().UTC()
	if err := s.repo.SaveBinding(binding); err != nil {
		return Binding{}, err
	}
	return binding, nil
}

func (s *Service) DeleteBinding(bindingID string) error {
	bindingID = strings.TrimSpace(bindingID)
	if bindingID == "" {
		return shared.Validation("binding id is required")
	}
	for _, binding := range s.Bindings() {
		if binding.ID == bindingID {
			return s.repo.DeleteBinding(bindingID)
		}
	}
	return shared.NotFound("template binding not found")
}

func (s *Service) SaveFixture(fixture TemplateFixture) (TemplateFixture, error) {
	defKey := strings.TrimSpace(fixture.TemplateKey)
	if defKey != "" {
		def, ok := s.Definition(defKey)
		if !ok {
			return TemplateFixture{}, shared.NotFound("template definition not found")
		}
		if strings.TrimSpace(fixture.TargetKind) == "" {
			fixture.TargetKind = def.TargetKind
		}
	}
	if strings.TrimSpace(fixture.TargetKind) == "" {
		return TemplateFixture{}, shared.Validation("fixture target_kind is required")
	}
	if strings.TrimSpace(fixture.FixtureKey) == "" {
		fixture.FixtureKey = shared.NewID("tmplfixture")
	}
	if strings.TrimSpace(fixture.Name) == "" {
		fixture.Name = fixture.FixtureKey
	}
	if fixture.Payload == nil {
		fixture.Payload = map[string]any{}
	}
	fixture.UpdatedAt = time.Now().UTC()
	if err := s.repo.SaveFixture(fixture); err != nil {
		return TemplateFixture{}, err
	}
	return fixture, nil
}

func (s *Service) Validate(req RenderRequest) []ValidationIssue {
	def, ok := s.Definition(strings.TrimSpace(req.TemplateKey))
	if !ok {
		return []ValidationIssue{{Code: "template_definition_not_found", Severity: "error", Message: "template definition not found"}}
	}
	version := Version{
		TemplateKey:  def.Key,
		RendererKind: normalizeRenderer(req.RendererKind),
		Body:         req.Body,
		Style:        req.Style,
	}
	if version.RendererKind == "" {
		version.RendererKind = def.RendererKind
	}
	if strings.TrimSpace(version.Body) == "" {
		version.Body = def.DefaultBody
	}
	if strings.TrimSpace(version.Style) == "" {
		version.Style = def.DefaultStyle
	}
	return s.validateVersion(def, version)
}

func (s *Service) Render(req RenderRequest) (RenderedOutput, error) {
	_, version, output, err := s.renderPrepared(req)
	if err != nil {
		return RenderedOutput{}, err
	}
	s.markVersionRendered(version, "succeeded", "")
	return output, nil
}

func (s *Service) Preview(req RenderRequest) (PreviewResponse, error) {
	def, version, htmlOutput, err := s.renderPrepared(RenderRequest{
		TemplateKey:    req.TemplateKey,
		RendererKind:   req.RendererKind,
		Body:           req.Body,
		Style:          req.Style,
		TargetKind:     req.TargetKind,
		TargetKey:      req.TargetKey,
		TargetID:       req.TargetID,
		Sample:         req.Sample,
		OrganizationID: req.OrganizationID,
		LocationID:     req.LocationID,
		ScopeType:      req.ScopeType,
		ScopeID:        req.ScopeID,
		Purpose:        req.Purpose,
		Channel:        req.Channel,
		Draft:          req.Draft,
		FixtureKey:     req.FixtureKey,
		Query:          req.Query,
		ReportView:     req.ReportView,
		Format:         "html",
	})
	if err != nil {
		return PreviewResponse{}, err
	}
	pdfOutput := PreviewOutput{Format: "pdf", Status: "ok"}
	pdfRendered, pdfErr := s.Render(mergeRenderRequest(req, "pdf"))
	if pdfErr != nil {
		pdfOutput.Status = "error"
		pdfOutput.Issues = []ValidationIssue{{Code: "pdf_render_failed", Severity: "error", Message: pdfErr.Error()}}
	} else {
		pdfOutput.ContentType = pdfRendered.ContentType
		pdfOutput.FileName = pdfRendered.FileName
		pdfOutput.Warnings = pdfRendered.Warnings
		pdfOutput.Issues = pdfRendered.Issues
	}
	printOutput := PreviewOutput{
		Format:      "print",
		Status:      "ok",
		ContentType: "text/html; charset=utf-8",
		FileName:    fileNameFor(def, req.TargetID, "html"),
		HTML:        htmlOutput.HTML,
		Warnings:    collectRendererWarnings(version, "print"),
		Issues:      htmlOutput.Issues,
	}
	debug, _ := s.ResolveBindingDebug(req)
	resp := PreviewResponse{
		TemplateKey:        def.Key,
		SelectedVersion:    version.Version,
		Mode:               map[bool]string{true: "draft", false: "published"}[req.Draft],
		DataSource:         htmlOutput.DataSource,
		RenderID:           htmlOutput.RenderID,
		GeneratedAt:        htmlOutput.GeneratedAt,
		BindingResolution:  debug,
		DataContextSummary: dataContextSummary(def, htmlOutput.DataSource, req),
		Outputs: []PreviewOutput{
			{Format: "html", Status: "ok", ContentType: htmlOutput.ContentType, FileName: htmlOutput.FileName, HTML: htmlOutput.HTML, Warnings: htmlOutput.Warnings, Issues: htmlOutput.Issues},
			pdfOutput,
			printOutput,
		},
		Warnings: append(append([]RendererWarning(nil), htmlOutput.Warnings...), printOutput.Warnings...),
	}
	if pdfOutput.Status == "error" {
		resp.Issues = append(resp.Issues, pdfOutput.Issues...)
	}
	if len(resp.Issues) == 0 {
		resp.Issues = htmlOutput.Issues
	}
	s.markVersionRendered(version, "previewed", "")
	return resp, nil
}

func (s *Service) resolveTemplate(req RenderRequest) (Definition, Version, error) {
	if key := strings.TrimSpace(req.TemplateKey); key != "" {
		def, ok := s.Definition(key)
		if !ok {
			return Definition{}, Version{}, shared.NotFound("template definition not found")
		}
		return def, s.activeVersion(def, req.Draft), nil
	}
	targetKind := strings.TrimSpace(req.TargetKind)
	targetKey := strings.TrimSpace(req.TargetKey)
	purpose := strings.TrimSpace(req.Purpose)
	channel := strings.TrimSpace(req.Channel)
	bindings := s.Bindings()
	for _, candidate := range resolveScopes(req) {
		for _, item := range bindings {
			if item.TargetKind != targetKind || item.TargetKey != targetKey {
				continue
			}
			if purpose != "" && item.Purpose != "" && item.Purpose != purpose {
				continue
			}
			if channel != "" && item.Channel != "" && item.Channel != channel {
				continue
			}
			if item.ScopeType != candidate.ScopeType || item.ScopeID != candidate.ScopeID {
				continue
			}
			def, ok := s.Definition(item.TemplateKey)
			if ok {
				return def, s.activeVersion(def, req.Draft), nil
			}
		}
	}
	for _, def := range s.Definitions() {
		if def.TargetKind == targetKind && def.TargetKey == targetKey {
			if purpose != "" && def.Purpose != "" && def.Purpose != purpose {
				continue
			}
			if channel != "" && def.Channel != "" && def.Channel != channel {
				continue
			}
			return def, s.activeVersion(def, req.Draft), nil
		}
	}
	return Definition{}, Version{}, shared.NotFound("template not resolved")
}

func (s *Service) activeVersion(def Definition, draft bool) Version {
	versions := s.Versions(def.Key)
	if draft {
		for _, item := range versions {
			if item.Status == "draft" {
				return item
			}
		}
	}
	for _, item := range versions {
		if item.Status == "published" {
			return item
		}
	}
	for _, item := range versions {
		if item.Status == "draft" {
			return item
		}
	}
	return Version{
		TemplateKey:  def.Key,
		Version:      1,
		Status:       "published",
		RendererKind: def.RendererKind,
		Body:         def.DefaultBody,
		Style:        def.DefaultStyle,
	}
}

func (s *Service) renderContext(req RenderRequest, def Definition) (map[string]any, error) {
	ctx := map[string]any{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"template": map[string]any{
			"key":   def.Key,
			"title": def.Title,
		},
	}
	if fixture, err := s.resolveFixture(req, def); err != nil {
		return nil, err
	} else if fixture != nil {
		switch def.TargetKind {
		case "document":
			ctx["document"] = fixture.Payload
		case "report":
			ctx["report"] = fixture.Payload
		default:
			return nil, shared.Validation("unsupported template target kind")
		}
		s.attachRelatedSources(ctx, def, fixture.Payload, req.Sample)
		return ctx, nil
	}
	switch def.TargetKind {
	case "document":
		if req.Sample || strings.TrimSpace(req.TargetID) == "" {
			ctx["document"] = sampleDocument(def)
			s.attachRelatedSources(ctx, def, ctx["document"], true)
			return ctx, nil
		}
		record, err := s.documents.Get(req.TargetID)
		if err != nil {
			return nil, err
		}
		ctx["document"] = normalizeDocument(record)
		s.attachRelatedSources(ctx, def, record, false)
	case "report":
		key := req.TargetKey
		if key == "" {
			key = def.TargetKey
		}
		if req.Sample {
			ctx["report"] = sampleReport(def, key)
			s.attachRelatedSources(ctx, def, ctx["report"], true)
			return ctx, nil
		}
		result, err := s.reporting.ExecuteView(key, req.Query, req.ReportView)
		if err != nil {
			return nil, err
		}
		ctx["report"] = result
		s.attachRelatedSources(ctx, def, result, false)
	default:
		return nil, shared.Validation("unsupported template target kind")
	}
	return ctx, nil
}

func (s *Service) renderPrepared(req RenderRequest) (Definition, Version, RenderedOutput, error) {
	resolvedDef, version, err := s.resolveTemplate(req)
	if err != nil {
		return Definition{}, Version{}, RenderedOutput{}, err
	}
	if strings.TrimSpace(req.Body) != "" {
		version.Body = req.Body
	}
	if strings.TrimSpace(req.Style) != "" {
		version.Style = req.Style
	}
	if renderer := normalizeRenderer(req.RendererKind); renderer != "" {
		version.RendererKind = renderer
	}
	issues := s.validateVersion(resolvedDef, version)
	if len(filterIssues(issues, "error")) > 0 {
		s.markVersionRendered(version, "failed", joinIssueMessages(issues))
		return Definition{}, Version{}, RenderedOutput{}, shared.Validation(joinIssueMessages(issues))
	}
	ctx, err := s.renderContext(req, resolvedDef)
	if err != nil {
		s.markVersionRendered(version, "failed", err.Error())
		return Definition{}, Version{}, RenderedOutput{}, err
	}
	htmlText, err := s.renderHTML(version, ctx)
	if err != nil {
		s.markVersionRendered(version, "failed", err.Error())
		return Definition{}, Version{}, RenderedOutput{}, err
	}
	format := strings.TrimSpace(req.Format)
	if format == "" {
		format = resolvedDef.DefaultFormat
		if format == "" {
			format = "html"
		}
	}
	renderID := shared.NewID("tmplrender")
	output := RenderedOutput{
		TemplateKey: resolvedDef.Key,
		Version:     version.Version,
		Format:      format,
		FileName:    fileNameFor(resolvedDef, req.TargetID, format),
		HTML:        htmlText,
		GeneratedAt: time.Now().UTC(),
		Official:    !req.Draft,
		RenderID:    renderID,
		DataSource:  renderDataSource(req),
		Warnings:    collectRendererWarnings(version, format),
		Issues:      filterIssues(issues, "warning"),
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "html", "print":
		output.ContentType = "text/html; charset=utf-8"
		return resolvedDef, version, output, nil
	case "pdf":
		pdfBytes, err := renderPDF(version, ctx, htmlText)
		if err != nil {
			s.markVersionRendered(version, "failed", err.Error())
			return Definition{}, Version{}, RenderedOutput{}, err
		}
		output.ContentType = "application/pdf"
		output.Bytes = pdfBytes
		return resolvedDef, version, output, nil
	default:
		output.ContentType = "text/html; charset=utf-8"
		return resolvedDef, version, output, nil
	}
}

func (s *Service) markVersionRendered(version Version, status, renderErr string) {
	if version.Version <= 0 || strings.TrimSpace(version.TemplateKey) == "" {
		return
	}
	version.LastPreviewedAt = time.Now().UTC()
	version.LastRenderStatus = strings.TrimSpace(status)
	version.LastRenderError = strings.TrimSpace(renderErr)
	version.LastRenderedAt = version.LastPreviewedAt
	_ = s.repo.SaveVersion(version)
}

func (s *Service) resolveFixture(req RenderRequest, def Definition) (*TemplateFixture, error) {
	if strings.TrimSpace(req.FixtureKey) != "" {
		for _, item := range s.Fixtures("", def.TargetKind) {
			if item.FixtureKey == req.FixtureKey {
				fixture := item
				return &fixture, nil
			}
		}
		return nil, shared.Validation("template fixture not found")
	}
	for _, item := range s.Fixtures(def.Key, def.TargetKind) {
		if item.TemplateKey == def.Key {
			fixture := item
			return &fixture, nil
		}
	}
	for _, item := range s.Fixtures("", def.TargetKind) {
		if strings.TrimSpace(item.TemplateKey) == "" {
			fixture := item
			return &fixture, nil
		}
	}
	return nil, nil
}

func (s *Service) validateVersion(def Definition, version Version) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	if strings.TrimSpace(version.Body) == "" {
		issues = append(issues, ValidationIssue{Code: "template_body_required", Severity: "error", Message: "template body is required"})
		return issues
	}
	switch normalizeRenderer(version.RendererKind) {
	case "html":
		if _, err := template.New(def.Key).Funcs(template.FuncMap{"upper": strings.ToUpper, "lower": strings.ToLower, "escape": html.EscapeString}).Parse(version.Body); err != nil {
			issues = append(issues, ValidationIssue{Code: "html_template_invalid", Severity: "error", Message: err.Error()})
		}
	case "visual":
		var visual VisualTemplate
		if err := json.Unmarshal([]byte(version.Body), &visual); err != nil {
			issues = append(issues, ValidationIssue{Code: "visual_template_invalid_json", Severity: "error", Message: "visual template body must be valid json"})
			return issues
		}
		issues = append(issues, validateVisualTemplate(visual)...)
	default:
		issues = append(issues, ValidationIssue{Code: "renderer_kind_invalid", Severity: "error", Message: "unsupported template renderer_kind"})
	}
	return issues
}

func validateVisualTemplate(visual VisualTemplate) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	if strings.TrimSpace(visual.SchemaVersion) != "" && !strings.EqualFold(strings.TrimSpace(visual.SchemaVersion), "visual-grid/v1") {
		issues = append(issues, ValidationIssue{Code: "visual_schema_unsupported", Path: "schema_version", Severity: "warning", Message: "schema_version is not visual-grid/v1"})
	}
	for sectionIndex, section := range visual.Sections {
		if len(section.Rows) == 0 {
			issues = append(issues, ValidationIssue{Code: "visual_section_empty", Path: fmt.Sprintf("sections[%d]", sectionIndex), Severity: "warning", Message: "section has no rows"})
		}
		for rowIndex, row := range section.Rows {
			if len(row.Columns) == 0 {
				issues = append(issues, ValidationIssue{Code: "visual_row_empty", Path: fmt.Sprintf("sections[%d].rows[%d]", sectionIndex, rowIndex), Severity: "warning", Message: "row has no columns"})
			}
			rowPath := fmt.Sprintf("sections[%d].rows[%d]", sectionIndex, rowIndex)
			issues = append(issues, validateVisualLayoutFields(rowPath, row.Width, row.Height, row.AlignX, row.AlignY, row.ContentAlignX, row.ContentAlignY)...)
			for colIndex, column := range row.Columns {
				columnPath := fmt.Sprintf("sections[%d].rows[%d].columns[%d]", sectionIndex, rowIndex, colIndex)
				if strings.TrimSpace(column.Width) == "" && (column.Span <= 0 || column.Span > 12) {
					issues = append(issues, ValidationIssue{Code: "visual_span_invalid", Path: columnPath + ".span", Severity: "error", Message: "column span must be between 1 and 12"})
				}
				issues = append(issues, validateVisualLayoutFields(columnPath, column.Width, column.Height, column.AlignX, column.AlignY, column.ContentAlignX, column.ContentAlignY)...)
				for blockIndex, block := range column.Blocks {
					path := fmt.Sprintf("%s.blocks[%d]", columnPath, blockIndex)
					switch strings.ToLower(strings.TrimSpace(block.Type)) {
					case "text", "divider", "image", "barcode", "qr", "signature":
					case "field":
						if strings.TrimSpace(block.Path) == "" {
							issues = append(issues, ValidationIssue{Code: "visual_field_path_required", Path: path + ".path", Severity: "warning", Message: "field block should declare a path"})
						}
					case "table":
						if strings.TrimSpace(block.RowsPath) == "" {
							issues = append(issues, ValidationIssue{Code: "visual_table_rows_path_required", Path: path + ".rows_path", Severity: "error", Message: "table block rows_path is required"})
						}
					case "totals":
						if strings.TrimSpace(block.RowsPath) == "" || strings.TrimSpace(block.Path) == "" {
							issues = append(issues, ValidationIssue{Code: "visual_totals_path_required", Path: path, Severity: "error", Message: "totals block requires rows_path and path"})
						}
					default:
						issues = append(issues, ValidationIssue{Code: "visual_block_type_unknown", Path: path + ".type", Severity: "warning", Message: "block type is not explicitly supported"})
					}
				}
			}
		}
	}
	return issues
}

func validateVisualLayoutFields(path, width, height, alignX, alignY, contentAlignX, contentAlignY string) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	if width != "" && strings.TrimSpace(width) == "" {
		issues = append(issues, ValidationIssue{Code: "visual_width_invalid", Path: path + ".width", Severity: "warning", Message: "width should be a non-empty css length or percentage"})
	}
	if height != "" && strings.TrimSpace(height) == "" {
		issues = append(issues, ValidationIssue{Code: "visual_height_invalid", Path: path + ".height", Severity: "warning", Message: "height should be a non-empty css length"})
	}
	if alignX != "" && normalizeVisualAlignX(alignX) == "" {
		issues = append(issues, ValidationIssue{Code: "visual_align_x_invalid", Path: path + ".align_x", Severity: "warning", Message: "align_x must be start, center, end, or stretch"})
	}
	if alignY != "" && normalizeVisualAlignY(alignY) == "" {
		issues = append(issues, ValidationIssue{Code: "visual_align_y_invalid", Path: path + ".align_y", Severity: "warning", Message: "align_y must be start, center, end, or stretch"})
	}
	if contentAlignX != "" && normalizeVisualContentAlignX(contentAlignX) == "" {
		issues = append(issues, ValidationIssue{Code: "visual_content_align_x_invalid", Path: path + ".content_align_x", Severity: "warning", Message: "content_align_x must be start, center, or end"})
	}
	if contentAlignY != "" && normalizeVisualContentAlignY(contentAlignY) == "" {
		issues = append(issues, ValidationIssue{Code: "visual_content_align_y_invalid", Path: path + ".content_align_y", Severity: "warning", Message: "content_align_y must be start, center, end, or stretch"})
	}
	return issues
}

func collectRendererWarnings(version Version, format string) []RendererWarning {
	if normalizeRenderer(version.RendererKind) != "visual" {
		return nil
	}
	var visual VisualTemplate
	if err := json.Unmarshal([]byte(version.Body), &visual); err != nil {
		return nil
	}
	format = strings.ToLower(strings.TrimSpace(format))
	warnings := make([]RendererWarning, 0)
	for _, section := range visual.Sections {
		for _, row := range section.Rows {
			for _, column := range row.Columns {
				for _, block := range column.Blocks {
					switch strings.ToLower(strings.TrimSpace(block.Type)) {
					case "image":
						if format == "pdf" && strings.TrimSpace(block.ImageURL) == "" {
							warnings = append(warnings, RendererWarning{Code: "image_placeholder", Renderer: format, Message: "image block without image_url will render as a placeholder"})
						}
					case "barcode", "qr", "signature":
						if format == "pdf" || format == "print" {
							warnings = append(warnings, RendererWarning{Code: "visual_block_simplified", Renderer: format, Message: "barcode, qr, and signature blocks render in simplified form"})
						}
					}
				}
			}
		}
	}
	return warnings
}

func joinIssueMessages(issues []ValidationIssue) string {
	messages := make([]string, 0, len(issues))
	for _, item := range issues {
		if item.Severity == "error" {
			messages = append(messages, item.Message)
		}
	}
	if len(messages) == 0 {
		for _, item := range issues {
			messages = append(messages, item.Message)
		}
	}
	return strings.Join(messages, "; ")
}

func filterIssues(issues []ValidationIssue, severity string) []ValidationIssue {
	if severity == "" {
		return append([]ValidationIssue(nil), issues...)
	}
	out := make([]ValidationIssue, 0, len(issues))
	for _, item := range issues {
		if item.Severity == severity {
			out = append(out, item)
		}
	}
	return out
}

func validateDefinition(def Definition) error {
	switch strings.TrimSpace(def.TargetKind) {
	case "document", "report":
	default:
		return shared.Validation("template target_kind must be document or report")
	}
	for _, source := range def.RelatedSources {
		if strings.TrimSpace(source.Key) == "" {
			return shared.Validation("template related_sources key is required")
		}
		if strings.TrimSpace(source.TargetKind) == "" || strings.TrimSpace(source.TargetKey) == "" {
			return shared.Validation("template related_sources target_kind and target_key are required")
		}
		switch strings.TrimSpace(source.TargetKind) {
		case "document":
		default:
			return shared.Validation("template related_sources target_kind must be document")
		}
		switch normalizeRelationMode(source.RelationMode) {
		case "direct", "indirect":
		default:
			return shared.Validation("template related_sources relation_mode must be direct or indirect")
		}
		switch strings.TrimSpace(source.SourceKind) {
		case "", "document_link", "report_row_document":
		default:
			return shared.Validation("template related_sources source_kind is invalid")
		}
	}
	return nil
}

func mergeRenderRequest(req RenderRequest, format string) RenderRequest {
	req.Format = format
	return req
}

func renderDataSource(req RenderRequest) string {
	if strings.TrimSpace(req.FixtureKey) != "" {
		return "fixture"
	}
	if req.Sample {
		return "sample"
	}
	return "live"
}

func dataContextSummary(def Definition, dataSource string, req RenderRequest) map[string]any {
	return map[string]any{
		"target_kind":     def.TargetKind,
		"target_key":      coalesceString(req.TargetKey, def.TargetKey),
		"target_id":       strings.TrimSpace(req.TargetID),
		"data_source":     dataSource,
		"fixture_key":     strings.TrimSpace(req.FixtureKey),
		"related_sources": def.RelatedSources,
	}
}

func coalesceString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *Service) renderHTML(version Version, ctx map[string]any) (string, error) {
	switch version.RendererKind {
	case "html":
		return executeHTMLTemplate(version, ctx)
	case "visual":
		return renderVisualTemplate(version, ctx)
	default:
		return "", shared.Validation("unsupported template renderer_kind")
	}
}

func executeHTMLTemplate(version Version, ctx map[string]any) (string, error) {
	body := version.Body
	if strings.TrimSpace(body) == "" {
		return "", shared.Validation("template body is required")
	}
	tpl, err := template.New(version.TemplateKey).Funcs(template.FuncMap{
		"upper":  strings.ToUpper,
		"lower":  strings.ToLower,
		"escape": html.EscapeString,
	}).Parse(body)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := tpl.Execute(&out, ctx); err != nil {
		return "", err
	}
	if strings.TrimSpace(version.Style) == "" {
		return out.String(), nil
	}
	return "<style>" + version.Style + "</style>" + out.String(), nil
}

func renderVisualTemplate(version Version, ctx map[string]any) (string, error) {
	var visual VisualTemplate
	if err := json.Unmarshal([]byte(version.Body), &visual); err != nil {
		return "", shared.Validation("visual template body must be valid json")
	}
	visual = normalizeVisualTemplate(visual)
	var out strings.Builder
	out.WriteString(`<section class="template-visual ` + html.EscapeString(presetClass(visual.Settings)) + ` ` + html.EscapeString(densityClass(visual.Settings.Density)) + `">`)
	if visual.Title != "" {
		out.WriteString(`<header class="template-page-title"><h1>` + html.EscapeString(visual.Title) + `</h1></header>`)
	}
	for _, section := range visual.Sections {
		out.WriteString(`<article class="template-section section-` + html.EscapeString(section.ID) + `">`)
		if section.Title != "" {
			out.WriteString(`<h2>` + html.EscapeString(section.Title) + `</h2>`)
		}
		for _, row := range section.Rows {
			out.WriteString(`<div class="template-row-shell"`)
			if style := visualRowShellStyle(row); style != "" {
				out.WriteString(` style="` + html.EscapeString(style) + `"`)
			}
			out.WriteString(`>`)
			out.WriteString(`<div class="template-row"`)
			if style := visualRowStyle(row); style != "" {
				out.WriteString(` style="` + html.EscapeString(style) + `"`)
			}
			out.WriteString(`>`)
			for _, cell := range row.Columns {
				span := cell.Span
				if span <= 0 || span > 12 {
					span = 12
				}
				out.WriteString(`<div class="template-cell span-` + fmt.Sprintf("%d", span) + `"`)
				if style := visualCellStyle(cell); style != "" {
					out.WriteString(` style="` + html.EscapeString(style) + `"`)
				}
				out.WriteString(`>`)
				for _, block := range cell.Blocks {
					if !visualVisible(ctx, block.VisibleIf) {
						continue
					}
					out.WriteString(renderVisualBlock(ctx, block))
				}
				out.WriteString(`</div>`)
			}
			out.WriteString(`</div>`)
			out.WriteString(`</div>`)
		}
		out.WriteString(`</article>`)
	}
	out.WriteString(`</section>`)
	if strings.TrimSpace(version.Style) != "" {
		return "<style>" + version.Style + "</style>" + out.String(), nil
	}
	return out.String(), nil
}

func visualRowShellStyle(row VisualRow) string {
	parts := []string{
		"display:flex",
		"width:100%",
		"align-items:" + visualCSSPosition(normalizeVisualAlignY(row.AlignY), false),
	}
	if justify := visualFlexJustifyPosition(normalizeVisualAlignX(row.AlignX)); justify != "" {
		parts = append(parts, "justify-content:"+justify)
	}
	if value := visualLength(row.Height); value != "" {
		parts = append(parts, "min-height:"+value)
	}
	return strings.Join(parts, ";")
}

func visualRowStyle(row VisualRow) string {
	parts := []string{
		"display:grid",
		"grid-template-columns:repeat(12,minmax(0,1fr))",
		"gap:12px",
		"width:" + visualWidthValue(row.Width),
		"justify-items:" + visualCSSPosition(normalizeVisualContentAlignX(row.ContentAlignX), true),
		"align-items:" + visualCSSPosition(normalizeVisualContentAlignY(row.ContentAlignY), false),
	}
	if value := visualLength(row.Height); value != "" {
		parts = append(parts, "min-height:"+value)
	}
	return strings.Join(parts, ";")
}

func visualCellStyle(cell VisualCell) string {
	parts := []string{
		"display:flex",
		"flex-direction:column",
		"gap:12px",
		"justify-content:" + visualCSSPosition(normalizeVisualContentAlignY(cell.ContentAlignY), false),
		"align-items:" + visualCSSPosition(normalizeVisualContentAlignX(cell.ContentAlignX), true),
	}
	if justify := visualFlexJustifyPosition(normalizeVisualContentAlignY(cell.ContentAlignY)); justify != "" {
		parts[3] = "justify-content:" + justify
	} else {
		parts = append(parts[:3], parts[4:]...)
	}
	if value := visualLength(cell.Width); value != "" {
		parts = append(parts, "grid-column:auto")
		parts = append(parts, "width:"+value)
		parts = append(parts, "justify-self:"+visualCSSPosition(normalizeVisualAlignX(cell.AlignX), true))
	} else {
		parts = append(parts, "grid-column:span "+fmt.Sprintf("%d", cell.Span)+" / span "+fmt.Sprintf("%d", cell.Span))
	}
	parts = append(parts, "align-self:"+visualCSSPosition(normalizeVisualAlignY(cell.AlignY), false))
	if value := visualLength(cell.Height); value != "" {
		parts = append(parts, "min-height:"+value)
	}
	return strings.Join(parts, ";")
}

func visualLength(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return value
}

func visualWidthValue(value string) string {
	if trimmed := visualLength(value); trimmed != "" {
		return trimmed
	}
	return "100%"
}

func normalizeVisualAlignX(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "start":
		return "start"
	case "center":
		return "center"
	case "end":
		return "end"
	case "stretch":
		return "stretch"
	default:
		return ""
	}
}

func normalizeVisualAlignY(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "start":
		return "start"
	case "center":
		return "center"
	case "end":
		return "end"
	case "stretch":
		return "stretch"
	default:
		return ""
	}
}

func normalizeVisualContentAlignX(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "start":
		return "start"
	case "center":
		return "center"
	case "end":
		return "end"
	default:
		return ""
	}
}

func normalizeVisualContentAlignY(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "start":
		return "start"
	case "center":
		return "center"
	case "end":
		return "end"
	case "stretch":
		return "stretch"
	default:
		return ""
	}
}

func visualCSSPosition(value string, horizontal bool) string {
	switch value {
	case "center":
		return "center"
	case "end":
		return "end"
	case "stretch":
		if horizontal {
			return "stretch"
		}
		return "stretch"
	default:
		if horizontal {
			return "start"
		}
		return "start"
	}
}

func visualFlexJustifyPosition(value string) string {
	switch value {
	case "center":
		return "center"
	case "end":
		return "flex-end"
	case "stretch":
		return ""
	default:
		return "flex-start"
	}
}

func renderVisualBlock(ctx map[string]any, block VisualBlock) string {
	alignClass := ""
	switch strings.ToLower(strings.TrimSpace(block.Align)) {
	case "center":
		alignClass = " align-center"
	case "right", "end":
		alignClass = " align-right"
	}
	emphasisClass := ""
	switch strings.ToLower(strings.TrimSpace(block.Emphasis)) {
	case "strong", "bold":
		emphasisClass = " emphasis-strong"
	case "muted":
		emphasisClass = " emphasis-muted"
	}
	sizeClass := ""
	switch strings.ToLower(strings.TrimSpace(block.FontSize)) {
	case "sm":
		sizeClass = " size-sm"
	case "lg":
		sizeClass = " size-lg"
	case "xl":
		sizeClass = " size-xl"
	}
	var out strings.Builder
	out.WriteString(`<div class="template-block block-` + html.EscapeString(block.Type) + alignClass + emphasisClass + sizeClass + `">`)
	switch block.Type {
	case "text":
		out.WriteString(`<p>` + html.EscapeString(block.Text) + `</p>`)
	case "field":
		value := fmt.Sprintf("%v", lookupPath(ctx, block.Path))
		if block.Format == "money" && value == "<nil>" {
			value = "0"
		}
		label := strings.TrimSpace(block.Label)
		if label != "" {
			out.WriteString(`<div class="template-field"><span class="template-field-label">` + html.EscapeString(label) + `</span><span class="template-field-value">` + html.EscapeString(value) + `</span></div>`)
		} else {
			out.WriteString(`<div class="template-value">` + html.EscapeString(value) + `</div>`)
		}
	case "image":
		if strings.TrimSpace(block.ImageURL) != "" {
			out.WriteString(`<img class="template-image" src="` + html.EscapeString(block.ImageURL) + `" alt="` + html.EscapeString(block.Alt) + `">`)
		} else {
			out.WriteString(`<div class="template-image placeholder">` + html.EscapeString(block.Label) + `</div>`)
		}
	case "divider":
		out.WriteString(`<hr class="template-divider">`)
	case "barcode", "qr":
		value := strings.TrimSpace(block.Value)
		if value == "" {
			value = fmt.Sprintf("%v", lookupPath(ctx, block.Path))
		}
		out.WriteString(`<div class="template-code"><span class="template-code-label">` + html.EscapeString(strings.ToUpper(block.Type)) + `</span><code>` + html.EscapeString(value) + `</code></div>`)
	case "signature":
		label := block.Label
		if strings.TrimSpace(label) == "" {
			label = "Authorized Signature"
		}
		out.WriteString(`<div class="template-signature"><div class="template-signature-line"></div><span>` + html.EscapeString(label) + `</span></div>`)
	case "totals":
		rows := normalizeSlice(lookupPath(ctx, block.RowsPath))
		total := 0.0
		for _, row := range rows {
			switch v := lookupPath(row, block.Path).(type) {
			case float64:
				total += v
			case float32:
				total += float64(v)
			case int:
				total += float64(v)
			case int64:
				total += float64(v)
			}
		}
		label := block.Label
		if strings.TrimSpace(label) == "" {
			label = "Total"
		}
		out.WriteString(`<div class="template-field total"><span class="template-field-label">` + html.EscapeString(label) + `</span><span class="template-field-value">` + html.EscapeString(fmt.Sprintf("%.2f", total)) + `</span></div>`)
	case "table":
		rows := normalizeSlice(lookupPath(ctx, block.RowsPath))
		out.WriteString(`<div class="template-table-wrap"><table class="template-table"><thead><tr>`)
		for _, col := range block.Columns {
			out.WriteString(`<th>` + html.EscapeString(col.Label) + `</th>`)
		}
		out.WriteString(`</tr></thead><tbody>`)
		for _, row := range rows {
			out.WriteString(`<tr>`)
			for _, col := range block.Columns {
				out.WriteString(`<td>` + html.EscapeString(fmt.Sprintf("%v", lookupPath(row, col.Path))) + `</td>`)
			}
			out.WriteString(`</tr>`)
		}
		if len(rows) == 0 {
			out.WriteString(`<tr><td colspan="` + fmt.Sprintf("%d", maxInt(1, len(block.Columns))) + `" class="template-empty">No rows</td></tr>`)
		}
		out.WriteString(`</tbody></table></div>`)
	default:
		out.WriteString(`<div class="template-value">` + html.EscapeString(block.Label) + `</div>`)
	}
	out.WriteString(`</div>`)
	return out.String()
}

func lookupPath(value any, path string) any {
	path = strings.TrimSpace(path)
	if path == "" {
		return value
	}
	current := normalizeValue(value)
	for _, part := range strings.Split(path, ".") {
		switch typed := current.(type) {
		case map[string]any:
			current = typed[part]
		case []any:
			return typed
		default:
			return nil
		}
	}
	return current
}

func normalizeValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case map[string]any:
		return typed
	case []any:
		return typed
	case document.Record:
		return normalizeDocument(typed)
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return typed
		}
		var out any
		if err := json.Unmarshal(raw, &out); err != nil {
			return typed
		}
		return out
	}
}

func normalizeDocument(record document.Record) map[string]any {
	lines := make([]any, 0, len(record.Lines))
	lineMaps := make([]map[string]any, 0, len(record.Lines))
	for _, line := range record.Lines {
		item := map[string]any{
			"ID":        line.ID,
			"LineNo":    line.LineNo,
			"LineType":  line.LineType,
			"Payload":   line.Payload,
			"Amount":    line.Amount,
			"id":        line.ID,
			"line_no":   line.LineNo,
			"line_type": line.LineType,
			"payload":   line.Payload,
			"amount":    line.Amount,
		}
		lines = append(lines, item)
		lineMaps = append(lineMaps, item)
	}
	header := map[string]any{
		"ID":              record.Header.ID,
		"Type":            record.Header.Type,
		"Status":          record.Header.Status,
		"Version":         record.Header.Version,
		"ETag":            record.Header.ETag,
		"OrganizationID":  record.Header.OrganizationID,
		"LocationID":      record.Header.LocationID,
		"Number":          record.Header.Number,
		"CreatedBy":       record.Header.CreatedBy,
		"UpdatedBy":       record.Header.UpdatedBy,
		"TotalAmount":     record.Header.TotalAmount,
		"id":              record.Header.ID,
		"type":            record.Header.Type,
		"status":          record.Header.Status,
		"version":         record.Header.Version,
		"etag":            record.Header.ETag,
		"organization_id": record.Header.OrganizationID,
		"location_id":     record.Header.LocationID,
		"number":          record.Header.Number,
		"created_by":      record.Header.CreatedBy,
		"updated_by":      record.Header.UpdatedBy,
		"total_amount":    record.Header.TotalAmount.AmountMinor,
		"currency":        record.Header.TotalAmount.Currency,
	}
	body := map[string]any{
		"DocumentID":     record.Body.DocumentID,
		"SchemaVersion":  record.Body.SchemaVersion,
		"Payload":        record.Body.Payload,
		"document_id":    record.Body.DocumentID,
		"schema_version": record.Body.SchemaVersion,
		"payload":        record.Body.Payload,
	}
	return map[string]any{
		"Header": header,
		"Body":   body,
		"Lines":  lineMaps,
		"header": header,
		"body":   body,
		"lines":  lines,
	}
}

func (s *Service) attachRelatedSources(ctx map[string]any, def Definition, primary any, sample bool) {
	if len(def.RelatedSources) == 0 {
		return
	}
	related := map[string]any{}
	relatedSingle := map[string]any{}
	for _, source := range def.RelatedSources {
		key := strings.TrimSpace(source.Key)
		if key == "" {
			continue
		}
		switch {
		case sample:
			items := []any{sampleRelatedDocument(source)}
			related[key] = items
			relatedSingle[key] = items[0]
		case def.TargetKind == "document":
			record, ok := primary.(document.Record)
			if !ok {
				continue
			}
			items := s.relatedDocumentsForRecord(record.Header.ID, source)
			related[key] = items
			if len(items) > 0 {
				relatedSingle[key] = items[0]
			}
		case def.TargetKind == "report":
			items := s.relatedDocumentsForReport(primary, source)
			related[key] = items
			if len(items) > 0 {
				relatedSingle[key] = items[0]
			}
		}
	}
	if len(related) > 0 {
		ctx["related_documents"] = related
		ctx["related_document"] = relatedSingle
		ctx["related"] = related
	}
}

func sampleRelatedDocument(source RelatedSource) map[string]any {
	sample := sampleDocument(Definition{TargetKey: source.TargetKey})
	if header, ok := sample["header"].(map[string]any); ok {
		header["number"] = "REL-" + strings.ToUpper(strings.ReplaceAll(source.TargetKey, ".", "-"))
	}
	if body, ok := sample["body"].(map[string]any); ok {
		if payload, ok := body["payload"].(map[string]any); ok {
			payload["title"] = strings.TrimSpace(source.Label)
			if payload["title"] == "" {
				payload["title"] = startCase(strings.ReplaceAll(source.TargetKey, ".", " "))
			}
		}
	}
	return sample
}

func (s *Service) relatedDocumentsForRecord(rootID string, source RelatedSource) []any {
	if s.documents == nil || strings.TrimSpace(rootID) == "" {
		return []any{}
	}
	candidates := traverseDocumentGraph(s.documents.List(), []string{rootID}, relationDepth(source))
	items := make([]any, 0, len(candidates))
	for _, item := range candidates {
		if item.Header.Type != source.TargetKey {
			continue
		}
		items = append(items, normalizeDocument(item))
	}
	return items
}

func (s *Service) relatedDocumentsForReport(primary any, source RelatedSource) []any {
	rows := normalizeSlice(lookupPath(primary, "rows"))
	seedIDs := make([]string, 0)
	path := strings.TrimSpace(source.DocumentIDPath)
	if path == "" {
		path = "document_id"
	}
	for _, row := range rows {
		values := lookupPath(row, path)
		switch typed := values.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				seedIDs = append(seedIDs, strings.TrimSpace(typed))
			}
		case []any:
			for _, item := range typed {
				if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
					seedIDs = append(seedIDs, strings.TrimSpace(value))
				}
			}
		}
	}
	if len(seedIDs) == 0 {
		return []any{}
	}
	candidates := traverseDocumentGraph(s.documents.List(), seedIDs, relationDepth(source))
	items := make([]any, 0, len(candidates))
	for _, item := range candidates {
		if item.Header.Type != source.TargetKey {
			continue
		}
		items = append(items, normalizeDocument(item))
	}
	return items
}

func normalizeRelationMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "indirect":
		return "indirect"
	default:
		return "direct"
	}
}

func relationDepth(source RelatedSource) int {
	if normalizeRelationMode(source.RelationMode) == "indirect" {
		if source.MaxDepth > 1 {
			return source.MaxDepth
		}
		return 2
	}
	return 1
}

func traverseDocumentGraph(records []document.Record, seedIDs []string, depth int) []document.Record {
	if depth <= 0 {
		return []document.Record{}
	}
	byID := make(map[string]document.Record, len(records))
	neighbors := make(map[string][]string, len(records))
	for _, record := range records {
		byID[record.Header.ID] = record
	}
	for _, record := range records {
		for _, link := range record.Links {
			neighbors[record.Header.ID] = append(neighbors[record.Header.ID], link.LinkedDocumentID)
			neighbors[link.LinkedDocumentID] = append(neighbors[link.LinkedDocumentID], record.Header.ID)
		}
	}
	queue := append([]string(nil), seedIDs...)
	distance := map[string]int{}
	for _, id := range seedIDs {
		if strings.TrimSpace(id) == "" {
			continue
		}
		distance[id] = 0
	}
	collected := make([]document.Record, 0)
	seen := map[string]bool{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		currentDistance, ok := distance[current]
		if !ok || currentDistance >= depth {
			continue
		}
		for _, next := range neighbors[current] {
			if _, visited := distance[next]; visited {
				continue
			}
			distance[next] = currentDistance + 1
			queue = append(queue, next)
			if record, exists := byID[next]; exists && !seen[next] {
				seen[next] = true
				collected = append(collected, record)
			}
		}
	}
	return collected
}

func startCase(value string) string {
	parts := strings.Fields(strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(value))
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}

func normalizeVisualTemplate(visual VisualTemplate) VisualTemplate {
	if strings.TrimSpace(visual.SchemaVersion) == "" {
		visual.SchemaVersion = "visual-grid/v1"
	}
	if strings.TrimSpace(visual.Settings.PaperPreset) == "" {
		visual.Settings.PaperPreset = "a4"
	}
	if strings.TrimSpace(visual.Settings.Orientation) == "" {
		visual.Settings.Orientation = "portrait"
	}
	if strings.TrimSpace(visual.Settings.Density) == "" {
		visual.Settings.Density = "comfortable"
	}
	if len(visual.Sections) == 0 {
		visual.Sections = defaultSections()
	}
	for i := range visual.Sections {
		if strings.TrimSpace(visual.Sections[i].ID) == "" {
			visual.Sections[i].ID = fmt.Sprintf("section-%d", i+1)
		}
		if len(visual.Sections[i].Rows) == 0 {
			visual.Sections[i].Rows = []VisualRow{{ID: fmt.Sprintf("%s-row-1", visual.Sections[i].ID), Columns: []VisualCell{{ID: fmt.Sprintf("%s-cell-1", visual.Sections[i].ID), Span: 12}}}}
		}
		for rowIdx := range visual.Sections[i].Rows {
			row := &visual.Sections[i].Rows[rowIdx]
			if strings.TrimSpace(row.ID) == "" {
				row.ID = fmt.Sprintf("%s-row-%d", visual.Sections[i].ID, rowIdx+1)
			}
			row.Width = visualLength(row.Width)
			row.Height = visualLength(row.Height)
			row.AlignX = normalizeVisualAlignX(row.AlignX)
			row.AlignY = normalizeVisualAlignY(row.AlignY)
			row.ContentAlignX = normalizeVisualContentAlignX(row.ContentAlignX)
			row.ContentAlignY = normalizeVisualContentAlignY(row.ContentAlignY)
			for cellIdx := range row.Columns {
				cell := &row.Columns[cellIdx]
				if strings.TrimSpace(cell.ID) == "" {
					cell.ID = fmt.Sprintf("%s-cell-%d", row.ID, cellIdx+1)
				}
				if cell.Span <= 0 || cell.Span > 12 {
					cell.Span = 12
				}
				cell.Width = visualLength(cell.Width)
				cell.Height = visualLength(cell.Height)
				cell.AlignX = normalizeVisualAlignX(cell.AlignX)
				cell.AlignY = normalizeVisualAlignY(cell.AlignY)
				cell.ContentAlignX = normalizeVisualContentAlignX(cell.ContentAlignX)
				cell.ContentAlignY = normalizeVisualContentAlignY(cell.ContentAlignY)
			}
		}
	}
	return visual
}

func defaultSections() []VisualSection {
	return []VisualSection{
		{ID: "header", Title: "Header", Kind: "header", Rows: []VisualRow{{ID: "header-row-1", Columns: []VisualCell{{ID: "header-cell-1", Span: 12}}}}},
		{ID: "body", Title: "Body", Kind: "body", Rows: []VisualRow{{ID: "body-row-1", Columns: []VisualCell{{ID: "body-cell-1", Span: 12}}}}},
		{ID: "footer", Title: "Footer", Kind: "footer", Rows: []VisualRow{{ID: "footer-row-1", Columns: []VisualCell{{ID: "footer-cell-1", Span: 12}}}}},
	}
}

func sampleDocument(def Definition) map[string]any {
	header := map[string]any{
		"ID":              "doc_sample",
		"Type":            def.TargetKey,
		"Status":          "approved",
		"Version":         1,
		"ETag":            "sample",
		"OrganizationID":  "org_default",
		"LocationID":      "loc_hq",
		"Number":          "SAMPLE-0001",
		"CreatedBy":       "user_admin",
		"UpdatedBy":       "user_admin",
		"TotalAmount":     map[string]any{"AmountMinor": 245000, "Currency": "IDR"},
		"id":              "doc_sample",
		"type":            def.TargetKey,
		"status":          "approved",
		"version":         1,
		"etag":            "sample",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"number":          "SAMPLE-0001",
		"created_by":      "user_admin",
		"updated_by":      "user_admin",
		"total_amount":    245000.00,
		"currency":        "IDR",
	}
	body := map[string]any{
		"Payload": map[string]any{
			"title":        "Sample Transaction",
			"customer":     "PT Nusantara",
			"practitioner": "dr. Sari",
			"notes":        "Printed from sample designer data",
		},
		"payload": map[string]any{
			"title":        "Sample Transaction",
			"customer":     "PT Nusantara",
			"practitioner": "dr. Sari",
			"notes":        "Printed from sample designer data",
		},
	}
	lines := []any{
		map[string]any{"LineNo": 1, "Payload": map[string]any{"name": "Item A", "qty": 1, "price": 125000.00}, "Amount": 125000.00, "line_no": 1, "payload": map[string]any{"name": "Item A", "qty": 1, "price": 125000.00}, "amount": 125000.00},
		map[string]any{"LineNo": 2, "Payload": map[string]any{"name": "Item B", "qty": 2, "price": 60000.00}, "Amount": 120000.00, "line_no": 2, "payload": map[string]any{"name": "Item B", "qty": 2, "price": 60000.00}, "amount": 120000.00},
	}
	return map[string]any{
		"Header": header,
		"Body":   body,
		"Lines":  lines,
		"header": header,
		"body":   body,
		"lines":  lines,
	}
}

func sampleReport(def Definition, key string) map[string]any {
	return map[string]any{
		"key":   key,
		"title": def.Title,
		"total": 3,
		"summary": map[string]any{
			"total": 3,
		},
		"rows": []any{
			map[string]any{"dimension_key": "approved", "label": "Approved", "total": 14},
			map[string]any{"dimension_key": "draft", "label": "Draft", "total": 6},
			map[string]any{"dimension_key": "cancelled", "label": "Cancelled", "total": 1},
		},
	}
}

func normalizeSlice(value any) []any {
	switch typed := normalizeValue(value).(type) {
	case []any:
		return typed
	default:
		return nil
	}
}

func visualVisible(ctx map[string]any, path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return true
	}
	value := lookupPath(ctx, path)
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	default:
		return true
	}
}

func presetClass(settings VisualSettings) string {
	switch strings.ToLower(strings.TrimSpace(settings.PaperPreset)) {
	case "receipt-80", "thermal-80":
		return "paper-receipt-80"
	case "receipt-58", "thermal-58":
		return "paper-receipt-58"
	case "a4":
		if strings.EqualFold(strings.TrimSpace(settings.Orientation), "landscape") {
			return "paper-a4-landscape"
		}
		return "paper-a4"
	default:
		return "paper-a4"
	}
}

func densityClass(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "compact":
		return "density-compact"
	default:
		return "density-comfortable"
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func pdfFromHTML(version Version, htmlText string) ([]byte, error) {
	orientation, unit, size := pdfPageSpec(version)
	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		OrientationStr: orientation,
		UnitStr:        unit,
		Size:           size,
	})
	pdf.SetMargins(10, 10, 10)
	pdf.SetAutoPageBreak(true, 10)
	pdf.AddPage()
	doc, err := xhtml.Parse(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	left, top, right, bottom := pdf.GetMargins()
	renderHTMLNodesToPDF(pdf, doc, left, top, right, bottom)
	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func renderHTMLNodesToPDF(pdf *gofpdf.Fpdf, node *xhtml.Node, left, top, right, _ float64) {
	pageWidth, _ := pdf.GetPageSize()
	contentWidth := pageWidth - left - right
	var render func(*xhtml.Node)
	render = func(current *xhtml.Node) {
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == xhtml.ElementNode {
				switch strings.ToLower(child.Data) {
				case "head", "style", "script", "meta", "link":
					continue
				case "h1":
					writePDFParagraph(pdf, contentWidth, collectNodeText(child), "B", 18, 8)
					pdf.Ln(1)
					continue
				case "h2":
					writePDFParagraph(pdf, contentWidth, collectNodeText(child), "B", 14, 6.5)
					pdf.Ln(1)
					continue
				case "h3":
					writePDFParagraph(pdf, contentWidth, collectNodeText(child), "B", 12, 6)
					pdf.Ln(1)
					continue
				case "p":
					writePDFParagraph(pdf, contentWidth, collectNodeText(child), "", 11, 5.5)
					pdf.Ln(1)
					continue
				case "hr":
					x := pdf.GetX()
					y := pdf.GetY() + 2
					pdf.Line(x, y, x+contentWidth, y)
					pdf.Ln(5)
					continue
				case "dl":
					renderDefinitionList(pdf, contentWidth, child)
					pdf.Ln(1)
					continue
				case "table":
					renderHTMLTablePDF(pdf, contentWidth, child)
					pdf.Ln(2)
					continue
				case "ul", "ol":
					renderHTMLListPDF(pdf, contentWidth, child, strings.ToLower(child.Data) == "ol")
					pdf.Ln(1)
					continue
				case "br":
					pdf.Ln(4)
					continue
				}
			}
			if child.Type == xhtml.TextNode {
				text := strings.Join(strings.Fields(html.UnescapeString(child.Data)), " ")
				if text != "" {
					writePDFParagraph(pdf, contentWidth, text, "", 11, 5.5)
				}
				continue
			}
			render(child)
		}
	}
	render(node)
}

func writePDFParagraph(pdf *gofpdf.Fpdf, width float64, text, style string, size, lineHeight float64) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	pdf.SetFont("Arial", style, size)
	pdf.MultiCell(width, lineHeight, text, "", "L", false)
}

func collectNodeText(node *xhtml.Node) string {
	var parts []string
	var walk func(*xhtml.Node)
	walk = func(current *xhtml.Node) {
		if current.Type == xhtml.TextNode {
			text := strings.Join(strings.Fields(html.UnescapeString(current.Data)), " ")
			if text != "" {
				parts = append(parts, text)
			}
		}
		if current.Type == xhtml.ElementNode && strings.EqualFold(current.Data, "br") {
			parts = append(parts, "\n")
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return strings.TrimSpace(strings.Join(parts, " "))
}

func renderDefinitionList(pdf *gofpdf.Fpdf, width float64, node *xhtml.Node) {
	labelWidth := width * 0.3
	valueWidth := width - labelWidth
	var pending string
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != xhtml.ElementNode {
			continue
		}
		switch strings.ToLower(child.Data) {
		case "dt":
			pending = collectNodeText(child)
		case "dd":
			value := collectNodeText(child)
			x := pdf.GetX()
			y := pdf.GetY()
			pdf.SetFont("Arial", "B", 10)
			pdf.SetXY(x, y)
			pdf.MultiCell(labelWidth, 5.5, pending, "", "L", false)
			pdf.SetXY(x+labelWidth, y)
			pdf.SetFont("Arial", "", 10)
			pdf.MultiCell(valueWidth, 5.5, value, "", "L", false)
		}
	}
}

func renderHTMLTablePDF(pdf *gofpdf.Fpdf, width float64, node *xhtml.Node) {
	rows := extractHTMLTableRows(node)
	if len(rows) == 0 {
		return
	}
	colCount := 0
	for _, row := range rows {
		if len(row.Cells) > colCount {
			colCount = len(row.Cells)
		}
	}
	colCount = maxInt(1, colCount)
	colWidth := width / float64(colCount)
	for _, row := range rows {
		pdf.SetX(pdf.GetX())
		for i := 0; i < colCount; i++ {
			cell := ""
			if i < len(row.Cells) {
				cell = row.Cells[i]
			}
			style := ""
			if row.Header {
				style = "B"
			}
			pdf.SetFont("Arial", style, 9)
			pdf.CellFormat(colWidth, 6, cell, "1", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
	}
}

type htmlTableRow struct {
	Header bool
	Cells  []string
}

func extractHTMLTableRows(node *xhtml.Node) []htmlTableRow {
	rows := make([]htmlTableRow, 0)
	var walk func(*xhtml.Node)
	walk = func(current *xhtml.Node) {
		if current.Type == xhtml.ElementNode && strings.EqualFold(current.Data, "tr") {
			row := htmlTableRow{}
			for child := current.FirstChild; child != nil; child = child.NextSibling {
				if child.Type != xhtml.ElementNode {
					continue
				}
				tag := strings.ToLower(child.Data)
				if tag != "th" && tag != "td" {
					continue
				}
				if tag == "th" {
					row.Header = true
				}
				row.Cells = append(row.Cells, collectNodeText(child))
			}
			if len(row.Cells) > 0 {
				rows = append(rows, row)
			}
			return
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return rows
}

func renderHTMLListPDF(pdf *gofpdf.Fpdf, width float64, node *xhtml.Node, ordered bool) {
	index := 0
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != xhtml.ElementNode || !strings.EqualFold(child.Data, "li") {
			continue
		}
		index++
		bullet := "- "
		if ordered {
			bullet = fmt.Sprintf("%d. ", index)
		}
		writePDFParagraph(pdf, width, bullet+collectNodeText(child), "", 10, 5.2)
	}
}

func pdfPageSpec(version Version) (string, string, gofpdf.SizeType) {
	if strings.EqualFold(version.RendererKind, "visual") {
		var visual VisualTemplate
		if json.Unmarshal([]byte(version.Body), &visual) == nil {
			visual = normalizeVisualTemplate(visual)
			switch strings.ToLower(strings.TrimSpace(visual.Settings.PaperPreset)) {
			case "receipt-80", "thermal-80":
				return "P", "mm", gofpdf.SizeType{Wd: 80, Ht: 220}
			case "receipt-58", "thermal-58":
				return "P", "mm", gofpdf.SizeType{Wd: 58, Ht: 220}
			case "a4":
				if strings.EqualFold(strings.TrimSpace(visual.Settings.Orientation), "landscape") {
					return "L", "mm", gofpdf.SizeType{Wd: 297, Ht: 210}
				}
				return "P", "mm", gofpdf.SizeType{Wd: 210, Ht: 297}
			}
		}
	}
	return "P", "mm", gofpdf.SizeType{Wd: 210, Ht: 297}
}

type resolvedScope struct {
	ScopeType string
	ScopeID   string
}

func resolveScopes(req RenderRequest) []resolvedScope {
	scopes := make([]resolvedScope, 0, 3)
	if strings.TrimSpace(req.ScopeType) != "" {
		scopes = append(scopes, resolvedScope{ScopeType: strings.TrimSpace(req.ScopeType), ScopeID: strings.TrimSpace(req.ScopeID)})
	}
	if strings.TrimSpace(req.LocationID) != "" {
		scopes = append(scopes, resolvedScope{ScopeType: "location", ScopeID: strings.TrimSpace(req.LocationID)})
	}
	if strings.TrimSpace(req.OrganizationID) != "" {
		scopes = append(scopes, resolvedScope{ScopeType: "organization", ScopeID: strings.TrimSpace(req.OrganizationID)})
	}
	scopes = append(scopes, resolvedScope{ScopeType: "deployment", ScopeID: ""})
	seen := map[string]bool{}
	out := make([]resolvedScope, 0, len(scopes))
	for _, item := range scopes {
		key := item.ScopeType + "|" + item.ScopeID
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func renderPDF(version Version, ctx map[string]any, htmlText string) ([]byte, error) {
	if strings.EqualFold(version.RendererKind, "visual") {
		return renderVisualPDF(version, ctx)
	}
	return pdfFromHTML(version, htmlText)
}

func renderVisualPDF(version Version, ctx map[string]any) ([]byte, error) {
	var visual VisualTemplate
	if err := json.Unmarshal([]byte(version.Body), &visual); err != nil {
		return nil, shared.Validation("visual template body must be valid json")
	}
	visual = normalizeVisualTemplate(visual)
	orientation, unit, size := pdfPageSpec(version)
	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		OrientationStr: orientation,
		UnitStr:        unit,
		Size:           size,
	})
	margin := 10.0
	if strings.Contains(strings.ToLower(strings.TrimSpace(visual.Settings.PaperPreset)), "receipt") {
		margin = 4
	}
	pdf.SetMargins(margin, margin, margin)
	pdf.SetAutoPageBreak(true, margin)
	pdf.AddPage()
	pageWidth, pageHeight := size.Wd, size.Ht
	if strings.EqualFold(orientation, "L") {
		pageWidth, pageHeight = size.Ht, size.Wd
	}
	contentWidth := pageWidth - (margin * 2)
	y := margin
	if strings.TrimSpace(visual.Title) != "" {
		pdf.SetXY(margin, y)
		pdf.SetFont("Arial", "B", 16)
		pdf.MultiCell(contentWidth, 7, visual.Title, "", "L", false)
		y = pdf.GetY() + 3
	}
	for _, section := range visual.Sections {
		if y > pageHeight-(margin*2) {
			pdf.AddPage()
			y = margin
		}
		if strings.TrimSpace(section.Title) != "" {
			pdf.SetXY(margin, y)
			pdf.SetFont("Arial", "B", 10)
			pdf.MultiCell(contentWidth, 5, strings.ToUpper(section.Title), "", "L", false)
			y = pdf.GetY() + 2
		}
		for _, row := range section.Rows {
			rowHeight := 0.0
			x := margin
			for _, cell := range row.Columns {
				cellWidth := contentWidth * (float64(cell.Span) / 12.0)
				rowHeight = maxFloat(rowHeight, estimateVisualCellHeight(cell, ctx, cellWidth))
			}
			for _, cell := range row.Columns {
				cellWidth := contentWidth * (float64(cell.Span) / 12.0)
				renderVisualCellPDF(pdf, ctx, cell, x, y, cellWidth, rowHeight)
				x += cellWidth
			}
			y += rowHeight + 4
		}
		y += 2
	}
	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func estimateVisualCellHeight(cell VisualCell, ctx map[string]any, cellWidth float64) float64 {
	height := 6.0
	for _, block := range cell.Blocks {
		if !visualVisible(ctx, block.VisibleIf) {
			continue
		}
		switch block.Type {
		case "table":
			rows := normalizeSlice(lookupPath(ctx, block.RowsPath))
			height += float64(maxInt(1, len(rows))+1) * 6
		case "signature":
			height += 14
		case "divider":
			height += 4
		default:
			height += 8
		}
	}
	return maxFloat(height, 18)
}

func renderVisualCellPDF(pdf *gofpdf.Fpdf, ctx map[string]any, cell VisualCell, x, y, width, height float64) {
	pdf.Rect(x, y, width, height, "D")
	currentY := y + 3
	for _, block := range cell.Blocks {
		if !visualVisible(ctx, block.VisibleIf) {
			continue
		}
		currentY = renderVisualBlockPDF(pdf, ctx, block, x+3, currentY, width-6)
	}
}

func renderVisualBlockPDF(pdf *gofpdf.Fpdf, ctx map[string]any, block VisualBlock, x, y, width float64) float64 {
	align := "L"
	switch strings.ToLower(strings.TrimSpace(block.Align)) {
	case "center":
		align = "C"
	case "right", "end":
		align = "R"
	}
	style := ""
	switch strings.ToLower(strings.TrimSpace(block.Emphasis)) {
	case "strong", "bold":
		style = "B"
	}
	fontSize := 10.0
	switch strings.ToLower(strings.TrimSpace(block.FontSize)) {
	case "sm":
		fontSize = 9
	case "lg":
		fontSize = 12
	case "xl":
		fontSize = 15
	}
	pdf.SetFont("Arial", style, fontSize)
	switch block.Type {
	case "text":
		pdf.SetXY(x, y)
		pdf.MultiCell(width, 5.5, block.Text, "", align, false)
		return pdf.GetY() + 2
	case "field":
		value := fmt.Sprintf("%v", lookupPath(ctx, block.Path))
		text := value
		if strings.TrimSpace(block.Label) != "" {
			text = block.Label + ": " + value
		}
		pdf.SetXY(x, y)
		pdf.MultiCell(width, 5.5, text, "", align, false)
		return pdf.GetY() + 2
	case "divider":
		pdf.Line(x, y+2, x+width, y+2)
		return y + 6
	case "barcode", "qr":
		value := strings.TrimSpace(block.Value)
		if value == "" {
			value = fmt.Sprintf("%v", lookupPath(ctx, block.Path))
		}
		pdf.SetXY(x, y)
		pdf.MultiCell(width, 5.5, strings.ToUpper(block.Type)+": "+value, "", align, false)
		return pdf.GetY() + 2
	case "image":
		label := block.Label
		if strings.TrimSpace(label) == "" {
			label = "Image"
		}
		pdf.SetXY(x, y)
		pdf.MultiCell(width, 5.5, label, "1", align, false)
		return pdf.GetY() + 2
	case "signature":
		pdf.Line(x, y+8, x+width, y+8)
		label := block.Label
		if strings.TrimSpace(label) == "" {
			label = "Authorized Signature"
		}
		pdf.SetXY(x, y+9)
		pdf.SetFont("Arial", "", 9)
		pdf.MultiCell(width, 5, label, "", align, false)
		return pdf.GetY() + 2
	case "totals":
		rows := normalizeSlice(lookupPath(ctx, block.RowsPath))
		total := 0.0
		for _, row := range rows {
			switch v := lookupPath(row, block.Path).(type) {
			case float64:
				total += v
			case float32:
				total += float64(v)
			case int:
				total += float64(v)
			case int64:
				total += float64(v)
			}
		}
		label := block.Label
		if strings.TrimSpace(label) == "" {
			label = "Total"
		}
		pdf.SetXY(x, y)
		pdf.MultiCell(width, 5.5, fmt.Sprintf("%s: %.2f", label, total), "", align, false)
		return pdf.GetY() + 2
	case "table":
		rows := normalizeSlice(lookupPath(ctx, block.RowsPath))
		colCount := maxInt(1, len(block.Columns))
		colWidth := width / float64(colCount)
		pdf.SetXY(x, y)
		pdf.SetFont("Arial", "B", 9)
		for _, col := range block.Columns {
			pdf.CellFormat(colWidth, 6, col.Label, "1", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
		pdf.SetFont("Arial", "", 9)
		if len(rows) == 0 {
			pdf.SetX(x)
			pdf.CellFormat(width, 6, "No rows", "1", 0, "L", false, 0, "")
			pdf.Ln(-1)
			return pdf.GetY() + 2
		}
		for _, row := range rows {
			pdf.SetX(x)
			for _, col := range block.Columns {
				value := fmt.Sprintf("%v", lookupPath(row, col.Path))
				pdf.CellFormat(colWidth, 6, value, "1", 0, "L", false, 0, "")
			}
			pdf.Ln(-1)
		}
		return pdf.GetY() + 2
	default:
		pdf.SetXY(x, y)
		pdf.MultiCell(width, 5.5, block.Label, "", align, false)
		return pdf.GetY() + 2
	}
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func htmlToPDFLines(input string) []string {
	value := strings.ReplaceAll(input, "</tr>", "\n")
	value = strings.ReplaceAll(value, "</p>", "\n")
	value = strings.ReplaceAll(value, "</div>", "\n")
	value = strings.ReplaceAll(value, "</section>", "\n")
	value = strings.ReplaceAll(value, "</article>", "\n")
	value = strings.ReplaceAll(value, "</h1>", "\n")
	value = strings.ReplaceAll(value, "</h2>", "\n")
	value = strings.ReplaceAll(value, "</h3>", "\n")
	value = regexp.MustCompile(`<style[\s\S]*?</style>`).ReplaceAllString(value, "")
	value = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	rawLines := strings.Split(value, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		lines = append(lines, "Generated output")
	}
	return lines
}

func normalizeRenderer(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "html":
		return "html"
	case "visual":
		return "visual"
	default:
		return ""
	}
}

func fileNameFor(def Definition, targetID, format string) string {
	suffix := strings.TrimSpace(targetID)
	if suffix == "" {
		suffix = strings.ReplaceAll(def.TargetKey, ".", "-")
	}
	ext := "html"
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "pdf":
		ext = "pdf"
	case "html", "":
		ext = "html"
	}
	return strings.ReplaceAll(def.Key, ".", "-") + "-" + suffix + "." + ext
}
