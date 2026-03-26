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
  token: string | null
  isAuthenticated: boolean
  isLoading: boolean
  hasCheckedAuth: boolean
  checkAuth: () => Promise<void>
  setUser: (user: User | null) => void
  setToken: (token: string | null) => void
  login: (user: User, token: string) => void
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
      token: null,
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
                token: null,
                isAuthenticated: false,
                isLoading: false,
                hasCheckedAuth: true,
              })
            }
          } else {
            set({
              user: null,
              token: null,
              isAuthenticated: false,
              isLoading: false,
              hasCheckedAuth: true,
            })
          }
        } catch {
          set({
            user: null,
            token: null,
            isAuthenticated: false,
            isLoading: false,
            hasCheckedAuth: true,
          })
        }
      },

      setUser: (user) =>
        set({
          user,
          isAuthenticated: !!user,
          hasCheckedAuth: true,
        }),

      setToken: (token) =>
        set({
          token,
          isAuthenticated: !!token,
          hasCheckedAuth: true,
        }),

      login: (user, token) =>
        set({
          user,
          token,
          isAuthenticated: true,
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
          token: null,
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
        token: state.token,
        isAuthenticated: state.isAuthenticated,
      }),
    }
  )
)
