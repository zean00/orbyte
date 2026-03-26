import { useEffect, useMemo, useRef, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { Shell } from '@/components/layout/Shell'
import { Modal } from '@/components/ui/Modal'
import { useShellStore } from '@/stores/shellStore'
import { fetchWorkspaceBootstrap, normalizeShellPath, pickText, toShellRoutes, type ActionDefinition, type CustomEntryDefinition, type DocumentFlowDefinition, type FieldDefinition, type SectionDefinition, type ViewDefinition } from '@/services/bootstrap'
import { useToast } from '@/components/ui/Toast'
import SettingsPage from '@/features/SettingsPage'

type RouteResolution = {
  status: 'ok' | 'not_found' | 'forbidden' | 'surface_mismatch'
  requested_path: string
  fallback_path?: string
  suggested_surface?: string
  message?: string
  render_mode?: 'generic' | 'custom' | 'flow'
  action?: ActionDefinition
  view?: ViewDefinition
  flow?: DocumentFlowDefinition
  custom_entry?: CustomEntryDefinition
}

type FormState = Record<string, unknown>
type ValidationErrors = Record<string, string>
type HttpError = Error & { status?: number }

export default function WorkspacePage() {
  const location = useLocation()
  const navigate = useNavigate()
  const { addToast } = useToast()
  const {
    currentSurface,
    locale,
    actions,
    defaultPath,
    setWorkspaceBootstrap,
    setRoutes,
    shellKind,
  } = useShellStore()
  const [route, setRoute] = useState<RouteResolution | null>(null)
  const [loading, setLoading] = useState(true)
  const pathname = normalizeLegacyWorkspacePath(location.pathname || '/')

  useEffect(() => {
    const currentPath = location.pathname || '/'
    const normalizedPath = normalizeLegacyWorkspacePath(currentPath)
    if (normalizedPath !== currentPath) {
      const target = `${normalizedPath}${location.search}${location.hash}`
      navigate(target, { replace: true })
    }
  }, [location.hash, location.pathname, location.search, navigate])

  useEffect(() => {
    if (pathname === '/' && defaultPath && defaultPath !== '/') {
      navigate(defaultPath, { replace: true })
    }
  }, [defaultPath, navigate, pathname])

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

  useEffect(() => {
    useShellStore.getState().setCurrentRoute(pathname)
  }, [pathname])

  async function reloadBootstrap(surface = currentSurface) {
    const bootstrap = await fetchWorkspaceBootstrap(surface)
    setWorkspaceBootstrap(bootstrap)
    setRoutes(toShellRoutes(bootstrap.menus, bootstrap.actions, bootstrap.locale, 'workspace'))
    return {
      ...bootstrap,
      default_path: normalizeShellPath(bootstrap.default_path, 'workspace'),
    }
  }

  async function handleSwitchSurface(surface: string, fallback?: string) {
    const bootstrap = await reloadBootstrap(surface)
    navigate(fallback || bootstrap.default_path || '/', { replace: true })
  }

  const content = useMemo(() => {
    if (pathname === '/settings') return <SettingsPage />
    if (loading) return <Panel title="Loading" status="Resolving route contract." />
    if (!route) return <Panel title="Unavailable" status="Route could not be resolved." />
    if (route.status !== 'ok') {
      return (
        <RecoveryPanel
          route={route}
          onDefault={() => navigate(route.fallback_path || defaultPath || '/', { replace: true })}
          onSwitchSurface={() => void handleSwitchSurface(route.suggested_surface || currentSurface, route.fallback_path)}
        />
      )
    }
    if (route.render_mode === 'flow' && route.flow) {
      return <FlowRouteView route={route} locale={locale} />
    }
    if (route.render_mode === 'custom' && route.custom_entry) {
      return <CustomRouteView entry={route.custom_entry} route={route} locale={locale} />
    }
    if (pathname === '/notifications') {
      return <NotificationsView />
    }
    if (route.view) {
      return (
        <GenericRouteView
          route={route}
          locale={locale}
          actions={actions}
          currentPath={pathname}
          onNavigate={(target) => navigate(target)}
          onToast={(message, variant = 'default') => addToast({ message, variant })}
        />
      )
    }
    return <Panel title="Unavailable" status="No renderer available for this route." />
  }, [actions, addToast, currentSurface, defaultPath, loading, locale, navigate, pathname, route])

  return (
    <Shell>
      {shellKind === 'workspace' ? content : <Panel title="Unavailable" status="Workspace shell is not active." />}
    </Shell>
  )
}

function Panel({ title, status, children }: { title: string; status?: string; children?: React.ReactNode }) {
  return (
    <section className="rounded-2xl border border-line bg-surface p-6 shadow-panel">
      <div className="mb-4">
        <h1 className="text-2xl font-bold text-body">{title}</h1>
        {status ? <p className="mt-1 text-sm text-muted">{status}</p> : null}
      </div>
      {children}
    </section>
  )
}

function RecoveryPanel({
  route,
  onDefault,
  onSwitchSurface,
}: {
  route: RouteResolution
  onDefault: () => void
  onSwitchSurface: () => void
}) {
  return (
    <Panel title={route.status === 'forbidden' ? 'Route forbidden' : 'Route unavailable'} status={route.message}>
      <div className="flex gap-3">
        {route.status === 'surface_mismatch' ? (
          <button onClick={onSwitchSurface} className="rounded-lg bg-accent px-4 py-2 text-white">
            Switch surface
          </button>
        ) : null}
        <button onClick={onDefault} className="rounded-lg border border-line px-4 py-2 text-body">
          Go to default
        </button>
      </div>
    </Panel>
  )
}

function GenericRouteView({
  route,
  locale,
  actions,
  currentPath,
  onNavigate,
  onToast,
}: {
  route: RouteResolution
  locale: string
  actions: ActionDefinition[]
  currentPath: string
  onNavigate: (target: string) => void
  onToast: (message: string, variant?: 'default' | 'success' | 'warning' | 'error') => void
}) {
  const view = route.view!
  const searchParams = useMemo(() => new URLSearchParams(window.location.search), [route.requested_path, window.location.search])

  if (view.kind === 'queue') {
    return <QueueView view={view} locale={locale} routeActions={actions} onNavigate={onNavigate} />
  }
  if (view.kind === 'list') {
    return <ListView view={view} locale={locale} routeActions={actions} currentPath={currentPath} onNavigate={onNavigate} />
  }
  if (view.kind === 'detail') {
    return <DetailView view={view} locale={locale} routeActions={actions} currentPath={currentPath} onNavigate={onNavigate} onToast={onToast} />
  }
  if (view.kind === 'dashboard') {
    return <DashboardView view={view} locale={locale} onNavigate={onNavigate} routeActions={actions} />
  }
  if (view.kind === 'form') {
    return <FormView view={view} locale={locale} currentPath={currentPath} searchParams={searchParams} onNavigate={onNavigate} onToast={onToast} />
  }
  return <Panel title={pickText(view, 'title', locale) || route.requested_path} status="View kind is not yet supported." />
}

function QueueView({
  view,
  locale,
  routeActions,
  onNavigate,
}: {
  view: ViewDefinition
  locale: string
  routeActions: ActionDefinition[]
  onNavigate: (target: string) => void
}) {
  const [payload, setPayload] = useState<{ items: Array<Record<string, unknown>>; summary?: Record<string, unknown> } | null>(null)
  const source = (view.projection_key || '').includes('approval') ? 'approvals' : 'tasks'
  const searchParams = useMemo(() => new URLSearchParams(window.location.search), [window.location.search])

  useEffect(() => {
    let mounted = true
    async function load() {
      const query = new URLSearchParams()
      for (const key of ['status', 'due', 'workflow_key', 'mine', 'requested_by_me']) {
        const value = searchParams.get(key)
        if (value) query.set(key, value)
      }
      const [itemsResponse, summaryResponse] = await Promise.all([
        fetchJSON<{ items: Array<Record<string, unknown>> }>(`/ui/data/worklist/${source}${query.toString() ? `?${query}` : ''}`),
        fetchJSON<Record<string, unknown>>(`/ui/data/worklist/summary${query.toString() ? `?${query}` : ''}`),
      ])
      if (!mounted) return
      setPayload({ items: itemsResponse.items || [], summary: summaryResponse })
    }
    void load()
    return () => {
      mounted = false
    }
  }, [searchParams, source])

  const title = pickText(view, 'title', locale) || 'Worklist'
  const items = payload?.items || []
  const summary = (source === 'approvals' ? payload?.summary?.approvals : payload?.summary?.tasks) as Record<string, unknown> | undefined

  return (
    <Panel title={title} status={pickText(view, 'empty_state', locale) || 'Operational queue for workflow work.'}>
      <div className="mb-4 grid grid-cols-1 gap-4 md:grid-cols-4">
        <MetricCard label="Total" value={String(summary?.total || items.length || 0)} />
        <MetricCard label={source === 'approvals' ? 'Pending' : 'Open'} value={String(summary?.[source === 'approvals' ? 'pending' : 'open'] || 0)} />
        <MetricCard label="Overdue" value={String(summary?.overdue || 0)} />
        <MetricCard label="Workflows" value={String(summary?.workflows || 0)} />
      </div>
      <DataTable
        columns={[
          { key: 'target_title', label: 'Target' },
          { key: 'status', label: 'Status' },
          { key: source === 'approvals' ? 'requested_by' : 'assignee_user_id', label: source === 'approvals' ? 'Requested By' : 'Assignee' },
          { key: 'due_at', label: 'Due' },
        ]}
        rows={items}
        emptyText="No work items."
        renderAction={(row) => (
          <button
            onClick={() => {
              const detailPath = routeForWorkItem(row, routeActions)
              if (detailPath) onNavigate(detailPath)
            }}
            className="rounded-lg border border-line px-3 py-1.5 text-sm text-body"
          >
            Open
          </button>
        )}
      />
    </Panel>
  )
}

function ListView({
  view,
  locale,
  routeActions,
  currentPath,
  onNavigate,
}: {
  view: ViewDefinition
  locale: string
  routeActions: ActionDefinition[]
  currentPath: string
  onNavigate: (target: string) => void
}) {
  const [payload, setPayload] = useState<{ items: Array<Record<string, unknown>>; total?: number } | null>(null)
  const searchParams = useMemo(() => new URLSearchParams(window.location.search), [window.location.search])
  const createTarget = view.model_key ? routeForModel(view.model_key, 'form', routeActions, currentPath) : routeForCreate(currentPath, routeActions)
  const activeSearch = searchParams.get('name') || ''
  const activeSort = searchParams.get('sort') || ''
  const activePageSize = searchParams.get('page_size') || ''

  useEffect(() => {
    let mounted = true
    async function load() {
      const query = new URLSearchParams()
      if (view.document_type) query.set('type', view.document_type)
      if (view.model_key) query.set('model', view.model_key)
      for (const key of ['status', 'name', 'sort', 'page', 'page_size']) {
        const value = searchParams.get(key)
        if (value) query.set(key, value)
      }
      const base = view.model_key ? '/ui/data/models' : '/ui/data/documents'
      const result = await fetchJSON<{ items: Array<Record<string, unknown>>; total?: number }>(`${base}?${query}`)
      if (!mounted) return
      setPayload(result)
    }
    void load()
    return () => {
      mounted = false
    }
  }, [searchParams, view.document_type, view.model_key])

  const columns = view.columns?.length
    ? view.columns
    : view.model_key
      ? [{ key: 'id', label: 'ID', path: 'id' }]
      : [
          { key: 'id', label: 'ID', path: 'header.id' },
          { key: 'status', label: 'Status', path: 'header.status' },
          { key: 'type', label: 'Type', path: 'header.type' },
        ]
  const filterOptions = (view.filters || []).filter((filter) => filter.type === 'enum' && filter.options?.length)
  const sortOptions = columns
    .map((column) => {
      const raw = column.path.split('.').pop() || column.key
      return { value: raw, label: pickText(column, 'label', locale) || humanize(raw) }
    })
    .filter((option, index, items) => items.findIndex((candidate) => candidate.value === option.value) === index)

  function applyListQuery(updates: Record<string, string>) {
    const next = new URLSearchParams(searchParams)
    Object.entries(updates).forEach(([key, value]) => {
      if (value) next.set(key, value)
      else next.delete(key)
    })
    next.delete('page')
    const target = next.toString() ? `${currentPath}?${next.toString()}` : currentPath
    onNavigate(target)
  }

  return (
    <Panel title={pickText(view, 'title', locale) || 'List'} status={pickText(view, 'empty_state', locale) || 'Standard list rendered from UI contracts.'}>
      <div className="mb-4 space-y-4">
        <div className="flex items-center justify-between">
          <div className="text-sm text-muted">Items {payload?.total ?? payload?.items?.length ?? 0}</div>
          {createTarget ? (
            <button onClick={() => onNavigate(createTarget)} className="rounded-lg bg-accent px-4 py-2 text-sm text-white">
              New
            </button>
          ) : null}
        </div>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-4">
          <label className="flex flex-col gap-1">
            <span className="text-xs font-semibold uppercase tracking-wide text-muted">Search</span>
            <input
              id="list-search"
              name="list_search"
              value={activeSearch}
              onChange={(e) => applyListQuery({ name: e.target.value })}
              placeholder="Search by name"
              className="h-10 rounded-lg border border-line bg-surface px-3 py-2 text-sm text-body"
            />
          </label>
          {filterOptions.map((filter) => {
            const selected = searchParams.get(filter.key) || ''
            return (
              <label key={filter.key} className="flex flex-col gap-1">
                <span className="text-xs font-semibold uppercase tracking-wide text-muted">{pickText(filter, 'label', locale) || filter.key}</span>
                <select
                  id={`filter-${filter.key}`}
                  name={`filter_${filter.key}`}
                  value={selected}
                  onChange={(e) => applyListQuery({ [filter.key]: e.target.value })}
                  className="h-10 rounded-lg border border-line bg-surface px-3 py-2 text-sm text-body"
                >
                  <option value="">All</option>
                  {(filter.options || []).map((option) => (
                    <option key={option} value={option}>
                      {humanize(option)}
                    </option>
                  ))}
                </select>
              </label>
            )
          })}
          <label className="flex flex-col gap-1">
            <span className="text-xs font-semibold uppercase tracking-wide text-muted">Sort</span>
            <select
              id="list-sort"
              name="list_sort"
              value={activeSort}
              onChange={(e) => applyListQuery({ sort: e.target.value })}
              className="h-10 rounded-lg border border-line bg-surface px-3 py-2 text-sm text-body"
            >
              <option value="">Default</option>
              {sortOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-xs font-semibold uppercase tracking-wide text-muted">Page Size</span>
            <select
              id="list-page-size"
              name="list_page_size"
              value={activePageSize}
              onChange={(e) => applyListQuery({ page_size: e.target.value })}
              className="h-10 rounded-lg border border-line bg-surface px-3 py-2 text-sm text-body"
            >
              <option value="">Default</option>
              <option value="10">10</option>
              <option value="20">20</option>
              <option value="50">50</option>
              <option value="100">100</option>
            </select>
          </label>
        </div>
      </div>
      <DataTable
        columns={columns.map((column) => ({ key: column.path, label: pickText(column, 'label', locale) || column.key }))}
        rows={payload?.items || []}
        emptyText={pickText(view, 'empty_state', locale) || 'No records.'}
        renderAction={(row) => {
          const id = String(view.model_key ? row.id || '' : resolvePath(row, 'header.id') || '')
          const detailTarget = view.model_key ? routeForModel(view.model_key, 'detail', routeActions, currentPath) : routeForDocument(view.document_type || '', 'detail', routeActions, currentPath)
          if (!detailTarget || !id) return null
          return (
            <button
              onClick={() => onNavigate(`${detailTarget}?id=${encodeURIComponent(id)}`)}
              className="rounded-lg border border-line px-3 py-1.5 text-sm text-body"
            >
              Open
            </button>
          )
        }}
      />
    </Panel>
  )
}

function DetailView({
  view,
  locale,
  routeActions,
  currentPath,
  onNavigate,
  onToast,
}: {
  view: ViewDefinition
  locale: string
  routeActions: ActionDefinition[]
  currentPath: string
  onNavigate: (target: string) => void
  onToast: (message: string, variant?: 'default' | 'success' | 'warning' | 'error') => void
}) {
  const [payload, setPayload] = useState<Record<string, unknown> | null>(null)
  const [reloadKey, setReloadKey] = useState(0)
  const [stepUpOpen, setStepUpOpen] = useState(false)
  const [pendingAction, setPendingAction] = useState('')
  const [stepUpCode, setStepUpCode] = useState('')
  const [stepUpError, setStepUpError] = useState('')
  const documentID = new URLSearchParams(window.location.search).get('id')

  useEffect(() => {
    let mounted = true
    async function load() {
      if (!documentID) return
      const base = view.model_key ? `/ui/data/models/${encodeURIComponent(view.model_key)}/${encodeURIComponent(documentID)}` : `/ui/data/documents/${encodeURIComponent(documentID)}`
      const result = await fetchJSON<Record<string, unknown>>(base)
      if (!mounted) return
      setPayload(result)
    }
    void load()
    return () => {
      mounted = false
    }
  }, [documentID, reloadKey, view.model_key])

  if (!documentID) {
    return <Panel title={pickText(view, 'title', locale) || 'Detail'} status="Select a record from a list to inspect it." />
  }

  const record = (payload?.record || payload) as Record<string, unknown> | null
  const header = (record?.header || {}) as Record<string, unknown>
  const sections = resolveSections(view)
  const editTarget = view.model_key ? routeForModel(view.model_key, 'form', routeActions, currentPath) : routeForEdit(currentPath, view.document_type || String(header.type || ''), routeActions)
  const cancelTarget = stripEditorSuffix(currentPath) || '/documents'
  const hasDocumentCancelAction = !!view.allowed_actions?.some((actionKey) => actionKey.toLowerCase() === 'cancel')

  async function handleAction(actionKey: string) {
    try {
      await invokeDocumentAction(String(header.id || ''), actionKey)
      onToast(`Action ${actionKey} applied`, 'success')
      setReloadKey((current) => current + 1)
    } catch (error) {
      const status = (error as HttpError).status
      const message = error instanceof Error ? error.message : 'Action failed'
      if (status === 403 && /step-up verification required/i.test(message) && (actionKey === 'approve' || actionKey === 'reject')) {
        setPendingAction(actionKey)
        setStepUpCode('')
        setStepUpError('')
        setStepUpOpen(true)
        return
      }
      onToast(message, 'error')
    }
  }

  async function handleVerifyStepUp() {
    const response = await fetch('/auth/2fa/approval/verify', {
      method: 'POST',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': readCookie('orbyte_csrf'),
      },
      body: JSON.stringify({ code: stepUpCode }),
    })
    if (!response.ok) throw await buildError(response)
  }

  return (
    <Panel title={pickText(view, 'title', locale) || 'Detail'} status={String(header.status || '')}>
      <div className="mb-4 flex flex-wrap gap-3">
        {!hasDocumentCancelAction ? (
          <button
            onClick={() => onNavigate(cancelTarget)}
            className="rounded-lg border border-line px-4 py-2 text-sm text-body"
          >
            Cancel
          </button>
        ) : null}
        {view.allowed_actions?.map((actionKey) => (
          <button
            key={actionKey}
            onClick={() => void handleAction(actionKey)}
            className="rounded-lg border border-line px-4 py-2 text-sm text-body"
          >
            {humanize(actionKey)}
          </button>
        ))}
        {editTarget ? (
          <button onClick={() => onNavigate(`${editTarget}?id=${encodeURIComponent(documentID)}`)} className="rounded-lg bg-accent px-4 py-2 text-sm text-white">
            Edit
          </button>
        ) : null}
      </div>
      <div className="space-y-4">
        {sections.map((section) => (
          <section key={section.key} className="rounded-xl border border-line p-4">
            <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">{pickText(section, 'title', locale) || section.key}</h2>
            <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
              {(section.fields || []).map((field) => (
                <article key={field.key} className="rounded-lg bg-surface p-3 dark:bg-ink/60">
                  <div className="text-xs uppercase tracking-wide text-muted">{pickText(field, 'label', locale) || field.key}</div>
                  <div className="mt-1 text-sm text-body">{displayValue(resolvePath(record, field.path))}</div>
                </article>
              ))}
            </div>
          </section>
        ))}
      </div>
      <Modal isOpen={stepUpOpen} onClose={() => setStepUpOpen(false)} title="Two-Factor Verification" size="sm">
        <div className="space-y-4">
          <p className="text-sm text-muted">Enter the code from Google Authenticator to continue with this approval action.</p>
          <input
            value={stepUpCode}
            onChange={(event) => setStepUpCode(event.target.value)}
            inputMode="numeric"
            autoComplete="one-time-code"
            className="h-10 w-full rounded-lg border border-line bg-surface px-3 py-2 text-sm text-body"
            placeholder="123456"
          />
          {stepUpError ? <div className="text-sm text-danger">{stepUpError}</div> : null}
          <div className="flex gap-3">
            <button onClick={() => setStepUpOpen(false)} className="rounded-lg border border-line px-4 py-2 text-body">
              Cancel
            </button>
            <button
              onClick={() => {
                void handleVerifyStepUp()
                  .then(async () => {
                    setStepUpOpen(false)
                    setStepUpError('')
                    if (pendingAction) {
                      await handleAction(pendingAction)
                    }
                  })
                  .catch((error) => setStepUpError(error instanceof Error ? error.message : 'Verification failed'))
              }}
              className="rounded-lg bg-accent px-4 py-2 text-white"
            >
              Verify
            </button>
          </div>
        </div>
      </Modal>
    </Panel>
  )
}

function DashboardView({
  view,
  locale,
  onNavigate,
  routeActions,
}: {
  view: ViewDefinition
  locale: string
  onNavigate: (target: string) => void
  routeActions: ActionDefinition[]
}) {
  const [payload, setPayload] = useState<Record<string, unknown> | null>(null)

  useEffect(() => {
    let mounted = true
    async function load() {
      const target = view.dataset_key
        ? `/ui/data/reporting/datasets/${encodeURIComponent(view.dataset_key)}`
        : view.projection_key === 'monitoring.summary'
          ? '/ui/data/monitoring/summary'
          : '/ui/data/analytics/snapshot'
      const result = await fetchJSON<Record<string, unknown>>(target)
      if (!mounted) return
      setPayload(result)
    }
    void load()
    return () => {
      mounted = false
    }
  }, [view.dataset_key, view.projection_key])

  return (
    <Panel title={pickText(view, 'title', locale) || 'Dashboard'}>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        {(view.cards || []).map((card) => (
          <button
            key={card.key}
            onClick={() => {
              const action = routeActions.find((item) => item.key === card.action_key)
              if (action) onNavigate(action.route_path)
            }}
            className="rounded-xl border border-line bg-surface p-5 text-left dark:bg-ink/60"
          >
            <div className="text-xs uppercase tracking-wide text-muted">{pickText(card, 'label', locale) || card.key}</div>
            <div className="mt-2 text-2xl font-bold text-body">{displayValue(resolvePath(payload, card.path))}</div>
          </button>
        ))}
      </div>
    </Panel>
  )
}

function FormView({
  view,
  locale,
  currentPath,
  searchParams,
  onNavigate,
  onToast,
}: {
  view: ViewDefinition
  locale: string
  currentPath: string
  searchParams: URLSearchParams
  onNavigate: (target: string) => void
  onToast: (message: string, variant?: 'default' | 'success' | 'warning' | 'error') => void
}) {
  const targetID = searchParams.get('id') || ''
  const [values, setValues] = useState<FormState>({})
  const [version, setVersion] = useState(0)
  const [etag, setETag] = useState('')
  const [errors, setErrors] = useState<ValidationErrors>({})

  useEffect(() => {
    let mounted = true
    async function load() {
      if (!targetID) return
      if (view.model_key) {
        const payload = await fetchJSON<Record<string, unknown>>(`/ui/data/models/${encodeURIComponent(view.model_key)}/${encodeURIComponent(targetID)}`)
        if (!mounted) return
        const record = payload.record as Record<string, unknown>
        setValues(((record.values as Record<string, unknown>) || {}) as FormState)
        setVersion(Number(record.version || 0))
        setErrors({})
      } else {
        const payload = await fetchJSON<Record<string, unknown>>(`/ui/data/documents/${encodeURIComponent(targetID)}`)
        if (!mounted) return
        const record = payload.record as Record<string, unknown>
        const header = (record.header || {}) as Record<string, unknown>
        const body = (record.body || {}) as Record<string, unknown>
        setValues(((body.payload as Record<string, unknown>) || {}) as FormState)
        setVersion(Number(header.version || 0))
        setETag(String(header.etag || ''))
        setErrors({})
      }
    }
    void load()
    return () => {
      mounted = false
    }
  }, [targetID, view.model_key])

  const sections = resolveSections(view)
  const validationFields = sections.flatMap((section) => section.fields || [])
  const cancelTarget = targetID
    ? (view.model_key
        ? routeForModel(view.model_key, 'detail', useShellStore.getState().actions, currentPath)
        : routeForDocument(view.document_type || '', 'detail', useShellStore.getState().actions, currentPath))
    : stripEditorSuffix(currentPath) || '/'

  async function handleSave() {
    const nextErrors = validateFieldCollection(validationFields, values, !!view.model_key, locale)
    setErrors(nextErrors)
    if (Object.keys(nextErrors).length) {
      onToast('Please fix the highlighted fields.', 'warning')
      return
    }

    if (view.model_key) {
      const response = await fetch(`/models/${encodeURIComponent(view.model_key)}${targetID ? `/${encodeURIComponent(targetID)}` : ''}`, {
        method: targetID ? 'PUT' : 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': readCookie('orbyte_csrf'),
        },
        body: JSON.stringify(targetID ? { values, expected_version: version } : { values }),
      })
      if (!response.ok) throw await buildError(response)
      const payload = await response.json()
      const created = payload.record || payload
      onToast(targetID ? 'Record updated.' : 'Record created.', 'success')
      if (!targetID && created?.id) {
        const target = routeForModel(view.model_key, 'detail', useShellStore.getState().actions, currentPath)
        if (target) onNavigate(`${target}?id=${encodeURIComponent(created.id)}`)
      }
      return
    }

    if (targetID) {
      const response = await fetch(`/documents/${encodeURIComponent(targetID)}`, {
        method: 'PUT',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': readCookie('orbyte_csrf'),
        },
        body: JSON.stringify({ payload: values, expected_version: version, expected_etag: etag }),
      })
      if (!response.ok) throw await buildError(response)
      onToast('Draft updated.', 'success')
      return
    }

    const response = await fetch('/documents', {
      method: 'POST',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': readCookie('orbyte_csrf'),
      },
      body: JSON.stringify({
        type: view.document_type,
        organization_id: 'org_default',
        payload: values,
      }),
    })
    if (!response.ok) throw await buildError(response)
    const created = await response.json()
    onToast('Record created.', 'success')
    const target = routeForDocument(view.document_type || '', 'detail', useShellStore.getState().actions, currentPath)
    if (target && created?.header?.id) onNavigate(`${target}?id=${encodeURIComponent(created.header.id)}`)
  }

  return (
    <Panel title={pickText(view, 'title', locale) || 'Editor'}>
      <div className="space-y-4">
        {sections.map((section) => (
          <section key={section.key} className="rounded-xl border border-line p-4">
            <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">{pickText(section, 'title', locale) || section.key}</h2>
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              {(section.fields || []).map((field) => (
                <FieldEditor
                  key={field.key}
                  field={field}
                  locale={locale}
                  values={values}
                  onChange={setValues}
                  model={!!view.model_key}
                  error={errors[field.key]}
                  onBlur={() =>
                    setErrors((current) => {
                      const message = validateFieldInput(field, resolvePath(values, normalizeFieldPath(field, !!view.model_key)), locale)
                      if (!message && !current[field.key]) return current
                      const next = { ...current }
                      if (message) next[field.key] = message
                      else delete next[field.key]
                      return next
                    })
                  }
                />
              ))}
            </div>
          </section>
        ))}
      </div>
      <div className="mt-6 flex gap-3">
        <button
          onClick={() => onNavigate(targetID && cancelTarget ? `${cancelTarget}?id=${encodeURIComponent(targetID)}` : cancelTarget)}
          className="rounded-lg border border-line px-4 py-2 text-body"
        >
          Cancel
        </button>
        <button
          onClick={() => {
            void handleSave().catch((error) => onToast(error instanceof Error ? error.message : 'Save failed', 'error'))
          }}
          className="rounded-lg bg-accent px-4 py-2 text-white"
        >
          {targetID ? 'Save' : 'Create'}
        </button>
      </div>
    </Panel>
  )
}

function FlowRouteView({ route, locale }: { route: RouteResolution; locale: string }) {
  const flow = route.flow!
  const navigate = useNavigate()
  const { addToast } = useToast()
  const documentID = new URLSearchParams(window.location.search).get('id') || ''
  const activeParam = new URLSearchParams(window.location.search).get('document_key') || ''
  const [draft, setDraft] = useState<Record<string, { payload: FormState }>>({})
  const [stepIndex, setStepIndex] = useState(0)
  const [activeDocKey, setActiveDocKey] = useState(activeParam)
  const [errors, setErrors] = useState<ValidationErrors>({})

  useEffect(() => {
    let mounted = true
    async function load() {
      if (!documentID) return
      const payload = await fetchJSON<Record<string, unknown>>(`/ui/data/documents/${encodeURIComponent(documentID)}`)
      if (!mounted) return
      const instance = payload.flow_instance as Record<string, unknown> | undefined
      const nextDraft: Record<string, { payload: FormState }> = {}
      const items = (instance?.items || []) as Array<Record<string, unknown>>
      for (const item of items) {
        const definition = item.definition as Record<string, unknown>
        const record = item.record as Record<string, unknown>
        const body = (record?.body || {}) as Record<string, unknown>
        nextDraft[String(definition?.key || '')] = { payload: ((body.payload as FormState) || {}) }
      }
      setDraft(nextDraft)
      setActiveDocKey(String(instance?.active_document_key || activeParam || ''))
      setErrors({})
    }
    void load()
    return () => {
      mounted = false
    }
  }, [activeParam, documentID])

  const steps = useMemo(() => resolveFlowSequence(flow, draft), [draft, flow])
  const currentStep = steps[stepIndex] || steps[0]
  const currentDocKey = activeDocKey || currentStep?.documents?.[0]?.key || ''

  function validateCurrentStep(): boolean {
    const nextErrors: ValidationErrors = {}
    for (const doc of currentStep?.documents || []) {
      const docErrors = validateFieldCollection(collectFlowFields(doc), draft[doc.key]?.payload || {}, false, locale, doc.key)
      Object.assign(nextErrors, docErrors)
    }
    setErrors(nextErrors)
    return Object.keys(nextErrors).length === 0
  }

  async function handleCommit() {
    const response = await fetch(`/document-flows/${encodeURIComponent(flow.key)}/commit`, {
      method: 'POST',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': readCookie('orbyte_csrf'),
      },
      body: JSON.stringify({
        organization_id: 'org_default',
        primary_document_id: documentID,
        documents: Object.fromEntries(Object.entries(draft).map(([key, value]) => [key, value.payload])),
      }),
    })
    if (!response.ok) throw await buildError(response)
    const payload = await response.json()
    const target = routeForDocument(payload.primary_document_type, 'detail', useShellStore.getState().actions, route.requested_path)
    if (target && payload.primary_document_id) {
      navigate(`${target}?id=${encodeURIComponent(payload.primary_document_id)}`, { replace: true })
    }
  }

  if (!currentStep) {
    return <Panel title={pickText(flow, 'title', locale) || 'Flow'} status="No flow steps are available." />
  }

  const cancelTarget = documentID
    ? `${routeForDocument(flow.primary_document_type, 'detail', useShellStore.getState().actions, route.requested_path)}?id=${encodeURIComponent(documentID)}`
    : stripEditorSuffix(route.requested_path) || route.fallback_path || '/documents'

  return (
    <Panel title={pickText(flow, 'title', locale) || 'Flow'} status={pickText(currentStep, 'title', locale)}>
      <div className="mb-4 flex flex-wrap gap-2">
        {steps.map((step, index) => (
          <button
            key={step.key}
            onClick={() => setStepIndex(index)}
            className={`rounded-lg px-3 py-2 text-sm ${index === stepIndex ? 'bg-accent text-white' : 'border border-line text-body'}`}
          >
            {pickText(step, 'title', locale) || step.key}
          </button>
        ))}
      </div>
      {currentStep.documents && currentStep.documents.length > 1 ? (
        <div className="mb-4 flex gap-2">
          {currentStep.documents.map((doc) => (
            <button
              key={doc.key}
              onClick={() => setActiveDocKey(doc.key)}
              className={`rounded-lg px-3 py-2 text-sm ${currentDocKey === doc.key ? 'bg-accent text-white' : 'border border-line text-body'}`}
            >
              {pickText(doc, 'title', locale) || doc.key}
            </button>
          ))}
        </div>
      ) : null}
      <div className="space-y-4">
        {(currentStep.documents || [])
          .filter((doc) => doc.key === currentDocKey)
          .map((doc) => (
            <section key={doc.key} className="rounded-xl border border-line p-4">
              <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">{pickText(doc, 'title', locale) || doc.key}</h2>
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                {collectFlowFields(doc).map((field) => (
                  <FieldEditor
                    key={`${doc.key}-${field.key}`}
                    field={field}
                    locale={locale}
                    values={draft[doc.key]?.payload || {}}
                    onChange={(updater) =>
                      setDraft((current) => ({
                        ...current,
                        [doc.key]: {
                          payload: typeof updater === 'function'
                            ? (updater as (state: FormState) => FormState)(current[doc.key]?.payload || {})
                            : updater,
                        },
                      }))
                    }
                    model={false}
                    error={errors[validationFieldKey(doc.key, field.key)]}
                    onBlur={() =>
                      setErrors((current) => {
                        const message = validateFieldInput(field, resolvePath(draft[doc.key]?.payload || {}, normalizeFieldPath(field, false)), locale)
                        const key = validationFieldKey(doc.key, field.key)
                        if (!message && !current[key]) return current
                        const next = { ...current }
                        if (message) next[key] = message
                        else delete next[key]
                        return next
                      })
                    }
                  />
                ))}
              </div>
            </section>
          ))}
      </div>
      <div className="mt-6 flex gap-3">
        <button
          onClick={() => navigate(cancelTarget, { replace: true })}
          className="rounded-lg border border-line px-4 py-2 text-body"
        >
          Cancel
        </button>
        <button
          disabled={stepIndex === 0}
          onClick={() => setStepIndex((current) => Math.max(0, current - 1))}
          className="rounded-lg border border-line px-4 py-2 text-body disabled:opacity-50"
        >
          Previous
        </button>
        <button
          onClick={() => {
            if (!validateCurrentStep()) {
              addToast({ message: 'Please fix the highlighted fields.', variant: 'warning' })
              return
            }
            if (stepIndex >= steps.length - 1) {
              void handleCommit().catch((error) => addToast({ message: error instanceof Error ? error.message : 'Save failed', variant: 'error' }))
              return
            }
            setStepIndex((current) => Math.min(steps.length - 1, current + 1))
          }}
          className="rounded-lg bg-accent px-4 py-2 text-white"
        >
          {stepIndex >= steps.length - 1 ? (documentID ? 'Save' : 'Create') : 'Next'}
        </button>
      </div>
    </Panel>
  )
}

function CustomRouteView({
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
        setError(err instanceof Error ? err.message : 'Custom page failed to load')
      }
    }
    void load()
    return () => {
      mounted = false
    }
  }, [entry.bundle_key, entry.component_export, locale, route])

  return (
    <Panel title={pickText(entry, 'title', locale) || entry.key} status={error || undefined}>
      <div ref={mountRef} />
    </Panel>
  )
}

function NotificationsView() {
  const [items, setItems] = useState<Array<Record<string, unknown>>>([])

  useEffect(() => {
    let mounted = true
    async function load() {
      const payload = await fetchJSON<{ items: Array<Record<string, unknown>> }>('/ui/data/notifications')
      if (!mounted) return
      setItems(payload.items || [])
    }
    void load()
    return () => {
      mounted = false
    }
  }, [])

  return (
    <Panel title="Notifications" status="Workflow messages and communication history.">
      <DataTable
        columns={[
          { key: 'title', label: 'Message' },
          { key: 'status', label: 'Status' },
          { key: 'created_at', label: 'Created' },
        ]}
        rows={items}
        emptyText="No notifications yet."
      />
    </Panel>
  )
}

function DataTable({
  columns,
  rows,
  emptyText,
  renderAction,
}: {
  columns: Array<{ key: string; label: string }>
  rows: Array<Record<string, unknown>>
  emptyText: string
  renderAction?: (row: Record<string, unknown>) => React.ReactNode
}) {
  return (
    <div className="overflow-hidden rounded-xl border border-line">
      <table className="min-w-full divide-y divide-line">
        <thead className="border-b border-line bg-accent-soft dark:bg-ink/60">
          <tr>
            {columns.map((column) => (
              <th key={column.key} className="px-4 py-3 text-left text-xs font-bold uppercase tracking-[0.14em] text-accent-dark dark:text-body">
                {column.label}
              </th>
            ))}
            {renderAction ? <th className="px-4 py-3" /> : null}
          </tr>
        </thead>
        <tbody className="divide-y divide-line bg-surface">
          {rows.length ? rows.map((row, index) => (
            <tr key={index}>
              {columns.map((column) => (
                <td key={column.key} className="px-4 py-3 text-sm text-body">
                  {displayValue(resolvePath(row, column.key))}
                </td>
              ))}
              {renderAction ? <td className="px-4 py-3 text-right">{renderAction(row)}</td> : null}
            </tr>
          )) : (
            <tr>
              <td colSpan={columns.length + (renderAction ? 1 : 0)} className="px-4 py-10 text-center text-sm text-muted">
                {emptyText}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <article className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
      <div className="text-xs font-semibold uppercase tracking-wide text-body">{label}</div>
      <div className="mt-2 text-2xl font-bold text-body">{value}</div>
    </article>
  )
}

function FieldEditor({
  field,
  locale,
  values,
  onChange,
  model,
  error,
  onBlur,
}: {
  field: FieldDefinition
  locale: string
  values: FormState
  onChange: React.Dispatch<React.SetStateAction<FormState>>
  model: boolean
  error?: string
  onBlur?: () => void
}) {
  const normalizedPath = normalizeFieldPath(field, model)
  const value = resolvePath(values, normalizedPath)
  const label = pickText(field, 'label', locale) || field.key
  const inputClassName = `h-10 rounded-lg border bg-surface px-3 py-2 text-sm text-body ${error ? 'border-danger' : 'border-line'}`
  const textareaClassName = `min-h-28 rounded-lg border bg-surface px-3 py-2 text-sm text-body ${error ? 'border-danger' : 'border-line'}`

  function update(next: unknown) {
    onChange((current) => assignPathValue(current, normalizedPath, next))
  }

  return (
    <label className="flex flex-col gap-1">
      <span className="text-sm font-medium text-body">{label}</span>
      {field.widget === 'textarea' ? (
        <textarea
          id={`field-${field.key}`}
          name={field.key}
          value={String(value ?? '')}
          onChange={(e) => update(e.target.value)}
          onBlur={onBlur}
          required={field.required}
          minLength={field.min_length}
          maxLength={field.max_length}
          className={textareaClassName}
        />
      ) : field.widget === 'select' || field.options?.length ? (
        <select
          id={`field-${field.key}`}
          name={field.key}
          value={String(value ?? '')}
          onChange={(e) => update(e.target.value)}
          onBlur={onBlur}
          required={field.required}
          className={inputClassName}
        >
          <option value="">Select an option</option>
          {(field.options || []).map((option) => (
            <option key={option} value={option}>
              {humanize(option)}
            </option>
          ))}
        </select>
      ) : field.type === 'bool' ? (
        <input
          type="checkbox"
          id={`field-${field.key}`}
          name={field.key}
          checked={Boolean(value)}
          onChange={(e) => update(e.target.checked)}
          onBlur={onBlur}
          className="h-4 w-4"
        />
      ) : (
        <input
          type={field.type === 'int' || field.type === 'number' ? 'number' : 'text'}
          id={`field-${field.key}`}
          name={field.key}
          value={String(value ?? '')}
          onChange={(e) => update(field.type === 'int' || field.type === 'number' ? (e.target.value === '' ? '' : Number(e.target.value)) : e.target.value)}
          onBlur={onBlur}
          required={field.required}
          minLength={field.min_length}
          maxLength={field.max_length}
          pattern={field.pattern}
          min={field.min_value}
          max={field.max_value}
          className={inputClassName}
          placeholder={pickText(field, 'placeholder', locale)}
        />
      )}
      {error ? <span className="text-xs text-danger">{error}</span> : null}
      {field.help_text ? <span className="text-xs text-muted">{pickText(field, 'help_text', locale) || field.help_text}</span> : null}
    </label>
  )
}

async function fetchJSON<T>(url: string): Promise<T> {
  const response = await fetch(url, { credentials: 'include' })
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

function displayValue(value: unknown): string {
  if (value == null) return ''
  if (typeof value === 'boolean') return value ? 'Yes' : 'No'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function resolvePath(payload: unknown, path: string): unknown {
  if (!path || payload == null) return payload
  return path.split('.').reduce<unknown>((current, key) => {
    if (current && typeof current === 'object' && key in (current as Record<string, unknown>)) {
      return (current as Record<string, unknown>)[key]
    }
    return undefined
  }, payload)
}

function assignPathValue(current: FormState, path: string, value: unknown): FormState {
  const next = structuredClone(current)
  const parts = path.split('.')
  let target: Record<string, unknown> = next
  while (parts.length > 1) {
    const key = parts.shift()!
    target[key] = (target[key] as Record<string, unknown>) || {}
    target = target[key] as Record<string, unknown>
  }
  const finalKey = parts[0]
  if (finalKey) target[finalKey] = value
  return next
}

function deriveSectionsFromFields(fields: FieldDefinition[]): SectionDefinition[] {
  return fields.length ? [{ key: 'main', title: 'Details', fields }] : []
}

function resolveSections(view: Pick<ViewDefinition, 'sections' | 'tabs' | 'fields'>): SectionDefinition[] {
  if (view.sections?.length) return view.sections
  if (view.tabs?.length) {
    return view.tabs.flatMap((tab) =>
      (tab.sections || []).map((section) => ({
        ...section,
        key: `${tab.key}.${section.key}`,
        title: section.title || tab.title,
        title_i18n: section.title_i18n || tab.title_i18n,
      }))
    )
  }
  return deriveSectionsFromFields(view.fields || [])
}

function normalizeActionPath(path: string): string {
  return normalizeShellPath(path, 'workspace').replace(/\/details(?=\/|$)/, '/detail')
}

function routeForCreate(currentPath: string, actions: ActionDefinition[]): string {
  const normalizedCurrent = normalizeActionPath(currentPath)
  const basePath = stripEditorSuffix(normalizedCurrent)
  const createAction = actions.find((item) => normalizeActionPath(item.route_path) === `${basePath}/new`)
  if (createAction?.route_path) return normalizeActionPath(createAction.route_path)

  const fallback = actions.find((item) => item.render_mode === 'flow' && normalizeActionPath(item.route_path).endsWith('/new'))
  if (fallback?.route_path) return normalizeActionPath(fallback.route_path)
  return ''
}

function routeForDocument(_documentType: string, kind: 'detail' | 'form', actions: ActionDefinition[], currentPath = '/documents'): string {
  const normalizedCurrent = normalizeActionPath(currentPath)
  const basePath = stripEditorSuffix(normalizedCurrent)
  const suffix = kind === 'detail' ? '/detail' : '/form'
  const action = actions.find((item) => normalizeActionPath(item.route_path) === `${basePath}${suffix}`)
  if (action?.route_path) return normalizeActionPath(action.route_path)

  const fallback = actions.find((item) => {
    const path = normalizeActionPath(item.route_path)
    return item.render_mode === 'generic' && item.view_key && path.includes('/documents') && path.endsWith(suffix)
  })
  if (fallback?.route_path) return normalizeActionPath(fallback.route_path)
  return kind === 'detail' ? '/documents/detail' : '/documents/form'
}

function routeForEdit(currentPath: string, documentType: string, actions: ActionDefinition[]): string {
  return routeForDocument(documentType, 'form', actions, currentPath)
}

function routeForModel(modelKey: string, kind: 'detail' | 'form', actions: ActionDefinition[], currentPath = ''): string {
  const normalizedCurrent = normalizeActionPath(currentPath)
  const basePath = stripEditorSuffix(normalizedCurrent)
  const suffix = `/${kind}`

  if (basePath) {
    const exact = actions.find((item) => normalizeActionPath(item.route_path) === `${basePath}${suffix}`)
    if (exact?.route_path) return normalizeActionPath(exact.route_path)
  }

  const fallback = actions.find((item) => {
    const path = normalizeActionPath(item.route_path)
    return item.render_mode === 'generic' && item.view_key && path.endsWith(suffix) && (item.view_key.includes(modelKey) || item.key.includes(modelKey))
  })
  return fallback?.route_path ? normalizeActionPath(fallback.route_path) : ''
}

function routeForWorkItem(row: Record<string, unknown>, actions: ActionDefinition[]): string {
  const documentType = String(row.document_type || '')
  const targetID = String(row.target_id || '')
  const path = routeForDocument(documentType, 'detail', actions, '/documents')
  return path && targetID ? `${path}?id=${encodeURIComponent(targetID)}` : ''
}

async function invokeDocumentAction(documentID: string, action: string): Promise<void> {
  const response = await fetch(`/documents/${encodeURIComponent(documentID)}/actions`, {
    method: 'POST',
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': readCookie('orbyte_csrf'),
    },
    body: JSON.stringify({ action }),
  })
  if (!response.ok) throw await buildError(response)
}

function humanize(value: string): string {
  return value.replace(/[_./-]+/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase())
}

function readCookie(name: string): string {
  const cookie = document.cookie
    .split('; ')
    .find((part) => part.startsWith(`${name}=`))
  return cookie ? decodeURIComponent(cookie.split('=').slice(1).join('=')) : ''
}

function collectFlowFields(doc: { fields?: FieldDefinition[]; sections?: SectionDefinition[]; tabs?: Array<{ sections?: SectionDefinition[] }> }): FieldDefinition[] {
  const fields: FieldDefinition[] = [...(doc.fields || [])]
  for (const section of doc.sections || []) {
    fields.push(...(section.fields || []))
  }
  for (const tab of doc.tabs || []) {
    for (const section of tab.sections || []) {
      fields.push(...(section.fields || []))
    }
  }
  return fields
}

function validateFieldCollection(fields: FieldDefinition[], values: FormState, model: boolean, locale: string, scope = ''): ValidationErrors {
  const errors: ValidationErrors = {}
  for (const field of fields) {
    if (field.read_only) continue
    const message = validateFieldInput(field, resolvePath(values, normalizeFieldPath(field, model)), locale)
    if (message) {
      errors[validationFieldKey(scope, field.key)] = message
    }
  }
  return errors
}

function validateFieldInput(field: FieldDefinition, value: unknown, locale: string): string {
  const label = pickText(field, 'label', locale) || humanize(field.key)
  const asString = typeof value === 'string' ? value : value == null ? '' : String(value)
  const trimmed = asString.trim()
  const isEmpty = field.type === 'bool' ? value == null : trimmed === ''

  if (field.required && isEmpty) {
    return `${label} is required.`
  }
  if (isEmpty) {
    return ''
  }
  if (field.options?.length && !field.options.includes(asString)) {
    return `${label} must be one of: ${field.options.join(', ')}.`
  }
  if (field.min_length && trimmed.length < field.min_length) {
    return `${label} must be at least ${field.min_length} characters.`
  }
  if (field.max_length && trimmed.length > field.max_length) {
    return `${label} must be at most ${field.max_length} characters.`
  }
  if (field.pattern) {
    try {
      const expression = new RegExp(field.pattern)
      if (!expression.test(asString)) {
        return `${label} has an invalid format.`
      }
    } catch {
      return `${label} has an invalid format.`
    }
  }
  if (field.type === 'int' || field.type === 'number' || field.min_value != null || field.max_value != null) {
    const numericValue = typeof value === 'number' ? value : Number(asString)
    if (Number.isNaN(numericValue)) {
      return `${label} must be a number.`
    }
    if (field.min_value != null && numericValue < field.min_value) {
      return `${label} must be at least ${field.min_value}.`
    }
    if (field.max_value != null && numericValue > field.max_value) {
      return `${label} must be at most ${field.max_value}.`
    }
  }
  return ''
}

function validationFieldKey(scope: string, fieldKey: string): string {
  return scope ? `${scope}:${fieldKey}` : fieldKey
}

function normalizeFieldPath(field: FieldDefinition, model: boolean): string {
  return model ? field.path.replace(/^values\./, '') : field.path.replace(/^body\.payload\./, '')
}

function stripEditorSuffix(path: string): string {
  return path.replace(/\/(details|detail|form|new)$/, '')
}

function normalizeLegacyWorkspacePath(path: string): string {
  return path.replace(/\/details(?=\/|$)/g, '/detail')
}

function resolveFlowSequence(flow: DocumentFlowDefinition, draft: Record<string, { payload: FormState }>) {
  const steps = flow.steps || []
  const map = new Map(steps.map((step) => [step.key, step]))
  const sequence: typeof steps = []
  const seen = new Set<string>()
  let current = steps[0]
  while (current && !seen.has(current.key)) {
    seen.add(current.key)
    sequence.push(current)
    let nextKey = current.next_step_key || ''
    for (const rule of current.next_rules || []) {
      const value = resolveFlowRuleValue(draft, rule.path)
      if (rule.truthy && value) {
        nextKey = rule.next_step_key
        break
      }
      if (rule.equals !== undefined && String(value ?? '') === String(rule.equals)) {
        nextKey = rule.next_step_key
        break
      }
      if (rule.in?.length && rule.in.includes(String(value ?? ''))) {
        nextKey = rule.next_step_key
        break
      }
    }
    current = nextKey ? map.get(nextKey) : undefined
  }
  return sequence
}

function resolveFlowRuleValue(draft: Record<string, { payload: FormState }>, path: string): unknown {
  const trimmed = path.trim()
  const documentMatch = trimmed.match(/^documents\.([^.]+)\.payload\.(.+)$/)
  if (documentMatch) {
    const docKey = documentMatch[1] || ''
    const payloadPath = documentMatch[2] || ''
    return resolvePath(draft[docKey]?.payload, payloadPath)
  }
  const rawPath = trimmed.replace(/^body\.payload\./, '').replace(/^payload\./, '')
  const docMatch = Object.values(draft).find((item) => resolvePath(item.payload, rawPath) !== undefined)
  return docMatch ? resolvePath(docMatch.payload, rawPath) : undefined
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
