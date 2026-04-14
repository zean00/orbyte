import { useEffect, useState } from 'react'
import { adminPathSupportsPagination, endpointForAdminPath } from './adminRouting'

export function useAdminPageData(path: string, search: string, enabled: boolean) {
  const [payload, setPayload] = useState<Record<string, unknown> | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    let mounted = true

    async function load() {
      if (!enabled) return
      setLoading(true)
      try {
        const target = endpointForAdminPath(path)
        if (!target) {
          setPayload(null)
          setLoading(false)
          return
        }
        const query = new URLSearchParams(search)
        const requestTarget = adminPathSupportsPagination(path)
          ? `${target}${query.toString() ? `?${query.toString()}` : ''}`
          : target
        const response = await fetch(requestTarget, { credentials: 'include' })
        const data = await response.json()
        if (!mounted) return
        setPayload(data)
      } finally {
        if (mounted) {
          setLoading(false)
        }
      }
    }

    void load()
    return () => {
      mounted = false
    }
  }, [enabled, path, search])

  return { payload, loading }
}
