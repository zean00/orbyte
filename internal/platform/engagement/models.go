package engagement

import "time"

type Program struct {
	Key              string    `json:"key"`
	Name             string    `json:"name"`
	SubjectType      string    `json:"subject_type,omitempty"`
	Status           string    `json:"status"`
	PublishedVersion int       `json:"published_version,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	UpdatedBy        string    `json:"updated_by,omitempty"`
}

type ProgramVersion struct {
	ProgramKey   string    `json:"program_key"`
	Version      int       `json:"version"`
	Status       string    `json:"status"`
	ChangeNote   string    `json:"change_note,omitempty"`
	Rules        []Rule    `json:"rules,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	PublishedAt  time.Time `json:"published_at,omitempty"`
	PublishedBy  string    `json:"published_by,omitempty"`
	LastError    string    `json:"last_error,omitempty"`
	LastReplayID string    `json:"last_replay_id,omitempty"`
}

type Rule struct {
	Key              string   `json:"key"`
	Name             string   `json:"name,omitempty"`
	SourceEventTypes []string `json:"source_event_types,omitempty"`
	SubjectSource    string   `json:"subject_source,omitempty"`
	AccountKey       string   `json:"account_key,omitempty"`
	Action           string   `json:"action"`
	FixedAmount      int      `json:"fixed_amount,omitempty"`
	AmountField      string   `json:"amount_field,omitempty"`
	Threshold        int      `json:"threshold,omitempty"`
	AchievementKey   string   `json:"achievement_key,omitempty"`
	TierKey          string   `json:"tier_key,omitempty"`
}

type JournalEntry struct {
	ID            string    `json:"id"`
	ProgramKey    string    `json:"program_key"`
	Version       int       `json:"version"`
	SubjectID     string    `json:"subject_id"`
	AccountKey    string    `json:"account_key"`
	EntryType     string    `json:"entry_type"`
	Amount        int       `json:"amount"`
	RuleKey       string    `json:"rule_key"`
	EventID       string    `json:"event_id"`
	EventType     string    `json:"event_type"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	OccurredAt    time.Time `json:"occurred_at"`
	CreatedAt     time.Time `json:"created_at"`
}

type BalanceSnapshot struct {
	ProgramKey string    `json:"program_key"`
	SubjectID  string    `json:"subject_id"`
	AccountKey string    `json:"account_key"`
	Balance    int       `json:"balance"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type QualificationState struct {
	ProgramKey string    `json:"program_key"`
	SubjectID  string    `json:"subject_id"`
	TierKey    string    `json:"tier_key,omitempty"`
	Score      int       `json:"score"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type AchievementGrant struct {
	ID             string    `json:"id"`
	ProgramKey     string    `json:"program_key"`
	SubjectID      string    `json:"subject_id"`
	AchievementKey string    `json:"achievement_key"`
	RuleKey        string    `json:"rule_key"`
	EventID        string    `json:"event_id"`
	GrantedAt      time.Time `json:"granted_at"`
}

type ConsumerState struct {
	ID          string    `json:"id"`
	ProgramKey  string    `json:"program_key"`
	Version     int       `json:"version"`
	EventTypes  []string  `json:"event_types,omitempty"`
	Processed   int       `json:"processed"`
	LastEventID string    `json:"last_event_id,omitempty"`
	LastEventAt time.Time `json:"last_event_at,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
	Status      string    `json:"status"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ReplayPlan struct {
	ProgramKey     string           `json:"program_key"`
	Version        int              `json:"version"`
	MatchingEvents int              `json:"matching_events"`
	Validation     ValidationReport `json:"validation"`
}

type ReplayRun struct {
	ID             string           `json:"id"`
	ProgramKey     string           `json:"program_key"`
	Version        int              `json:"version"`
	Status         string           `json:"status"`
	MatchingEvents int              `json:"matching_events"`
	Processed      int              `json:"processed"`
	Error          string           `json:"error,omitempty"`
	StartedAt      time.Time        `json:"started_at"`
	CompletedAt    time.Time        `json:"completed_at,omitempty"`
	CreatedBy      string           `json:"created_by,omitempty"`
	Validation     ValidationReport `json:"validation"`
	JobID          string           `json:"job_id,omitempty"`
}

type ValidationIssue struct {
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

type ValidationReport struct {
	Valid  bool              `json:"valid"`
	Issues []ValidationIssue `json:"issues,omitempty"`
}

type SubjectView struct {
	ProgramKey    string              `json:"program_key"`
	SubjectID     string              `json:"subject_id"`
	Balances      []BalanceSnapshot   `json:"balances,omitempty"`
	Qualification *QualificationState `json:"qualification,omitempty"`
	Achievements  []AchievementGrant  `json:"achievements,omitempty"`
	RecentJournal []JournalEntry      `json:"recent_journal,omitempty"`
}
