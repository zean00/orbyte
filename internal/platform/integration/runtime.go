package integration

import (
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/policy"
)

func (s *Service) AttachRuntime(policySvc *policy.Service, jobSvc *jobs.Service) {
	s.AttachPolicy(policySvc)
	s.AttachJobs(jobSvc)
}
