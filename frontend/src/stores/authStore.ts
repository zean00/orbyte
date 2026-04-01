import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface User {
  id: string
  email: string
  name: string
  avatarUrl?: string
  roles: string[]
}

interface AuthState {
  user: User | null
  isAuthenticated: boolean
  isLoading: boolean
  hasCheckedAuth: boolean
  checkAuth: () => Promise<void>
  setAuthenticatedUser: (user: User | null) => void
  clearAuth: () => void
  logout: () => void
}

function getCookie(name: string): string {
  const cookie = document.cookie
    .split('; ')
    .find((entry) => entry.startsWith(`${name}=`))
  return cookie ? decodeURIComponent(cookie.split('=').slice(1).join('=')) : ''
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      isAuthenticated: false,
      isLoading: false,
      hasCheckedAuth: false,

      checkAuth: async () => {
        set({ isLoading: true })
        try {
          const response = await fetch('/auth/session', {
            credentials: 'include',
          })
          if (response.ok) {
            const data = await response.json()
            if (data.authenticated && data.user_id) {
              set({
                user: { id: data.user_id, name: data.user_id, email: '', roles: [] },
                isAuthenticated: true,
                isLoading: false,
                hasCheckedAuth: true,
              })
            } else {
              set({
                user: null,
                isAuthenticated: false,
                isLoading: false,
                hasCheckedAuth: true,
              })
            }
          } else {
            set({
              user: null,
              isAuthenticated: false,
              isLoading: false,
              hasCheckedAuth: true,
            })
          }
        } catch {
          set({
            user: null,
            isAuthenticated: false,
            isLoading: false,
            hasCheckedAuth: true,
          })
        }
      },

      setAuthenticatedUser: (user) =>
        set({
          user,
          isAuthenticated: !!user,
          isLoading: false,
          hasCheckedAuth: true,
        }),

      clearAuth: () =>
        set({
          user: null,
          isAuthenticated: false,
          isLoading: false,
          hasCheckedAuth: true,
        }),

      logout: async () => {
        const response = await fetch('/auth/logout', {
          method: 'POST',
          credentials: 'include',
          headers: {
            'X-CSRF-Token': getCookie('orbyte_csrf'),
          },
        })
        if (!response.ok) {
          throw new Error(`Logout failed: ${response.status}`)
        }
        set({
          user: null,
          isAuthenticated: false,
          isLoading: false,
          hasCheckedAuth: true,
        })
      },
    }),
    {
      name: 'orbyte-auth',
      partialize: (state) => ({
        user: state.user,
        isAuthenticated: state.isAuthenticated,
      }),
    }
  )
)
