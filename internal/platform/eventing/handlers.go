package eventing

import (
	"context"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/search"
)

type DocumentProjectionHandler struct {
	documents *document.Service
	search    *search.Service
}

func NewDocumentProjectionHandler(documents *document.Service, searchSvc *search.Service) *DocumentProjectionHandler {
	return &DocumentProjectionHandler{documents: documents, search: searchSvc}
}

func (h *DocumentProjectionHandler) Handle(_ context.Context, event Event) error {
	record, err := h.documents.Get(event.AggregateID)
	if err != nil {
		return err
	}
	h.search.RefreshDocument(record)
	return nil
}

type ModelSearchIndexHandler struct {
	models *model.Service
	search *search.Service
}

func NewModelSearchIndexHandler(models *model.Service, searchSvc *search.Service) *ModelSearchIndexHandler {
	return &ModelSearchIndexHandler{models: models, search: searchSvc}
}

func (h *ModelSearchIndexHandler) Handle(_ context.Context, event Event) error {
	modelKey, _ := event.Payload["model_key"].(string)
	if modelKey == "" {
		return nil
	}
	record, err := h.models.Get(modelKey, event.AggregateID)
	if err != nil {
		return err
	}
	h.search.RefreshModel(record)
	return nil
}
