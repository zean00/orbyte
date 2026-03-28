package httpx

import (
	"strings"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
)

func buildAdminModuleConsolePayload(cfg *config.Service, ident *identity.Service, p principal, detail module.ScopedDetail, organizationID, locationID string) map[string]any {
	consoleDef := detail.Manifest.AdminConsole
	title := strings.TrimSpace(consoleDef.Title)
	if title == "" {
		title = detail.Manifest.Name + " Console"
	}
	description := strings.TrimSpace(consoleDef.Description)
	sections := make([]map[string]any, 0, len(consoleDef.Sections))
	for _, section := range consoleDef.Sections {
		if !principalAllowsAll(ident, p, section.RequiredPermissions) {
			continue
		}
		item := map[string]any{
			"key":                  section.Key,
			"title":                section.Title,
			"title_i18n":           section.TitleI18n,
			"description":          section.Description,
			"description_i18n":     section.DescriptionI18n,
			"kind":                 section.Kind,
			"required_permissions": section.RequiredPermissions,
		}
		switch section.Kind {
		case module.AdminConsoleSectionSettingsForm:
			if section.ConfigKey == "" || cfg == nil {
				continue
			}
			definition, ok := cfg.Definition(section.ConfigKey)
			if !ok {
				continue
			}
			item["config_key"] = section.ConfigKey
			item["definition"] = definition
			if entry, ok := cfg.Resolve(section.ConfigKey, organizationID, locationID); ok {
				item["entry"] = entry
			}
			item["editable"] = principalAllowsAll(ident, p, []string{"configuration.manage"})
		case module.AdminConsoleSectionResourceLinks, module.AdminConsoleSectionWorkflowLinks, module.AdminConsoleSectionTemplateLinks:
			links := make([]module.AdminConsoleLinkDefinition, 0, len(section.Links))
			for _, link := range section.Links {
				if !principalAllowsAll(ident, p, link.RequiredPermissions) {
					continue
				}
				links = append(links, link)
			}
			item["links"] = links
		default:
			continue
		}
		sections = append(sections, item)
	}
	return map[string]any{
		"module": detail,
		"console": map[string]any{
			"title":            title,
			"title_i18n":       consoleDef.TitleI18n,
			"description":      description,
			"description_i18n": consoleDef.DescriptionI18n,
			"sections":         sections,
		},
	}
}
