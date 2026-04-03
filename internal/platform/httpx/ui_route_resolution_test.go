package httpx

import (
	"testing"
	"time"

	"orbyte/internal/platform/i18n"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/organization"
)

type countingIdentityRepository struct {
	identity.Repository
	findSessionCalls    int
	findUserCalls       int
	roleBindingsCalls   int
	rolePermissionsCall int
}

func (r *countingIdentityRepository) FindSession(id string) (identity.Session, bool) {
	r.findSessionCalls++
	return r.Repository.FindSession(id)
}

func (r *countingIdentityRepository) FindUser(id string) (identity.User, bool) {
	r.findUserCalls++
	return r.Repository.FindUser(id)
}

func (r *countingIdentityRepository) RoleBindings() []identity.RoleBinding {
	r.roleBindingsCalls++
	return r.Repository.RoleBindings()
}

func (r *countingIdentityRepository) RolePermissions() []identity.RolePermission {
	r.rolePermissionsCall++
	return r.Repository.RolePermissions()
}

func TestResolveUIRouteWithResolverCachesPermissionLookups(t *testing.T) {
	now := time.Now().UTC()
	baseRepo := identity.NewMemoryRepository(
		[]identity.User{{
			ID:                "user_admin",
			Username:          "admin",
			Status:            "active",
			DefaultLocationID: "loc_hq",
			CreatedAt:         now,
			UpdatedAt:         now,
		}},
		[]identity.Role{{
			ID:        "role_worklist",
			Key:       "worklist_user",
			Name:      "Worklist User",
			ScopeType: "deployment",
			CreatedAt: now,
			UpdatedAt: now,
		}},
		[]identity.Permission{{
			Key:      "document.list",
			Module:   "document",
			Action:   "list",
			Resource: "document",
		}},
		[]identity.RoleBinding{{
			ID:            "binding_worklist",
			UserID:        "user_admin",
			RoleID:        "role_worklist",
			ScopeType:     "deployment",
			EffectiveFrom: now.Add(-time.Hour),
			Status:        "active",
		}},
		[]identity.RolePermission{{
			RoleID:        "role_worklist",
			PermissionKey: "document.list",
		}},
		nil,
		[]identity.Session{{
			ID:                "sess_admin",
			UserID:            "user_admin",
			Status:            "active",
			IssuedAt:          now.Add(-time.Hour),
			ExpiresAt:         now.Add(time.Hour),
			LastSeenAt:        now,
			CurrentLocationID: "loc_hq",
		}},
		nil,
	)
	repo := &countingIdentityRepository{Repository: baseRepo}
	ident := identity.NewServiceWithRepository(organization.NewService(), repo)

	modules := module.NewService()
	err := modules.Register(module.Manifest{
		Key:          "documents",
		Name:         "Documents",
		Version:      "1.0.0",
		DomainFamily: "core",
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{{
				Key:                 "documents.worklist",
				Label:               "Worklist",
				LabelI18n:           i18n.LocalizedText{"en": "Worklist"},
				Surface:             module.UISurfaceWorklist,
				ActionKey:           "documents.worklist.tasks",
				Order:               1,
				RequiredPermissions: []string{"document.list"},
			}},
			Actions: []module.ActionDefinition{{
				Key:                 "documents.worklist.tasks",
				Label:               "Task Queue",
				LabelI18n:           i18n.LocalizedText{"en": "Task Queue"},
				Surface:             module.UISurfaceWorklist,
				Kind:                "navigate",
				RoutePath:           "/worklist",
				ViewKey:             "documents.worklist.tasks",
				RenderMode:          module.RenderModeGeneric,
				RequiredPermissions: []string{"document.list"},
			}},
			Views: []module.ViewDefinition{{
				Key:                 "documents.worklist.tasks",
				Title:               "Task Queue",
				TitleI18n:           i18n.LocalizedText{"en": "Task Queue"},
				Surface:             module.UISurfaceWorklist,
				Kind:                "queue",
				ProjectionKey:       "workflow.tasks",
				RequiredPermissions: []string{"document.list"},
			}},
		},
	}, "tester")
	if err != nil {
		t.Fatalf("register manifest: %v", err)
	}

	p := principal{
		kind:              userPrincipal,
		userID:            "user_admin",
		effectiveUserID:   "user_admin",
		sessionID:         "sess_admin",
		currentLocationID: "loc_hq",
	}

	response := resolveUIRouteWithResolver(newUIBootstrapResolver(ident, modules, p), module.UISurfaceWorklist, "/worklist")
	if response.Status != "ok" {
		t.Fatalf("expected ok route resolution, got %+v", response)
	}
	if repo.findSessionCalls > 1 {
		t.Fatalf("expected cached session lookup, got %d", repo.findSessionCalls)
	}
	if repo.rolePermissionsCall > 1 {
		t.Fatalf("expected cached role permission lookup, got %d", repo.rolePermissionsCall)
	}
	if repo.roleBindingsCalls > 2 {
		t.Fatalf("expected cached role binding lookups, got %d", repo.roleBindingsCalls)
	}
	if repo.findUserCalls > 2 {
		t.Fatalf("expected bounded user lookups, got %d", repo.findUserCalls)
	}
}
