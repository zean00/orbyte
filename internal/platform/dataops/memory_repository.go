package dataops

import "sort"

type MemoryRepository struct {
	artifacts   map[string]Artifact
	operations  map[string]OperationRun
	checkpoints []IncrementalCheckpoint
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		artifacts:  map[string]Artifact{},
		operations: map[string]OperationRun{},
	}
}

func (r *MemoryRepository) SaveArtifact(artifact Artifact) error {
	r.artifacts[artifact.ID] = artifact
	return nil
}

func (r *MemoryRepository) ListArtifacts() []Artifact {
	items := make([]Artifact, 0, len(r.artifacts))
	for _, item := range r.artifacts {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items
}

func (r *MemoryRepository) GetArtifact(id string) (Artifact, bool) {
	item, ok := r.artifacts[id]
	return item, ok
}

func (r *MemoryRepository) SaveOperation(run OperationRun) error {
	r.operations[run.ID] = run
	return nil
}

func (r *MemoryRepository) GetOperation(id string) (OperationRun, bool) {
	item, ok := r.operations[id]
	return item, ok
}

func (r *MemoryRepository) ListOperations() []OperationRun {
	items := make([]OperationRun, 0, len(r.operations))
	for _, item := range r.operations {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].StartedAt.After(items[j].StartedAt) })
	return items
}

func (r *MemoryRepository) SaveCheckpoint(item IncrementalCheckpoint) error {
	r.checkpoints = append(r.checkpoints, item)
	sort.Slice(r.checkpoints, func(i, j int) bool { return r.checkpoints[i].CreatedAt.After(r.checkpoints[j].CreatedAt) })
	return nil
}

func (r *MemoryRepository) ListCheckpoints() []IncrementalCheckpoint {
	return append([]IncrementalCheckpoint(nil), r.checkpoints...)
}
