import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { clearWorkspaceBootstrapCache } from '@/services/bootstrap'
import { setWorkspaceCacheSession } from '@/features/workspace/workspaceCache'

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

async function postLogout(): Promise<Response> {
  return fetch('/auth/logout', {
    method: 'POST',
    credentials: 'include',
    headers: {
      'X-CSRF-Token': getCookie('orbyte_csrf'),
    },
  })
}

function resetWorkspaceClientCaches(sessionKey: string): void {
  clearWorkspaceBootstrapCache()
  setWorkspaceCacheSession(sessionKey)
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
              resetWorkspaceClientCaches(String(data.user_id))
              set({
                user: { id: data.user_id, name: data.user_id, email: '', roles: [] },
                isAuthenticated: true,
                isLoading: false,
                hasCheckedAuth: true,
              })
            } else {
              resetWorkspaceClientCaches('anonymous')
              set({
                user: null,
                isAuthenticated: false,
                isLoading: false,
                hasCheckedAuth: true,
              })
            }
          } else {
            resetWorkspaceClientCaches('anonymous')
            set({
              user: null,
              isAuthenticated: false,
              isLoading: false,
              hasCheckedAuth: true,
            })
          }
        } catch {
          resetWorkspaceClientCaches('anonymous')
          set({
            user: null,
            isAuthenticated: false,
            isLoading: false,
            hasCheckedAuth: true,
          })
        }
      },

      setAuthenticatedUser: (user) =>
        {
          resetWorkspaceClientCaches(user?.id || 'anonymous')
          set({
            user,
            isAuthenticated: !!user,
            isLoading: false,
            hasCheckedAuth: true,
          })
        },

      clearAuth: () =>
        {
          resetWorkspaceClientCaches('anonymous')
          set({
            user: null,
            isAuthenticated: false,
            isLoading: false,
            hasCheckedAuth: true,
          })
        },

      logout: async () => {
        let response = await postLogout()
        if (response.status === 403) {
          await fetch('/auth/session', {
            credentials: 'include',
          })
          response = await postLogout()
        }
        if (!response.ok && response.status !== 401) {
          throw new Error(`Logout failed: ${response.status}`)
        }
        resetWorkspaceClientCaches('anonymous')
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
