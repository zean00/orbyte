package authz

import (
	"testing"
	"time"

	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/organization"
)

func TestDecideUnavailableService(t *testing.T) {
	var svc *Service
	decision := svc.Decide(Request{})
	if decision.Allowed || decision.Reason != "authorization service is unavailable" {
		t.Fatalf("unexpected decision: %+v", decision)
	}

	svc = NewService(nil)
	decision = svc.Decide(Request{})
	if decision.Allowed || decision.Reason != "authorization service is unavailable" {
		t.Fatalf("unexpected decision for nil identity: %+v", decision)
	}
}

func TestDecideUserAuthorization(t *testing.T) {
	org := organization.NewService()
	ident := identity.NewService(org)
	svc := NewService(ident)

	t.Run("allowed", func(t *testing.T) {
		decision := svc.Decide(Request{
			Subject: Subject{
				Kind:              SubjectUser,
				SessionID:         "sess_admin",
				CurrentLocationID: "loc_hq",
			},
			PermissionKey: "platform.context.read",
			LocationID:    "loc_hq",
		})
		if !decision.Allowed {
			t.Fatalf("expected allowed decision, got %+v", decision)
		}
	})

	t.Run("permission denied", func(t *testing.T) {
		decision := svc.Decide(Request{
			Subject: Subject{
				Kind:      SubjectUser,
				SessionID: "sess_admin",
			},
			PermissionKey: "nonexistent.permission",
		})
		if decision.Allowed || decision.Reason != "permission denied" {
			t.Fatalf("expected permission denied, got %+v", decision)
		}
	})

	t.Run("step up required", func(t *testing.T) {
		decision := svc.Decide(Request{
			Subject: Subject{
				Kind:           SubjectUser,
				SessionID:      "sess_admin",
				StepUpVerified: false,
			},
			PermissionKey: "platform.context.read",
			RequireStepUp: true,
		})
		if decision.Allowed || !decision.RequireStepUp || decision.Reason != "step-up verification required" {
			t.Fatalf("expected step-up requirement, got %+v", decision)
		}
	})

	t.Run("step up satisfied", func(t *testing.T) {
		decision := svc.Decide(Request{
			Subject: Subject{
				Kind:           SubjectUser,
				SessionID:      "sess_admin",
				StepUpVerified: true,
			},
			PermissionKey: "platform.context.read",
			RequireStepUp: true,
		})
		if !decision.Allowed {
			t.Fatalf("expected allowed decision with step-up, got %+v", decision)
		}
	})

	t.Run("session expired", func(t *testing.T) {
		now := time.Now().UTC()
		repo := identity.NewMemoryRepository(
			[]identity.User{{ID: "u1", Username: "admin", Status: "active", DefaultLocationID: "loc_hq", CreatedAt: now, UpdatedAt: now}},
			[]identity.Role{{ID: "r1", Key: "admin", Name: "Admin", ScopeType: "deployment", CreatedAt: now, UpdatedAt: now}},
			[]identity.Permission{{Key: "platform.context.read", Module: "platform", Action: "read", Resource: "context"}},
			[]identity.RoleBinding{{ID: "rb1", UserID: "u1", RoleID: "r1", ScopeType: "deployment", EffectiveFrom: now.Add(-time.Hour), Status: "active"}},
			[]identity.RolePermission{{RoleID: "r1", PermissionKey: "platform.context.read"}},
			nil,
			[]identity.Session{{ID: "sess_expired", UserID: "u1", Status: "active", IssuedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-time.Hour), LastSeenAt: now.Add(-time.Hour), CurrentLocationID: "loc_hq"}},
			nil,
		)
		expiredSvc := NewService(identity.NewServiceWithRepository(org, repo))
		decision := expiredSvc.Decide(Request{
			Subject: Subject{
				Kind:      SubjectUser,
				SessionID: "sess_expired",
			},
			PermissionKey: "platform.context.read",
		})
		if decision.Allowed || decision.Reason != "session expired" {
			t.Fatalf("expected session expired, got %+v", decision)
		}
	})
}

func TestDecideServiceAuthorization(t *testing.T) {
	org := organization.NewService()
	ident := identity.NewService(org)
	svc := NewService(ident)

	t.Run("allowed", func(t *testing.T) {
		decision := svc.Decide(Request{
			Subject: Subject{
				Kind:      SubjectService,
				ServiceID: "sp_projection_worker",
			},
			ServiceOperation: "projection.refresh",
		})
		if !decision.Allowed {
			t.Fatalf("expected allowed service decision, got %+v", decision)
		}
	})

	t.Run("operation denied", func(t *testing.T) {
		decision := svc.Decide(Request{
			Subject: Subject{
				Kind:      SubjectService,
				ServiceID: "sp_projection_worker",
			},
			ServiceOperation: "identity.manage_users",
		})
		if decision.Allowed || decision.Reason != "operation not allowed" {
			t.Fatalf("expected operation denial, got %+v", decision)
		}
	})
}

func TestDecideUnknownSubject(t *testing.T) {
	svc := NewService(identity.NewService(organization.NewService()))
	decision := svc.Decide(Request{})
	if decision.Allowed || decision.Reason != "authentication required" {
		t.Fatalf("expected authentication required, got %+v", decision)
	}
}
