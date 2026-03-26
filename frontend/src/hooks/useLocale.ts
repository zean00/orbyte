import { usePreferencesStore } from '@/stores/preferencesStore'
import en from '@/i18n/en.json'
import id from '@/i18n/id.json'

const translations: Record<string, typeof en> = { en, id }

type TranslationKey = string

export function useLocale() {
  const { locale, setLocale } = usePreferencesStore()

  const t = (key: TranslationKey): string => {
    const keys = key.split('.')
    let value: unknown = translations[locale] || translations['en']

    for (const k of keys) {
      if (value && typeof value === 'object' && k in value) {
        value = (value as Record<string, unknown>)[k]
      } else {
        return key
      }
    }

    return typeof value === 'string' ? value : key
  }

  return { locale, setLocale, t }
}
