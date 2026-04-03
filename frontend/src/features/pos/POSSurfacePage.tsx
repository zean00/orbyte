import { useEffect, useRef, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { useShellStore } from '@/stores/shellStore'
import { fetchWorkspaceBootstrap, pickText, toShellRoutes, type CustomEntryDefinition } from '@/services/bootstrap'

type RouteResolution = {
  status: 'ok' | 'not_found' | 'forbidden' | 'surface_mismatch'
  requested_path: string
  fallback_path?: string
  suggested_surface?: string
  message?: string
  render_mode?: 'generic' | 'custom' | 'flow'
  custom_entry?: CustomEntryDefinition
}

export default function POSSurfacePage() {
  const location = useLocation()
  const navigate = useNavigate()
  const {
    locale,
    currentSurface,
    workspaceBootstrap,
    setCurrentRoute,
    setWorkspaceBootstrap,
    setRoutes,
  } = useShellStore()
  const [route, setRoute] = useState<RouteResolution | null>(null)
  const [loading, setLoading] = useState(true)
  const pathname = location.pathname || '/pos/terminal'
  const surface = 'pos'

  useEffect(() => {
    setCurrentRoute(pathname)
  }, [pathname, setCurrentRoute])

  async function switchSurface(nextSurface: string) {
    const bootstrap = await fetchWorkspaceBootstrap(nextSurface)
    setWorkspaceBootstrap(bootstrap)
    setRoutes(toShellRoutes(bootstrap.menus, bootstrap.actions, bootstrap.locale, 'workspace'))
    navigate(useShellStore.getState().defaultPath || '/', { replace: true })
  }

  useEffect(() => {
    let mounted = true

    async function resolveRoute() {
      setLoading(true)
      try {
        const bootstrap =
          currentSurface === surface && workspaceBootstrap?.surface === surface
            ? workspaceBootstrap
            : await fetchWorkspaceBootstrap(surface)
        if (!mounted) return
        setWorkspaceBootstrap(bootstrap)
        setRoutes(toShellRoutes(bootstrap.menus, bootstrap.actions, bootstrap.locale, 'workspace'))
        const response = await fetch(
          `/ui/routes/resolve?path=${encodeURIComponent(pathname)}&surface=${encodeURIComponent(surface)}`,
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
  }, [currentSurface, pathname, setRoutes, setWorkspaceBootstrap, workspaceBootstrap])

  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_top,_rgba(201,94,54,0.16),_transparent_36%),linear-gradient(180deg,_var(--color-shell),_color-mix(in_srgb,var(--color-shell)_82%,#0b0b0d_18%))] text-body">
      <header className="sticky top-0 z-20 border-b border-line/80 bg-surface/90 backdrop-blur">
        <div className="mx-auto flex max-w-[1800px] items-center justify-between gap-4 px-6 py-4">
          <div>
            <p className="text-xs font-bold uppercase tracking-[0.22em] text-accent-dark">POS Surface</p>
            <h1 className="text-2xl font-black tracking-tight text-body">Orbyte Checkout</h1>
          </div>
          <div className="flex items-center gap-3">
            <span className="rounded-full border border-line bg-shell px-3 py-1 text-xs font-bold uppercase tracking-[0.16em] text-muted">
              {locale.toUpperCase()}
            </span>
            <button
              type="button"
              onClick={() => void switchSurface('backoffice')}
              className="rounded-xl border border-line bg-surface px-4 py-2 text-sm font-semibold text-body transition hover:border-accent hover:text-accent"
            >
              Backoffice
            </button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-[1800px] px-4 py-4 md:px-6">
        {loading ? <POSPanel title="Loading POS" status="Preparing cashier surface." /> : null}
        {!loading && route?.status !== 'ok' ? (
          <POSPanel title="POS unavailable" status={route?.message || 'POS route could not be resolved.'}>
            <div className="flex gap-3">
              <button
                type="button"
                onClick={() => void switchSurface('backoffice')}
                className="rounded-xl bg-accent px-4 py-2 text-sm font-semibold text-white"
              >
                Go to backoffice
              </button>
            </div>
          </POSPanel>
        ) : null}
        {!loading && route?.status === 'ok' && route.custom_entry ? (
          <POSCustomEntry entry={route.custom_entry} route={route} locale={locale} />
        ) : null}
      </main>
    </div>
  )
}

function POSCustomEntry({
  entry,
  route,
  locale,
}: {
  entry: CustomEntryDefinition
  route: RouteResolution
  locale: string
}) {
  const mountRef = useRef<HTMLDivElement | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let mounted = true
    async function load() {
      try {
        const renderFn = await loadBundleExport(entry.bundle_key, entry.component_export)
        if (!mounted || !mountRef.current) return
        mountRef.current.innerHTML = ''
        await renderFn({
          mount: mountRef.current,
          route,
          api: frontendAPI,
          params: Object.fromEntries(new URLSearchParams(window.location.search).entries()),
          locale,
          t: (key: string) => key,
        })
      } catch (err) {
        if (!mounted) return
        setError(err instanceof Error ? err.message : 'POS module failed to load')
      }
    }
    void load()
    return () => {
      mounted = false
    }
  }, [entry.bundle_key, entry.component_export, locale, route])

  return (
    <POSPanel title={pickText(entry, 'title', locale) || entry.key} status={error || undefined} padded={false}>
      <div ref={mountRef} className="min-h-[calc(100vh-10rem)]" />
    </POSPanel>
  )
}

function POSPanel({
  title,
  status,
  children,
  padded = true,
}: {
  title: string
  status?: string
  children?: React.ReactNode
  padded?: boolean
}) {
  return (
    <section className="overflow-hidden rounded-[1.5rem] border border-line bg-surface shadow-panel">
      <div className={padded ? 'border-b border-line px-6 py-5' : 'border-b border-line px-6 py-5'}>
        <h2 className="text-xl font-black tracking-tight text-body">{title}</h2>
        {status ? <p className="mt-1 text-sm text-muted">{status}</p> : null}
      </div>
      <div className={padded ? 'p-6' : 'p-0'}>{children}</div>
    </section>
  )
}

async function loadBundleExport(bundleKey: string, exportName: string): Promise<(args: Record<string, unknown>) => Promise<void> | void> {
  const globalWindow = window as Window & {
    ClinicModuleBundles?: Record<string, Record<string, (args: Record<string, unknown>) => Promise<void> | void>>
  }
  if (!globalWindow.ClinicModuleBundles?.[bundleKey]) {
    await new Promise<void>((resolve, reject) => {
      const script = document.createElement('script')
      script.src = `/ui/assets/modules/${encodeURIComponent(bundleKey)}.js`
      script.onload = () => resolve()
      script.onerror = () => reject(new Error('Failed to load module bundle'))
      document.head.appendChild(script)
    })
  }
  const bundle = globalWindow.ClinicModuleBundles?.[bundleKey]
  const fn = bundle?.[exportName]
  if (!fn) throw new Error('Module export not found')
  return fn
}

async function frontendAPI(path: string, options?: RequestInit) {
  const response = await fetch(path, {
    ...options,
    credentials: 'include',
  })
  if (!response.ok) throw await buildError(response)
  return response.json()
}

async function buildError(response: Response): Promise<Error> {
  try {
    const payload = await response.json()
    return Object.assign(new Error(payload?.error?.message || `Request failed: ${response.status}`), { status: response.status })
  } catch {
    return Object.assign(new Error(`Request failed: ${response.status}`), { status: response.status })
  }
}
