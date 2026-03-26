import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface PreferencesState {
  darkMode: boolean
  locale: string
  setDarkMode: (dark: boolean) => void
  toggleDarkMode: () => void
  setLocale: (locale: string) => void
}

export const usePreferencesStore = create<PreferencesState>()(
  persist(
    (set, get) => ({
      darkMode: window.matchMedia('(prefers-color-scheme: dark)').matches,
      locale: 'en',

      setDarkMode: (dark) => {
        set({ darkMode: dark })
        if (dark) {
          document.documentElement.classList.add('dark')
        } else {
          document.documentElement.classList.remove('dark')
        }
      },

      toggleDarkMode: () => {
        const newDark = !get().darkMode
        set({ darkMode: newDark })
        if (newDark) {
          document.documentElement.classList.add('dark')
        } else {
          document.documentElement.classList.remove('dark')
        }
      },

      setLocale: (locale) => set({ locale }),
    }),
    {
      name: 'orbyte-preferences',
      partialize: (state) => ({
        darkMode: state.darkMode,
        locale: state.locale,
      }),
    }
  )
)
