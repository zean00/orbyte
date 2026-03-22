package integration

import (
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/secretstore"
)

func (s *Service) AttachRuntime(policySvc *policy.Service, jobSvc *jobs.Service, secretSvc *secretstore.Service) {
	s.AttachPolicy(policySvc)
	s.AttachJobs(jobSvc)
	s.AttachSecrets(secretSvc)
}
