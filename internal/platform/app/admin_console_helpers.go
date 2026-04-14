package app

import "orbyte/internal/platform/module"

func adminConsoleLink(key, labelEn, labelID, routePath, descriptionEn, descriptionID, permission string) module.AdminConsoleLinkDefinition {
	link := module.AdminConsoleLinkDefinition{
		Key:             key,
		Label:           labelEn,
		LabelI18n:       localize(labelEn, labelID),
		RoutePath:       routePath,
		Description:     descriptionEn,
		DescriptionI18n: localize(descriptionEn, descriptionID),
	}
	if permission != "" {
		link.RequiredPermissions = []string{permission}
	}
	return link
}
