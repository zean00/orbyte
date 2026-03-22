package dataops

type Repository interface {
	SaveArtifact(artifact Artifact) error
	ListArtifacts() []Artifact
	GetArtifact(id string) (Artifact, bool)
	SaveOperation(run OperationRun) error
	GetOperation(id string) (OperationRun, bool)
	ListOperations() []OperationRun
	SaveCheckpoint(item IncrementalCheckpoint) error
	ListCheckpoints() []IncrementalCheckpoint
}
