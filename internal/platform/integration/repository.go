package integration

type Repository interface {
	SaveSystem(system ExternalSystem) error
	ListSystems() []ExternalSystem
	GetSystem(key string) (ExternalSystem, bool)
	SaveEndpoint(endpoint Endpoint) error
	ListEndpoints() []Endpoint
	GetEndpoint(key string) (Endpoint, bool)
	SaveContract(contract Contract) error
	ListContracts() []Contract
	GetContract(key string, version int) (Contract, bool)
	SaveMapping(mapping Mapping) error
	ListMappings() []Mapping
	SaveSubmission(record SubmissionRecord) error
	ListSubmissions() []SubmissionRecord
	GetSubmission(id string) (SubmissionRecord, bool)
	SaveSubmissionAttempt(attempt SubmissionAttempt) error
	ListSubmissionAttempts(submissionID string) []SubmissionAttempt
	SaveDeadLetter(record DeadLetterRecord) error
	ListDeadLetters() []DeadLetterRecord
	GetDeadLetter(id string) (DeadLetterRecord, bool)
}
