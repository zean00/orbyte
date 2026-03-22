package mcp

import (
	"fmt"
	"strings"

	"orbyte/internal/platform/engagement"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/shared"
)

func (s *Server) engagementProgramList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = actor
	_ = arguments
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	items := s.engagement.ListPrograms()
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d engagement programs.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) engagementProgramGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = actor
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	key := stringArg(arguments, "program_key")
	if key == "" {
		return nil, true, shared.Validation("program_key is required")
	}
	item, ok := s.engagement.GetProgram(key)
	if !ok {
		return nil, true, shared.NotFound("engagement program not found")
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded engagement program %s.", key)}},
		"structuredContent": item,
	}, true, nil
}

func (s *Server) engagementProgramCreate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	item, err := s.engagement.CreateProgram(
		stringArg(arguments, "program_key"),
		stringArg(arguments, "name"),
		firstNonEmpty(stringArg(arguments, "subject_type"), "generic"),
		workflowActorID(actor),
	)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Created engagement program %s.", item.Key)}},
		"structuredContent": item,
	}, true, nil
}

func (s *Server) engagementProgramUpdate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	item, err := s.engagement.UpdateProgram(
		stringArg(arguments, "program_key"),
		stringArg(arguments, "name"),
		stringArg(arguments, "subject_type"),
		stringArg(arguments, "status"),
		workflowActorID(actor),
	)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Updated engagement program %s.", item.Key)}},
		"structuredContent": item,
	}, true, nil
}

func (s *Server) engagementVersionCreate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	item, err := s.engagement.CreateDraftVersion(stringArg(arguments, "program_key"), workflowActorID(actor), stringArg(arguments, "change_note"))
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Created engagement draft v%d for %s.", item.Version, item.ProgramKey)}},
		"structuredContent": item,
	}, true, nil
}

func (s *Server) engagementVersionSave(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	version := intArg(arguments, "version")
	if version <= 0 {
		return nil, true, shared.Validation("version is required")
	}
	var rules []engagement.Rule
	if err := decodeObjectArg(arguments, "rules", &rules); err != nil {
		return nil, true, err
	}
	item, err := s.engagement.SaveVersion(stringArg(arguments, "program_key"), version, rules, workflowActorID(actor), stringArg(arguments, "change_note"))
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Saved engagement draft v%d for %s.", item.Version, item.ProgramKey)}},
		"structuredContent": item,
	}, true, nil
}

func (s *Server) engagementVersionValidate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = actor
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	version, ok := s.engagement.GetVersion(stringArg(arguments, "program_key"), intArg(arguments, "version"))
	if !ok {
		return nil, true, shared.NotFound("engagement program version not found")
	}
	report := s.engagement.ValidateVersion(version)
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Validated engagement version v%d for %s.", version.Version, version.ProgramKey)}},
		"structuredContent": report,
	}, true, nil
}

func (s *Server) engagementVersionPublish(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	item, err := s.engagement.PublishVersion(stringArg(arguments, "program_key"), intArg(arguments, "version"), workflowActorID(actor))
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Published engagement version v%d for %s.", item.Version, item.ProgramKey)}},
		"structuredContent": item,
	}, true, nil
}

func (s *Server) engagementSubjectGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = actor
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	programKey := stringArg(arguments, "program_key")
	subjectID := stringArg(arguments, "subject_id")
	if programKey == "" || subjectID == "" {
		return nil, true, shared.Validation("program_key and subject_id are required")
	}
	view := s.engagement.GetSubject(programKey, subjectID)
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded engagement subject %s for %s.", subjectID, programKey)}},
		"structuredContent": view,
	}, true, nil
}

func (s *Server) engagementAccountList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = actor
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	items := s.engagement.ListAccounts(stringArg(arguments, "program_key"), stringArg(arguments, "subject_id"))
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d engagement accounts.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) engagementBalanceGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = actor
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	item, ok := s.engagement.GetBalance(stringArg(arguments, "program_key"), stringArg(arguments, "subject_id"), firstNonEmpty(stringArg(arguments, "account_key"), "default"))
	if !ok {
		return nil, true, shared.NotFound("engagement balance not found")
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded engagement balance for %s.", item.SubjectID)}},
		"structuredContent": item,
	}, true, nil
}

func (s *Server) engagementJournalList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = actor
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	items := s.engagement.ListJournal(stringArg(arguments, "program_key"), stringArg(arguments, "subject_id"), stringArg(arguments, "account_key"))
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d engagement journal entries.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) engagementQualificationGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = actor
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	item, ok := s.engagement.GetQualification(stringArg(arguments, "program_key"), stringArg(arguments, "subject_id"))
	if !ok {
		return nil, true, shared.NotFound("engagement qualification not found")
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded engagement qualification for %s.", item.SubjectID)}},
		"structuredContent": item,
	}, true, nil
}

func (s *Server) engagementAchievementList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = actor
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	items := s.engagement.ListAchievements(stringArg(arguments, "program_key"), stringArg(arguments, "subject_id"))
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d engagement achievements.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) engagementConsumerList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = actor
	_ = arguments
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	items := s.engagement.ListConsumers()
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d engagement consumers.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) engagementConsumerGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = actor
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	item, ok := s.engagement.GetConsumer(stringArg(arguments, "consumer_id"))
	if !ok {
		return nil, true, shared.NotFound("engagement consumer not found")
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded engagement consumer %s.", item.ID)}},
		"structuredContent": item,
	}, true, nil
}

func (s *Server) engagementReplayPlan(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = actor
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	item, err := s.engagement.ReplayPlan(stringArg(arguments, "program_key"), intArg(arguments, "version"))
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Built engagement replay plan for %s.", item.ProgramKey)}},
		"structuredContent": item,
	}, true, nil
}

func (s *Server) engagementReplayRun(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	run, job, err := s.engagement.StartReplay(stringArg(arguments, "program_key"), intArg(arguments, "version"), workflowActorID(actor))
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Queued engagement replay %s.", run.ID)}},
		"structuredContent": map[string]any{"run": run, "job": job},
	}, true, nil
}

func (s *Server) engagementReplayGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = actor
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	item, ok := s.engagement.GetReplayRun(stringArg(arguments, "replay_run_id"))
	if !ok {
		return nil, true, shared.NotFound("engagement replay run not found")
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded engagement replay %s.", item.ID)}},
		"structuredContent": item,
	}, true, nil
}

func (s *Server) engagementSimulationRun(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = actor
	if s == nil || s.engagement == nil {
		return nil, false, nil
	}
	var event eventing.Event
	if err := decodeObjectArg(arguments, "event", &event); err != nil {
		return nil, true, err
	}
	result, err := s.engagement.SimulationRun(stringArg(arguments, "program_key"), intArg(arguments, "version"), event)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Simulated engagement outcomes for %s.", strings.TrimSpace(event.Type))}},
		"structuredContent": result,
	}, true, nil
}
