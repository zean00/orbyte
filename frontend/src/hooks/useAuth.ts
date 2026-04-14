import { useAuthStore } from '@/stores/authStore'

export function useAuth() {
  const {
    user,
    isAuthenticated,
    isLoading,
    hasCheckedAuth,
    setAuthenticatedUser,
    clearAuth,
    logout,
    checkAuth,
  } = useAuthStore()

  return {
    user,
    isAuthenticated,
    isLoading,
    hasCheckedAuth,
    setAuthenticatedUser,
    clearAuth,
    logout,
    checkAuth,
  }
}
