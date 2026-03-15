package searchruntime

import (
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/securityfields"
)

func Attach(searchSvc *search.Service, documents *document.Service, models *model.Service, jobsSvc *jobs.Service, fieldSecurity *securityfields.Service, eventingSvc *eventing.Service) {
	if searchSvc == nil {
		return
	}
	searchSvc.AttachSources(documents, models)
	searchSvc.AttachFieldSecurity(fieldSecurity)
	searchSvc.AttachJobs(jobsSvc)
	if eventingSvc == nil {
		return
	}
	for _, eventType := range []string{
		"document.updated",
		"document.submitted",
		"document.approved",
		"document.reject",
		"document.reopened",
		"document.cancelled",
	} {
		eventingSvc.RegisterHandler(eventType, eventing.NewDocumentProjectionHandler(documents, searchSvc))
	}
	for _, eventType := range []string{"model.record.created", "model.record.updated"} {
		eventingSvc.RegisterHandler(eventType, eventing.NewModelSearchIndexHandler(models, searchSvc))
	}
}
