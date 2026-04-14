import { useEffect, useState } from 'react'
import {
  clearWorkspaceRouteInFlight,
  readWorkspaceRouteCache,
  readWorkspaceRouteInFlight,
  writeWorkspaceRouteCache,
  writeWorkspaceRouteInFlight,
} from './workspaceCache'
import type { RouteResolution } from './workspaceTypes'

type UseWorkspaceRouteResolutionArgs = {
  pathname: string
  currentSurface: string
}

export function useWorkspaceRouteResolution({
  pathname,
  currentSurface,
}: UseWorkspaceRouteResolutionArgs) {
  const cacheKey = `${currentSurface}:${pathname}`
  const cachedRoute = pathname === '/settings' ? null : readWorkspaceRouteCache(cacheKey)
  const [route, setRoute] = useState<RouteResolution | null>(cachedRoute)
  const [loading, setLoading] = useState(() => (pathname === '/settings' ? false : !cachedRoute))

  useEffect(() => {
    let mounted = true

    async function resolveRoute() {
      if (pathname === '/settings') {
        if (!mounted) return
        setRoute(null)
        setLoading(false)
        return
      }
      const cached = readWorkspaceRouteCache(cacheKey)
      if (cached) {
        if (!mounted) return
        setRoute(cached)
        setLoading(false)
        return
      }
      if (mounted) {
        setRoute(null)
      }
      setLoading(true)
      try {
        const request =
          readWorkspaceRouteInFlight(cacheKey) ||
          fetch(
            `/ui/routes/resolve?path=${encodeURIComponent(pathname)}&surface=${encodeURIComponent(currentSurface)}`,
            { credentials: 'include' }
          ).then(async (response) => {
            const payload = (await response.json()) as RouteResolution
            writeWorkspaceRouteCache(cacheKey, payload)
            clearWorkspaceRouteInFlight(cacheKey)
            return payload
          }).catch((error) => {
            clearWorkspaceRouteInFlight(cacheKey)
            throw error
          })
        writeWorkspaceRouteInFlight(cacheKey, request)
        const payload = await request
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
  }, [cacheKey, currentSurface, pathname])

  return { route: cachedRoute || route, loading: cachedRoute ? false : loading }
}
