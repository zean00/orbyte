import type { RouteResolution } from './workspaceTypes'

type ListPayload = { items: Array<Record<string, unknown>>; total?: number }

const LIST_CACHE_TTL_MS = 30_000

let workspaceCacheSession = 'anonymous'

const routeCache = new Map<string, RouteResolution>()
const routeInFlight = new Map<string, Promise<RouteResolution>>()
const listCache = new Map<string, { payload: ListPayload; cachedAt: number }>()
const listInFlight = new Map<string, Promise<ListPayload>>()

function scopeKey(key: string): string {
  return `${workspaceCacheSession}:${key}`
}

export function setWorkspaceCacheSession(sessionKey: string): void {
  if (workspaceCacheSession === sessionKey) return
  workspaceCacheSession = sessionKey
  clearWorkspaceViewCaches()
}

export function clearWorkspaceViewCaches(): void {
  routeCache.clear()
  routeInFlight.clear()
  listCache.clear()
  listInFlight.clear()
}

export function readWorkspaceRouteCache(key: string): RouteResolution | null {
  return routeCache.get(scopeKey(key)) || null
}

export function writeWorkspaceRouteCache(key: string, payload: RouteResolution): void {
  routeCache.set(scopeKey(key), payload)
}

export function readWorkspaceRouteInFlight(key: string): Promise<RouteResolution> | null {
  return routeInFlight.get(scopeKey(key)) || null
}

export function writeWorkspaceRouteInFlight(key: string, request: Promise<RouteResolution>): void {
  routeInFlight.set(scopeKey(key), request)
}

export function clearWorkspaceRouteInFlight(key: string): void {
  routeInFlight.delete(scopeKey(key))
}

export function readWorkspaceListCache(key: string): ListPayload | null {
  const cached = listCache.get(scopeKey(key))
  if (!cached) return null
  if (Date.now() - cached.cachedAt >= LIST_CACHE_TTL_MS) {
    listCache.delete(scopeKey(key))
    return null
  }
  return cached.payload
}

export function writeWorkspaceListCache(key: string, payload: ListPayload): void {
  listCache.set(scopeKey(key), { payload, cachedAt: Date.now() })
}

export function clearWorkspaceListCache(key: string): void {
  listCache.delete(scopeKey(key))
}

export function readWorkspaceListInFlight(key: string): Promise<ListPayload> | null {
  return listInFlight.get(scopeKey(key)) || null
}

export function writeWorkspaceListInFlight(key: string, request: Promise<ListPayload>): void {
  listInFlight.set(scopeKey(key), request)
}

export function clearWorkspaceListInFlight(key: string): void {
  listInFlight.delete(scopeKey(key))
}
