import { useEffect, useState } from 'react'
import { endpointForAdminPath } from './adminRouting'

export function useAdminPageData(path: string, enabled: boolean) {
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
        const response = await fetch(target, { credentials: 'include' })
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
  }, [enabled, path])

  return { payload, loading }
}
