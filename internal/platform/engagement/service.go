package engagement

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/shared"
)

const JobReplayProgram = "engagement.replay_program"

type Service struct {
	repo Repository

	mu                 sync.Mutex
	eventing           *eventing.Service
	jobs               *jobs.Service
	registeredHandlers map[string]bool
}

func NewService() *Service {
	return NewServiceWithRepository(NewMemoryRepository())
}

func NewServiceWithRepository(repo Repository) *Service {
	if repo == nil {
		repo = NewMemoryRepository()
	}
	return &Service{
		repo:               repo,
		registeredHandlers: map[string]bool{},
	}
}

func (s *Service) AttachRuntime(eventingSvc *eventing.Service, jobsSvc *jobs.Service) {
	s.eventing = eventingSvc
	s.jobs = jobsSvc
	if jobsSvc != nil {
		jobsSvc.RegisterHandler(JobReplayProgram, func(_ context.Context, payload map[string]any) (map[string]any, error) {
			runID := strings.TrimSpace(stringValue(payload["replay_run_id"]))
			if runID == "" {
				return nil, shared.Validation("replay_run_id is required")
			}
			return s.executeReplay(runID)
		})
	}
	s.ensurePublishedHandlers()
}

func (s *Service) ListPrograms() []Program {
	return s.repo.ListPrograms()
}

func (s *Service) GetProgram(key string) (Program, bool) {
	return s.repo.GetProgram(strings.TrimSpace(key))
}

func (s *Service) CreateProgram(key, name, subjectType, actorID string) (Program, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return Program{}, shared.Validation("program key is required")
	}
	if _, ok := s.repo.GetProgram(key); ok {
		return Program{}, shared.Conflict("engagement program already exists")
	}
	now := time.Now().UTC()
	item := Program{
		Key:         key,
		Name:        firstNonEmpty(strings.TrimSpace(name), key),
		SubjectType: firstNonEmpty(strings.TrimSpace(subjectType), "generic"),
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
		UpdatedBy:   actorID,
	}
	if err := s.repo.SaveProgram(item); err != nil {
		return Program{}, err
	}
	return item, nil
}

func (s *Service) UpdateProgram(key, name, subjectType, status, actorID string) (Program, error) {
	item, ok := s.repo.GetProgram(strings.TrimSpace(key))
	if !ok {
		return Program{}, shared.NotFound("engagement program not found")
	}
	if strings.TrimSpace(name) != "" {
		item.Name = strings.TrimSpace(name)
	}
	if strings.TrimSpace(subjectType) != "" {
		item.SubjectType = strings.TrimSpace(subjectType)
	}
	if strings.TrimSpace(status) != "" {
		item.Status = strings.TrimSpace(status)
	}
	item.UpdatedAt = time.Now().UTC()
	item.UpdatedBy = actorID
	if err := s.repo.SaveProgram(item); err != nil {
		return Program{}, err
	}
	return item, nil
}

func (s *Service) CreateDraftVersion(programKey, actorID, changeNote string) (ProgramVersion, error) {
	program, ok := s.repo.GetProgram(strings.TrimSpace(programKey))
	if !ok {
		return ProgramVersion{}, shared.NotFound("engagement program not found")
	}
	versionNo := 1
	latestRules := []Rule(nil)
	for _, item := range s.repo.ListVersions(program.Key) {
		if item.Version >= versionNo {
			versionNo = item.Version + 1
		}
		if item.Status == "published" || item.Status == "draft" {
			latestRules = cloneRules(item.Rules)
		}
	}
	now := time.Now().UTC()
	version := ProgramVersion{
		ProgramKey:  program.Key,
		Version:     versionNo,
		Status:      "draft",
		ChangeNote:  strings.TrimSpace(changeNote),
		Rules:       latestRules,
		CreatedAt:   now,
		UpdatedAt:   now,
		PublishedAt: time.Time{},
	}
	if err := s.repo.SaveVersion(version); err != nil {
		return ProgramVersion{}, err
	}
	return version, nil
}

func (s *Service) GetVersion(programKey string, version int) (ProgramVersion, bool) {
	if version > 0 {
		return s.repo.GetVersion(strings.TrimSpace(programKey), version)
	}
	items := s.repo.ListVersions(strings.TrimSpace(programKey))
	if len(items) == 0 {
		return ProgramVersion{}, false
	}
	return items[len(items)-1], true
}

func (s *Service) SaveVersion(programKey string, version int, rules []Rule, actorID, changeNote string) (ProgramVersion, error) {
	item, ok := s.repo.GetVersion(strings.TrimSpace(programKey), version)
	if !ok {
		return ProgramVersion{}, shared.NotFound("engagement program version not found")
	}
	if item.Status != "draft" {
		return ProgramVersion{}, shared.Validation("only draft versions can be updated")
	}
	item.Rules = cloneRules(rules)
	item.ChangeNote = firstNonEmpty(strings.TrimSpace(changeNote), item.ChangeNote)
	item.UpdatedAt = time.Now().UTC()
	validation := s.ValidateVersion(item)
	if !validation.Valid {
		item.LastError = validation.Issues[0].Message
	} else {
		item.LastError = ""
	}
	if err := s.repo.SaveVersion(item); err != nil {
		return ProgramVersion{}, err
	}
	program, _ := s.repo.GetProgram(item.ProgramKey)
	program.UpdatedAt = item.UpdatedAt
	program.UpdatedBy = actorID
	_ = s.repo.SaveProgram(program)
	return item, nil
}

func (s *Service) ValidateVersion(version ProgramVersion) ValidationReport {
	issues := make([]ValidationIssue, 0)
	if strings.TrimSpace(version.ProgramKey) == "" {
		issues = append(issues, ValidationIssue{Code: "program_required", Path: "program_key", Message: "program_key is required"})
	}
	if len(version.Rules) == 0 {
		issues = append(issues, ValidationIssue{Code: "rules_required", Path: "rules", Message: "at least one rule is required"})
	}
	seen := map[string]bool{}
	for i, rule := range version.Rules {
		path := fmt.Sprintf("rules[%d]", i)
		if strings.TrimSpace(rule.Key) == "" {
			issues = append(issues, ValidationIssue{Code: "rule_key_required", Path: path + ".key", Message: "rule key is required"})
		} else if seen[rule.Key] {
			issues = append(issues, ValidationIssue{Code: "rule_key_duplicate", Path: path + ".key", Message: "rule key must be unique"})
		}
		seen[rule.Key] = true
		if len(rule.SourceEventTypes) == 0 {
			issues = append(issues, ValidationIssue{Code: "rule_events_required", Path: path + ".source_event_types", Message: "source_event_types is required"})
		}
		switch strings.TrimSpace(rule.Action) {
		case "credit_points":
			if rule.FixedAmount == 0 && strings.TrimSpace(rule.AmountField) == "" {
				issues = append(issues, ValidationIssue{Code: "amount_required", Path: path, Message: "credit_points rules require fixed_amount or amount_field"})
			}
		case "grant_achievement":
			if strings.TrimSpace(rule.AchievementKey) == "" {
				issues = append(issues, ValidationIssue{Code: "achievement_key_required", Path: path + ".achievement_key", Message: "grant_achievement rules require achievement_key"})
			}
			if rule.Threshold <= 0 {
				issues = append(issues, ValidationIssue{Code: "threshold_required", Path: path + ".threshold", Message: "grant_achievement rules require a positive threshold"})
			}
		case "set_tier":
			if strings.TrimSpace(rule.TierKey) == "" {
				issues = append(issues, ValidationIssue{Code: "tier_key_required", Path: path + ".tier_key", Message: "set_tier rules require tier_key"})
			}
			if rule.Threshold <= 0 {
				issues = append(issues, ValidationIssue{Code: "threshold_required", Path: path + ".threshold", Message: "set_tier rules require a positive threshold"})
			}
		default:
			issues = append(issues, ValidationIssue{Code: "rule_action_invalid", Path: path + ".action", Message: "rule action is invalid"})
		}
		if strings.TrimSpace(rule.SubjectSource) == "" {
			issues = append(issues, ValidationIssue{Code: "subject_source_required", Path: path + ".subject_source", Message: "subject_source is required"})
		}
	}
	return ValidationReport{Valid: len(issues) == 0, Issues: issues}
}

func (s *Service) PublishVersion(programKey string, version int, actorID string) (ProgramVersion, error) {
	item, ok := s.repo.GetVersion(strings.TrimSpace(programKey), version)
	if !ok {
		return ProgramVersion{}, shared.NotFound("engagement program version not found")
	}
	validation := s.ValidateVersion(item)
	if !validation.Valid {
		return ProgramVersion{}, shared.Validation("engagement program version is invalid")
	}
	for _, existing := range s.repo.ListVersions(item.ProgramKey) {
		if existing.Status != "published" || existing.Version == item.Version {
			continue
		}
		existing.Status = "archived"
		existing.UpdatedAt = time.Now().UTC()
		_ = s.repo.SaveVersion(existing)
	}
	item.Status = "published"
	item.PublishedAt = time.Now().UTC()
	item.PublishedBy = actorID
	item.UpdatedAt = item.PublishedAt
	item.LastError = ""
	if err := s.repo.SaveVersion(item); err != nil {
		return ProgramVersion{}, err
	}
	program, _ := s.repo.GetProgram(item.ProgramKey)
	program.PublishedVersion = item.Version
	program.UpdatedAt = item.PublishedAt
	program.UpdatedBy = actorID
	_ = s.repo.SaveProgram(program)
	s.ensureHandlers(eventTypesForRules(item.Rules))
	s.upsertConsumerState(item, "idle", "", "", time.Time{}, 0)
	return item, nil
}

func (s *Service) ListAccounts(programKey, subjectID string) []BalanceSnapshot {
	return s.repo.ListBalances(strings.TrimSpace(programKey), strings.TrimSpace(subjectID))
}

func (s *Service) GetBalance(programKey, subjectID, accountKey string) (BalanceSnapshot, bool) {
	return s.repo.GetBalance(strings.TrimSpace(programKey), strings.TrimSpace(subjectID), firstNonEmpty(strings.TrimSpace(accountKey), "default"))
}

func (s *Service) ListJournal(programKey, subjectID, accountKey string) []JournalEntry {
	return s.repo.ListJournal(strings.TrimSpace(programKey), strings.TrimSpace(subjectID), strings.TrimSpace(accountKey))
}

func (s *Service) GetQualification(programKey, subjectID string) (QualificationState, bool) {
	return s.repo.GetQualification(strings.TrimSpace(programKey), strings.TrimSpace(subjectID))
}

func (s *Service) ListAchievements(programKey, subjectID string) []AchievementGrant {
	return s.repo.ListAchievements(strings.TrimSpace(programKey), strings.TrimSpace(subjectID))
}

func (s *Service) GetSubject(programKey, subjectID string) SubjectView {
	return SubjectView{
		ProgramKey:    strings.TrimSpace(programKey),
		SubjectID:     strings.TrimSpace(subjectID),
		Balances:      s.ListAccounts(programKey, subjectID),
		Achievements:  s.ListAchievements(programKey, subjectID),
		RecentJournal: s.ListJournal(programKey, subjectID, ""),
		Qualification: qualificationPtr(s.repo.GetQualification(strings.TrimSpace(programKey), strings.TrimSpace(subjectID))),
	}
}

func (s *Service) ListConsumers() []ConsumerState {
	return s.repo.ListConsumerStates()
}

func (s *Service) GetConsumer(id string) (ConsumerState, bool) {
	return s.repo.GetConsumerState(strings.TrimSpace(id))
}

func (s *Service) ReplayPlan(programKey string, version int) (ReplayPlan, error) {
	item, ok := s.resolveVersion(programKey, version)
	if !ok {
		return ReplayPlan{}, shared.NotFound("engagement program version not found")
	}
	program, ok := s.repo.GetProgram(item.ProgramKey)
	if !ok {
		return ReplayPlan{}, shared.NotFound("engagement program not found")
	}
	if program.PublishedVersion != item.Version {
		return ReplayPlan{}, shared.Validation("engagement replay is only allowed for the currently published version")
	}
	validation := s.ValidateVersion(item)
	events := s.matchingEvents(item)
	return ReplayPlan{
		ProgramKey:     item.ProgramKey,
		Version:        item.Version,
		MatchingEvents: len(events),
		Validation:     validation,
	}, nil
}

func (s *Service) StartReplay(programKey string, version int, actorID string) (ReplayRun, jobs.Job, error) {
	if s.jobs == nil {
		return ReplayRun{}, jobs.Job{}, shared.Conflict("engagement replay jobs are not configured")
	}
	plan, err := s.ReplayPlan(programKey, version)
	if err != nil {
		return ReplayRun{}, jobs.Job{}, err
	}
	if !plan.Validation.Valid {
		return ReplayRun{}, jobs.Job{}, shared.Validation("engagement replay validation failed")
	}
	run := ReplayRun{
		ID:             shared.ChildID("engagement", "replay"),
		ProgramKey:     plan.ProgramKey,
		Version:        plan.Version,
		Status:         jobs.StatusQueued,
		MatchingEvents: plan.MatchingEvents,
		StartedAt:      time.Now().UTC(),
		CreatedBy:      actorID,
		Validation:     plan.Validation,
	}
	if err := s.repo.SaveReplayRun(run); err != nil {
		return ReplayRun{}, jobs.Job{}, err
	}
	job, err := s.jobs.EnqueueUnique(JobReplayProgram, map[string]any{"replay_run_id": run.ID}, JobReplayProgram+":"+run.ID)
	if err != nil {
		return ReplayRun{}, jobs.Job{}, err
	}
	run.JobID = job.ID
	if err := s.repo.SaveReplayRun(run); err != nil {
		return ReplayRun{}, jobs.Job{}, err
	}
	return run, job, nil
}

func (s *Service) GetReplayRun(id string) (ReplayRun, bool) {
	return s.repo.GetReplayRun(strings.TrimSpace(id))
}

func (s *Service) ListReplayRuns() []ReplayRun {
	return s.repo.ListReplayRuns()
}

func (s *Service) SimulationRun(programKey string, version int, event eventing.Event) (map[string]any, error) {
	item, ok := s.resolveVersion(programKey, version)
	if !ok {
		return nil, shared.NotFound("engagement program version not found")
	}
	validation := s.ValidateVersion(item)
	if !validation.Valid {
		return nil, shared.Validation("engagement program version is invalid")
	}
	outcomes := make([]map[string]any, 0)
	for _, rule := range item.Rules {
		if !containsString(rule.SourceEventTypes, event.Type) {
			continue
		}
		subjectID := resolveSubjectID(rule.SubjectSource, event)
		if subjectID == "" {
			continue
		}
		accountKey := firstNonEmpty(strings.TrimSpace(rule.AccountKey), "default")
		outcome := map[string]any{
			"rule_key":    rule.Key,
			"subject_id":  subjectID,
			"account_key": accountKey,
			"action":      rule.Action,
		}
		switch rule.Action {
		case "credit_points":
			outcome["amount"] = resolveAmount(rule, event)
		case "grant_achievement":
			outcome["achievement_key"] = rule.AchievementKey
			outcome["threshold"] = rule.Threshold
		case "set_tier":
			outcome["tier_key"] = rule.TierKey
			outcome["threshold"] = rule.Threshold
		}
		outcomes = append(outcomes, outcome)
	}
	return map[string]any{
		"program_key": programKey,
		"version":     item.Version,
		"event_type":  event.Type,
		"outcomes":    outcomes,
	}, nil
}

func (s *Service) ProcessEvent(event eventing.Event) error {
	for _, version := range s.publishedVersions() {
		if err := s.processVersionEvent(version, event); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensurePublishedHandlers() {
	for _, version := range s.publishedVersions() {
		s.ensureHandlers(eventTypesForRules(version.Rules))
	}
}

func (s *Service) ensureHandlers(eventTypes []string) {
	if s.eventing == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, eventType := range eventTypes {
		eventType = strings.TrimSpace(eventType)
		if eventType == "" || s.registeredHandlers[eventType] {
			continue
		}
		s.eventing.RegisterHandler(eventType, handlerFunc(func(_ context.Context, event eventing.Event) error {
			return s.ProcessEvent(event)
		}))
		s.registeredHandlers[eventType] = true
	}
}

func (s *Service) processVersionEvent(version ProgramVersion, event eventing.Event) error {
	matched := false
	for _, rule := range version.Rules {
		if !containsString(rule.SourceEventTypes, event.Type) {
			continue
		}
		matched = true
		subjectID := resolveSubjectID(rule.SubjectSource, event)
		if subjectID == "" {
			continue
		}
		accountKey := firstNonEmpty(strings.TrimSpace(rule.AccountKey), "default")
		idempotencyKey := fmt.Sprintf("%s|v%d|%s|%s|%s|%s", version.ProgramKey, version.Version, rule.Key, event.ID, subjectID, accountKey)
		if !s.repo.MarkProcessed(idempotencyKey) {
			continue
		}
		if err := s.applyRule(version, rule, subjectID, accountKey, event); err != nil {
			s.upsertConsumerState(version, "failed", event.ID, err.Error(), event.OccurredAt, 0)
			return err
		}
		s.incrementConsumer(version, event)
	}
	if matched {
		s.upsertConsumerState(version, "active", event.ID, "", event.OccurredAt, 0)
	}
	return nil
}

func (s *Service) applyRule(version ProgramVersion, rule Rule, subjectID, accountKey string, event eventing.Event) error {
	switch rule.Action {
	case "credit_points":
		amount := resolveAmount(rule, event)
		if amount == 0 {
			return nil
		}
		balance, _ := s.repo.GetBalance(version.ProgramKey, subjectID, accountKey)
		balance.ProgramKey = version.ProgramKey
		balance.SubjectID = subjectID
		balance.AccountKey = accountKey
		balance.Balance += amount
		balance.UpdatedAt = time.Now().UTC()
		if err := s.repo.SaveBalance(balance); err != nil {
			return err
		}
		return s.repo.SaveJournalEntry(JournalEntry{
			ID:            shared.ChildID("engagement", "journal"),
			ProgramKey:    version.ProgramKey,
			Version:       version.Version,
			SubjectID:     subjectID,
			AccountKey:    accountKey,
			EntryType:     "credit",
			Amount:        amount,
			RuleKey:       rule.Key,
			EventID:       event.ID,
			EventType:     event.Type,
			CorrelationID: event.CorrelationID,
			OccurredAt:    event.OccurredAt,
			CreatedAt:     time.Now().UTC(),
		})
	case "grant_achievement":
		balance, _ := s.repo.GetBalance(version.ProgramKey, subjectID, accountKey)
		if balance.Balance < rule.Threshold || s.repo.HasAchievement(version.ProgramKey, subjectID, rule.AchievementKey) {
			return nil
		}
		return s.repo.SaveAchievement(AchievementGrant{
			ID:             shared.ChildID("engagement", "achievement"),
			ProgramKey:     version.ProgramKey,
			SubjectID:      subjectID,
			AchievementKey: rule.AchievementKey,
			RuleKey:        rule.Key,
			EventID:        event.ID,
			GrantedAt:      time.Now().UTC(),
		})
	case "set_tier":
		balance, _ := s.repo.GetBalance(version.ProgramKey, subjectID, accountKey)
		if balance.Balance < rule.Threshold {
			return nil
		}
		return s.repo.SaveQualification(QualificationState{
			ProgramKey: version.ProgramKey,
			SubjectID:  subjectID,
			TierKey:    rule.TierKey,
			Score:      balance.Balance,
			UpdatedAt:  time.Now().UTC(),
		})
	default:
		return shared.Validation("engagement rule action is invalid")
	}
}

func (s *Service) executeReplay(runID string) (map[string]any, error) {
	run, ok := s.repo.GetReplayRun(strings.TrimSpace(runID))
	if !ok {
		return nil, shared.NotFound("engagement replay run not found")
	}
	run.Status = jobs.StatusRunning
	_ = s.repo.SaveReplayRun(run)
	version, ok := s.resolveVersion(run.ProgramKey, run.Version)
	if !ok {
		run.Status = jobs.StatusFailed
		run.Error = "engagement program version not found"
		run.CompletedAt = time.Now().UTC()
		_ = s.repo.SaveReplayRun(run)
		return nil, shared.NotFound(run.Error)
	}
	program, ok := s.repo.GetProgram(run.ProgramKey)
	if !ok {
		run.Status = jobs.StatusFailed
		run.Error = "engagement program not found"
		run.CompletedAt = time.Now().UTC()
		_ = s.repo.SaveReplayRun(run)
		return nil, shared.NotFound(run.Error)
	}
	if program.PublishedVersion != run.Version {
		run.Status = jobs.StatusFailed
		run.Error = "engagement replay is only allowed for the currently published version"
		run.CompletedAt = time.Now().UTC()
		_ = s.repo.SaveReplayRun(run)
		return nil, shared.Validation(run.Error)
	}
	if err := s.repo.ClearProgramState(run.ProgramKey); err != nil {
		run.Status = jobs.StatusFailed
		run.Error = err.Error()
		run.CompletedAt = time.Now().UTC()
		_ = s.repo.SaveReplayRun(run)
		return nil, err
	}
	processed := 0
	for _, item := range s.matchingEvents(version) {
		if err := s.processVersionEvent(version, item); err != nil {
			run.Status = jobs.StatusFailed
			run.Error = err.Error()
			run.Processed = processed
			run.CompletedAt = time.Now().UTC()
			_ = s.repo.SaveReplayRun(run)
			return nil, err
		}
		processed++
	}
	run.Status = jobs.StatusSucceeded
	run.Processed = processed
	run.CompletedAt = time.Now().UTC()
	_ = s.repo.SaveReplayRun(run)
	version.LastReplayID = run.ID
	_ = s.repo.SaveVersion(version)
	return map[string]any{"replay_run_id": run.ID, "processed": processed}, nil
}

func (s *Service) matchingEvents(version ProgramVersion) []eventing.Event {
	if s.eventing == nil {
		return nil
	}
	items := make([]eventing.Event, 0)
	for _, event := range s.eventing.ListEvents() {
		if containsString(eventTypesForRules(version.Rules), event.Type) {
			items = append(items, event)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].OccurredAt.Before(items[j].OccurredAt) })
	return items
}

func (s *Service) resolveVersion(programKey string, version int) (ProgramVersion, bool) {
	if version > 0 {
		return s.repo.GetVersion(strings.TrimSpace(programKey), version)
	}
	program, ok := s.repo.GetProgram(strings.TrimSpace(programKey))
	if !ok {
		return ProgramVersion{}, false
	}
	if program.PublishedVersion > 0 {
		return s.repo.GetVersion(program.Key, program.PublishedVersion)
	}
	items := s.repo.ListVersions(program.Key)
	if len(items) == 0 {
		return ProgramVersion{}, false
	}
	return items[len(items)-1], true
}

func (s *Service) publishedVersions() []ProgramVersion {
	items := make([]ProgramVersion, 0)
	for _, program := range s.repo.ListPrograms() {
		if !strings.EqualFold(strings.TrimSpace(program.Status), "active") {
			continue
		}
		if program.PublishedVersion == 0 {
			continue
		}
		if version, ok := s.repo.GetVersion(program.Key, program.PublishedVersion); ok {
			items = append(items, version)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ProgramKey == items[j].ProgramKey {
			return items[i].Version < items[j].Version
		}
		return items[i].ProgramKey < items[j].ProgramKey
	})
	return items
}

func eventTypesForRules(rules []Rule) []string {
	seen := map[string]bool{}
	items := make([]string, 0)
	for _, rule := range rules {
		for _, eventType := range rule.SourceEventTypes {
			eventType = strings.TrimSpace(eventType)
			if eventType == "" || seen[eventType] {
				continue
			}
			seen[eventType] = true
			items = append(items, eventType)
		}
	}
	sort.Strings(items)
	return items
}

func resolveSubjectID(source string, event eventing.Event) string {
	switch strings.TrimSpace(source) {
	case "actor_id":
		return strings.TrimSpace(event.ActorID)
	case "aggregate_id":
		return strings.TrimSpace(event.AggregateID)
	case "organization_id":
		return strings.TrimSpace(event.OrganizationID)
	case "location_id":
		return strings.TrimSpace(event.LocationID)
	default:
		if strings.HasPrefix(strings.TrimSpace(source), "payload.") {
			return strings.TrimSpace(stringValue(resolvePayloadPath(event.Payload, strings.TrimPrefix(strings.TrimSpace(source), "payload."))))
		}
		return ""
	}
}

func resolvePayloadPath(payload map[string]any, path string) any {
	current := any(payload)
	for _, part := range strings.Split(strings.TrimSpace(path), ".") {
		if part == "" {
			continue
		}
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[part]
	}
	return current
}

func resolveAmount(rule Rule, event eventing.Event) int {
	if rule.FixedAmount != 0 {
		return rule.FixedAmount
	}
	value := resolvePayloadPath(event.Payload, rule.AmountField)
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(typed))
		return n
	default:
		return 0
	}
}

func (s *Service) upsertConsumerState(version ProgramVersion, status, lastEventID, lastError string, lastEventAt time.Time, processed int) {
	id := fmt.Sprintf("%s:v%d", version.ProgramKey, version.Version)
	state, _ := s.repo.GetConsumerState(id)
	state.ID = id
	state.ProgramKey = version.ProgramKey
	state.Version = version.Version
	state.EventTypes = eventTypesForRules(version.Rules)
	state.Status = firstNonEmpty(strings.TrimSpace(status), state.Status, "idle")
	state.LastError = strings.TrimSpace(lastError)
	if strings.TrimSpace(lastEventID) != "" {
		state.LastEventID = strings.TrimSpace(lastEventID)
	}
	if !lastEventAt.IsZero() {
		state.LastEventAt = lastEventAt
	}
	state.Processed += processed
	state.UpdatedAt = time.Now().UTC()
	_ = s.repo.SaveConsumerState(state)
}

func (s *Service) incrementConsumer(version ProgramVersion, event eventing.Event) {
	s.upsertConsumerState(version, "active", event.ID, "", event.OccurredAt, 1)
}

func qualificationPtr(state QualificationState, ok bool) *QualificationState {
	if !ok {
		return nil
	}
	out := state
	return &out
}

type handlerFunc func(context.Context, eventing.Event) error

func (h handlerFunc) Handle(ctx context.Context, event eventing.Event) error { return h(ctx, event) }

func cloneRules(items []Rule) []Rule {
	out := make([]Rule, 0, len(items))
	for _, item := range items {
		item.SourceEventTypes = append([]string(nil), item.SourceEventTypes...)
		out = append(out, item)
	}
	return out
}

func containsString(items []string, candidate string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, item := range values {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}
