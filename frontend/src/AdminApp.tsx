import { Routes, Route } from 'react-router-dom'
import { ToastProvider } from '@/components/ui/Toast'
import { PageLoading } from '@/components/feedback/Loading'
import { lazy, Suspense, useEffect, useState } from 'react'
import { useDarkMode } from '@/hooks/useDarkMode'
import { useAuth } from '@/hooks/useAuth'
import { fetchAdminBootstrap, toShellRoutes } from '@/services/bootstrap'
import { useShellStore } from '@/stores/shellStore'

const AdminWorkspacePage = lazy(() => import('@/features/admin/AdminWorkspacePage'))

function PageLoader() {
  return (
    <div className="flex h-full items-center justify-center">
      <PageLoading />
    </div>
  )
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading, hasCheckedAuth, checkAuth } = useAuth()

  useEffect(() => {
    void checkAuth()
  }, [checkAuth])

  if (isLoading || !hasCheckedAuth) return <PageLoader />
  if (!isAuthenticated) {
    window.location.href = '/ui/login'
    return null
  }
  return <>{children}</>
}

function BootstrapLoader({ children }: { children: React.ReactNode }) {
  const { hasCheckedAuth } = useAuth()
  const [ready, setReady] = useState(false)
  const setAdminBootstrap = useShellStore((state) => state.setAdminBootstrap)
  const setRoutes = useShellStore((state) => state.setRoutes)

  useEffect(() => {
    let mounted = true
    async function run() {
      if (!hasCheckedAuth) return
      try {
        const bootstrap = await fetchAdminBootstrap()
        if (!mounted) return
        setAdminBootstrap(bootstrap)
        setRoutes(toShellRoutes(bootstrap.menus, bootstrap.actions, bootstrap.locale, 'admin'))
        setReady(true)
      } catch (error) {
        if (!mounted) return
        if ((error as { status?: number }).status === 401) {
          window.location.href = '/ui/login'
          return
        }
        throw error
      }
    }
    void run()
    return () => {
      mounted = false
    }
  }, [hasCheckedAuth, setAdminBootstrap, setRoutes])

  if (!hasCheckedAuth || !ready) return <PageLoader />
  return <>{children}</>
}

export default function AdminApp() {
  useDarkMode()

  return (
    <ToastProvider>
      <ProtectedRoute>
        <BootstrapLoader>
          <Suspense fallback={<PageLoader />}>
            <Routes>
              <Route path="*" element={<AdminWorkspacePage />} />
            </Routes>
          </Suspense>
        </BootstrapLoader>
      </ProtectedRoute>
    </ToastProvider>
  )
}
