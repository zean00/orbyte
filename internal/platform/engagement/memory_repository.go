package engagement

import (
	"sort"
	"strings"
)

type MemoryRepository struct {
	programs       map[string]Program
	versions       map[string]map[int]ProgramVersion
	journal        []JournalEntry
	balances       map[string]BalanceSnapshot
	qualifications map[string]QualificationState
	achievements   map[string]AchievementGrant
	processed      map[string]struct{}
	consumers      map[string]ConsumerState
	replays        map[string]ReplayRun
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		programs:       map[string]Program{},
		versions:       map[string]map[int]ProgramVersion{},
		balances:       map[string]BalanceSnapshot{},
		qualifications: map[string]QualificationState{},
		achievements:   map[string]AchievementGrant{},
		processed:      map[string]struct{}{},
		consumers:      map[string]ConsumerState{},
		replays:        map[string]ReplayRun{},
	}
}

func (r *MemoryRepository) SaveProgram(program Program) error {
	r.programs[program.Key] = program
	return nil
}

func (r *MemoryRepository) GetProgram(key string) (Program, bool) {
	item, ok := r.programs[strings.TrimSpace(key)]
	return item, ok
}

func (r *MemoryRepository) ListPrograms() []Program {
	items := make([]Program, 0, len(r.programs))
	for _, item := range r.programs {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (r *MemoryRepository) SaveVersion(version ProgramVersion) error {
	if _, ok := r.versions[version.ProgramKey]; !ok {
		r.versions[version.ProgramKey] = map[int]ProgramVersion{}
	}
	r.versions[version.ProgramKey][version.Version] = version
	return nil
}

func (r *MemoryRepository) GetVersion(programKey string, version int) (ProgramVersion, bool) {
	items, ok := r.versions[strings.TrimSpace(programKey)]
	if !ok {
		return ProgramVersion{}, false
	}
	item, ok := items[version]
	return item, ok
}

func (r *MemoryRepository) ListVersions(programKey string) []ProgramVersion {
	items := make([]ProgramVersion, 0)
	for _, item := range r.versions[strings.TrimSpace(programKey)] {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Version < items[j].Version })
	return items
}

func (r *MemoryRepository) SaveJournalEntry(entry JournalEntry) error {
	r.journal = append(r.journal, entry)
	return nil
}

func (r *MemoryRepository) ListJournal(programKey, subjectID, accountKey string) []JournalEntry {
	items := make([]JournalEntry, 0)
	for _, item := range r.journal {
		if strings.TrimSpace(programKey) != "" && item.ProgramKey != programKey {
			continue
		}
		if strings.TrimSpace(subjectID) != "" && item.SubjectID != subjectID {
			continue
		}
		if strings.TrimSpace(accountKey) != "" && item.AccountKey != accountKey {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].OccurredAt.Before(items[j].OccurredAt) })
	return items
}

func (r *MemoryRepository) SaveBalance(balance BalanceSnapshot) error {
	r.balances[balanceKey(balance.ProgramKey, balance.SubjectID, balance.AccountKey)] = balance
	return nil
}

func (r *MemoryRepository) GetBalance(programKey, subjectID, accountKey string) (BalanceSnapshot, bool) {
	item, ok := r.balances[balanceKey(programKey, subjectID, accountKey)]
	return item, ok
}

func (r *MemoryRepository) ListBalances(programKey, subjectID string) []BalanceSnapshot {
	items := make([]BalanceSnapshot, 0)
	for _, item := range r.balances {
		if strings.TrimSpace(programKey) != "" && item.ProgramKey != programKey {
			continue
		}
		if strings.TrimSpace(subjectID) != "" && item.SubjectID != subjectID {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].AccountKey < items[j].AccountKey })
	return items
}

func (r *MemoryRepository) SaveQualification(state QualificationState) error {
	r.qualifications[qualificationKey(state.ProgramKey, state.SubjectID)] = state
	return nil
}

func (r *MemoryRepository) GetQualification(programKey, subjectID string) (QualificationState, bool) {
	item, ok := r.qualifications[qualificationKey(programKey, subjectID)]
	return item, ok
}

func (r *MemoryRepository) SaveAchievement(grant AchievementGrant) error {
	r.achievements[achievementKey(grant.ProgramKey, grant.SubjectID, grant.AchievementKey)] = grant
	return nil
}

func (r *MemoryRepository) ListAchievements(programKey, subjectID string) []AchievementGrant {
	items := make([]AchievementGrant, 0)
	for _, item := range r.achievements {
		if strings.TrimSpace(programKey) != "" && item.ProgramKey != programKey {
			continue
		}
		if strings.TrimSpace(subjectID) != "" && item.SubjectID != subjectID {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].GrantedAt.Before(items[j].GrantedAt) })
	return items
}

func (r *MemoryRepository) HasAchievement(programKey, subjectID, achievementID string) bool {
	_, ok := r.achievements[achievementKey(programKey, subjectID, achievementID)]
	return ok
}

func (r *MemoryRepository) MarkProcessed(idempotencyKey string) bool {
	if _, ok := r.processed[idempotencyKey]; ok {
		return false
	}
	r.processed[idempotencyKey] = struct{}{}
	return true
}

func (r *MemoryRepository) ClearProgramState(programKey string) error {
	filtered := r.journal[:0]
	for _, item := range r.journal {
		if item.ProgramKey != programKey {
			filtered = append(filtered, item)
		}
	}
	r.journal = filtered
	for key, item := range r.balances {
		if item.ProgramKey == programKey {
			delete(r.balances, key)
		}
	}
	for key, item := range r.qualifications {
		if item.ProgramKey == programKey {
			delete(r.qualifications, key)
		}
	}
	for key, item := range r.achievements {
		if item.ProgramKey == programKey {
			delete(r.achievements, key)
		}
	}
	prefix := strings.TrimSpace(programKey) + "|"
	for key := range r.processed {
		if strings.HasPrefix(key, prefix) {
			delete(r.processed, key)
		}
	}
	for key, item := range r.consumers {
		if item.ProgramKey == programKey {
			item.Processed = 0
			item.LastEventID = ""
			item.LastEventAt = item.UpdatedAt
			item.LastError = ""
			item.Status = "idle"
			r.consumers[key] = item
		}
	}
	return nil
}

func (r *MemoryRepository) SaveConsumerState(state ConsumerState) error {
	r.consumers[state.ID] = state
	return nil
}

func (r *MemoryRepository) GetConsumerState(id string) (ConsumerState, bool) {
	item, ok := r.consumers[strings.TrimSpace(id)]
	return item, ok
}

func (r *MemoryRepository) ListConsumerStates() []ConsumerState {
	items := make([]ConsumerState, 0, len(r.consumers))
	for _, item := range r.consumers {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func (r *MemoryRepository) SaveReplayRun(run ReplayRun) error {
	r.replays[run.ID] = run
	return nil
}

func (r *MemoryRepository) GetReplayRun(id string) (ReplayRun, bool) {
	item, ok := r.replays[strings.TrimSpace(id)]
	return item, ok
}

func (r *MemoryRepository) ListReplayRuns() []ReplayRun {
	items := make([]ReplayRun, 0, len(r.replays))
	for _, item := range r.replays {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].StartedAt.After(items[j].StartedAt) })
	return items
}

func balanceKey(programKey, subjectID, accountKey string) string {
	return strings.TrimSpace(programKey) + "|" + strings.TrimSpace(subjectID) + "|" + strings.TrimSpace(accountKey)
}

func qualificationKey(programKey, subjectID string) string {
	return strings.TrimSpace(programKey) + "|" + strings.TrimSpace(subjectID)
}

func achievementKey(programKey, subjectID, key string) string {
	return strings.TrimSpace(programKey) + "|" + strings.TrimSpace(subjectID) + "|" + strings.TrimSpace(key)
}
