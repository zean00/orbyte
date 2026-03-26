import { useEffect } from 'react'
import { usePreferencesStore } from '@/stores/preferencesStore'

export function useDarkMode() {
  const { darkMode, setDarkMode, toggleDarkMode } = usePreferencesStore()

  useEffect(() => {
    document.documentElement.classList.remove('dark')
    if (darkMode) {
      document.documentElement.classList.add('dark')
    }
  }, [darkMode])

  return { darkMode, setDarkMode, toggleDarkMode }
}
