import { useAuthStore } from '@/stores/authStore'

export function useAuth() {
  const { user, token, isAuthenticated, isLoading, hasCheckedAuth, login, logout, checkAuth } = useAuthStore()

  return {
    user,
    token,
    isAuthenticated,
    isLoading,
    hasCheckedAuth,
    login,
    logout,
    checkAuth,
  }
}
