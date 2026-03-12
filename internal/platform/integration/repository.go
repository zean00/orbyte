package integration

type Repository interface {
	SaveSystem(system ExternalSystem) error
	ListSystems() []ExternalSystem
	GetSystem(key string) (ExternalSystem, bool)
	SaveSubmission(record SubmissionRecord) error
	ListSubmissions() []SubmissionRecord
	GetSubmission(id string) (SubmissionRecord, bool)
}
