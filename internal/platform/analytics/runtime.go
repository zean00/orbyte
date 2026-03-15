package analytics

import "orbyte/internal/platform/jobs"

func (s *Service) AttachRuntime(jobSvc *jobs.Service) {
	s.AttachJobs(jobSvc)
}
