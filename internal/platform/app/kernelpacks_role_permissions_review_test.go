package app

import (
	"testing"

	"orbyte/internal/modules"
	"orbyte/internal/platform/module"
)

func TestKernelPackRoleTemplatePermissionsAreRegistered(t *testing.T) {
	permissions := map[string]string{}
	manifests := append([]struct {
		key   string
		items []module.Manifest
	}{}, struct {
		key   string
		items []module.Manifest
	}{key: "built_in", items: builtInModuleManifests()})
	if clinicManifests, err := modules.ForProfile(modules.ProfileClinic); err == nil {
		manifests = append(manifests, struct {
			key   string
			items []module.Manifest
		}{key: modules.ProfileClinic, items: clinicManifests})
	}
	for _, group := range manifests {
		for _, manifest := range group.items {
			for _, permission := range manifest.Security.Permissions {
				permissions[permission.Key] = manifest.Key
			}
			for _, role := range manifest.Security.RoleTemplates {
				for _, permissionKey := range role.PermissionKeys {
					if _, ok := permissions[permissionKey]; !ok {
						t.Fatalf("manifest set %s manifest %s role %s references unregistered permission %s", group.key, manifest.Key, role.Key, permissionKey)
					}
				}
			}
		}
	}
}

func TestCommercialDocumentsAllowProductionLinks(t *testing.T) {
	manifest := commercialCoreKernelPackManifest()
	for _, def := range manifest.Documents {
		if def.Type != "sales_order" {
			continue
		}
		for _, linkType := range def.AllowedLinkTypes {
			if linkType == "production_for" {
				return
			}
		}
		t.Fatalf("expected sales_order to allow production_for links, got %+v", def.AllowedLinkTypes)
	}
	t.Fatal("expected sales_order definition")
}
