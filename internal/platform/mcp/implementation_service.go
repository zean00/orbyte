package mcp

import (
	"sort"
	"strings"
	"sync"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/featureflags"
	"orbyte/internal/platform/shared"
)

type ImplementationSession struct {
	ID           string                     `json:"id"`
	Name         string                     `json:"name,omitempty"`
	ActorID      string                     `json:"actor_id,omitempty"`
	Status       string                     `json:"status"`
	Context      ImplementationContext      `json:"context"`
	StagedPlan   ImplementationPlanEnvelope `json:"staged_plan"`
	ChangeSets   []ImplementationChangeSet  `json:"change_sets,omitempty"`
	Checkpoints  []ImplementationCheckpoint `json:"checkpoints,omitempty"`
	CreatedAt    time.Time                  `json:"created_at"`
	UpdatedAt    time.Time                  `json:"updated_at"`
	LastCommitAt time.Time                  `json:"last_commit_at,omitempty"`
}

type ImplementationContext struct {
	OrganizationID  string `json:"organization_id,omitempty"`
	LocationID      string `json:"location_id,omitempty"`
	OperatingUnitID string `json:"operating_unit_id,omitempty"`
}

type ImplementationPlanEnvelope struct {
	Bundle                 configBundle                       `json:"bundle"`
	RoleGrants             []implementationRoleGrant          `json:"role_grants,omitempty"`
	ModuleActions          []implementationModuleAction       `json:"module_actions,omitempty"`
	SystemConfigUpdates    []integrationConfigUpdate          `json:"system_config_updates,omitempty"`
	EndpointConfigUpdates  []integrationConfigUpdate          `json:"endpoint_config_updates,omitempty"`
	ReferenceRecordUpserts []implementationReferenceUpsert    `json:"reference_record_upserts,omitempty"`
	PolicyModuleUpdates    []implementationPolicyModuleUpdate `json:"policy_module_updates,omitempty"`
}

type implementationModuleAction struct {
	ModuleKey string `json:"module_key"`
	Enabled   bool   `json:"enabled"`
}

type integrationConfigUpdate struct {
	Key      string         `json:"key"`
	Settings map[string]any `json:"settings"`
}

type implementationReferenceUpsert struct {
	TypeKey       string         `json:"type_key"`
	Key           string         `json:"key"`
	DisplayName   string         `json:"display_name"`
	Scope         string         `json:"scope,omitempty"`
	ScopeID       string         `json:"scope_id,omitempty"`
	Status        string         `json:"status,omitempty"`
	Value         map[string]any `json:"value,omitempty"`
	ExternalCodes []string       `json:"external_codes,omitempty"`
}

type implementationPolicyModuleUpdate struct {
	HookKey string `json:"hook_key"`
	Scope   string `json:"scope,omitempty"`
	ScopeID string `json:"scope_id,omitempty"`
	Source  string `json:"source"`
}

type ImplementationChangeSet struct {
	ID         string                    `json:"id"`
	SessionID  string                    `json:"session_id"`
	Status     string                    `json:"status"`
	CreatedAt  time.Time                 `json:"created_at"`
	AppliedAt  time.Time                 `json:"applied_at,omitempty"`
	AppliedBy  string                    `json:"applied_by,omitempty"`
	Operations []ImplementationOperation `json:"operations,omitempty"`
}

type ImplementationOperation struct {
	Kind       string         `json:"kind"`
	Action     string         `json:"action"`
	TargetKey  string         `json:"target_key"`
	Scope      string         `json:"scope,omitempty"`
	ScopeID    string         `json:"scope_id,omitempty"`
	Before     map[string]any `json:"before,omitempty"`
	After      map[string]any `json:"after,omitempty"`
	Reversible bool           `json:"reversible"`
}

type ImplementationCheckpoint struct {
	ID          string    `json:"id"`
	ChangeSetID string    `json:"change_set_id,omitempty"`
	Name        string    `json:"name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by,omitempty"`
}

type ImplementationRollbackPlan struct {
	SessionID         string                    `json:"session_id"`
	ChangeSetID       string                    `json:"change_set_id"`
	Reversible        bool                      `json:"reversible"`
	Operations        []ImplementationOperation `json:"operations,omitempty"`
	ManualRemediation []string                  `json:"manual_remediation,omitempty"`
}

type ImplementationVerificationReport struct {
	Passed      bool                  `json:"passed"`
	Checks      []map[string]any      `json:"checks,omitempty"`
	Warnings    []string              `json:"warnings,omitempty"`
	Context     ImplementationContext `json:"context"`
	GeneratedAt time.Time             `json:"generated_at"`
}

type ImplementationService struct {
	mu       sync.RWMutex
	sessions map[string]ImplementationSession
}

func NewImplementationService() *ImplementationService {
	return &ImplementationService{sessions: map[string]ImplementationSession{}}
}

func (s *ImplementationService) Create(actorID, name string, ctx ImplementationContext) ImplementationSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	session := ImplementationSession{
		ID:        shared.NewID("impl"),
		Name:      strings.TrimSpace(name),
		ActorID:   strings.TrimSpace(actorID),
		Status:    "open",
		Context:   ctx,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.sessions[session.ID] = session
	return session
}

func (s *ImplementationService) Get(id string) (ImplementationSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.sessions[strings.TrimSpace(id)]
	return cloneImplementationSession(item), ok
}

func (s *ImplementationService) List() []ImplementationSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]ImplementationSession, 0, len(s.sessions))
	for _, session := range s.sessions {
		items = append(items, cloneImplementationSession(session))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items
}

func (s *ImplementationService) Save(session ImplementationSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session.UpdatedAt = time.Now().UTC()
	s.sessions[session.ID] = cloneImplementationSession(session)
}

func (s *ImplementationService) Close(id string) (ImplementationSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.sessions[strings.TrimSpace(id)]
	if !ok {
		return ImplementationSession{}, false
	}
	item.Status = "closed"
	item.UpdatedAt = time.Now().UTC()
	s.sessions[item.ID] = item
	return cloneImplementationSession(item), true
}

func cloneImplementationSession(item ImplementationSession) ImplementationSession {
	copy := item
	copy.StagedPlan.Bundle.ConfigEntries = append([]config.Entry(nil), item.StagedPlan.Bundle.ConfigEntries...)
	copy.StagedPlan.Bundle.FeatureFlags = append([]featureflags.Value(nil), item.StagedPlan.Bundle.FeatureFlags...)
	copy.StagedPlan.RoleGrants = append([]implementationRoleGrant(nil), item.StagedPlan.RoleGrants...)
	copy.StagedPlan.ModuleActions = append([]implementationModuleAction(nil), item.StagedPlan.ModuleActions...)
	copy.StagedPlan.SystemConfigUpdates = append([]integrationConfigUpdate(nil), item.StagedPlan.SystemConfigUpdates...)
	copy.StagedPlan.EndpointConfigUpdates = append([]integrationConfigUpdate(nil), item.StagedPlan.EndpointConfigUpdates...)
	copy.StagedPlan.ReferenceRecordUpserts = append([]implementationReferenceUpsert(nil), item.StagedPlan.ReferenceRecordUpserts...)
	copy.StagedPlan.PolicyModuleUpdates = append([]implementationPolicyModuleUpdate(nil), item.StagedPlan.PolicyModuleUpdates...)
	copy.ChangeSets = append([]ImplementationChangeSet(nil), item.ChangeSets...)
	copy.Checkpoints = append([]ImplementationCheckpoint(nil), item.Checkpoints...)
	return copy
}
