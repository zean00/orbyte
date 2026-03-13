package application

import "orbyte/internal/platform/model"

type modelUnitOfWorkRepository struct {
	uow      UnitOfWork
	fallback model.Repository
}

func newModelUnitOfWorkRepository(uow UnitOfWork, fallback model.Repository) model.Repository {
	return &modelUnitOfWorkRepository{uow: uow, fallback: fallback}
}

func (r *modelUnitOfWorkRepository) SaveDefinition(def model.Definition) error {
	if r.fallback == nil {
		return nil
	}
	return r.fallback.SaveDefinition(def)
}

func (r *modelUnitOfWorkRepository) ListDefinitions() []model.Definition {
	if r.fallback == nil {
		return nil
	}
	return r.fallback.ListDefinitions()
}

func (r *modelUnitOfWorkRepository) GetDefinition(key string) (model.Definition, bool) {
	item, err := r.uow.GetModelDefinition(key)
	return item, err == nil
}

func (r *modelUnitOfWorkRepository) SaveRecord(record model.Record) error {
	if record.Version <= 1 {
		return r.uow.CreateModelRecord(record)
	}
	return r.uow.UpdateModelRecord(record.Version-1, record)
}

func (r *modelUnitOfWorkRepository) DeleteRecord(modelKey, id string) error {
	return r.uow.DeleteModelRecord(modelKey, id)
}

func (r *modelUnitOfWorkRepository) GetRecord(modelKey, id string) (model.Record, bool) {
	item, err := r.uow.GetModelRecord(modelKey, id)
	return item, err == nil
}

func (r *modelUnitOfWorkRepository) ListRecords(modelKey string) []model.Record {
	items, err := r.uow.ListModelRecords(modelKey)
	if err != nil {
		return nil
	}
	return items
}
