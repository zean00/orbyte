package app

import "orbyte/internal/platform/i18n"

func localize(en, id string) i18n.LocalizedText {
	return i18n.LocalizedText{"en": en, "id": id}
}
