import { type ReactNode, useEffect, useMemo, useRef, useState } from 'react'
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
type CommercialFormCatalog = {
  partiesByID: Record<string, Record<string, unknown>>
  invoicesByID: Record<string, Record<string, unknown>>
  paymentsByID: Record<string, Record<string, unknown>>
  itemsByCode: Record<string, Record<string, unknown>>
  itemCategoriesByCode: Record<string, Record<string, unknown>>
  uomsByCode: Record<string, Record<string, unknown>>
  taxCodesByCode: Record<string, Record<string, unknown>>
  taxProfilesByCode: Record<string, Record<string, unknown>>
  priceListsByCode: Record<string, Record<string, unknown>>
  priceListItemsByKey: Record<string, Record<string, unknown>>
  paymentMethodsByCode: Record<string, Record<string, unknown>>
}

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
      if (!view.model_key && documentListNeedsPayload(view)) {
        query.set('include_payload', '1')
      }
      const queryKeys = new Set(['name', 'sort', 'page', 'page_size'])
      for (const filter of view.filters || []) {
        queryKeys.add(filter.key)
      }
      for (const key of queryKeys) {
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
  const totalItems = payload?.total ?? payload?.items?.length ?? 0
  const listStatus = totalItems > 0
    ? `Showing ${totalItems} item${totalItems === 1 ? '' : 's'}.`
    : (pickText(view, 'empty_state', locale) || 'Standard list rendered from UI contracts.')

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
    <Panel title={pickText(view, 'title', locale) || 'List'} status={listStatus}>
      <div className="mb-4 space-y-4">
        <div className="flex items-center justify-between">
          <div className="text-sm text-muted">Items {totalItems}</div>
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
  const [commercialSummary, setCommercialSummary] = useState<Record<string, unknown> | null>(null)
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
      if (view.model_key === 'party') {
        try {
          const summary = await fetchJSON<Record<string, unknown>>(`/ui/data/commercial/parties/${encodeURIComponent(documentID)}/summary`)
          if (!mounted) return
          setCommercialSummary(summary)
        } catch {
          if (!mounted) return
          setCommercialSummary(null)
        }
      } else {
        setCommercialSummary(null)
      }
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
  const visibleActions = (view.allowed_actions || []).filter((actionKey) =>
    actionVisibleForStatus(actionKey, String(header.status || ''), String(header.type || view.document_type || '')),
  )
  const hasDocumentCancelAction = !!visibleActions.some((actionKey) => actionKey.toLowerCase() === 'cancel')
  const canEdit = !!editTarget && !isCommercialDocumentLocked(String(header.type || ''), String(header.status || ''))

  async function handleAction(actionKey: string) {
    if (String(header.type || '') === 'sales_order' && actionKey === 'generate_invoice') {
      try {
        const created = await invokeCommercialAction(`/commercial/orders/${encodeURIComponent(String(header.id || ''))}/generate-invoice`)
        onToast('Invoice generated.', 'success')
        const target = routeForDocument('invoice', 'detail', routeActions, '/commercial/invoices')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Invoice generation failed', 'error')
      }
      return
    }
    if (String(header.type || '') === 'invoice' && actionKey === 'register_payment') {
      try {
        const created = await invokeCommercialAction(`/commercial/invoices/${encodeURIComponent(String(header.id || ''))}/register-payment`)
        onToast('Payment draft generated.', 'success')
        const target = routeForDocument('payment_receipt', 'detail', routeActions, '/commercial/payments')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Payment registration failed', 'error')
      }
      return
    }
    if (String(header.type || '') === 'invoice' && actionKey === 'issue_credit_note') {
      try {
        const created = await invokeCommercialAction(`/commercial/invoices/${encodeURIComponent(String(header.id || ''))}/issue-credit-note`)
        onToast('Credit note draft generated.', 'success')
        const target = routeForDocument('credit_note', 'detail', routeActions, '/commercial/credit-notes')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Credit note generation failed', 'error')
      }
      return
    }
    if (String(header.type || '') === 'credit_note' && actionKey === 'register_refund') {
      try {
        const created = await invokeCommercialAction(`/commercial/credit-notes/${encodeURIComponent(String(header.id || ''))}/register-refund`)
        onToast('Refund draft generated.', 'success')
        const target = routeForDocument('payment_refund', 'detail', routeActions, '/commercial/refunds')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Refund generation failed', 'error')
      }
      return
    }
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
        {visibleActions.map((actionKey) => (
          <button
            key={actionKey}
            onClick={() => void handleAction(actionKey)}
            className="rounded-lg border border-line px-4 py-2 text-sm text-body"
          >
            {humanize(actionKey)}
          </button>
        ))}
        {canEdit ? (
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
                  <div className="mt-1 text-sm text-body">{renderDetailFieldValue(field, resolvePath(record, field.path))}</div>
                </article>
              ))}
            </div>
          </section>
        ))}
        {view.model_key === 'party' && commercialSummary ? (
          <PartyCommercialSummaryPanel summary={commercialSummary} routeActions={routeActions} onNavigate={onNavigate} />
        ) : null}
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
      const search = new URLSearchParams(window.location.search)
      if (view.projection_key === 'commercial.party_statement' && !search.get('party_id')) {
        if (!mounted) return
        setPayload(null)
        return
      }
      const target = view.dataset_key
        ? `/ui/data/reporting/datasets/${encodeURIComponent(view.dataset_key)}`
        : view.projection_key === 'commercial.receivables.summary'
          ? '/ui/data/commercial/receivables/summary'
        : view.projection_key === 'commercial.party_statement'
          ? `/ui/data/commercial/parties/${encodeURIComponent(search.get('party_id') || '')}/summary${buildStatementQuery(search)}`
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

  const statementSearch = new URLSearchParams(window.location.search)
  const partyID = statementSearch.get('party_id') || ''
  const receivablesItems = view.projection_key === 'commercial.receivables.summary'
    ? asRecordList(resolvePath(payload, 'items'))
    : []

  return (
    <Panel title={pickText(view, 'title', locale) || 'Dashboard'}>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        {(view.cards || []).map((card) => (
          <button
            key={card.key}
            onClick={() => {
              const target = dashboardCardTarget(view, card.key, routeActions)
              if (target) onNavigate(target)
            }}
            className="rounded-xl border border-line bg-surface p-5 text-left dark:bg-ink/60"
          >
            <div className="text-xs uppercase tracking-wide text-muted">{pickText(card, 'label', locale) || card.key}</div>
            <div className="mt-2 text-2xl font-bold text-body">{displayValue(resolvePath(payload, card.path))}</div>
          </button>
        ))}
      </div>
      {receivablesItems.length ? (
        <section className="mt-6 rounded-xl border border-line p-4">
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Receivables Aging</h2>
          <DataTable
            columns={[
              { key: 'number', label: 'Invoice' },
              { key: 'party_name', label: 'Customer' },
              { key: 'due_date', label: 'Due Date' },
              { key: 'paid_amount', label: 'Paid' },
              { key: 'credited', label: 'Credited' },
              { key: 'refunded', label: 'Refunded' },
              { key: 'balance_due', label: 'Open Balance' },
              { key: 'aging_bucket', label: 'Aging' },
            ]}
            rows={receivablesItems}
            emptyText="No receivables."
            renderAction={(row) => {
              const detailPath = routeForDocument('invoice', 'detail', routeActions, '/commercial/invoices')
              const documentID = String(row.id || '')
              if (!detailPath || !documentID) return null
              return (
                <button onClick={() => onNavigate(`${detailPath}?id=${encodeURIComponent(documentID)}`)} className="rounded-lg border border-line px-3 py-1.5 text-sm text-body">
                  Open
                </button>
              )
            }}
          />
        </section>
      ) : null}
      {view.projection_key === 'commercial.party_statement' ? (
        partyID ? (
          <section className="mt-6">
            <PartyCommercialSummaryPanel summary={payload || {}} routeActions={routeActions} onNavigate={onNavigate} />
          </section>
        ) : (
          <section className="mt-6 rounded-xl border border-line p-4 text-sm text-muted">
            Select a customer from party detail to open a statement.
          </section>
        )
      ) : null}
    </Panel>
  )
}

function dashboardCardTarget(view: ViewDefinition, cardKey: string, routeActions: ActionDefinition[]): string {
  const action = routeActions.find((item) => item.key === view.cards?.find((card) => card.key === cardKey)?.action_key)
  const basePath = action?.route_path
  if (!basePath) return ''
  if (view.projection_key !== 'commercial.receivables.summary') {
    return basePath
  }
  switch (cardKey) {
    case 'open_invoice_count':
    case 'open_balance_total':
      return `${basePath}?receivable_state=open`
    case 'overdue_invoice_count':
    case 'overdue_balance_total':
      return `${basePath}?receivable_state=overdue`
    case 'due_today_total':
      return `${basePath}?receivable_state=due_today`
    case 'current_balance_total':
      return `${basePath}?receivable_state=current`
    case 'paid_amount_total':
      return `${basePath}?status=received`
    case 'refunded_amount_total': {
      const separator = basePath.includes('?') ? '&' : '?'
      return `${basePath}${separator}status=refunded`
    }
    default:
      return basePath
  }
}

function buildStatementQuery(search: URLSearchParams): string {
  const query = new URLSearchParams()
  const from = search.get('from')
  const to = search.get('to')
  if (from) query.set('from', from)
  if (to) query.set('to', to)
  const encoded = query.toString()
  return encoded ? `?${encoded}` : ''
}

function PartyCommercialSummaryPanel({
  summary,
  routeActions,
  onNavigate,
}: {
  summary: Record<string, unknown>
  routeActions: ActionDefinition[]
  onNavigate: (target: string) => void
}) {
  const openInvoices = asRecordList(resolvePath(summary, 'open_invoices'))
  const activities = asRecordList(resolvePath(summary, 'activities')).slice(0, 10)
  const invoiceDetailPath = routeForDocument('invoice', 'detail', routeActions, '/commercial/invoices')
  const statementPath = routeActions.find((action) => action.key === 'commercial.party_statement.dashboard')?.route_path || ''
  const partyID = String(resolvePath(summary, 'party_id') || '')

  return (
    <section className="space-y-4 rounded-xl border border-line p-4">
      <div className="flex items-center justify-between gap-3">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-muted">Commercial Summary</h2>
        {statementPath && partyID ? (
          <button onClick={() => onNavigate(`${statementPath}?party_id=${encodeURIComponent(partyID)}`)} className="rounded-lg border border-line px-3 py-2 text-sm text-body">
            Open Statement
          </button>
        ) : null}
      </div>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
        <MetricCard label="Open Invoices" value={displayValue(resolvePath(summary, 'open_invoice_count'))} />
        <MetricCard label="Open Balance" value={displayValue(resolvePath(summary, 'open_balance_total'))} />
        <MetricCard label="Collected" value={displayValue(resolvePath(summary, 'paid_amount_total'))} />
        <MetricCard label="Refunded" value={displayValue(resolvePath(summary, 'refunded_amount_total'))} />
      </div>
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <section>
          <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Open Invoices</h3>
          <DataTable
            columns={[
              { key: 'number', label: 'Invoice' },
              { key: 'due_date', label: 'Due Date' },
              { key: 'paid_amount', label: 'Paid' },
              { key: 'credited', label: 'Credited' },
              { key: 'refunded', label: 'Refunded' },
              { key: 'balance_due', label: 'Open Balance' },
            ]}
            rows={openInvoices}
            emptyText="No open invoices."
            renderAction={(row) => {
              const documentID = String(row.id || '')
              if (!invoiceDetailPath || !documentID) return null
              return (
                <button onClick={() => onNavigate(`${invoiceDetailPath}?id=${encodeURIComponent(documentID)}`)} className="rounded-lg border border-line px-3 py-1.5 text-sm text-body">
                  Open
                </button>
              )
            }}
          />
        </section>
        <section>
          <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Recent Activity</h3>
          <DataTable
            columns={[
              { key: 'date', label: 'Date' },
              { key: 'type', label: 'Type' },
              { key: 'number', label: 'Number' },
              { key: 'counter', label: 'Counterparty Doc' },
              { key: 'amount', label: 'Amount' },
              { key: 'status', label: 'Status' },
            ]}
            rows={activities}
            emptyText="No commercial activity yet."
          />
        </section>
      </div>
    </section>
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
  const [recordStatus, setRecordStatus] = useState('')
  const [errors, setErrors] = useState<ValidationErrors>({})
  const [catalog, setCatalog] = useState<CommercialFormCatalog>({ partiesByID: {}, invoicesByID: {}, paymentsByID: {}, itemsByCode: {}, itemCategoriesByCode: {}, uomsByCode: {}, taxCodesByCode: {}, taxProfilesByCode: {}, priceListsByCode: {}, priceListItemsByKey: {}, paymentMethodsByCode: {} })

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
        setRecordStatus('')
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
        setRecordStatus(String(header.status || ''))
        setErrors({})
      }
    }
    void load()
    return () => {
      mounted = false
    }
  }, [targetID, view.model_key])

  useEffect(() => {
    let mounted = true
    async function loadCatalog() {
      const documentType = String(view.document_type || '')
      const modelKey = String(view.model_key || '')
      const needsCatalog =
        ['sales_order', 'invoice', 'credit_note', 'payment_receipt', 'payment_refund'].includes(documentType) ||
        ['party', 'commercial_item', 'commercial_price_list_item'].includes(modelKey)
      if (!needsCatalog) {
        if (!mounted) return
        setCatalog({ partiesByID: {}, invoicesByID: {}, paymentsByID: {}, itemsByCode: {}, itemCategoriesByCode: {}, uomsByCode: {}, taxCodesByCode: {}, taxProfilesByCode: {}, priceListsByCode: {}, priceListItemsByKey: {}, paymentMethodsByCode: {} })
        return
      }
      try {
        const [partiesPayload, invoicesPayload, paymentsPayload, itemsPayload, categoriesPayload, uomsPayload, taxPayload, taxProfilesPayload, priceListsPayload, priceListItemsPayload, paymentPayload] = await Promise.all([
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/party'),
          fetchJSON<{ items: Array<Record<string, unknown>>; total?: number }>('/ui/data/documents?type=invoice&page_size=200&include_payload=1'),
          fetchJSON<{ items: Array<Record<string, unknown>>; total?: number }>('/ui/data/documents?type=payment_receipt&page_size=200&include_payload=1'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/commercial_item'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/commercial_item_category'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/commercial_uom'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/commercial_tax_code'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/commercial_tax_profile'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/commercial_price_list'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/commercial_price_list_item'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/payment_method'),
        ])
        const openInvoiceIDs = (invoicesPayload.items || [])
          .filter((item) => {
            const status = String(resolvePath(item, 'header.status') || '')
            return status === 'issued' || status === 'partially_paid'
          })
          .map((item) => String(resolvePath(item, 'header.id') || ''))
          .filter(Boolean)
        const invoiceDetails = await Promise.all(
          openInvoiceIDs.map(async (id) => {
            try {
              return await fetchJSON<Record<string, unknown>>(`/ui/data/documents/${encodeURIComponent(id)}`)
            } catch {
              return null
            }
          }),
        )
        const invoicesByID = Object.fromEntries(
          invoiceDetails
            .map((detail) => (detail?.record || detail) as Record<string, unknown> | null)
            .filter((detail): detail is Record<string, unknown> => !!detail)
            .map((detail) => [String(resolvePath(detail, 'header.id') || ''), detail]),
        )
        const paymentIDs = (paymentsPayload.items || [])
          .filter((item) => String(resolvePath(item, 'header.status') || '') === 'received')
          .map((item) => String(resolvePath(item, 'header.id') || ''))
          .filter(Boolean)
        const paymentDetails = await Promise.all(
          paymentIDs.map(async (id) => {
            try {
              return await fetchJSON<Record<string, unknown>>(`/ui/data/documents/${encodeURIComponent(id)}`)
            } catch {
              return null
            }
          }),
        )
        const paymentsByID = Object.fromEntries(
          paymentDetails
            .map((detail) => (detail?.record || detail) as Record<string, unknown> | null)
            .filter((detail): detail is Record<string, unknown> => !!detail)
            .map((detail) => [String(resolvePath(detail, 'header.id') || ''), detail]),
        )
        if (!mounted) return
        setCatalog({
          partiesByID: Object.fromEntries((partiesPayload.items || []).map((item) => [String(item.id || ''), item])),
          invoicesByID,
          paymentsByID,
          itemsByCode: Object.fromEntries((itemsPayload.items || []).map((item) => [String(resolvePath(item, 'values.sku') || ''), item])),
          itemCategoriesByCode: Object.fromEntries((categoriesPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          uomsByCode: Object.fromEntries((uomsPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          taxCodesByCode: Object.fromEntries((taxPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          taxProfilesByCode: Object.fromEntries((taxProfilesPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          priceListsByCode: Object.fromEntries((priceListsPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          priceListItemsByKey: Object.fromEntries((priceListItemsPayload.items || []).map((item) => [`${String(resolvePath(item, 'values.price_list_code') || '')}|${String(resolvePath(item, 'values.item_code') || '')}`, item])),
          paymentMethodsByCode: Object.fromEntries((paymentPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
        })
        setValues((current) => normalizeCommercialFormState(current, String(view.document_type || ''), {
          partiesByID: Object.fromEntries((partiesPayload.items || []).map((item) => [String(item.id || ''), item])),
          invoicesByID,
          paymentsByID,
          itemsByCode: Object.fromEntries((itemsPayload.items || []).map((item) => [String(resolvePath(item, 'values.sku') || ''), item])),
          itemCategoriesByCode: Object.fromEntries((categoriesPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          uomsByCode: Object.fromEntries((uomsPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          taxCodesByCode: Object.fromEntries((taxPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          taxProfilesByCode: Object.fromEntries((taxProfilesPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          priceListsByCode: Object.fromEntries((priceListsPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          priceListItemsByKey: Object.fromEntries((priceListItemsPayload.items || []).map((item) => [`${String(resolvePath(item, 'values.price_list_code') || '')}|${String(resolvePath(item, 'values.item_code') || '')}`, item])),
          paymentMethodsByCode: Object.fromEntries((paymentPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
        }))
      } catch {
        if (!mounted) return
        setCatalog({ partiesByID: {}, invoicesByID: {}, paymentsByID: {}, itemsByCode: {}, itemCategoriesByCode: {}, uomsByCode: {}, taxCodesByCode: {}, taxProfilesByCode: {}, priceListsByCode: {}, priceListItemsByKey: {}, paymentMethodsByCode: {} })
      }
    }
    void loadCatalog()
    return () => {
      mounted = false
    }
  }, [view.document_type, view.model_key])

  const sections = resolveSections(view)
  const validationFields = sections.flatMap((section) => section.fields || [])
  const cancelTarget = targetID
    ? (view.model_key
        ? routeForModel(view.model_key, 'detail', useShellStore.getState().actions, currentPath)
        : routeForDocument(view.document_type || '', 'detail', useShellStore.getState().actions, currentPath))
    : stripEditorSuffix(currentPath) || '/'
  const formLocked = !view.model_key && targetID && isCommercialDocumentLocked(String(view.document_type || ''), recordStatus)

  if (formLocked) {
    return (
      <Panel title={pickText(view, 'title', locale) || 'Editor'} status={`Editing is unavailable while this record is ${recordStatus || 'locked'}.`}>
        <div className="mt-2 flex gap-3">
          <button
            onClick={() => onNavigate(cancelTarget ? `${cancelTarget}?id=${encodeURIComponent(targetID)}` : stripEditorSuffix(currentPath) || '/')}
            className="rounded-lg border border-line px-4 py-2 text-body"
          >
            Back to detail
          </button>
        </div>
      </Panel>
    )
  }

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
                  catalog={catalog}
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

function documentListNeedsPayload(view: ViewDefinition): boolean {
  return (view.columns || []).some((column) => {
    const path = String(column.path || '')
    return path.startsWith('body.') || path.startsWith('lines') || path.startsWith('links') || path.startsWith('attachments') || path === 'header.number'
  })
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
                    catalog={{ partiesByID: {}, invoicesByID: {}, paymentsByID: {}, itemsByCode: {}, itemCategoriesByCode: {}, uomsByCode: {}, taxCodesByCode: {}, taxProfilesByCode: {}, priceListsByCode: {}, priceListItemsByKey: {}, paymentMethodsByCode: {} }}
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
  catalog,
  error,
  onBlur,
}: {
  field: FieldDefinition
  locale: string
  values: FormState
  onChange: React.Dispatch<React.SetStateAction<FormState>>
  model: boolean
  catalog: CommercialFormCatalog
  error?: string
  onBlur?: () => void
}) {
  const normalizedPath = normalizeFieldPath(field, model)
  const value = resolvePath(values, normalizedPath)
  const label = pickText(field, 'label', locale) || field.key
  const placeholder = pickText(field, 'placeholder', locale)
  const inputClassName = `h-10 rounded-lg border bg-surface px-3 py-2 text-sm text-body ${error ? 'border-danger' : 'border-line'}`
  const textareaClassName = `min-h-28 rounded-lg border bg-surface px-3 py-2 text-sm text-body ${error ? 'border-danger' : 'border-line'}`
  const catalogOptions = commercialSelectOptions(normalizedPath, catalog)

  function update(next: unknown) {
    onChange((current) => applyFieldUpdate(current, normalizedPath, next, catalog))
  }

  if (field.widget === 'commercial_lines' || field.widget === 'commercial_allocations' || field.widget === 'commercial_refund_allocations' || field.widget === 'commercial_journal_lines') {
    return (
      <div className="flex flex-col gap-1">
        <span className="text-sm font-medium text-body">{label}</span>
        <CommercialArrayFieldEditor
          fieldKey={field.key}
          widget={field.widget}
          value={value}
          values={values}
          catalog={catalog}
          onChange={(rows, patch) =>
            onChange((current) => {
              let next = applyCommercialArrayUpdate(current, normalizedPath, field.widget || '', rows, catalog)
              if ((field.widget === 'commercial_allocations' || field.widget === 'commercial_refund_allocations') && patch) {
                if (patch.amount_received != null && (patch.replace_amount_received || toNumber(resolvePath(current, 'amount_received')) <= 0)) {
                  next = assignPathValue(next, 'amount_received', patch.amount_received)
                }
                if (patch.amount_refunded != null && (patch.replace_amount_refunded || toNumber(resolvePath(current, 'amount_refunded')) <= 0)) {
                  next = assignPathValue(next, 'amount_refunded', patch.amount_refunded)
                }
                if (patch.payment_reference && !String(resolvePath(current, 'payment_reference') || '')) {
                  next = assignPathValue(next, 'payment_reference', patch.payment_reference)
                }
                if (patch.refund_reference && !String(resolvePath(current, 'refund_reference') || '')) {
                  next = assignPathValue(next, 'refund_reference', patch.refund_reference)
                }
                if (patch.party_id && !String(resolvePath(current, 'party_id') || '')) {
                  next = applyFieldUpdate(next, 'party_id', patch.party_id, catalog)
                }
                if (patch.party_name && !String(resolvePath(current, 'party_name') || '')) {
                  next = assignPathValue(next, 'party_name', patch.party_name)
                }
                if (patch.currency_code && !String(resolvePath(current, 'currency_code') || '')) {
                  next = assignPathValue(next, 'currency_code', patch.currency_code)
                }
              }
              return next
            })
          }
        />
        {error ? <span className="text-xs text-danger">{error}</span> : null}
        {field.help_text ? <span className="text-xs text-muted">{pickText(field, 'help_text', locale) || field.help_text}</span> : null}
      </div>
    )
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
          placeholder={placeholder}
          required={field.required}
          minLength={field.min_length}
          maxLength={field.max_length}
          className={textareaClassName}
        />
      ) : field.widget === 'select' || field.options?.length || catalogOptions.length ? (
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
          {(catalogOptions.length ? catalogOptions : (field.options || []).map((option) => ({ value: option, label: humanize(option) }))).map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
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
          placeholder={placeholder}
        />
      )}
      {error ? <span className="text-xs text-danger">{error}</span> : null}
      {field.help_text ? <span className="text-xs text-muted">{pickText(field, 'help_text', locale) || field.help_text}</span> : null}
    </label>
  )
}

function CommercialArrayFieldEditor({
  fieldKey,
  widget,
  value,
  values,
  catalog,
  onChange,
}: {
  fieldKey: string
  widget: string
  value: unknown
  values: FormState
  catalog: CommercialFormCatalog
  onChange: (rows: Array<Record<string, unknown>>, patch?: Record<string, unknown>) => void
}) {
  const rows = asRecordList(value)
  const columns = commercialArrayColumns(widget, catalog, values)
  const openInvoices = widget === 'commercial_allocations' ? commercialOpenInvoices(catalog, values) : []
  const openInvoiceBalance = openInvoices.reduce((sum, invoice) => sum + toNumber(resolvePath(invoice, 'body.payload.balance_due_amount')), 0)
  const refundablePayments = widget === 'commercial_refund_allocations' ? commercialRefundablePayments(catalog, values) : []
  const refundablePaymentBalance = refundablePayments.reduce((sum, payment) => {
    const amount = toNumber(resolvePath(payment, 'body.payload.amount_received'))
    const refunded = toNumber(resolvePath(payment, 'body.payload.refunded_amount'))
    return sum + roundMoney(Math.max(amount - refunded, 0))
  }, 0)

  function updateRow(index: number, key: string, nextValue: unknown) {
    let patch: Record<string, unknown> | undefined
    const nextRows = rows.map((row, rowIndex) => {
      if (rowIndex !== index) return row
      const updated = { ...row, [key]: nextValue }
      if (widget === 'commercial_allocations' && key === 'invoice_id') {
        const invoice = catalog.invoicesByID[String(nextValue || '')]
        const invoiceNumber = String(resolvePath(invoice, 'header.number') || '')
        const openAmount = toNumber(resolvePath(invoice, 'body.payload.balance_due_amount'))
        const partyID = String(resolvePath(invoice, 'body.payload.party_id') || '')
        const partyName = String(resolvePath(invoice, 'body.payload.party_name') || '')
        const currencyCode = String(resolvePath(invoice, 'body.payload.currency_code') || '')
        if (invoiceNumber) updated.invoice_number = invoiceNumber
        if (!toNumber(updated.amount) && openAmount > 0) updated.amount = openAmount
        patch = {
          amount_received: openAmount > 0 ? openAmount : undefined,
          payment_reference: invoiceNumber || undefined,
          party_id: partyID || undefined,
          party_name: partyName || undefined,
          currency_code: currencyCode || undefined,
        }
      }
      if (widget === 'commercial_refund_allocations' && key === 'payment_id') {
        const payment = catalog.paymentsByID[String(nextValue || '')]
        const paymentNumber = String(resolvePath(payment, 'header.number') || '')
        const methodCode = String(resolvePath(payment, 'body.payload.payment_method_code') || '')
        const clearingAccount = String(resolvePath(payment, 'body.payload.clearing_account_code') || '')
        const paidAmount = toNumber(resolvePath(payment, 'body.payload.amount_received'))
        const refundedAmount = toNumber(resolvePath(payment, 'body.payload.refunded_amount'))
        const remainingAmount = roundMoney(Math.max(paidAmount - refundedAmount, 0))
        if (paymentNumber) updated.payment_number = paymentNumber
        if (!toNumber(updated.amount) && remainingAmount > 0) updated.amount = remainingAmount
        patch = {
          amount_refunded: remainingAmount > 0 ? remainingAmount : undefined,
          refund_reference: paymentNumber || undefined,
          payment_method_code: methodCode || undefined,
          clearing_account_code: clearingAccount || undefined,
        }
      }
      return updated
    })
    onChange(normalizeCommercialRows(nextRows, widget, catalog), patch)
  }

  function addRow() {
    onChange(normalizeCommercialRows([...rows, commercialArrayDefaultRow(widget)], widget, catalog))
  }

  function removeRow(index: number) {
    onChange(normalizeCommercialRows(rows.filter((_, rowIndex) => rowIndex !== index), widget, catalog))
  }

  function autoAllocate(useFullOpenBalance: boolean) {
    if (widget !== 'commercial_allocations') return
    const targetAmount = useFullOpenBalance ? openInvoiceBalance : toNumber(resolvePath(values, 'amount_received'))
    const { rows: nextRows, allocatedAmount } = buildAllocationRows(openInvoices, targetAmount)
    onChange(normalizeCommercialRows(nextRows, widget, catalog), {
      amount_received: allocatedAmount,
      replace_amount_received: useFullOpenBalance || targetAmount <= 0,
    })
  }

  function autoAllocateRefund(useFullRefundableBalance: boolean) {
    if (widget !== 'commercial_refund_allocations') return
    const targetAmount = useFullRefundableBalance ? refundablePaymentBalance : toNumber(resolvePath(values, 'amount_refunded'))
    const { rows: nextRows, allocatedAmount } = buildRefundAllocationRows(refundablePayments, targetAmount)
    onChange(normalizeCommercialRows(nextRows, widget, catalog), {
      amount_refunded: allocatedAmount,
      replace_amount_refunded: useFullRefundableBalance || targetAmount <= 0,
    })
  }

  return (
    <div className="space-y-3 rounded-xl border border-line p-3">
      {widget === 'commercial_allocations' ? (
        <div className="flex flex-wrap items-center gap-3 rounded-lg border border-line bg-accent-soft/30 px-3 py-2 text-sm text-body">
          <span>
            Open invoices for payer: <strong>{openInvoices.length}</strong>
          </span>
          <span>
            Open balance: <strong>{roundMoney(openInvoiceBalance)}</strong>
          </span>
          <button type="button" onClick={() => autoAllocate(false)} className="rounded-lg border border-line px-3 py-2 text-body" disabled={!openInvoices.length}>
            Auto Allocate Receipt
          </button>
          <button type="button" onClick={() => autoAllocate(true)} className="rounded-lg border border-line px-3 py-2 text-body" disabled={!openInvoices.length}>
            Use Full Open Balance
          </button>
          <button type="button" onClick={() => onChange([])} className="rounded-lg border border-line px-3 py-2 text-body" disabled={!rows.length}>
            Clear Allocations
          </button>
        </div>
      ) : null}
      {widget === 'commercial_refund_allocations' ? (
        <div className="flex flex-wrap items-center gap-3 rounded-lg border border-line bg-accent-soft/30 px-3 py-2 text-sm text-body">
          <span>
            Refundable receipts: <strong>{refundablePayments.length}</strong>
          </span>
          <span>
            Refundable balance: <strong>{roundMoney(refundablePaymentBalance)}</strong>
          </span>
          <button type="button" onClick={() => autoAllocateRefund(false)} className="rounded-lg border border-line px-3 py-2 text-body" disabled={!refundablePayments.length}>
            Auto Allocate Refund
          </button>
          <button type="button" onClick={() => autoAllocateRefund(true)} className="rounded-lg border border-line px-3 py-2 text-body" disabled={!refundablePayments.length}>
            Use Full Refundable Balance
          </button>
          <button type="button" onClick={() => onChange([])} className="rounded-lg border border-line px-3 py-2 text-body" disabled={!rows.length}>
            Clear Refund Allocations
          </button>
        </div>
      ) : null}
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-line text-sm">
          <thead className="border-b border-line bg-accent-soft dark:bg-ink/60">
            <tr>
              {columns.map((column) => (
                <th key={column.key} className="px-3 py-2 text-left text-xs font-bold uppercase tracking-[0.14em] text-accent-dark dark:text-body">
                  {column.label}
                </th>
              ))}
              <th className="px-3 py-2" />
            </tr>
          </thead>
          <tbody className="divide-y divide-line bg-surface">
            {rows.length ? rows.map((row, index) => (
              <tr key={`${widget}-${index}`}>
                {columns.map((column) => (
                  <td key={column.key} className="px-3 py-2 align-top">
                    {column.readOnly ? (
                      <div className="h-10 rounded-lg border border-line bg-accent-soft/40 px-3 py-2 text-sm text-body">
                        {displayValue(row[column.key])}
                      </div>
                    ) : column.options?.length ? (
                      <select
                        id={`field-${fieldKey}-${index}-${column.key}`}
                        name={`${fieldKey}[${index}].${column.key}`}
                        className="h-10 w-full rounded-lg border border-line bg-surface px-3 py-2 text-sm text-body"
                        value={String(row[column.key] ?? '')}
                        onChange={(event) => updateRow(index, column.key, event.target.value)}
                      >
                        <option value="">Select an option</option>
                        {column.options.map((option) => (
                          <option key={option.value} value={option.value}>
                            {option.label}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <input
                        type={column.type === 'number' ? 'number' : 'text'}
                        id={`field-${fieldKey}-${index}-${column.key}`}
                        name={`${fieldKey}[${index}].${column.key}`}
                        className="h-10 w-full rounded-lg border border-line bg-surface px-3 py-2 text-sm text-body"
                        value={String(row[column.key] ?? '')}
                        onChange={(event) => updateRow(index, column.key, column.type === 'number' ? (event.target.value === '' ? '' : Number(event.target.value)) : event.target.value)}
                      />
                    )}
                  </td>
                ))}
                <td className="px-3 py-2 text-right">
                  <button type="button" onClick={() => removeRow(index)} className="rounded-lg border border-line px-3 py-2 text-body">
                    Remove
                  </button>
                </td>
              </tr>
            )) : (
              <tr>
                <td colSpan={columns.length + 1} className="px-3 py-6 text-center text-sm text-muted">
                  No rows yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <button type="button" onClick={addRow} className="rounded-lg border border-line px-4 py-2 text-body">
        Add Row
      </button>
    </div>
  )
}

function renderDetailFieldValue(field: FieldDefinition, value: unknown): ReactNode {
  if (field.widget === 'commercial_lines' || field.widget === 'commercial_allocations' || field.widget === 'commercial_refund_allocations' || field.widget === 'commercial_journal_lines') {
    const rows = asRecordList(value)
    const columns = commercialArrayColumns(field.widget)
    if (!rows.length) return <span className="text-muted">No rows.</span>
    return (
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-line text-sm">
          <thead className="border-b border-line bg-accent-soft dark:bg-ink/60">
            <tr>
              {columns.map((column) => (
                <th key={column.key} className="px-3 py-2 text-left text-xs font-bold uppercase tracking-[0.14em] text-accent-dark dark:text-body">
                  {column.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-line bg-surface">
            {rows.map((row, index) => (
              <tr key={`${field.key}-${index}`}>
                {columns.map((column) => (
                  <td key={column.key} className="px-3 py-2 align-top text-body">
                    {displayValue(row[column.key])}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )
  }
  return displayValue(value)
}

async function fetchJSON<T>(url: string): Promise<T> {
  const response = await fetch(url, { credentials: 'include' })
  if (!response.ok) throw await buildError(response)
  return response.json()
}

async function invokeCommercialAction(url: string): Promise<Record<string, unknown>> {
  const response = await fetch(url, {
    method: 'POST',
    credentials: 'include',
    headers: {
      'X-CSRF-Token': readCookie('orbyte_csrf'),
    },
  })
  if (!response.ok) throw await buildError(response)
  return response.json() as Promise<Record<string, unknown>>
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

function asRecordList(value: unknown): Array<Record<string, unknown>> {
  if (!Array.isArray(value)) return []
  return value.map((item) => (item && typeof item === 'object' ? { ...(item as Record<string, unknown>) } : {}))
}

function commercialArrayColumns(widget: string, catalog?: CommercialFormCatalog, values?: FormState): Array<{ key: string; label: string; type: 'text' | 'number'; readOnly?: boolean; options?: Array<{ value: string; label: string }> }> {
  switch (widget) {
    case 'commercial_lines':
      return [
        { key: 'item_code', label: 'Item', type: 'text', options: commercialSelectOptions('item_code', catalog) },
        { key: 'description', label: 'Description', type: 'text' },
        { key: 'uom_code', label: 'UOM', type: 'text', options: commercialSelectOptions('uom_code', catalog) },
        { key: 'quantity', label: 'Qty', type: 'number' },
        { key: 'unit_price', label: 'Unit Price', type: 'number' },
        { key: 'discount_amount', label: 'Discount', type: 'number' },
        { key: 'tax_code', label: 'Tax Code', type: 'text', options: commercialSelectOptions('tax_code', catalog) },
        { key: 'tax_rate', label: 'Tax Rate %', type: 'number' },
        { key: 'line_subtotal', label: 'Subtotal', type: 'number', readOnly: true },
        { key: 'tax_amount', label: 'Tax', type: 'number', readOnly: true },
        { key: 'line_total', label: 'Total', type: 'number', readOnly: true },
      ]
    case 'commercial_allocations':
      return [
        { key: 'invoice_number', label: 'Invoice', type: 'text', readOnly: true },
        { key: 'invoice_id', label: 'Invoice ID', type: 'text', options: commercialSelectOptions('invoice_id', catalog, values) },
        { key: 'amount', label: 'Amount', type: 'number' },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'commercial_refund_allocations':
      return [
        { key: 'payment_number', label: 'Receipt', type: 'text', readOnly: true },
        { key: 'payment_id', label: 'Receipt ID', type: 'text', options: commercialSelectOptions('source_payment_id', catalog, values) },
        { key: 'amount', label: 'Amount', type: 'number' },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'commercial_journal_lines':
      return [
        { key: 'account_code', label: 'Account', type: 'text' },
        { key: 'description', label: 'Description', type: 'text' },
        { key: 'debit', label: 'Debit', type: 'number' },
        { key: 'credit', label: 'Credit', type: 'number' },
      ]
    default:
      return []
  }
}

function commercialArrayDefaultRow(widget: string): Record<string, unknown> {
  switch (widget) {
    case 'commercial_lines':
      return { item_code: '', description: '', uom_code: '', quantity: 1, unit_price: 0, discount_amount: 0, tax_code: '', tax_rate: 0, line_subtotal: 0, tax_amount: 0, line_total: 0 }
    case 'commercial_allocations':
      return { invoice_number: '', invoice_id: '', amount: 0, note: '' }
    case 'commercial_refund_allocations':
      return { payment_number: '', payment_id: '', amount: 0, note: '' }
    case 'commercial_journal_lines':
      return { account_code: '', description: '', debit: 0, credit: 0 }
    default:
      return {}
  }
}

function commercialSelectOptions(path: string, catalog?: CommercialFormCatalog, values?: FormState): Array<{ value: string; label: string }> {
  if (!catalog) return []
  switch (path) {
    case 'party_id':
      return Object.entries(catalog.partiesByID)
        .filter(([value]) => value)
        .map(([value, item]) => ({
          value,
          label: String(resolvePath(item, 'values.display_name') || resolvePath(item, 'values.name') || value),
        }))
        .sort((left, right) => left.label.localeCompare(right.label))
    case 'tax_profile_code':
      return Object.values(catalog.taxProfilesByCode)
        .map((item) => ({
          value: String(resolvePath(item, 'values.code') || ''),
          label: `${String(resolvePath(item, 'values.code') || '')} - ${String(resolvePath(item, 'values.name') || resolvePath(item, 'values.title') || '')}`.trim(),
        }))
        .filter((option) => option.value)
        .sort((left, right) => left.label.localeCompare(right.label))
    case 'category_code':
      return Object.values(catalog.itemCategoriesByCode)
        .map((item) => {
          const code = String(resolvePath(item, 'values.code') || '')
          const name = String(resolvePath(item, 'values.name') || '')
          return { value: code, label: name ? `${code} - ${name}` : code }
        })
        .filter((option) => option.value)
        .sort((left, right) => left.label.localeCompare(right.label))
    case 'uom_code':
      return Object.values(catalog.uomsByCode)
        .map((item) => {
          const code = String(resolvePath(item, 'values.code') || '')
          const name = String(resolvePath(item, 'values.name') || '')
          const symbol = String(resolvePath(item, 'values.symbol') || '')
          return { value: code, label: [code, name, symbol].filter(Boolean).join(' - ') }
        })
        .filter((option) => option.value)
        .sort((left, right) => left.label.localeCompare(right.label))
    case 'default_price_list_code':
    case 'price_list_code':
      return Object.values(catalog.priceListsByCode)
        .map((item) => {
          const code = String(resolvePath(item, 'values.code') || '')
          const name = String(resolvePath(item, 'values.name') || '')
          return { value: code, label: name ? `${code} - ${name}` : code }
        })
        .filter((option) => option.value)
        .sort((left, right) => left.label.localeCompare(right.label))
    case 'default_tax_code':
    case 'tax_code':
      return Object.values(catalog.taxCodesByCode)
        .map((item) => {
          const code = String(resolvePath(item, 'values.code') || '')
          const name = String(resolvePath(item, 'values.name') || '')
          return { value: code, label: name ? `${code} - ${name}` : code }
        })
        .filter((option) => option.value)
        .sort((left, right) => left.label.localeCompare(right.label))
    case 'payment_method_code':
      return Object.values(catalog.paymentMethodsByCode)
        .map((item) => {
          const code = String(resolvePath(item, 'values.code') || '')
          const name = String(resolvePath(item, 'values.name') || '')
          return { value: code, label: name ? `${code} - ${name}` : code }
        })
        .filter((option) => option.value)
        .sort((left, right) => left.label.localeCompare(right.label))
    case 'source_payment_id':
      return commercialRefundablePayments(catalog, values)
        .map((item) => {
          const id = String(resolvePath(item, 'header.id') || '')
          const number = String(resolvePath(item, 'header.number') || id)
          const method = String(resolvePath(item, 'body.payload.payment_method_code') || '')
          const amount = toNumber(resolvePath(item, 'body.payload.amount_received'))
          const refunded = toNumber(resolvePath(item, 'body.payload.refunded_amount'))
          const remaining = roundMoney(Math.max(amount - refunded, 0))
          return {
            value: id,
            label: `${number}${method ? ` - ${humanize(method)}` : ''}${remaining > 0 ? ` (${remaining})` : ''}`,
          }
        })
        .filter((option) => option.value)
        .sort((left, right) => left.label.localeCompare(right.label))
    case 'invoice_id':
      return commercialOpenInvoices(catalog, values)
        .map((item) => {
          const id = String(resolvePath(item, 'header.id') || '')
          const number = String(resolvePath(item, 'header.number') || id)
          const partyName = String(resolvePath(item, 'body.payload.party_name') || '')
          const balance = toNumber(resolvePath(item, 'body.payload.balance_due_amount'))
          return {
            value: id,
            label: `${number}${partyName ? ` - ${partyName}` : ''}${balance > 0 ? ` (${balance})` : ''}`,
          }
        })
        .filter((option) => option.value)
        .sort((left, right) => left.label.localeCompare(right.label))
    case 'item_code':
      return Object.values(catalog.itemsByCode)
        .map((item) => {
          const code = String(resolvePath(item, 'values.sku') || '')
          const name = String(resolvePath(item, 'values.name') || '')
          return { value: code, label: name ? `${code} - ${name}` : code }
        })
        .filter((option) => option.value)
        .sort((left, right) => left.label.localeCompare(right.label))
    default:
      return []
  }
}

function commercialOpenInvoices(catalog?: CommercialFormCatalog, values?: FormState): Array<Record<string, unknown>> {
  if (!catalog) return []
  const partyID = String(resolvePath(values || {}, 'party_id') || '')
  return Object.values(catalog.invoicesByID)
    .filter((item) => {
      const balance = toNumber(resolvePath(item, 'body.payload.balance_due_amount'))
      if (balance <= 0) return false
      if (!partyID) return true
      return String(resolvePath(item, 'body.payload.party_id') || '') === partyID
    })
    .sort((left, right) => {
      const leftDue = String(resolvePath(left, 'body.payload.due_date') || '')
      const rightDue = String(resolvePath(right, 'body.payload.due_date') || '')
      if (leftDue !== rightDue) return leftDue.localeCompare(rightDue)
      return String(resolvePath(left, 'header.number') || '').localeCompare(String(resolvePath(right, 'header.number') || ''))
    })
}

function commercialRefundablePayments(catalog?: CommercialFormCatalog, values?: FormState): Array<Record<string, unknown>> {
  if (!catalog) return []
  const invoiceID = String(resolvePath(values || {}, 'source_invoice_id') || '')
  if (!invoiceID) return []
  return Object.values(catalog.paymentsByID)
    .filter((item) => {
      const status = String(resolvePath(item, 'header.status') || '')
      if (status !== 'received') return false
      const amount = toNumber(resolvePath(item, 'body.payload.amount_received'))
      const refunded = toNumber(resolvePath(item, 'body.payload.refunded_amount'))
      if (roundMoney(Math.max(amount - refunded, 0)) <= 0) return false
      const links = (resolvePath(item, 'links') as Array<Record<string, unknown>> | undefined) || []
      return links.some((link) => String(link.link_type || '') === 'payment_for' && String(link.linked_document_id || '') === invoiceID)
    })
    .sort((left, right) => {
      const leftDate = String(resolvePath(left, 'body.payload.receipt_date') || '')
      const rightDate = String(resolvePath(right, 'body.payload.receipt_date') || '')
      if (leftDate !== rightDate) return leftDate.localeCompare(rightDate)
      return String(resolvePath(left, 'header.number') || '').localeCompare(String(resolvePath(right, 'header.number') || ''))
    })
}

function buildAllocationRows(invoices: Array<Record<string, unknown>>, requestedAmount: number): { rows: Array<Record<string, unknown>>; allocatedAmount: number } {
  let remaining = roundMoney(requestedAmount)
  const allocateAll = remaining <= 0
  const rows: Array<Record<string, unknown>> = []
  let allocatedAmount = 0
  for (const invoice of invoices) {
    const balance = roundMoney(toNumber(resolvePath(invoice, 'body.payload.balance_due_amount')))
    if (balance <= 0) continue
    const allocationAmount = allocateAll ? balance : roundMoney(Math.min(balance, remaining))
    if (allocationAmount <= 0) continue
    rows.push({
      invoice_id: String(resolvePath(invoice, 'header.id') || ''),
      invoice_number: String(resolvePath(invoice, 'header.number') || ''),
      amount: allocationAmount,
      note: '',
    })
    allocatedAmount = roundMoney(allocatedAmount + allocationAmount)
    if (!allocateAll) {
      remaining = roundMoney(Math.max(remaining-allocationAmount, 0))
      if (remaining <= 0) break
    }
  }
  return { rows, allocatedAmount }
}

function buildRefundAllocationRows(payments: Array<Record<string, unknown>>, requestedAmount: number): { rows: Array<Record<string, unknown>>; allocatedAmount: number } {
  let remaining = roundMoney(requestedAmount)
  const allocateAll = remaining <= 0
  const rows: Array<Record<string, unknown>> = []
  let allocatedAmount = 0
  for (const payment of payments) {
    const amount = toNumber(resolvePath(payment, 'body.payload.amount_received'))
    const refunded = toNumber(resolvePath(payment, 'body.payload.refunded_amount'))
    const balance = roundMoney(Math.max(amount - refunded, 0))
    if (balance <= 0) continue
    const allocationAmount = allocateAll ? balance : roundMoney(Math.min(balance, remaining))
    if (allocationAmount <= 0) continue
    rows.push({
      payment_id: String(resolvePath(payment, 'header.id') || ''),
      payment_number: String(resolvePath(payment, 'header.number') || ''),
      amount: allocationAmount,
      note: '',
    })
    allocatedAmount = roundMoney(allocatedAmount + allocationAmount)
    if (!allocateAll) {
      remaining = roundMoney(Math.max(remaining - allocationAmount, 0))
      if (remaining <= 0) break
    }
  }
  return { rows, allocatedAmount }
}

function applyCommercialArrayUpdate(current: FormState, path: string, widget: string, rows: Array<Record<string, unknown>>, catalog?: CommercialFormCatalog): FormState {
  let next = assignPathValue(current, path, rows)
  switch (widget) {
    case 'commercial_lines': {
      const defaultTaxCode = String(resolvePath(current, 'default_tax_code') || '')
      const rowsWithDefaults = rows.map((row) => ({
        ...row,
        tax_code: row.tax_code || defaultTaxCode,
      }))
      const normalizedRows = normalizeCommercialRows(rowsWithDefaults, widget, catalog, current)
      const subtotalAmount = normalizedRows.reduce((sum, row) => sum + toNumber(row.line_subtotal), 0)
      const taxAmount = normalizedRows.reduce((sum, row) => sum + toNumber(row.tax_amount), 0)
      const totalAmount = normalizedRows.reduce((sum, row) => sum + toNumber(row.line_total), 0)
      next = assignPathValue(next, path, normalizedRows)
      next = assignPathValue(next, 'subtotal_amount', roundMoney(subtotalAmount))
      next = assignPathValue(next, 'tax_amount', roundMoney(taxAmount))
      next = assignPathValue(next, 'total_amount', roundMoney(totalAmount))
      return next
    }
    case 'commercial_allocations': {
      const normalizedRows = rows.map((row) => ({ ...row, amount: toNumber(row.amount) }))
      const currentAmountReceived = toNumber(resolvePath(current, 'amount_received'))
      const appliedAmount = normalizedRows.reduce((sum, row) => sum + toNumber(row.amount), 0)
      const amountReceived = currentAmountReceived > 0 ? currentAmountReceived : roundMoney(appliedAmount)
      next = assignPathValue(next, path, normalizedRows)
      next = assignPathValue(next, 'amount_received', amountReceived)
      next = assignPathValue(next, 'unapplied_amount', roundMoney(Math.max(amountReceived-appliedAmount, 0)))
      return next
    }
    case 'commercial_refund_allocations': {
      const normalizedRows = rows.map((row) => ({ ...row, amount: toNumber(row.amount) })) as Array<Record<string, unknown>>
      const refundedAmount = normalizedRows.reduce((sum, row) => sum + toNumber(row.amount), 0)
      next = assignPathValue(next, path, normalizedRows)
      next = assignPathValue(next, 'amount_refunded', roundMoney(refundedAmount))
      if (normalizedRows.length === 1) {
        const firstRow = normalizedRows[0] || {}
        next = assignPathValue(next, 'source_payment_id', String(firstRow.payment_id || ''))
        next = assignPathValue(next, 'source_payment_number', String(firstRow.payment_number || ''))
      } else {
        next = assignPathValue(next, 'source_payment_id', '')
        next = assignPathValue(next, 'source_payment_number', '')
      }
      return next
    }
    case 'commercial_journal_lines': {
      const normalizedRows = rows.map((row) => ({ ...row, debit: toNumber(row.debit), credit: toNumber(row.credit) }))
      const debitTotal = normalizedRows.reduce((sum, row) => sum + toNumber(row.debit), 0)
      const creditTotal = normalizedRows.reduce((sum, row) => sum + toNumber(row.credit), 0)
      next = assignPathValue(next, path, normalizedRows)
      next = assignPathValue(next, 'total_amount', roundMoney(Math.max(debitTotal, creditTotal)))
      return next
    }
    default:
      return next
  }
}

function applyFieldUpdate(current: FormState, path: string, value: unknown, catalog?: CommercialFormCatalog): FormState {
  const next = assignPathValue(current, path, value)
  if (path === 'default_tax_code') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) {
      return applyCommercialArrayUpdate(next, 'lines', 'commercial_lines', lines, catalog)
    }
  }
  if (path === 'party_id') {
    const partyID = String(value || '')
    const party = catalog?.partiesByID?.[partyID]
    let withParty = next
    const partyName = String(resolvePath(party, 'values.display_name') || resolvePath(party, 'values.name') || '')
    const currencyCode = String(resolvePath(party, 'values.currency_code') || '')
    const taxProfileCode = String(resolvePath(party, 'values.tax_profile_code') || '')
    const priceListCode = String(resolvePath(party, 'values.default_price_list_code') || '')
    const paymentTermDays = resolvePath(party, 'values.payment_term_days')
    const profile = catalog?.taxProfilesByCode?.[taxProfileCode]
    const profileDefaultTaxCode = String(resolvePath(profile, 'values.default_tax_code') || '')
    const profilePaymentTermDays = toNumber(resolvePath(profile, 'values.payment_term_days'))
    if (partyName) {
      withParty = assignPathValue(withParty, 'party_name', partyName)
    }
    if (currencyCode && !String(resolvePath(withParty, 'currency_code') || '')) {
      withParty = assignPathValue(withParty, 'currency_code', currencyCode)
    }
    if (taxProfileCode && !String(resolvePath(withParty, 'tax_profile_code') || '')) {
      withParty = assignPathValue(withParty, 'tax_profile_code', taxProfileCode)
    }
    if (priceListCode && !String(resolvePath(withParty, 'price_list_code') || '')) {
      withParty = assignPathValue(withParty, 'price_list_code', priceListCode)
    }
    if (profileDefaultTaxCode && !String(resolvePath(withParty, 'default_tax_code') || '')) {
      withParty = assignPathValue(withParty, 'default_tax_code', profileDefaultTaxCode)
    }
    const resolvedPaymentTermDays = toNumber(paymentTermDays) || profilePaymentTermDays
    if (resolvedPaymentTermDays > 0 && !toNumber(resolvePath(withParty, 'payment_term_days'))) {
      withParty = assignPathValue(withParty, 'payment_term_days', resolvedPaymentTermDays)
      const baseDate = String(resolvePath(withParty, 'invoice_date') || resolvePath(withParty, 'order_date') || '')
      if (baseDate) {
        const dueDate = addDaysToDate(baseDate, resolvedPaymentTermDays)
        if (dueDate) {
          withParty = assignPathValue(withParty, 'due_date', dueDate)
        }
      }
    }
    const lines = asRecordList(resolvePath(withParty, 'lines'))
    if (lines.length) {
      return applyCommercialArrayUpdate(withParty, 'lines', 'commercial_lines', lines, catalog)
    }
    return withParty
  }
  if (path === 'price_list_code') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) {
      return applyCommercialArrayUpdate(next, 'lines', 'commercial_lines', lines, catalog)
    }
    return next
  }
  if (path === 'tax_profile_code') {
    const profileCode = String(value || '')
    const profile = catalog?.taxProfilesByCode?.[profileCode]
    let withProfile = next
    const defaultTaxCode = resolvePath(profile, 'values.default_tax_code')
    const paymentTermDays = resolvePath(profile, 'values.payment_term_days')
    if (typeof defaultTaxCode === 'string' && defaultTaxCode) {
      withProfile = assignPathValue(withProfile, 'default_tax_code', defaultTaxCode)
    }
    if (paymentTermDays != null && paymentTermDays !== '') {
      withProfile = assignPathValue(withProfile, 'payment_term_days', toNumber(paymentTermDays))
      const baseDate = String(resolvePath(withProfile, 'invoice_date') || resolvePath(withProfile, 'order_date') || '')
      if (baseDate) {
        withProfile = assignPathValue(withProfile, 'due_date', addDaysToDate(baseDate, toNumber(paymentTermDays)))
      }
    }
    const lines = asRecordList(resolvePath(withProfile, 'lines'))
    if (lines.length) {
      return applyCommercialArrayUpdate(withProfile, 'lines', 'commercial_lines', lines, catalog)
    }
    return withProfile
  }
  if ((path === 'invoice_date' || path === 'order_date') && toNumber(resolvePath(next, 'payment_term_days')) > 0) {
    const dueDate = addDaysToDate(String(value || ''), toNumber(resolvePath(next, 'payment_term_days')))
    if (dueDate) {
      return assignPathValue(next, 'due_date', dueDate)
    }
  }
  if (path === 'payment_term_days') {
    const baseDate = String(resolvePath(next, 'invoice_date') || resolvePath(next, 'order_date') || '')
    const dueDate = addDaysToDate(baseDate, toNumber(value))
    if (dueDate) {
      return assignPathValue(next, 'due_date', dueDate)
    }
  }
  if (path === 'payment_method_code') {
    const methodCode = String(value || '')
    const clearingAccount = resolvePath(catalog?.paymentMethodsByCode?.[methodCode], 'values.clearing_account_code')
    if (typeof clearingAccount === 'string' && clearingAccount) {
      return assignPathValue(next, 'clearing_account_code', clearingAccount)
    }
  }
  if (path === 'source_payment_id') {
    const paymentID = String(value || '')
    const payment = catalog?.paymentsByID?.[paymentID]
    let updated = next
    const paymentNumber = String(resolvePath(payment, 'header.number') || '')
    const methodCode = String(resolvePath(payment, 'body.payload.payment_method_code') || '')
    const clearingAccount = String(resolvePath(payment, 'body.payload.clearing_account_code') || '')
    const paidAmount = toNumber(resolvePath(payment, 'body.payload.amount_received'))
    const refundedAmount = toNumber(resolvePath(payment, 'body.payload.refunded_amount'))
    const remainingAmount = roundMoney(Math.max(paidAmount - refundedAmount, 0))
    if (paymentNumber) updated = assignPathValue(updated, 'source_payment_number', paymentNumber)
    if (methodCode) updated = assignPathValue(updated, 'payment_method_code', methodCode)
    if (clearingAccount) updated = assignPathValue(updated, 'clearing_account_code', clearingAccount)
    if (paymentNumber && !String(resolvePath(updated, 'refund_reference') || '')) {
      updated = assignPathValue(updated, 'refund_reference', paymentNumber)
    }
    const currentRefund = toNumber(resolvePath(updated, 'amount_refunded'))
    if ((currentRefund <= 0 || currentRefund > remainingAmount) && remainingAmount > 0) {
      updated = assignPathValue(updated, 'amount_refunded', remainingAmount)
    }
    return updated
  }
  if (path === 'amount_refunded') {
    const refundAllocations = asRecordList(resolvePath(next, 'refund_allocations'))
    if (refundAllocations.length) {
      return applyCommercialArrayUpdate(next, 'refund_allocations', 'commercial_refund_allocations', refundAllocations, catalog)
    }
  }
  if (path === 'amount_received') {
    const allocations = asRecordList(resolvePath(next, 'allocations'))
    if (allocations.length) {
      return applyCommercialArrayUpdate(next, 'allocations', 'commercial_allocations', allocations, catalog)
    }
    return assignPathValue(next, 'unapplied_amount', roundMoney(Math.max(toNumber(value), 0)))
  }
  return next
}

function normalizeCommercialFormState(current: FormState, documentType: string, catalog: CommercialFormCatalog): FormState {
  let next = current
  const partyID = String(resolvePath(next, 'party_id') || '')
  const party = catalog.partiesByID[partyID]
  if (party) {
    const partyName = String(resolvePath(party, 'values.display_name') || resolvePath(party, 'values.name') || '')
    const currencyCode = String(resolvePath(party, 'values.currency_code') || '')
    const taxProfileCode = String(resolvePath(party, 'values.tax_profile_code') || '')
    const priceListCode = String(resolvePath(party, 'values.default_price_list_code') || '')
    const paymentTermDays = toNumber(resolvePath(party, 'values.payment_term_days'))
    if (!resolvePath(next, 'party_name') && partyName) {
      next = assignPathValue(next, 'party_name', partyName)
    }
    if (!resolvePath(next, 'currency_code') && currencyCode) {
      next = assignPathValue(next, 'currency_code', currencyCode)
    }
    if (!resolvePath(next, 'tax_profile_code') && taxProfileCode) {
      next = assignPathValue(next, 'tax_profile_code', taxProfileCode)
    }
    if (!resolvePath(next, 'price_list_code') && priceListCode) {
      next = assignPathValue(next, 'price_list_code', priceListCode)
    }
    if (!resolvePath(next, 'payment_term_days') && paymentTermDays > 0) {
      next = assignPathValue(next, 'payment_term_days', paymentTermDays)
    }
  }
  const profileCode = String(resolvePath(next, 'tax_profile_code') || '')
  const profile = catalog.taxProfilesByCode[profileCode]
  if (profile) {
    const defaultTaxCode = String(resolvePath(profile, 'values.default_tax_code') || '')
    if (!resolvePath(next, 'default_tax_code') && defaultTaxCode) {
      next = assignPathValue(next, 'default_tax_code', defaultTaxCode)
    }
    const paymentTermDays = toNumber(resolvePath(profile, 'values.payment_term_days'))
    if (!resolvePath(next, 'payment_term_days') && paymentTermDays > 0) {
      next = assignPathValue(next, 'payment_term_days', paymentTermDays)
    }
  }
  if (documentType === 'sales_order' || documentType === 'invoice') {
    const baseDate = String(resolvePath(next, 'invoice_date') || resolvePath(next, 'order_date') || '')
    const paymentTermDays = toNumber(resolvePath(next, 'payment_term_days'))
    if (baseDate && paymentTermDays > 0 && !resolvePath(next, 'due_date')) {
      next = assignPathValue(next, 'due_date', addDaysToDate(baseDate, paymentTermDays))
    }
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) {
      return applyCommercialArrayUpdate(next, 'lines', 'commercial_lines', lines, catalog)
    }
    return next
  }
  if (documentType === 'payment_receipt') {
    next = applyFieldUpdate(next, 'payment_method_code', resolvePath(next, 'payment_method_code'), catalog)
    const allocations = asRecordList(resolvePath(next, 'allocations'))
    if (allocations.length) {
      return applyCommercialArrayUpdate(next, 'allocations', 'commercial_allocations', allocations, catalog)
    }
    return next
  }
  if (documentType === 'payment_refund') {
    const refundAllocations = asRecordList(resolvePath(next, 'refund_allocations'))
    if (refundAllocations.length) {
      next = applyCommercialArrayUpdate(next, 'refund_allocations', 'commercial_refund_allocations', refundAllocations, catalog)
    }
    next = applyFieldUpdate(next, 'source_payment_id', resolvePath(next, 'source_payment_id'), catalog)
    next = applyFieldUpdate(next, 'payment_method_code', resolvePath(next, 'payment_method_code'), catalog)
    return next
  }
  return next
}

function normalizeCommercialRows(rows: Array<Record<string, unknown>>, widget: string, catalog?: CommercialFormCatalog, values?: FormState): Array<Record<string, unknown>> {
  if (widget === 'commercial_refund_allocations') {
    return rows.map((row) => {
      const paymentID = String(row.payment_id || '')
      const payment = catalog?.paymentsByID?.[paymentID]
      const paymentNumber = String(row.payment_number || resolvePath(payment, 'header.number') || '')
      const amountReceived = toNumber(resolvePath(payment, 'body.payload.amount_received'))
      const refundedAmount = toNumber(resolvePath(payment, 'body.payload.refunded_amount'))
      const remainingAmount = roundMoney(Math.max(amountReceived - refundedAmount, 0))
      let amount = toNumber(row.amount)
      if (remainingAmount > 0 && (amount <= 0 || amount > remainingAmount)) {
        amount = remainingAmount
      }
      return {
        ...row,
        payment_number: paymentNumber,
        amount,
      }
    })
  }
  if (widget !== 'commercial_lines') return rows
  return rows.map((row) => {
    const itemCode = String(row.item_code || '')
    const item = catalog?.itemsByCode?.[itemCode]
    const itemDescription = resolvePath(item, 'values.description') || resolvePath(item, 'values.name')
    const itemUOMCode = resolvePath(item, 'values.uom_code')
    const priceListCode = String(resolvePath(values || {}, 'price_list_code') || '')
    const priceListItem = catalog?.priceListItemsByKey?.[`${priceListCode}|${itemCode}`]
    const itemUnitPrice = resolvePath(priceListItem, 'values.unit_price') ?? resolvePath(item, 'values.base_price') ?? resolvePath(item, 'values.unit_price')
    const itemTaxCode = resolvePath(item, 'values.tax_code')
    const itemRevenueAccount = resolvePath(item, 'values.revenue_account_code')
    const priceListTaxCode = resolvePath(priceListItem, 'values.tax_code')
    const priceListRevenueAccount = resolvePath(priceListItem, 'values.revenue_account_code')
    const profile = catalog?.taxProfilesByCode?.[String(resolvePath(values || {}, 'tax_profile_code') || row.tax_profile_code || '')]
    const profileTaxCode = resolvePath(profile, 'values.default_tax_code')
    const profileTaxMode = resolvePath(profile, 'values.price_tax_mode')
    const defaultTaxCode = resolvePath(values || {}, 'default_tax_code')
    const taxCode = String(row.tax_code || defaultTaxCode || profileTaxCode || priceListTaxCode || itemTaxCode || '')
    const tax = catalog?.taxCodesByCode?.[taxCode]
    const taxRate = toNumber(row.tax_rate || resolvePath(tax, 'values.rate_percent'))
    const taxMode = String(resolvePath(tax, 'values.mode') || profileTaxMode || row.tax_mode || 'exclusive')
    const quantity = toNumber(row.quantity) || 1
    const unitPrice = toNumber(row.unit_price || itemUnitPrice)
    const discountAmount = toNumber(row.discount_amount)
    const grossAmount = Math.max(quantity * unitPrice - discountAmount, 0)
    const breakdown = calculateTaxBreakdown(grossAmount, taxRate, taxMode)
    return {
      ...row,
      description: String(row.description || itemDescription || ''),
      uom_code: String(row.uom_code || itemUOMCode || ''),
      unit_price: unitPrice,
      tax_code: taxCode,
      tax_rate: taxRate,
      tax_mode: taxMode,
      revenue_account_code: row.revenue_account_code || priceListRevenueAccount || itemRevenueAccount || '',
      tax_account_code: row.tax_account_code || resolvePath(tax, 'values.tax_account_code') || '',
      quantity,
      discount_amount: discountAmount,
      line_subtotal: breakdown.subtotal,
      tax_amount: breakdown.tax,
      line_total: breakdown.total,
    }
  })
}

function calculateTaxBreakdown(grossAmount: number, taxRate: number, taxMode: string): { subtotal: number; tax: number; total: number } {
  const mode = String(taxMode || 'exclusive').toLowerCase()
  if (mode === 'inclusive') {
    if (taxRate <= 0) return { subtotal: roundMoney(grossAmount), tax: 0, total: roundMoney(grossAmount) }
    const subtotal = roundMoney(grossAmount / (1 + taxRate / 100))
    return { subtotal, tax: roundMoney(grossAmount - subtotal), total: roundMoney(grossAmount) }
  }
  if (mode === 'exempt') {
    return { subtotal: roundMoney(grossAmount), tax: 0, total: roundMoney(grossAmount) }
  }
  const subtotal = roundMoney(grossAmount)
  const tax = roundMoney(subtotal * taxRate / 100)
  return { subtotal, tax, total: roundMoney(subtotal + tax) }
}

function toNumber(value: unknown): number {
  const numeric = typeof value === 'number' ? value : Number(value || 0)
  return Number.isFinite(numeric) ? numeric : 0
}

function roundMoney(value: number): number {
  return Math.round(value * 100) / 100
}

function addDaysToDate(baseDate: string, days: number): string {
  if (!baseDate || !Number.isFinite(days) || days <= 0) return ''
  const date = new Date(`${baseDate}T00:00:00`)
  if (Number.isNaN(date.getTime())) return ''
  date.setDate(date.getDate() + days)
  return date.toISOString().slice(0, 10)
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

function actionVisibleForStatus(actionKey: string, status: string, documentType: string): boolean {
  const normalizedAction = actionKey.toLowerCase()
  const normalizedStatus = status.toLowerCase()
  const normalizedType = documentType.toLowerCase()
  if (!normalizedStatus) return true
  switch (normalizedAction) {
    case 'submit':
      return normalizedStatus === 'draft'
    case 'approve':
    case 'reject':
      return normalizedStatus === 'submitted'
    case 'cancel':
      if (normalizedStatus === 'draft' || normalizedStatus === 'submitted') return true
      if (normalizedType === 'invoice' && normalizedStatus === 'issued') return true
      if (normalizedType === 'payment_receipt' && normalizedStatus === 'received') return true
      if (normalizedType === 'payment_refund' && normalizedStatus === 'refunded') return true
      return false
    case 'reopen':
      return normalizedStatus !== 'draft' && normalizedStatus !== 'submitted'
    case 'generate_invoice':
      return normalizedStatus === 'confirmed'
    case 'register_payment':
      return normalizedStatus === 'issued' || normalizedStatus === 'partially_paid'
    case 'issue_credit_note':
      return normalizedStatus === 'issued' || normalizedStatus === 'partially_paid' || normalizedStatus === 'paid'
    case 'register_refund':
      return normalizedStatus === 'issued'
    default:
      return true
  }
}

function isCommercialDocumentLocked(documentType: string, status: string): boolean {
  const normalizedType = documentType.toLowerCase()
  const normalizedStatus = status.toLowerCase()
  if (!['sales_order', 'invoice', 'credit_note', 'payment_receipt', 'payment_refund', 'ledger_posting'].includes(normalizedType)) return false
  if (!normalizedStatus) return false
  return normalizedStatus !== 'draft' && normalizedStatus !== 'rejected'
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
