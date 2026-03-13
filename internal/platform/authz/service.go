package authz

import (
	"strings"

	"orbyte/internal/platform/identity"
)

type SubjectKind string

const (
	SubjectUser    SubjectKind = "user"
	SubjectService SubjectKind = "service"
)

type Subject struct {
	Kind              SubjectKind
	UserID            string
	SessionID         string
	ServiceID         string
	CurrentLocationID string
	AuthMethod        string
	StepUpVerified    bool
}

type Request struct {
	Subject          Subject
	PermissionKey    string
	ServiceOperation string
	LocationID       string
	RequireStepUp    bool
}

type Decision struct {
	Allowed      bool     `json:"allowed"`
	Reason       string   `json:"reason,omitempty"`
	Constraints  []string `json:"constraints,omitempty"`
	RequireStepUp bool    `json:"require_step_up,omitempty"`
}

type Service struct {
	identity *identity.Service
}

func NewService(ident *identity.Service) *Service {
	return &Service{identity: ident}
}

func (s *Service) Decide(req Request) Decision {
	if s == nil || s.identity == nil {
		return Decision{Allowed: false, Reason: "authorization service is unavailable"}
	}
	switch req.Subject.Kind {
	case SubjectUser:
		decision := s.identity.DecideSession(req.Subject.SessionID, strings.TrimSpace(req.PermissionKey), strings.TrimSpace(req.LocationID))
		if !decision.Allowed {
			return Decision{Allowed: false, Reason: decision.Reason, Constraints: append([]string(nil), decision.Constraints...)}
		}
		if req.RequireStepUp && !req.Subject.StepUpVerified {
			return Decision{Allowed: false, Reason: "step-up verification required", RequireStepUp: true}
		}
		return Decision{Allowed: true, Constraints: append([]string(nil), decision.Constraints...)}
	case SubjectService:
		decision := s.identity.DecideServicePrincipal(req.Subject.ServiceID, strings.TrimSpace(req.ServiceOperation))
		if !decision.Allowed {
			return Decision{Allowed: false, Reason: decision.Reason, Constraints: append([]string(nil), decision.Constraints...)}
		}
		return Decision{Allowed: true, Constraints: append([]string(nil), decision.Constraints...)}
	default:
		return Decision{Allowed: false, Reason: "authentication required"}
	}
}
