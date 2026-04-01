import { useEffect, useState } from 'react'
import type { RouteResolution } from './workspaceTypes'

type UseWorkspaceRouteResolutionArgs = {
  pathname: string
  currentSurface: string
}

export function useWorkspaceRouteResolution({
  pathname,
  currentSurface,
}: UseWorkspaceRouteResolutionArgs) {
  const [route, setRoute] = useState<RouteResolution | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let mounted = true

    async function resolveRoute() {
      if (pathname === '/settings') {
        if (!mounted) return
        setRoute(null)
        setLoading(false)
        return
      }
      setLoading(true)
      try {
        const response = await fetch(
          `/ui/routes/resolve?path=${encodeURIComponent(pathname)}&surface=${encodeURIComponent(currentSurface)}`,
          { credentials: 'include' }
        )
        const payload = (await response.json()) as RouteResolution
        if (!mounted) return
        setRoute(payload)
      } catch (error) {
        if (!mounted) return
        setRoute({
          status: 'not_found',
          requested_path: pathname,
          message: error instanceof Error ? error.message : 'Route resolution failed',
        })
      } finally {
        if (mounted) setLoading(false)
      }
    }

    void resolveRoute()
    return () => {
      mounted = false
    }
  }, [currentSurface, pathname])

  return { route, loading }
}
