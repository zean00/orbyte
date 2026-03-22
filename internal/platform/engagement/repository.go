package engagement

type Repository interface {
	SaveProgram(program Program) error
	GetProgram(key string) (Program, bool)
	ListPrograms() []Program
	SaveVersion(version ProgramVersion) error
	GetVersion(programKey string, version int) (ProgramVersion, bool)
	ListVersions(programKey string) []ProgramVersion
	SaveJournalEntry(entry JournalEntry) error
	ListJournal(programKey, subjectID, accountKey string) []JournalEntry
	SaveBalance(balance BalanceSnapshot) error
	GetBalance(programKey, subjectID, accountKey string) (BalanceSnapshot, bool)
	ListBalances(programKey, subjectID string) []BalanceSnapshot
	SaveQualification(state QualificationState) error
	GetQualification(programKey, subjectID string) (QualificationState, bool)
	SaveAchievement(grant AchievementGrant) error
	ListAchievements(programKey, subjectID string) []AchievementGrant
	HasAchievement(programKey, subjectID, achievementKey string) bool
	MarkProcessed(idempotencyKey string) bool
	ClearProgramState(programKey string) error
	SaveConsumerState(state ConsumerState) error
	GetConsumerState(id string) (ConsumerState, bool)
	ListConsumerStates() []ConsumerState
	SaveReplayRun(run ReplayRun) error
	GetReplayRun(id string) (ReplayRun, bool)
	ListReplayRuns() []ReplayRun
}
