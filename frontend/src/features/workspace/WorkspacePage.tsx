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
  vendorsByID: Record<string, Record<string, unknown>>
  invoicesByID: Record<string, Record<string, unknown>>
  billsByID: Record<string, Record<string, unknown>>
  paymentsByID: Record<string, Record<string, unknown>>
  productsByCode: Record<string, Record<string, unknown>>
  itemsByCode: Record<string, Record<string, unknown>>
  variantDimensionsByCode: Record<string, Record<string, unknown>>
  variantValuesByKey: Record<string, Record<string, unknown>>
  itemCategoriesByCode: Record<string, Record<string, unknown>>
  uomsByCode: Record<string, Record<string, unknown>>
  warehousesByCode: Record<string, Record<string, unknown>>
  workCentersByCode: Record<string, Record<string, unknown>>
  inventoryBatchesByID: Record<string, Record<string, unknown>>
  bomsByID: Record<string, Record<string, unknown>>
  bomVersionsByID: Record<string, Record<string, unknown>>
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
    return <DashboardView view={view} locale={locale} onNavigate={onNavigate} routeActions={actions} onToast={onToast} />
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
  const [procurementSummary, setProcurementSummary] = useState<Record<string, unknown> | null>(null)
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
      if (view.model_key === 'vendor_profile') {
        try {
          const summary = await fetchJSON<Record<string, unknown>>(`/ui/data/procurement/vendors/${encodeURIComponent(documentID)}/summary`)
          if (!mounted) return
          setProcurementSummary(summary)
        } catch {
          if (!mounted) return
          setProcurementSummary(null)
        }
      } else {
        setProcurementSummary(null)
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
  const canEdit =
    !!editTarget &&
    !isCommercialDocumentLocked(String(header.type || ''), String(header.status || '')) &&
    !isProcurementDocumentLocked(String(header.type || ''), String(header.status || '')) &&
    !isFulfillmentDocumentLocked(String(header.type || ''), String(header.status || '')) &&
    !isReturnsDocumentLocked(String(header.type || ''), String(header.status || '')) &&
    !isSupplierReturnsDocumentLocked(String(header.type || ''), String(header.status || '')) &&
    !isProductionDocumentLocked(String(header.type || ''), String(header.status || ''))

  async function handleAction(actionKey: string) {
    if (String(header.type || '') === 'sales_fulfillment' && actionKey === 'register_return') {
      try {
        const created = await invokeCommercialAction(`/returns/fulfillments/${encodeURIComponent(String(header.id || ''))}/register-return`)
        onToast('Return draft generated.', 'success')
        const target = routeForDocument('sales_return', 'detail', routeActions, '/returns/returns')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Return generation failed', 'error')
      }
      return
    }
    if (String(header.type || '') === 'sales_fulfillment' && actionKey === 'register_delivery') {
      try {
        const created = await invokeCommercialAction(`/delivery/fulfillments/${encodeURIComponent(String(header.id || ''))}/register-delivery`)
        onToast('Delivery draft generated.', 'success')
        const target = routeForDocument('delivery_order', 'detail', routeActions, '/delivery/orders')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Delivery generation failed', 'error')
      }
      return
    }
    if (String(header.type || '') === 'sales_order' && actionKey === 'generate_fulfillment') {
      try {
        const created = await invokeCommercialAction(`/commercial/orders/${encodeURIComponent(String(header.id || ''))}/generate-fulfillment`)
        onToast('Fulfillment draft generated.', 'success')
        const target = routeForDocument('sales_fulfillment', 'detail', routeActions, '/fulfillment/fulfillments')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Fulfillment generation failed', 'error')
      }
      return
    }
    if (String(header.type || '') === 'sales_order' && actionKey === 'generate_production_order') {
      try {
        const created = await invokeCommercialAction(`/commercial/orders/${encodeURIComponent(String(header.id || ''))}/generate-production-order`)
        const items = Array.isArray((created as Record<string, unknown>).items) ? ((created as Record<string, unknown>).items as Array<Record<string, unknown>>) : []
        onToast(items.length > 1 ? `${items.length} production orders generated.` : 'Production order draft generated.', 'success')
        const firstID = items.length ? resolvePath(items[0], 'header.id') : null
        const target = routeForDocument('production_order', 'detail', routeActions, '/production/orders')
        if (target && firstID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(firstID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Production order generation failed', 'error')
      }
      return
    }
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
    if (String(header.type || '') === 'sales_return' && actionKey === 'register_return_receipt') {
      try {
        const created = await invokeCommercialAction(`/returns/returns/${encodeURIComponent(String(header.id || ''))}/register-receipt`)
        onToast('Return receipt draft generated.', 'success')
        const target = routeForDocument('return_receipt', 'detail', routeActions, '/returns/receipts')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Return receipt generation failed', 'error')
      }
      return
    }
    if (String(header.type || '') === 'sales_return' && actionKey === 'issue_credit_note') {
      try {
        const created = await invokeCommercialAction(`/returns/returns/${encodeURIComponent(String(header.id || ''))}/issue-credit-note`)
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
    if (String(header.type || '') === 'sales_return' && actionKey === 'register_refund') {
      try {
        const created = await invokeCommercialAction(`/returns/returns/${encodeURIComponent(String(header.id || ''))}/register-refund`)
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
    if (String(header.type || '') === 'sales_return' && actionKey === 'create_replacement_order') {
      try {
        const created = await invokeCommercialAction(`/returns/returns/${encodeURIComponent(String(header.id || ''))}/create-replacement-order`)
        onToast('Replacement order draft generated.', 'success')
        const target = routeForDocument('sales_order', 'detail', routeActions, '/commercial/orders')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Replacement order generation failed', 'error')
      }
      return
    }
    if (String(header.type || '') === 'purchase_request' && actionKey === 'generate_purchase_order') {
      try {
        const created = await invokeCommercialAction(`/procurement/requests/${encodeURIComponent(String(header.id || ''))}/generate-purchase-order`)
        onToast('Purchase order draft generated.', 'success')
        const target = routeForDocument('purchase_order', 'detail', routeActions, '/procurement/orders')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Purchase order generation failed', 'error')
      }
      return
    }
    if (String(header.type || '') === 'purchase_order' && actionKey === 'register_receipt') {
      try {
        const created = await invokeCommercialAction(`/procurement/orders/${encodeURIComponent(String(header.id || ''))}/register-receipt`)
        onToast('Goods receipt draft generated.', 'success')
        const target = routeForDocument('goods_receipt', 'detail', routeActions, '/procurement/receipts')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Receipt registration failed', 'error')
      }
      return
    }
    if (String(header.type || '') === 'production_order' && actionKey === 'register_production_issue') {
      try {
        const created = await invokeCommercialAction(`/production/orders/${encodeURIComponent(String(header.id || ''))}/register-issue`)
        onToast('Production issue draft generated.', 'success')
        const target = routeForDocument('production_issue', 'detail', routeActions, '/production/issues')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Production issue generation failed', 'error')
      }
      return
    }
    if (String(header.type || '') === 'production_order' && actionKey === 'register_production_output') {
      try {
        const created = await invokeCommercialAction(`/production/orders/${encodeURIComponent(String(header.id || ''))}/register-output`)
        onToast('Production output draft generated.', 'success')
        const target = routeForDocument('production_output', 'detail', routeActions, '/production/outputs')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Production output generation failed', 'error')
      }
      return
    }
    if (String(header.type || '') === 'purchase_order' && actionKey === 'register_vendor_bill') {
      try {
        const created = await invokeCommercialAction(`/procurement/orders/${encodeURIComponent(String(header.id || ''))}/register-vendor-bill`)
        onToast('Vendor bill draft generated.', 'success')
        const target = routeForDocument('vendor_bill', 'detail', routeActions, '/procurement/bills')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Vendor bill generation failed', 'error')
      }
      return
    }
    if (String(header.type || '') === 'goods_receipt' && actionKey === 'register_vendor_bill') {
      try {
        const created = await invokeCommercialAction(`/procurement/receipts/${encodeURIComponent(String(header.id || ''))}/register-vendor-bill`)
        onToast('Vendor bill draft generated.', 'success')
        const target = routeForDocument('vendor_bill', 'detail', routeActions, '/procurement/bills')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Vendor bill generation failed', 'error')
      }
      return
    }
    if (String(header.type || '') === 'goods_receipt' && actionKey === 'register_supplier_return') {
      try {
        const created = await invokeCommercialAction(`/procurement/receipts/${encodeURIComponent(String(header.id || ''))}/register-supplier-return`)
        onToast('Supplier return draft generated.', 'success')
        const target = routeForDocument('supplier_return', 'detail', routeActions, '/supplier-returns/returns')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Supplier return generation failed', 'error')
      }
      return
    }
    if (String(header.type || '') === 'vendor_bill' && actionKey === 'register_payment_out') {
      try {
        const created = await invokeCommercialAction(`/procurement/bills/${encodeURIComponent(String(header.id || ''))}/register-payment`)
        onToast('Payment-out draft generated.', 'success')
        const target = routeForDocument('payment_out', 'detail', routeActions, '/procurement/payments')
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
    if (String(header.type || '') === 'vendor_bill' && actionKey === 'issue_vendor_credit_note') {
      try {
        const created = await invokeCommercialAction(`/procurement/bills/${encodeURIComponent(String(header.id || ''))}/issue-credit-note`)
        onToast('Vendor credit draft generated.', 'success')
        const target = routeForDocument('vendor_credit_note', 'detail', routeActions, '/procurement/credits')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Vendor credit generation failed', 'error')
      }
      return
    }
    if (String(header.type || '') === 'vendor_bill' && actionKey === 'register_supplier_return') {
      try {
        const created = await invokeCommercialAction(`/procurement/bills/${encodeURIComponent(String(header.id || ''))}/register-supplier-return`)
        onToast('Supplier return draft generated.', 'success')
        const target = routeForDocument('supplier_return', 'detail', routeActions, '/supplier-returns/returns')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Supplier return generation failed', 'error')
      }
      return
    }
    if (String(header.type || '') === 'supplier_return' && actionKey === 'issue_vendor_credit_note') {
      try {
        const created = await invokeCommercialAction(`/supplier-returns/returns/${encodeURIComponent(String(header.id || ''))}/issue-vendor-credit`)
        onToast('Vendor credit draft generated.', 'success')
        const target = routeForDocument('vendor_credit_note', 'detail', routeActions, '/procurement/credits')
        const createdID = resolvePath(created, 'header.id')
        if (target && createdID) {
          onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
          return
        }
        setReloadKey((current) => current + 1)
      } catch (error) {
        onToast(error instanceof Error ? error.message : 'Vendor credit generation failed', 'error')
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
        {view.model_key === 'vendor_profile' && procurementSummary ? (
          <VendorProcurementSummaryPanel summary={procurementSummary} routeActions={routeActions} onNavigate={onNavigate} />
        ) : null}
        {view.model_key === 'commercial_product' ? (
          <ProductVariantsPanel product={record || {}} routeActions={routeActions} onNavigate={onNavigate} onToast={onToast} />
        ) : null}
        {view.model_key === 'inventory_batch' && record ? (
          <>
            <InventoryBatchControlPanel batch={record} onToast={onToast} onChanged={() => setReloadKey((current) => current + 1)} />
            <InventoryBatchTracePanel batch={record} onToast={onToast} />
          </>
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
  onToast,
}: {
  view: ViewDefinition
  locale: string
  onNavigate: (target: string) => void
  routeActions: ActionDefinition[]
  onToast: (message: string, variant?: 'default' | 'success' | 'warning' | 'error') => void
}) {
  const [payload, setPayload] = useState<Record<string, unknown> | null>(null)
  const [selectedReplenishmentKeys, setSelectedReplenishmentKeys] = useState<string[]>([])
  const [selectedProposalIDs, setSelectedProposalIDs] = useState<string[]>([])
  const searchKey = window.location.search

  useEffect(() => {
    let mounted = true
    async function load() {
      const search = new URLSearchParams(searchKey)
      if (view.projection_key === 'commercial.party_statement' && !search.get('party_id')) {
        if (!mounted) return
        setPayload(null)
        return
      }
      const target = view.dataset_key
        ? `/ui/data/reporting/datasets/${encodeURIComponent(view.dataset_key)}`
        : view.projection_key === 'commercial.receivables.summary'
          ? '/ui/data/commercial/receivables/summary'
        : view.projection_key === 'procurement.payables.summary'
          ? '/ui/data/procurement/payables/summary'
        : view.projection_key === 'inventory.summary'
          ? '/ui/data/inventory/summary'
        : view.projection_key === 'planning.replenishment.summary'
          ? `/ui/data/planning/replenishment/summary${searchKey || ''}`
        : view.projection_key === 'planning.runs.summary'
          ? '/ui/data/planning/runs'
        : view.projection_key === 'planning.proposals.summary'
          ? `/ui/data/planning/runs/${encodeURIComponent(search.get('run_id') || '')}/proposals`
        : view.projection_key === 'commercial.party_statement'
          ? `/ui/data/commercial/parties/${encodeURIComponent(search.get('party_id') || '')}/summary${buildStatementQuery(search)}`
        : view.projection_key === 'procurement.vendor_statement'
          ? `/ui/data/procurement/vendors/${encodeURIComponent(search.get('vendor_id') || '')}/summary${buildStatementQuery(search)}`
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
  }, [searchKey, view.dataset_key, view.projection_key])

  const statementSearch = new URLSearchParams(window.location.search)
  const partyID = statementSearch.get('party_id') || ''
  const vendorID = statementSearch.get('vendor_id') || ''
  const receivablesItems = view.projection_key === 'commercial.receivables.summary'
    ? asRecordList(resolvePath(payload, 'items'))
    : []
  const payablesItems = view.projection_key === 'procurement.payables.summary'
    ? asRecordList(resolvePath(payload, 'items'))
    : []
  const inventoryBatches = view.projection_key === 'inventory.summary'
    ? asRecordList(resolvePath(payload, 'batches'))
    : []
  const planningRuns = view.projection_key === 'planning.runs.summary'
    ? asRecordList(resolvePath(payload, 'items'))
    : []
  const replenishmentItems = view.projection_key === 'planning.replenishment.summary'
    ? asRecordList(resolvePath(payload, 'items'))
    : []
  const planningProposals = view.projection_key === 'planning.proposals.summary'
    ? asRecordList(resolvePath(payload, 'items'))
    : []
  useEffect(() => {
    if (view.projection_key !== 'planning.replenishment.summary') return
    const currentKeys = new Set(replenishmentItems.map((row) => `${String(row.item_code || '')}|${String(row.warehouse_code || '')}`))
    setSelectedReplenishmentKeys((existing) => existing.filter((key) => currentKeys.has(key)))
  }, [replenishmentItems, view.projection_key])
  useEffect(() => {
    if (view.projection_key !== 'planning.proposals.summary') return
    const currentIDs = new Set(planningProposals.map((row) => String(row.id || '')))
    setSelectedProposalIDs((existing) => existing.filter((id) => currentIDs.has(id)))
  }, [planningProposals, view.projection_key])

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
      {payablesItems.length ? (
        <section className="mt-6 rounded-xl border border-line p-4">
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Payables Aging</h2>
          <DataTable
            columns={[
              { key: 'number', label: 'Bill' },
              { key: 'vendor_name', label: 'Vendor' },
              { key: 'due_date', label: 'Due Date' },
              { key: 'paid_amount', label: 'Paid' },
              { key: 'credited', label: 'Credited' },
              { key: 'balance_due', label: 'Open Balance' },
              { key: 'aging_bucket', label: 'Aging' },
            ]}
            rows={payablesItems}
            emptyText="No payables."
            renderAction={(row) => {
              const detailPath = routeForDocument('vendor_bill', 'detail', routeActions, '/procurement/bills')
              const recordID = String(row.id || '')
              if (!detailPath || !recordID) return null
              return (
                <button onClick={() => onNavigate(`${detailPath}?id=${encodeURIComponent(recordID)}`)} className="rounded-lg border border-line px-3 py-1.5 text-sm text-body">
                  Open
                </button>
              )
            }}
          />
        </section>
      ) : null}
      {inventoryBatches.length ? (
        <section className="mt-6 rounded-xl border border-line p-4">
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Batch Stock</h2>
          <DataTable
            columns={[
              { key: 'item_code', label: 'Item' },
              { key: 'warehouse_code', label: 'Warehouse' },
              { key: 'batch_code', label: 'Batch' },
              { key: 'expiration_date', label: 'Expiration' },
              { key: 'status', label: 'Status' },
              { key: 'on_hand_quantity', label: 'On Hand' },
              { key: 'available_quantity', label: 'Available' },
            ]}
            rows={inventoryBatches}
            emptyText="No tracked batches."
          />
        </section>
      ) : null}
      {view.projection_key === 'planning.replenishment.summary' ? (
        <section className="mt-6 rounded-xl border border-line p-4">
          <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
            <h2 className="text-sm font-semibold uppercase tracking-wide text-muted">Replenishment Candidates</h2>
            <div className="flex flex-wrap gap-2">
              <button
                onClick={async () => {
                  const search = new URLSearchParams(window.location.search)
                  try {
                    const response = await fetch('/ui/data/planning/runs', {
                      method: 'POST',
                      credentials: 'include',
                      headers: {
                        'Content-Type': 'application/json',
                        'X-CSRF-Token': readCookie('orbyte_csrf'),
                      },
                      body: JSON.stringify({
                        warehouse_code: search.get('warehouse_code') || '',
                        item_code: search.get('item_code') || '',
                        category_code: search.get('category_code') || '',
                        coverage_status: search.get('coverage_status') || '',
                        shortage_only: search.get('shortage_only') === '1' || search.get('shortage_only') === 'true',
                        has_inbound: search.get('has_inbound') === '1' || search.get('has_inbound') === 'true',
                        has_preferred_vendor: search.get('has_preferred_vendor') === '1' || search.get('has_preferred_vendor') === 'true',
                      }),
                    })
                    if (!response.ok) throw await buildError(response)
                    const result = (await response.json()) as Record<string, unknown>
                    const runID = String(result.run_id || '')
                    if (!runID) throw new Error('Planning run was created without an id.')
                    onToast('Planning run created.', 'success')
                    onNavigate(`/planning/proposals?run_id=${encodeURIComponent(runID)}`)
                  } catch (error) {
                    onToast(error instanceof Error ? error.message : 'Failed to create planning run.', 'error')
                  }
                }}
                className="rounded-lg border border-line px-3 py-1.5 text-sm text-body"
              >
                Create Planning Run
              </button>
              <button
                disabled={!selectedReplenishmentKeys.length}
                onClick={async () => {
                  const purchaseRequestDetailPath = routeForDocument('purchase_request', 'detail', routeActions, '/procurement/requests')
                  const selectedRows = replenishmentItems.filter((row) => selectedReplenishmentKeys.includes(`${String(row.item_code || '')}|${String(row.warehouse_code || '')}`))
                  if (!selectedRows.length) return
                  try {
                    const response = await fetch('/ui/data/planning/replenishment/generate-purchase-request', {
                      method: 'POST',
                      credentials: 'include',
                      headers: {
                        'Content-Type': 'application/json',
                        'X-CSRF-Token': readCookie('orbyte_csrf'),
                      },
                      body: JSON.stringify({
                        items: selectedRows.map((row) => ({
                          item_code: String(row.item_code || ''),
                          warehouse_code: String(row.warehouse_code || ''),
                          quantity: Number(row.normalized_request_quantity || row.suggested_request_quantity || 0),
                        })),
                      }),
                    })
                    if (!response.ok) throw await buildError(response)
                    const result = (await response.json()) as Record<string, unknown>
                    const records = asRecordList(result.records)
                    setSelectedReplenishmentKeys([])
                    if (records.length === 1 && purchaseRequestDetailPath) {
                      const firstRecord = records[0]
                      const recordID = String(firstRecord?.record_id || '')
                      if (recordID) {
                        onToast('Purchase request created.', 'success')
                        onNavigate(`${purchaseRequestDetailPath}?id=${encodeURIComponent(recordID)}`)
                        return
                      }
                    }
                    onToast(`${records.length || Number(result.record_count || 0)} purchase requests created.`, 'success')
                  } catch (error) {
                    onToast(error instanceof Error ? error.message : 'Failed to generate purchase requests.', 'error')
                  }
                }}
                className="rounded-lg border border-line px-3 py-1.5 text-sm text-body disabled:cursor-not-allowed disabled:opacity-50"
              >
                Generate Purchase Requests
              </button>
            </div>
          </div>
          <div className="overflow-hidden rounded-xl border border-line">
            <table className="min-w-full divide-y divide-line">
              <thead className="border-b border-line bg-accent-soft dark:bg-ink/60">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-bold uppercase tracking-[0.14em] text-accent-dark dark:text-body">
                    <input
                      type="checkbox"
                      checked={replenishmentItems.length > 0 && replenishmentItems.every((row) => selectedReplenishmentKeys.includes(`${String(row.item_code || '')}|${String(row.warehouse_code || '')}`) || Number(row.normalized_request_quantity || row.suggested_request_quantity || 0) <= 0)}
                      onChange={(event) => {
                        if (event.target.checked) {
                          setSelectedReplenishmentKeys(replenishmentItems.filter((row) => Number(row.normalized_request_quantity || row.suggested_request_quantity || 0) > 0).map((row) => `${String(row.item_code || '')}|${String(row.warehouse_code || '')}`))
                          return
                        }
                        setSelectedReplenishmentKeys([])
                      }}
                    />
                  </th>
                  {[
                    'Item',
                    'Warehouse',
                    'Preferred Vendor',
                    'On Hand',
                    'Reserved',
                    'Inbound',
                    'Requested',
                    'Ordered',
                    'Received',
                    'Net',
                    'Forecast',
                    'Shortage Date',
                    'Order By',
                    'Reorder',
                    'Target',
                    'Demand',
                    'Uncovered',
                    'Suggested',
                    'Normalized',
                    'Coverage',
                    'MOQ',
                    'Pack',
                    'Lead Time',
                  ].map((label) => (
                    <th key={label} className="px-4 py-3 text-left text-xs font-bold uppercase tracking-[0.14em] text-accent-dark dark:text-body">{label}</th>
                  ))}
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-line bg-surface">
                {replenishmentItems.length ? replenishmentItems.map((row, index) => {
                  const rowKey = `${String(row.item_code || '')}|${String(row.warehouse_code || '')}`
                  const suggestedQuantity = Number(row.normalized_request_quantity || row.suggested_request_quantity || 0)
                  const purchaseRequestDetailPath = routeForDocument('purchase_request', 'detail', routeActions, '/procurement/requests')
                  const requestRefs = asRecordList(row.purchase_request_refs)
                  const orderRefs = asRecordList(row.purchase_order_refs)
                  return (
                    <tr key={index}>
                      <td className="px-4 py-3 text-sm text-body">
                        <input
                          type="checkbox"
                          disabled={suggestedQuantity <= 0}
                          checked={selectedReplenishmentKeys.includes(rowKey)}
                          onChange={(event) => {
                            setSelectedReplenishmentKeys((existing) => event.target.checked
                              ? [...existing, rowKey]
                              : existing.filter((value) => value !== rowKey))
                          }}
                        />
                      </td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'item_code'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'warehouse_code'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'preferred_vendor_name'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'on_hand_quantity'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'reserved_quantity'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'inbound_quantity'))}</td>
                      <td className="px-4 py-3 text-sm text-body">
                        <div>{displayValue(resolvePath(row, 'requested_quantity'))}</div>
                        {requestRefs.length ? <div className="mt-1 text-xs text-muted">{requestRefs.map((ref) => String(ref.number || ref.id || '')).join(', ')}</div> : null}
                      </td>
                      <td className="px-4 py-3 text-sm text-body">
                        <div>{displayValue(resolvePath(row, 'ordered_quantity'))}</div>
                        {orderRefs.length ? <div className="mt-1 text-xs text-muted">{orderRefs.map((ref) => String(ref.number || ref.id || '')).join(', ')}</div> : null}
                      </td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'received_quantity'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'net_available_quantity'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'forecast_demand_quantity'))}</td>
                      <td className="px-4 py-3 text-sm text-body">
                        <div>{displayValue(resolvePath(row, 'projected_shortage_date'))}</div>
                        {Boolean(row.time_critical) ? <div className="mt-1 text-xs text-red-600">Time critical</div> : null}
                      </td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'recommended_order_by_date'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'reorder_point_quantity'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'target_stock_quantity'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'sales_demand_quantity'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'uncovered_shortage_quantity'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'suggested_request_quantity'))}</td>
                      <td className="px-4 py-3 text-sm text-body">
                        <div>{displayValue(resolvePath(row, 'normalized_request_quantity'))}</div>
                        {String(row.planning_quantity_rule || '') !== 'none' ? <div className="mt-1 text-xs text-muted">{displayValue(resolvePath(row, 'planning_quantity_rule'))}</div> : null}
                      </td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'coverage_status'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'minimum_order_quantity'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'pack_size'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'lead_time_days'))}</td>
                      <td className="px-4 py-3 text-right">
                        {suggestedQuantity > 0 && purchaseRequestDetailPath ? (
                          <button
                            onClick={async () => {
                              try {
                                const response = await fetch('/ui/data/planning/replenishment/generate-purchase-request', {
                                  method: 'POST',
                                  credentials: 'include',
                                  headers: {
                                    'Content-Type': 'application/json',
                                    'X-CSRF-Token': readCookie('orbyte_csrf'),
                                  },
                                  body: JSON.stringify({
                                    items: [{
                                      item_code: String(row.item_code || ''),
                                      warehouse_code: String(row.warehouse_code || ''),
                                      quantity: suggestedQuantity,
                                    }],
                                  }),
                                })
                                if (!response.ok) throw await buildError(response)
                                const result = (await response.json()) as Record<string, unknown>
                                const recordID = String(result.record_id || '')
                                if (!recordID) throw new Error('Purchase request was created without an id.')
                                onToast('Purchase request created.', 'success')
                                onNavigate(`${purchaseRequestDetailPath}?id=${encodeURIComponent(recordID)}`)
                              } catch (error) {
                                onToast(error instanceof Error ? error.message : 'Failed to generate purchase request.', 'error')
                              }
                            }}
                            className="rounded-lg border border-line px-3 py-1.5 text-sm text-body"
                          >
                            Generate PR
                          </button>
                        ) : null}
                      </td>
                    </tr>
                  )
                }) : (
                  <tr>
                    <td colSpan={25} className="px-4 py-10 text-center text-sm text-muted">
                      No replenishment candidates.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </section>
      ) : null}
      {view.projection_key === 'planning.runs.summary' ? (
        <section className="mt-6 rounded-xl border border-line p-4">
          <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
            <h2 className="text-sm font-semibold uppercase tracking-wide text-muted">Saved Planning Runs</h2>
            <button
              onClick={() => onNavigate('/planning/replenishment')}
              className="rounded-lg border border-line px-3 py-1.5 text-sm text-body"
            >
              Open Replenishment
            </button>
          </div>
          <DataTable
            columns={[
              { key: 'run_date', label: 'Run Date' },
              { key: 'warehouse_code', label: 'Warehouse' },
              { key: 'proposal_count', label: 'Proposals' },
              { key: 'projected_shortage_item_count', label: 'Shortages' },
              { key: 'total_forecast_demand_quantity', label: 'Forecast Demand' },
              { key: 'due_soon_count', label: 'Due Soon' },
              { key: 'status', label: 'Status' },
            ]}
            rows={planningRuns}
            emptyText="No planning runs."
            renderAction={(row) => {
              const runID = String(row.id || '')
              if (!runID) return null
              return (
                <button onClick={() => onNavigate(`/planning/proposals?run_id=${encodeURIComponent(runID)}`)} className="rounded-lg border border-line px-3 py-1.5 text-sm text-body">
                  Open
                </button>
              )
            }}
          />
        </section>
      ) : null}
      {view.projection_key === 'planning.proposals.summary' ? (
        <section className="mt-6 rounded-xl border border-line p-4">
          <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 className="text-sm font-semibold uppercase tracking-wide text-muted">Planning Proposals</h2>
              <div className="mt-1 text-xs text-muted">
                Run {displayValue(resolvePath(payload, 'run.run_date'))} · Warehouse {displayValue(resolvePath(payload, 'run.warehouse_code')) || 'All'}
              </div>
            </div>
            <div className="flex flex-wrap gap-2">
              <button
                onClick={() => onNavigate('/planning/runs')}
                className="rounded-lg border border-line px-3 py-1.5 text-sm text-body"
              >
                Back to Runs
              </button>
              <button
                disabled={!selectedProposalIDs.length}
                onClick={async () => {
                  const purchaseRequestDetailPath = routeForDocument('purchase_request', 'detail', routeActions, '/procurement/requests')
                  try {
                    const response = await fetch('/ui/data/planning/proposals/convert-purchase-request', {
                      method: 'POST',
                      credentials: 'include',
                      headers: {
                        'Content-Type': 'application/json',
                        'X-CSRF-Token': readCookie('orbyte_csrf'),
                      },
                      body: JSON.stringify({ proposal_ids: selectedProposalIDs }),
                    })
                    if (!response.ok) throw await buildError(response)
                    const result = (await response.json()) as Record<string, unknown>
                    const records = asRecordList(result.records)
                    setSelectedProposalIDs([])
                    if (records.length === 1 && purchaseRequestDetailPath) {
                      const recordID = String(records[0]?.record_id || '')
                      if (recordID) {
                        onToast('Purchase request created.', 'success')
                        onNavigate(`${purchaseRequestDetailPath}?id=${encodeURIComponent(recordID)}`)
                        return
                      }
                    }
                    onToast(`${records.length || Number(result.record_count || 0)} purchase requests created.`, 'success')
                    const refreshed = await fetchJSON<Record<string, unknown>>(`/ui/data/planning/runs/${encodeURIComponent(new URLSearchParams(window.location.search).get('run_id') || '')}/proposals`)
                    setPayload(refreshed)
                  } catch (error) {
                    onToast(error instanceof Error ? error.message : 'Failed to convert planning proposals.', 'error')
                  }
                }}
                className="rounded-lg border border-line px-3 py-1.5 text-sm text-body disabled:cursor-not-allowed disabled:opacity-50"
              >
                Convert to Purchase Requests
              </button>
            </div>
          </div>
          <div className="overflow-hidden rounded-xl border border-line">
            <table className="min-w-full divide-y divide-line">
              <thead className="border-b border-line bg-accent-soft dark:bg-ink/60">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-bold uppercase tracking-[0.14em] text-accent-dark dark:text-body">
                    <input
                      type="checkbox"
                      checked={planningProposals.length > 0 && planningProposals.every((row) => selectedProposalIDs.includes(String(row.id || '')) || String(row.conversion_status || '') === 'converted' || Number(row.normalized_request_quantity || 0) <= 0)}
                      onChange={(event) => {
                        if (event.target.checked) {
                          setSelectedProposalIDs(planningProposals.filter((row) => String(row.conversion_status || '') !== 'converted' && Number(row.normalized_request_quantity || 0) > 0).map((row) => String(row.id || '')))
                          return
                        }
                        setSelectedProposalIDs([])
                      }}
                    />
                  </th>
                  {['Item', 'Warehouse', 'Vendor', 'Forecast', 'Demand', 'Inbound ETA', 'Shortage Date', 'Order By', 'Suggested', 'Normalized', 'Coverage', 'Conversion'].map((label) => (
                    <th key={label} className="px-4 py-3 text-left text-xs font-bold uppercase tracking-[0.14em] text-accent-dark dark:text-body">{label}</th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-line bg-surface">
                {planningProposals.length ? planningProposals.map((row, index) => {
                  const proposalID = String(row.id || '')
                  const normalizedQuantity = Number(row.normalized_request_quantity || 0)
                  const conversionStatus = String(row.conversion_status || '')
                  return (
                    <tr key={proposalID || index}>
                      <td className="px-4 py-3 text-sm text-body">
                        <input
                          type="checkbox"
                          disabled={conversionStatus === 'converted' || normalizedQuantity <= 0}
                          checked={selectedProposalIDs.includes(proposalID)}
                          onChange={(event) => {
                            setSelectedProposalIDs((existing) => event.target.checked
                              ? [...existing, proposalID]
                              : existing.filter((value) => value !== proposalID))
                          }}
                        />
                      </td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'item_code'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'warehouse_code'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'preferred_vendor_name'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'forecast_demand_quantity'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'sales_demand_quantity'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'next_inbound_date'))}</td>
                      <td className="px-4 py-3 text-sm text-body">
                        <div>{displayValue(resolvePath(row, 'projected_shortage_date'))}</div>
                        {Boolean(row.time_critical) ? <div className="mt-1 text-xs text-red-600">Time critical</div> : null}
                      </td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'recommended_order_by_date'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'suggested_request_quantity'))}</td>
                      <td className="px-4 py-3 text-sm text-body">
                        <div>{displayValue(resolvePath(row, 'normalized_request_quantity'))}</div>
                        {String(row.planning_quantity_rule || '') !== 'none' ? <div className="mt-1 text-xs text-muted">{displayValue(resolvePath(row, 'planning_quantity_rule'))}</div> : null}
                      </td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'coverage_status'))}</td>
                      <td className="px-4 py-3 text-sm text-body">{displayValue(resolvePath(row, 'conversion_status'))}</td>
                    </tr>
                  )
                }) : (
                  <tr>
                    <td colSpan={13} className="px-4 py-10 text-center text-sm text-muted">
                      No saved planning proposals.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
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
      {view.projection_key === 'procurement.vendor_statement' ? (
        vendorID ? (
          <section className="mt-6">
            <VendorProcurementSummaryPanel summary={payload || {}} routeActions={routeActions} onNavigate={onNavigate} />
          </section>
        ) : (
          <section className="mt-6 rounded-xl border border-line p-4 text-sm text-muted">
            Select a vendor from vendor detail to open a statement.
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
    if (view.projection_key === 'inventory.summary') {
      switch (cardKey) {
        case 'expired_batch_count':
          return `${basePath}?status=expired`
        case 'near_expiry_batch_count':
          return `${basePath}?status=near_expiry`
        case 'quarantined_batch_count':
          return `${basePath}?status=quarantined`
        case 'blocked_batch_count':
          return `${basePath}?status=blocked`
        case 'recalled_batch_count':
          return `${basePath}?status=recalled`
        default:
          return basePath
      }
    }
    if (view.projection_key === 'planning.replenishment.summary') {
      switch (cardKey) {
        case 'shortage_item_count':
        case 'total_shortage_quantity':
        case 'total_suggested_request_quantity':
          return `${basePath}?shortage_only=1`
        default:
          return basePath
      }
    }
    if (view.projection_key !== 'procurement.payables.summary') {
      return basePath
    }
    switch (cardKey) {
      case 'open_bill_count':
      case 'open_balance_total':
        return `${basePath}?payable_state=open`
      case 'overdue_bill_count':
      case 'overdue_balance_total':
        return `${basePath}?payable_state=overdue`
      case 'due_today_total':
        return `${basePath}?payable_state=due_today`
      case 'current_balance_total':
        return `${basePath}?payable_state=current`
      case 'paid_amount_total':
        return `${basePath}?status=paid`
      case 'credited_amount_total':
        return `${basePath}?status=issued`
      default:
        return basePath
    }
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

function VendorProcurementSummaryPanel({
  summary,
  routeActions,
  onNavigate,
}: {
  summary: Record<string, unknown>
  routeActions: ActionDefinition[]
  onNavigate: (target: string) => void
}) {
  const openBills = asRecordList(resolvePath(summary, 'open_bills'))
  const activities = asRecordList(resolvePath(summary, 'activities')).slice(0, 10)
  const billDetailPath = routeForDocument('vendor_bill', 'detail', routeActions, '/procurement/bills')
  const statementPath = routeActions.find((action) => action.key === 'procurement.vendor_statement.dashboard')?.route_path || ''
  const vendorID = String(resolvePath(summary, 'vendor_id') || '')

  return (
    <section className="space-y-4 rounded-xl border border-line p-4">
      <div className="flex items-center justify-between gap-3">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-muted">Procurement Summary</h2>
        {statementPath && vendorID ? (
          <button onClick={() => onNavigate(`${statementPath}?vendor_id=${encodeURIComponent(vendorID)}`)} className="rounded-lg border border-line px-3 py-2 text-sm text-body">
            Open Statement
          </button>
        ) : null}
      </div>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
        <MetricCard label="Open Bills" value={displayValue(resolvePath(summary, 'open_bill_count'))} />
        <MetricCard label="Open Balance" value={displayValue(resolvePath(summary, 'open_balance_total'))} />
        <MetricCard label="Paid" value={displayValue(resolvePath(summary, 'paid_amount_total'))} />
        <MetricCard label="Credited" value={displayValue(resolvePath(summary, 'credited_amount_total'))} />
      </div>
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <section>
          <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Open Bills</h3>
          <DataTable
            columns={[
              { key: 'number', label: 'Bill' },
              { key: 'due_date', label: 'Due Date' },
              { key: 'paid_amount', label: 'Paid' },
              { key: 'credited', label: 'Credited' },
              { key: 'balance_due', label: 'Open Balance' },
            ]}
            rows={openBills}
            emptyText="No open bills."
            renderAction={(row) => {
              const recordID = String(row.id || '')
              if (!billDetailPath || !recordID) return null
              return (
                <button onClick={() => onNavigate(`${billDetailPath}?id=${encodeURIComponent(recordID)}`)} className="rounded-lg border border-line px-3 py-1.5 text-sm text-body">
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
            emptyText="No procurement activity yet."
          />
        </section>
      </div>
    </section>
  )
}

function InventoryBatchControlPanel({
  batch,
  onToast,
  onChanged,
}: {
  batch: Record<string, unknown>
  onToast: (message: string, variant?: 'default' | 'success' | 'warning' | 'error') => void
  onChanged: () => void
}) {
  const batchID = String(resolvePath(batch, 'id') || '')
  const status = String(resolvePath(batch, 'values.status') || '')
  const [submitting, setSubmitting] = useState('')

  const actions = (() => {
    switch (status) {
      case 'quarantined':
        return ['release', 'block', 'recall']
      case 'blocked':
        return ['unblock', 'quarantine', 'recall']
      case 'recalled':
        return ['release']
      default:
        return ['quarantine', 'block', 'recall']
    }
  })()

  async function handleAction(action: string) {
    if (!batchID) return
    setSubmitting(action)
    try {
      const response = await fetch(`/ui/data/inventory/batches/${encodeURIComponent(batchID)}/actions`, {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': readCookie('orbyte_csrf'),
        },
        body: JSON.stringify({
          action,
          reason: action,
        }),
      })
      if (!response.ok) throw await buildError(response)
      onToast(`Batch ${humanize(action)} applied.`, 'success')
      onChanged()
    } catch (error) {
      onToast(error instanceof Error ? error.message : 'Batch action failed', 'error')
    } finally {
      setSubmitting('')
    }
  }

  return (
    <section className="rounded-xl border border-line p-4">
      <div className="flex flex-wrap items-center gap-3">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-muted">Batch Controls</h2>
        {actions.map((action) => (
          <button
            key={action}
            type="button"
            disabled={submitting !== '' && submitting !== action}
            onClick={() => void handleAction(action)}
            className="rounded-lg border border-line px-3 py-2 text-sm text-body disabled:opacity-50"
          >
            {submitting === action ? 'Working...' : humanize(action)}
          </button>
        ))}
      </div>
    </section>
  )
}

function InventoryBatchTracePanel({
  batch,
  onToast,
}: {
  batch: Record<string, unknown>
  onToast: (message: string, variant?: 'default' | 'success' | 'warning' | 'error') => void
}) {
  const batchID = String(resolvePath(batch, 'id') || '')
  const [trace, setTrace] = useState<Record<string, unknown> | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let mounted = true
    async function loadTrace() {
      if (!batchID) return
      setLoading(true)
      try {
        const payload = await fetchJSON<Record<string, unknown>>(`/ui/data/inventory/batches/${encodeURIComponent(batchID)}/trace`)
        if (!mounted) return
        setTrace(payload)
      } catch (error) {
        if (!mounted) return
        setTrace(null)
        onToast(error instanceof Error ? error.message : 'Batch trace failed', 'error')
      } finally {
        if (mounted) setLoading(false)
      }
    }
    void loadTrace()
    return () => {
      mounted = false
    }
  }, [batchID, onToast])

  const nodes = asRecordList(resolvePath(trace, 'nodes'))
  const producedInto = asRecordList(resolvePath(trace, 'produced_into'))
  const consumedFrom = asRecordList(resolvePath(trace, 'consumed_from'))
  const summary = (resolvePath(trace, 'summary') || {}) as Record<string, unknown>

  return (
    <section className="space-y-4 rounded-xl border border-line p-4">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-muted">Batch Trace</h2>
      {loading ? <div className="text-sm text-muted">Loading batch trace.</div> : null}
      {!loading && trace ? (
        <>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
            <MetricCard label="On Hand" value={displayValue(resolvePath(summary, 'on_hand_quantity'))} />
            <MetricCard label="Available" value={displayValue(resolvePath(summary, 'available_quantity'))} />
            <MetricCard label="Movements" value={displayValue(resolvePath(summary, 'movement_count'))} />
            <MetricCard label="Linked Docs" value={displayValue(resolvePath(summary, 'document_node_count'))} />
          </div>
          <section>
            <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Document Chain</h3>
            <DataTable
              columns={[
                { key: 'date', label: 'Date' },
                { key: 'type', label: 'Type' },
                { key: 'number', label: 'Number' },
                { key: 'movement_reason', label: 'Reason' },
                { key: 'quantity_delta', label: 'Delta' },
                { key: 'status', label: 'Status' },
              ]}
              rows={nodes}
              emptyText="No trace nodes."
            />
          </section>
          <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
            <section>
              <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Produced Into</h3>
              <DataTable
                columns={[
                  { key: 'production_order_number', label: 'Production Order' },
                  { key: 'production_output_number', label: 'Output' },
                  { key: 'item_code', label: 'Item' },
                  { key: 'batch_code', label: 'Batch' },
                  { key: 'output_quantity', label: 'Quantity' },
                ]}
                rows={producedInto}
                emptyText="This batch has not produced downstream lots."
              />
            </section>
            <section>
              <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Consumed From</h3>
              <DataTable
                columns={[
                  { key: 'production_order_number', label: 'Production Order' },
                  { key: 'production_issue_number', label: 'Issue' },
                  { key: 'item_code', label: 'Item' },
                  { key: 'batch_code', label: 'Batch' },
                  { key: 'quantity', label: 'Quantity' },
                ]}
                rows={consumedFrom}
                emptyText="No upstream production consumption links."
              />
            </section>
          </div>
        </>
      ) : null}
    </section>
  )
}

function ProductVariantsPanel({
  product,
  routeActions,
  onNavigate,
  onToast,
}: {
  product: Record<string, unknown>
  routeActions: ActionDefinition[]
  onNavigate: (target: string) => void
  onToast: (message: string, variant?: 'default' | 'success' | 'warning' | 'error') => void
}) {
  const productID = String(resolvePath(product, 'id') || '')
  const productCode = String(resolvePath(product, 'values.code') || '')
  const dimensionCodes = parseDimensionCodes(resolvePath(product, 'values.variant_dimension_codes'))
  const [dimensionDefs, setDimensionDefs] = useState<Array<Record<string, unknown>>>([])
  const [variantValues, setVariantValues] = useState<Array<Record<string, unknown>>>([])
  const [variants, setVariants] = useState<Array<Record<string, unknown>>>([])
  const [selected, setSelected] = useState<Record<string, string[]>>({})
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    let mounted = true
    async function load() {
      if (!productCode) return
      setLoading(true)
      try {
        const [dimensionsPayload, valuesPayload, itemsPayload] = await Promise.all([
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/commercial_variant_dimension?page_size=200'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/commercial_variant_value?page_size=500'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>(`/models/commercial_item?product_code=${encodeURIComponent(productCode)}&page_size=500`),
        ])
        if (!mounted) return
        setDimensionDefs((dimensionsPayload.items || []).filter((item) => dimensionCodes.includes(String(resolvePath(item, 'values.code') || ''))))
        setVariantValues(valuesPayload.items || [])
        setVariants(itemsPayload.items || [])
      } catch {
        if (!mounted) return
        setDimensionDefs([])
        setVariantValues([])
        setVariants([])
      } finally {
        if (mounted) setLoading(false)
      }
    }
    void load()
    return () => {
      mounted = false
    }
  }, [productCode, dimensionCodes.join('|')])

  const valuesByDimension = useMemo(() => {
    const grouped: Record<string, Array<Record<string, unknown>>> = {}
    for (const item of variantValues) {
      const dimensionCode = String(resolvePath(item, 'values.dimension_code') || '')
      if (!dimensionCode || !dimensionCodes.includes(dimensionCode)) continue
      if (!grouped[dimensionCode]) grouped[dimensionCode] = []
      grouped[dimensionCode].push(item)
    }
    for (const key of Object.keys(grouped)) {
      const dimensionValues = grouped[key]
      if (!dimensionValues) continue
      dimensionValues.sort((left, right) => {
        const leftOrder = toNumber(resolvePath(left, 'values.sort_order'))
        const rightOrder = toNumber(resolvePath(right, 'values.sort_order'))
        if (leftOrder !== rightOrder) return leftOrder - rightOrder
        return String(resolvePath(left, 'values.name') || resolvePath(left, 'values.code') || '').localeCompare(String(resolvePath(right, 'values.name') || resolvePath(right, 'values.code') || ''))
      })
    }
    return grouped
  }, [dimensionCodes, variantValues])

  const canGenerate = dimensionCodes.length > 0 && dimensionCodes.every((code) => (selected[code] || []).length > 0)

  async function handleGenerate() {
    if (!productID || !canGenerate) return
    setSubmitting(true)
    try {
      const response = await fetch(`/commercial/products/${encodeURIComponent(productID)}/generate-variants`, {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': readCookie('orbyte_csrf'),
        },
        body: JSON.stringify({
          dimensions: dimensionCodes.map((dimensionCode) => ({
            dimension_code: dimensionCode,
            value_codes: selected[dimensionCode] || [],
          })),
        }),
      })
      if (!response.ok) throw await buildError(response)
      const refreshed = await fetchJSON<{ items: Array<Record<string, unknown>> }>(`/models/commercial_item?product_code=${encodeURIComponent(productCode)}&page_size=500`)
      setVariants(refreshed.items || [])
      onToast('Variants generated.', 'success')
    } catch (error) {
      onToast(error instanceof Error ? error.message : 'Variant generation failed', 'error')
    } finally {
      setSubmitting(false)
    }
  }

  const itemDetailPath = routeForModel('commercial_item', 'detail', routeActions, '/commercial/items')

  return (
    <section className="rounded-xl border border-line p-4">
      <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Variants</h2>
      {!dimensionCodes.length ? (
        <div className="text-sm text-muted">No variant dimensions are configured for this product.</div>
      ) : (
        <div className="space-y-4">
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            {dimensionCodes.map((dimensionCode) => {
              const dimension = dimensionDefs.find((item) => String(resolvePath(item, 'values.code') || '') === dimensionCode)
              const options = valuesByDimension[dimensionCode] || []
              return (
                <div key={dimensionCode} className="rounded-lg border border-line p-3">
                  <div className="mb-2 text-sm font-medium text-body">
                    {String(resolvePath(dimension, 'values.name') || dimensionCode)}
                  </div>
                  <div className="space-y-2">
                    {options.length ? options.map((item) => {
                      const valueCode = String(resolvePath(item, 'values.code') || '')
                      const checked = (selected[dimensionCode] || []).includes(valueCode)
                      return (
                        <label key={`${dimensionCode}-${valueCode}`} className="flex items-center gap-2 text-sm text-body">
                          <input
                            type="checkbox"
                            checked={checked}
                            onChange={(event) =>
                              setSelected((current) => ({
                                ...current,
                                [dimensionCode]: event.target.checked
                                  ? [...(current[dimensionCode] || []), valueCode]
                                  : (current[dimensionCode] || []).filter((value) => value !== valueCode),
                              }))
                            }
                          />
                          <span>{String(resolvePath(item, 'values.name') || valueCode)}</span>
                        </label>
                      )
                    }) : <div className="text-sm text-muted">No active values for this dimension.</div>}
                  </div>
                </div>
              )
            })}
          </div>
          <div className="flex items-center gap-3">
            <button type="button" onClick={() => void handleGenerate()} disabled={!canGenerate || submitting} className="rounded-lg bg-accent px-4 py-2 text-sm text-white disabled:opacity-50">
              Generate Variants
            </button>
            <span className="text-sm text-muted">Choose one or more values for each configured dimension.</span>
          </div>
        </div>
      )}
      <div className="mt-4 overflow-x-auto">
        <table className="min-w-full divide-y divide-line text-sm">
          <thead className="border-b border-line bg-accent-soft dark:bg-ink/60">
            <tr>
              <th className="px-3 py-2 text-left text-xs font-bold uppercase tracking-[0.14em] text-accent-dark dark:text-body">SKU</th>
              <th className="px-3 py-2 text-left text-xs font-bold uppercase tracking-[0.14em] text-accent-dark dark:text-body">Name</th>
              <th className="px-3 py-2 text-left text-xs font-bold uppercase tracking-[0.14em] text-accent-dark dark:text-body">Variant</th>
              <th className="px-3 py-2 text-left text-xs font-bold uppercase tracking-[0.14em] text-accent-dark dark:text-body">Base Price</th>
              <th className="px-3 py-2 text-left text-xs font-bold uppercase tracking-[0.14em] text-accent-dark dark:text-body">Status</th>
              <th className="px-3 py-2" />
            </tr>
          </thead>
          <tbody className="divide-y divide-line bg-surface">
            {loading ? (
              <tr><td colSpan={6} className="px-3 py-6 text-center text-sm text-muted">Loading variants.</td></tr>
            ) : variants.length ? variants.map((item) => {
              const id = String(item.id || '')
              return (
                <tr key={id}>
                  <td className="px-3 py-2 text-body">{displayValue(resolvePath(item, 'values.sku'))}</td>
                  <td className="px-3 py-2 text-body">{displayValue(resolvePath(item, 'values.name'))}</td>
                  <td className="px-3 py-2 text-body">{displayValue(resolvePath(item, 'values.variant_label'))}</td>
                  <td className="px-3 py-2 text-body">{displayValue(resolvePath(item, 'values.base_price'))}</td>
                  <td className="px-3 py-2 text-body">{displayValue(resolvePath(item, 'values.status'))}</td>
                  <td className="px-3 py-2 text-right">
                    {itemDetailPath ? (
                      <button type="button" onClick={() => onNavigate(`${itemDetailPath}?id=${encodeURIComponent(id)}`)} className="rounded-lg border border-line px-3 py-2 text-body">
                        Open
                      </button>
                    ) : null}
                  </td>
                </tr>
              )
            }) : (
              <tr><td colSpan={6} className="px-3 py-6 text-center text-sm text-muted">No variants exist yet.</td></tr>
            )}
          </tbody>
        </table>
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
  const [catalog, setCatalog] = useState<CommercialFormCatalog>({ partiesByID: {}, vendorsByID: {}, invoicesByID: {}, billsByID: {}, paymentsByID: {}, productsByCode: {}, itemsByCode: {}, variantDimensionsByCode: {}, variantValuesByKey: {}, itemCategoriesByCode: {}, uomsByCode: {}, warehousesByCode: {}, workCentersByCode: {}, inventoryBatchesByID: {}, bomsByID: {}, bomVersionsByID: {}, taxCodesByCode: {}, taxProfilesByCode: {}, priceListsByCode: {}, priceListItemsByKey: {}, paymentMethodsByCode: {} })

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
        ['sales_order', 'invoice', 'credit_note', 'payment_receipt', 'payment_refund', 'purchase_request', 'purchase_order', 'goods_receipt', 'vendor_bill', 'payment_out', 'vendor_credit_note', 'sales_fulfillment', 'delivery_order', 'sales_return', 'return_receipt', 'supplier_return', 'stock_receipt', 'stock_issue', 'stock_adjustment', 'stock_transfer', 'production_order', 'production_issue', 'production_output', 'recall_case', 'recall_action'].includes(documentType) ||
        ['party', 'vendor_profile', 'commercial_product', 'commercial_item', 'commercial_variant_dimension', 'commercial_variant_value', 'commercial_price_list_item', 'inventory_batch', 'warehouse', 'production_bom', 'production_bom_version', 'production_work_center'].includes(modelKey)
      if (!needsCatalog) {
        if (!mounted) return
        setCatalog({ partiesByID: {}, vendorsByID: {}, invoicesByID: {}, billsByID: {}, paymentsByID: {}, productsByCode: {}, itemsByCode: {}, variantDimensionsByCode: {}, variantValuesByKey: {}, itemCategoriesByCode: {}, uomsByCode: {}, warehousesByCode: {}, workCentersByCode: {}, inventoryBatchesByID: {}, bomsByID: {}, bomVersionsByID: {}, taxCodesByCode: {}, taxProfilesByCode: {}, priceListsByCode: {}, priceListItemsByKey: {}, paymentMethodsByCode: {} })
        return
      }
      try {
        const [partiesPayload, vendorsPayload, invoicesPayload, billsPayload, paymentsPayload, productsPayload, itemsPayload, variantDimensionsPayload, variantValuesPayload, categoriesPayload, uomsPayload, warehousesPayload, workCentersPayload, inventoryBatchesPayload, bomsPayload, bomVersionsPayload, taxPayload, taxProfilesPayload, priceListsPayload, priceListItemsPayload, paymentPayload] = await Promise.all([
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/party'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/vendor_profile'),
          fetchJSON<{ items: Array<Record<string, unknown>>; total?: number }>('/ui/data/documents?type=invoice&page_size=200&include_payload=1'),
          fetchJSON<{ items: Array<Record<string, unknown>>; total?: number }>('/ui/data/documents?type=vendor_bill&page_size=200&include_payload=1'),
          fetchJSON<{ items: Array<Record<string, unknown>>; total?: number }>('/ui/data/documents?type=payment_receipt&page_size=200&include_payload=1'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/commercial_product'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/commercial_item'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/commercial_variant_dimension'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/commercial_variant_value'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/commercial_item_category'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/commercial_uom'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/warehouse'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/production_work_center'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/inventory_batch?page_size=500'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/production_bom'),
          fetchJSON<{ items: Array<Record<string, unknown>> }>('/models/production_bom_version'),
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
        const openBillIDs = (billsPayload.items || [])
          .filter((item) => {
            const status = String(resolvePath(item, 'header.status') || '')
            return status === 'issued' || status === 'partially_paid'
          })
          .map((item) => String(resolvePath(item, 'header.id') || ''))
          .filter(Boolean)
        const billDetails = await Promise.all(
          openBillIDs.map(async (id) => {
            try {
              return await fetchJSON<Record<string, unknown>>(`/ui/data/documents/${encodeURIComponent(id)}`)
            } catch {
              return null
            }
          }),
        )
        const billsByID = Object.fromEntries(
          billDetails
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
          vendorsByID: Object.fromEntries((vendorsPayload.items || []).map((item) => [String(item.id || ''), item])),
          invoicesByID,
          billsByID,
          paymentsByID,
          productsByCode: Object.fromEntries((productsPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          itemsByCode: Object.fromEntries((itemsPayload.items || []).map((item) => [String(resolvePath(item, 'values.sku') || ''), item])),
          variantDimensionsByCode: Object.fromEntries((variantDimensionsPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          variantValuesByKey: Object.fromEntries((variantValuesPayload.items || []).map((item) => [`${String(resolvePath(item, 'values.dimension_code') || '')}|${String(resolvePath(item, 'values.code') || '')}`, item])),
          itemCategoriesByCode: Object.fromEntries((categoriesPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          uomsByCode: Object.fromEntries((uomsPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          warehousesByCode: Object.fromEntries((warehousesPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          workCentersByCode: Object.fromEntries((workCentersPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          inventoryBatchesByID: Object.fromEntries((inventoryBatchesPayload.items || []).map((item) => [String(resolvePath(item, 'id') || ''), item])),
          bomsByID: Object.fromEntries((bomsPayload.items || []).map((item) => [String(item.id || ''), item])),
          bomVersionsByID: Object.fromEntries((bomVersionsPayload.items || []).map((item) => [String(item.id || ''), item])),
          taxCodesByCode: Object.fromEntries((taxPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          taxProfilesByCode: Object.fromEntries((taxProfilesPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          priceListsByCode: Object.fromEntries((priceListsPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          priceListItemsByKey: Object.fromEntries((priceListItemsPayload.items || []).map((item) => [`${String(resolvePath(item, 'values.price_list_code') || '')}|${String(resolvePath(item, 'values.item_code') || '')}`, item])),
          paymentMethodsByCode: Object.fromEntries((paymentPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
        })
        setValues((current) => normalizeCommercialFormState(current, String(view.document_type || ''), {
          partiesByID: Object.fromEntries((partiesPayload.items || []).map((item) => [String(item.id || ''), item])),
          vendorsByID: Object.fromEntries((vendorsPayload.items || []).map((item) => [String(item.id || ''), item])),
          invoicesByID,
          billsByID,
          paymentsByID,
          productsByCode: Object.fromEntries((productsPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          itemsByCode: Object.fromEntries((itemsPayload.items || []).map((item) => [String(resolvePath(item, 'values.sku') || ''), item])),
          variantDimensionsByCode: Object.fromEntries((variantDimensionsPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          variantValuesByKey: Object.fromEntries((variantValuesPayload.items || []).map((item) => [`${String(resolvePath(item, 'values.dimension_code') || '')}|${String(resolvePath(item, 'values.code') || '')}`, item])),
          itemCategoriesByCode: Object.fromEntries((categoriesPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          uomsByCode: Object.fromEntries((uomsPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          warehousesByCode: Object.fromEntries((warehousesPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          workCentersByCode: Object.fromEntries((workCentersPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          inventoryBatchesByID: Object.fromEntries((inventoryBatchesPayload.items || []).map((item) => [String(resolvePath(item, 'id') || ''), item])),
          bomsByID: Object.fromEntries((bomsPayload.items || []).map((item) => [String(item.id || ''), item])),
          bomVersionsByID: Object.fromEntries((bomVersionsPayload.items || []).map((item) => [String(item.id || ''), item])),
          taxCodesByCode: Object.fromEntries((taxPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          taxProfilesByCode: Object.fromEntries((taxProfilesPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          priceListsByCode: Object.fromEntries((priceListsPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
          priceListItemsByKey: Object.fromEntries((priceListItemsPayload.items || []).map((item) => [`${String(resolvePath(item, 'values.price_list_code') || '')}|${String(resolvePath(item, 'values.item_code') || '')}`, item])),
          paymentMethodsByCode: Object.fromEntries((paymentPayload.items || []).map((item) => [String(resolvePath(item, 'values.code') || ''), item])),
        }))
      } catch {
        if (!mounted) return
        setCatalog({ partiesByID: {}, vendorsByID: {}, invoicesByID: {}, billsByID: {}, paymentsByID: {}, productsByCode: {}, itemsByCode: {}, variantDimensionsByCode: {}, variantValuesByKey: {}, itemCategoriesByCode: {}, uomsByCode: {}, warehousesByCode: {}, workCentersByCode: {}, inventoryBatchesByID: {}, bomsByID: {}, bomVersionsByID: {}, taxCodesByCode: {}, taxProfilesByCode: {}, priceListsByCode: {}, priceListItemsByKey: {}, paymentMethodsByCode: {} })
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
  const formLocked = !view.model_key && targetID && (isCommercialDocumentLocked(String(view.document_type || ''), recordStatus) || isProcurementDocumentLocked(String(view.document_type || ''), recordStatus) || isFulfillmentDocumentLocked(String(view.document_type || ''), recordStatus) || isReturnsDocumentLocked(String(view.document_type || ''), recordStatus) || isSupplierReturnsDocumentLocked(String(view.document_type || ''), recordStatus) || isProductionDocumentLocked(String(view.document_type || ''), recordStatus) || isRecallDocumentLocked(String(view.document_type || ''), recordStatus))

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
    if (hasUnresolvedVariantSelections(values)) {
      onToast('Select a concrete variant SKU for every product-based line before saving.', 'warning')
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
                    catalog={{ partiesByID: {}, vendorsByID: {}, invoicesByID: {}, billsByID: {}, paymentsByID: {}, productsByCode: {}, itemsByCode: {}, variantDimensionsByCode: {}, variantValuesByKey: {}, itemCategoriesByCode: {}, uomsByCode: {}, warehousesByCode: {}, workCentersByCode: {}, inventoryBatchesByID: {}, bomsByID: {}, bomVersionsByID: {}, taxCodesByCode: {}, taxProfilesByCode: {}, priceListsByCode: {}, priceListItemsByKey: {}, paymentMethodsByCode: {} }}
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

  if (
    field.widget === 'commercial_lines' ||
    field.widget === 'commercial_allocations' ||
    field.widget === 'commercial_refund_allocations' ||
    field.widget === 'commercial_journal_lines' ||
    field.widget === 'procurement_lines' ||
    field.widget === 'procurement_receipt_lines' ||
    field.widget === 'procurement_allocations' ||
    field.widget === 'fulfillment_lines' ||
    field.widget === 'delivery_lines' ||
    field.widget === 'return_lines' ||
    field.widget === 'supplier_return_lines' ||
    field.widget === 'inventory_lines' ||
    field.widget === 'inventory_transfer_lines' ||
    field.widget === 'production_component_lines' ||
    field.widget === 'production_issue_lines' ||
    field.widget === 'production_stage_lines' ||
    field.widget === 'trace_batches'
  ) {
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
              if ((field.widget === 'commercial_allocations' || field.widget === 'commercial_refund_allocations' || field.widget === 'procurement_allocations') && patch) {
                if (patch.amount_received != null && (patch.replace_amount_received || toNumber(resolvePath(current, 'amount_received')) <= 0)) {
                  next = assignPathValue(next, 'amount_received', patch.amount_received)
                }
                if (patch.amount_paid != null && (patch.replace_amount_paid || toNumber(resolvePath(current, 'amount_paid')) <= 0)) {
                  next = assignPathValue(next, 'amount_paid', patch.amount_paid)
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
                if (patch.vendor_id && !String(resolvePath(current, 'vendor_id') || '')) {
                  next = applyFieldUpdate(next, 'vendor_id', patch.vendor_id, catalog)
                }
                if (patch.party_name && !String(resolvePath(current, 'party_name') || '')) {
                  next = assignPathValue(next, 'party_name', patch.party_name)
                }
                if (patch.vendor_name && !String(resolvePath(current, 'vendor_name') || '')) {
                  next = assignPathValue(next, 'vendor_name', patch.vendor_name)
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
  const openBills = widget === 'procurement_allocations' ? procurementOpenBills(catalog, values) : []
  const openBillBalance = openBills.reduce((sum, bill) => sum + toNumber(resolvePath(bill, 'body.payload.balance_due_amount')), 0)
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
      if (key === 'product_code') {
        updated.variant_signature = ''
        updated.item_code = ''
      }
      if (key === 'variant_signature') {
        updated.item_code = resolveVariantItemCodeFromRow(updated, catalog)
      }
      if (key === 'item_code') {
        const item = catalog.itemsByCode[String(nextValue || '')]
        updated.product_code = String(resolvePath(item, 'values.product_code') || updated.product_code || '')
        updated.variant_signature = String(resolvePath(item, 'values.variant_signature') || updated.variant_signature || '')
      }
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
      if (widget === 'procurement_allocations' && key === 'bill_id') {
        const bill = catalog.billsByID[String(nextValue || '')]
        const billNumber = String(resolvePath(bill, 'header.number') || '')
        const openAmount = toNumber(resolvePath(bill, 'body.payload.balance_due_amount'))
        const vendorID = String(resolvePath(bill, 'body.payload.vendor_id') || '')
        const vendorName = String(resolvePath(bill, 'body.payload.vendor_name') || '')
        const currencyCode = String(resolvePath(bill, 'body.payload.currency_code') || '')
        if (billNumber) updated.bill_number = billNumber
        if (!toNumber(updated.amount) && openAmount > 0) updated.amount = openAmount
        patch = {
          amount_paid: openAmount > 0 ? openAmount : undefined,
          vendor_id: vendorID || undefined,
          vendor_name: vendorName || undefined,
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
      if (widget === 'trace_batches' && key === 'batch_id') {
        const batch = catalog.inventoryBatchesByID[String(nextValue || '')]
        updated.item_code = String(resolvePath(batch, 'values.item_code') || '')
        updated.warehouse_code = String(resolvePath(batch, 'values.warehouse_code') || '')
        updated.batch_code = String(resolvePath(batch, 'values.batch_code') || '')
        updated.expiration_date = String(resolvePath(batch, 'values.expiration_date') || '')
        updated.status = String(resolvePath(batch, 'values.status') || '')
        updated.on_hand_quantity = toNumber(resolvePath(batch, 'values.on_hand_quantity'))
        updated.reserved_quantity = toNumber(resolvePath(batch, 'values.reserved_quantity'))
        updated.available_quantity = toNumber(resolvePath(batch, 'values.available_quantity'))
      }
      if (widget === 'production_component_lines' && key === 'component_item_code' && !String(updated.actual_item_code || '')) {
        updated.actual_item_code = String(nextValue || '')
      }
      if (widget === 'production_issue_lines' && key === 'actual_item_code') {
        updated.item_code = String(nextValue || '')
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

  function autoAllocateProcurement(useFullOpenBalance: boolean) {
    if (widget !== 'procurement_allocations') return
    const targetAmount = useFullOpenBalance ? openBillBalance : toNumber(resolvePath(values, 'amount_paid'))
    const { rows: nextRows, allocatedAmount } = buildProcurementAllocationRows(openBills, targetAmount)
    onChange(normalizeCommercialRows(nextRows, widget, catalog), {
      amount_paid: allocatedAmount,
      replace_amount_paid: useFullOpenBalance || targetAmount <= 0,
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
      {widget === 'procurement_allocations' ? (
        <div className="flex flex-wrap items-center gap-3 rounded-lg border border-line bg-accent-soft/30 px-3 py-2 text-sm text-body">
          <span>
            Open bills for vendor: <strong>{openBills.length}</strong>
          </span>
          <span>
            Open balance: <strong>{roundMoney(openBillBalance)}</strong>
          </span>
          <button type="button" onClick={() => autoAllocateProcurement(false)} className="rounded-lg border border-line px-3 py-2 text-body" disabled={!openBills.length}>
            Auto Allocate Payment
          </button>
          <button type="button" onClick={() => autoAllocateProcurement(true)} className="rounded-lg border border-line px-3 py-2 text-body" disabled={!openBills.length}>
            Use Full Open Balance
          </button>
          <button type="button" onClick={() => onChange([])} className="rounded-lg border border-line px-3 py-2 text-body" disabled={!rows.length}>
            Clear Allocations
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
                    ) : resolveCommercialColumnOptions(column.key, widget, row, catalog, values, column.options).length ? (
                      (() => {
                        const options = resolveCommercialColumnOptions(column.key, widget, row, catalog, values, column.options)
                        return (
                      <select
                        id={`field-${fieldKey}-${index}-${column.key}`}
                        name={`${fieldKey}[${index}].${column.key}`}
                        className="h-10 w-full rounded-lg border border-line bg-surface px-3 py-2 text-sm text-body"
                        value={String(row[column.key] ?? '')}
                        onChange={(event) => updateRow(index, column.key, event.target.value)}
                      >
                        <option value="">Select an option</option>
                        {options.map((option) => (
                          <option key={option.value} value={option.value}>
                            {option.label}
                          </option>
                        ))}
                      </select>
                        )
                      })()
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
  if (
    field.widget === 'commercial_lines' ||
    field.widget === 'commercial_allocations' ||
    field.widget === 'commercial_refund_allocations' ||
    field.widget === 'commercial_journal_lines' ||
    field.widget === 'procurement_lines' ||
    field.widget === 'procurement_receipt_lines' ||
    field.widget === 'procurement_allocations' ||
    field.widget === 'fulfillment_lines' ||
    field.widget === 'delivery_lines' ||
    field.widget === 'return_lines' ||
    field.widget === 'supplier_return_lines' ||
    field.widget === 'inventory_lines' ||
    field.widget === 'inventory_transfer_lines' ||
    field.widget === 'production_component_lines' ||
    field.widget === 'production_issue_lines' ||
    field.widget === 'production_stage_lines' ||
    field.widget === 'trace_batches'
  ) {
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
  if (field.widget === 'json') {
    return <pre className="overflow-x-auto whitespace-pre-wrap text-xs text-body">{JSON.stringify(value ?? {}, null, 2)}</pre>
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
        { key: 'product_code', label: 'Product', type: 'text', options: commercialSelectOptions('product_code', catalog) },
        { key: 'variant_signature', label: 'Variant', type: 'text' },
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
    case 'procurement_lines':
      return [
        { key: 'product_code', label: 'Product', type: 'text', options: commercialSelectOptions('product_code', catalog) },
        { key: 'variant_signature', label: 'Variant', type: 'text' },
        { key: 'item_code', label: 'Item', type: 'text', options: commercialSelectOptions('item_code', catalog) },
        { key: 'description', label: 'Description', type: 'text' },
        { key: 'uom_code', label: 'UOM', type: 'text', options: commercialSelectOptions('uom_code', catalog) },
        { key: 'quantity', label: 'Qty', type: 'number' },
        { key: 'received_qty', label: 'Received', type: 'number' },
        { key: 'billed_qty', label: 'Billed', type: 'number' },
        { key: 'unit_price', label: 'Unit Price', type: 'number' },
        { key: 'tax_code', label: 'Tax Code', type: 'text', options: commercialSelectOptions('tax_code', catalog) },
        { key: 'line_total', label: 'Total', type: 'number', readOnly: true },
      ]
    case 'procurement_receipt_lines':
      return [
        { key: 'product_code', label: 'Product', type: 'text', readOnly: true },
        { key: 'variant_signature', label: 'Variant', type: 'text', readOnly: true },
        { key: 'item_code', label: 'Item', type: 'text', readOnly: true },
        { key: 'description', label: 'Description', type: 'text', readOnly: true },
        { key: 'uom_code', label: 'UOM', type: 'text', readOnly: true },
        { key: 'ordered_qty', label: 'Ordered', type: 'number', readOnly: true },
        { key: 'received_qty', label: 'Receive Now', type: 'number' },
        { key: 'cumulative_received_qty', label: 'Cumulative', type: 'number', readOnly: true },
        { key: 'warehouse_code', label: 'Warehouse', type: 'text', options: commercialSelectOptions('warehouse_code', catalog) },
        { key: 'batch_code', label: 'Batch', type: 'text' },
        { key: 'expiration_date', label: 'Expiration', type: 'text' },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'procurement_allocations':
      return [
        { key: 'bill_number', label: 'Bill', type: 'text', readOnly: true },
        { key: 'bill_id', label: 'Bill ID', type: 'text', options: commercialSelectOptions('bill_id', catalog, values) },
        { key: 'amount', label: 'Amount', type: 'number' },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'fulfillment_lines':
      return [
        { key: 'product_code', label: 'Product', type: 'text', readOnly: true },
        { key: 'variant_signature', label: 'Variant', type: 'text', readOnly: true },
        { key: 'item_code', label: 'Item', type: 'text', readOnly: true },
        { key: 'description', label: 'Description', type: 'text', readOnly: true },
        { key: 'warehouse_code', label: 'Warehouse', type: 'text', options: commercialSelectOptions('warehouse_code', catalog) },
        { key: 'batch_code', label: 'Batch', type: 'text' },
        { key: 'expiration_date', label: 'Expiration', type: 'text', readOnly: true },
        { key: 'uom_code', label: 'UOM', type: 'text', readOnly: true },
        { key: 'ordered_quantity', label: 'Ordered', type: 'number', readOnly: true },
        { key: 'quantity', label: 'Reserve', type: 'number' },
        { key: 'available_quantity', label: 'Available', type: 'number', readOnly: true },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'delivery_lines':
      return [
        { key: 'product_code', label: 'Product', type: 'text', readOnly: true },
        { key: 'variant_signature', label: 'Variant', type: 'text', readOnly: true },
        { key: 'item_code', label: 'Item', type: 'text', readOnly: true },
        { key: 'description', label: 'Description', type: 'text', readOnly: true },
        { key: 'warehouse_code', label: 'Warehouse', type: 'text', options: commercialSelectOptions('warehouse_code', catalog) },
        { key: 'batch_code', label: 'Batch', type: 'text', readOnly: true },
        { key: 'expiration_date', label: 'Expiration', type: 'text', readOnly: true },
        { key: 'uom_code', label: 'UOM', type: 'text', readOnly: true },
        { key: 'fulfilled_quantity', label: 'Fulfilled', type: 'number', readOnly: true },
        { key: 'quantity', label: 'Deliver Qty', type: 'number' },
        { key: 'tracking_number', label: 'Tracking', type: 'text' },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'return_lines':
      return [
        { key: 'product_code', label: 'Product', type: 'text', readOnly: true },
        { key: 'variant_signature', label: 'Variant', type: 'text', readOnly: true },
        { key: 'item_code', label: 'Item', type: 'text', readOnly: true },
        { key: 'description', label: 'Description', type: 'text', readOnly: true },
        { key: 'warehouse_code', label: 'Warehouse', type: 'text', options: commercialSelectOptions('warehouse_code', catalog) },
        { key: 'batch_code', label: 'Batch', type: 'text' },
        { key: 'expiration_date', label: 'Expiration', type: 'text' },
        { key: 'uom_code', label: 'UOM', type: 'text', readOnly: true },
        { key: 'fulfilled_quantity', label: 'Fulfilled', type: 'number', readOnly: true },
        { key: 'quantity', label: 'Return Qty', type: 'number' },
        {
          key: 'disposition',
          label: 'Disposition',
          type: 'text',
          options: [
            { value: 'restock', label: 'Restock' },
            { value: 'quarantine', label: 'Quarantine' },
            { value: 'block', label: 'Block' },
          ],
        },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'supplier_return_lines':
      return [
        { key: 'item_code', label: 'Item', type: 'text', readOnly: true },
        { key: 'description', label: 'Description', type: 'text', readOnly: true },
        { key: 'warehouse_code', label: 'Warehouse', type: 'text', options: commercialSelectOptions('warehouse_code', catalog) },
        { key: 'batch_code', label: 'Batch', type: 'text' },
        { key: 'expiration_date', label: 'Expiration', type: 'text' },
        { key: 'uom_code', label: 'UOM', type: 'text', readOnly: true },
        { key: 'received_quantity', label: 'Received', type: 'number', readOnly: true },
        { key: 'quantity', label: 'Return Qty', type: 'number' },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'inventory_lines':
      return [
        { key: 'product_code', label: 'Product', type: 'text', options: commercialSelectOptions('product_code', catalog) },
        { key: 'variant_signature', label: 'Variant', type: 'text' },
        { key: 'item_code', label: 'Item', type: 'text', options: commercialSelectOptions('item_code', catalog) },
        { key: 'description', label: 'Description', type: 'text' },
        { key: 'warehouse_code', label: 'Warehouse', type: 'text', options: commercialSelectOptions('warehouse_code', catalog) },
        { key: 'batch_code', label: 'Batch', type: 'text' },
        { key: 'expiration_date', label: 'Expiration', type: 'text' },
        { key: 'uom_code', label: 'UOM', type: 'text', options: commercialSelectOptions('uom_code', catalog) },
        { key: 'quantity', label: 'Quantity', type: 'number' },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'inventory_transfer_lines':
      return [
        { key: 'product_code', label: 'Product', type: 'text', options: commercialSelectOptions('product_code', catalog) },
        { key: 'variant_signature', label: 'Variant', type: 'text' },
        { key: 'item_code', label: 'Item', type: 'text', options: commercialSelectOptions('item_code', catalog) },
        { key: 'description', label: 'Description', type: 'text' },
        { key: 'source_warehouse_code', label: 'Source Warehouse', type: 'text', options: commercialSelectOptions('source_warehouse_code', catalog) },
        { key: 'target_warehouse_code', label: 'Target Warehouse', type: 'text', options: commercialSelectOptions('target_warehouse_code', catalog) },
        { key: 'batch_code', label: 'Batch', type: 'text' },
        { key: 'expiration_date', label: 'Expiration', type: 'text' },
        { key: 'uom_code', label: 'UOM', type: 'text', options: commercialSelectOptions('uom_code', catalog) },
        { key: 'quantity', label: 'Quantity', type: 'number' },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'production_component_lines':
      return [
        { key: 'component_item_code', label: 'Component Item', type: 'text', options: commercialSelectOptions('item_code', catalog) },
        { key: 'actual_item_code', label: 'Actual Item', type: 'text', options: commercialSelectOptions('item_code', catalog) },
        { key: 'description', label: 'Description', type: 'text' },
        { key: 'warehouse_code', label: 'Warehouse', type: 'text', options: commercialSelectOptions('warehouse_code', catalog) },
        { key: 'uom_code', label: 'UOM', type: 'text', options: commercialSelectOptions('uom_code', catalog) },
        { key: 'quantity_per_unit', label: 'Qty / Unit', type: 'number' },
        { key: 'quantity', label: 'Planned Qty', type: 'number' },
        { key: 'issued_quantity', label: 'Issued Qty', type: 'number', readOnly: true },
        { key: 'reserved_quantity', label: 'Reserved Qty', type: 'number', readOnly: true },
        { key: 'shortage_quantity', label: 'Shortage Qty', type: 'number', readOnly: true },
        { key: 'available_quantity', label: 'Available', type: 'number', readOnly: true },
        { key: 'allowed_substitute_item_codes', label: 'Allowed Subs', type: 'text' },
        { key: 'reservation_status', label: 'Reservation', type: 'text', readOnly: true },
        { key: 'substitution_status', label: 'Substitution', type: 'text', readOnly: true },
      ]
    case 'production_issue_lines':
      return [
        { key: 'planned_item_code', label: 'Planned Item', type: 'text', readOnly: true },
        { key: 'actual_item_code', label: 'Actual Item', type: 'text', options: commercialSelectOptions('item_code', catalog) },
        { key: 'description', label: 'Description', type: 'text' },
        { key: 'warehouse_code', label: 'Warehouse', type: 'text', options: commercialSelectOptions('warehouse_code', catalog) },
        { key: 'batch_code', label: 'Batch', type: 'text' },
        { key: 'expiration_date', label: 'Expiration', type: 'text' },
        { key: 'uom_code', label: 'UOM', type: 'text', options: commercialSelectOptions('uom_code', catalog) },
        { key: 'quantity', label: 'Issue Qty', type: 'number' },
        { key: 'reserved_quantity', label: 'Reserved Qty', type: 'number', readOnly: true },
        { key: 'available_quantity', label: 'Available', type: 'number', readOnly: true },
        { key: 'allowed_substitute_item_codes', label: 'Allowed Subs', type: 'text', readOnly: true },
        { key: 'substitution_status', label: 'Substitution', type: 'text', readOnly: true },
      ]
    case 'production_stage_lines':
      return [
        { key: 'stage_code', label: 'Stage Code', type: 'text' },
        { key: 'stage_name', label: 'Stage', type: 'text' },
        { key: 'sequence', label: 'Seq', type: 'number' },
        { key: 'work_center_code', label: 'Work Center', type: 'text', options: commercialSelectOptions('work_center_code', catalog) },
        { key: 'status', label: 'Status', type: 'text', options: [
          { value: 'pending', label: 'Pending' },
          { value: 'ready', label: 'Ready' },
          { value: 'in_progress', label: 'In Progress' },
          { value: 'completed', label: 'Completed' },
          { value: 'skipped', label: 'Skipped' },
        ] },
        { key: 'required', label: 'Required', type: 'text', options: [
          { value: 'true', label: 'Yes' },
          { value: 'false', label: 'No' },
        ] },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'trace_batches':
      return [
        { key: 'batch_id', label: 'Batch', type: 'text', options: commercialSelectOptions('batch_id', catalog) },
        { key: 'item_code', label: 'Item', type: 'text', readOnly: true },
        { key: 'warehouse_code', label: 'Warehouse', type: 'text', readOnly: true },
        { key: 'batch_code', label: 'Batch Code', type: 'text', readOnly: true },
        { key: 'expiration_date', label: 'Expiration', type: 'text', readOnly: true },
        { key: 'status', label: 'Status', type: 'text', readOnly: true },
        { key: 'available_quantity', label: 'Available', type: 'number', readOnly: true },
      ]
    default:
      return []
  }
}

function commercialArrayDefaultRow(widget: string): Record<string, unknown> {
  switch (widget) {
    case 'commercial_lines':
      return { product_code: '', variant_signature: '', item_code: '', description: '', uom_code: '', quantity: 1, unit_price: 0, discount_amount: 0, tax_code: '', tax_rate: 0, line_subtotal: 0, tax_amount: 0, line_total: 0 }
    case 'commercial_allocations':
      return { invoice_number: '', invoice_id: '', amount: 0, note: '' }
    case 'commercial_refund_allocations':
      return { payment_number: '', payment_id: '', amount: 0, note: '' }
    case 'commercial_journal_lines':
      return { account_code: '', description: '', debit: 0, credit: 0 }
    case 'procurement_lines':
      return { product_code: '', variant_signature: '', item_code: '', description: '', uom_code: '', quantity: 1, received_qty: 0, billed_qty: 0, unit_price: 0, tax_code: '', tax_rate: 0, line_subtotal: 0, tax_amount: 0, line_total: 0 }
    case 'procurement_receipt_lines':
      return { product_code: '', variant_signature: '', item_code: '', description: '', uom_code: '', ordered_qty: 0, received_qty: 0, cumulative_received_qty: 0, warehouse_code: '', batch_code: '', expiration_date: '', note: '' }
    case 'procurement_allocations':
      return { bill_number: '', bill_id: '', amount: 0, note: '' }
    case 'fulfillment_lines':
      return { product_code: '', variant_signature: '', item_code: '', description: '', warehouse_code: '', batch_code: '', expiration_date: '', uom_code: '', ordered_quantity: 0, quantity: 0, available_quantity: 0, note: '' }
    case 'delivery_lines':
      return { product_code: '', variant_signature: '', item_code: '', description: '', warehouse_code: '', batch_code: '', expiration_date: '', uom_code: '', fulfilled_quantity: 0, quantity: 0, tracking_number: '', note: '' }
    case 'return_lines':
      return { product_code: '', variant_signature: '', item_code: '', description: '', warehouse_code: '', batch_code: '', expiration_date: '', uom_code: '', fulfilled_quantity: 0, quantity: 0, disposition: 'restock', note: '' }
    case 'supplier_return_lines':
      return { item_code: '', description: '', warehouse_code: '', batch_code: '', expiration_date: '', uom_code: '', received_quantity: 0, quantity: 0, note: '' }
    case 'inventory_lines':
      return { product_code: '', variant_signature: '', item_code: '', description: '', warehouse_code: '', batch_code: '', expiration_date: '', uom_code: '', quantity: 0, note: '' }
    case 'inventory_transfer_lines':
      return { product_code: '', variant_signature: '', item_code: '', description: '', source_warehouse_code: '', target_warehouse_code: '', batch_code: '', expiration_date: '', uom_code: '', quantity: 0, note: '' }
    case 'production_component_lines':
      return { component_item_code: '', actual_item_code: '', description: '', warehouse_code: '', uom_code: '', quantity_per_unit: 0, quantity: 0, issued_quantity: 0, reserved_quantity: 0, shortage_quantity: 0, available_quantity: 0, allowed_substitute_item_codes: '', reservation_status: 'unreserved', substitution_status: 'planned' }
    case 'production_issue_lines':
      return { planned_item_code: '', actual_item_code: '', description: '', warehouse_code: '', batch_code: '', expiration_date: '', uom_code: '', quantity: 0, reserved_quantity: 0, available_quantity: 0, allowed_substitute_item_codes: '', substitution_status: 'planned' }
    case 'production_stage_lines':
      return { stage_code: '', stage_name: '', sequence: 1, work_center_code: '', status: 'pending', required: 'true', note: '' }
    case 'trace_batches':
      return { batch_id: '', item_code: '', warehouse_code: '', batch_code: '', expiration_date: '', status: '', on_hand_quantity: 0, reserved_quantity: 0, available_quantity: 0 }
    default:
      return {}
  }
}

function commercialSelectOptions(path: string, catalog?: CommercialFormCatalog, values?: FormState): Array<{ value: string; label: string }> {
  if (!catalog) return []
  switch (path) {
    case 'product_code':
      return Object.values(catalog.productsByCode)
        .map((item) => {
          const code = String(resolvePath(item, 'values.code') || '')
          const name = String(resolvePath(item, 'values.name') || '')
          return { value: code, label: name ? `${code} - ${name}` : code }
        })
        .filter((option) => option.value)
        .sort((left, right) => left.label.localeCompare(right.label))
    case 'dimension_code':
      return Object.values(catalog.variantDimensionsByCode)
        .map((item) => {
          const code = String(resolvePath(item, 'values.code') || '')
          const name = String(resolvePath(item, 'values.name') || '')
          return { value: code, label: name ? `${code} - ${name}` : code }
        })
        .filter((option) => option.value)
        .sort((left, right) => left.label.localeCompare(right.label))
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
    case 'bom_id':
      return Object.values(catalog.bomsByID)
        .map((item) => {
          const id = String(item.id || '')
          const code = String(resolvePath(item, 'values.code') || '')
          const name = String(resolvePath(item, 'values.name') || '')
          return { value: id, label: [code, name].filter(Boolean).join(' - ') || id }
        })
        .filter((option) => option.value)
        .sort((left, right) => left.label.localeCompare(right.label))
    case 'bom_version_id':
      return Object.values(catalog.bomVersionsByID)
        .map((item) => {
          const id = String(item.id || '')
          const bomCode = String(resolvePath(item, 'values.bom_code') || '')
          const versionCode = String(resolvePath(item, 'values.version_code') || '')
          return { value: id, label: [bomCode, versionCode].filter(Boolean).join(' - ') || id }
        })
        .filter((option) => option.value)
        .sort((left, right) => left.label.localeCompare(right.label))
    case 'warehouse_code':
    case 'source_warehouse_code':
    case 'target_warehouse_code':
      return Object.values(catalog.warehousesByCode)
        .map((item) => {
          const code = String(resolvePath(item, 'values.code') || '')
          const name = String(resolvePath(item, 'values.name') || '')
          return { value: code, label: name ? `${code} - ${name}` : code }
        })
        .filter((option) => option.value)
        .sort((left, right) => left.label.localeCompare(right.label))
    case 'work_center_code':
      return Object.values(catalog.workCentersByCode)
        .map((item) => {
          const code = String(resolvePath(item, 'values.code') || '')
          const name = String(resolvePath(item, 'values.name') || '')
          return { value: code, label: name ? `${code} - ${name}` : code }
        })
        .filter((option) => option.value)
        .sort((left, right) => left.label.localeCompare(right.label))
    case 'batch_id':
      return Object.values(catalog.inventoryBatchesByID)
        .map((item) => {
          const id = String(resolvePath(item, 'id') || '')
          const itemCode = String(resolvePath(item, 'values.item_code') || '')
          const warehouseCode = String(resolvePath(item, 'values.warehouse_code') || '')
          const batchCode = String(resolvePath(item, 'values.batch_code') || '')
          return { value: id, label: [itemCode, warehouseCode, batchCode].filter(Boolean).join(' - ') || id }
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
    case 'vendor_id':
      return Object.entries(catalog.vendorsByID)
        .filter(([value]) => value)
        .map(([value, item]) => ({
          value,
          label: String(resolvePath(item, 'values.vendor_name') || value),
        }))
        .sort((left, right) => left.label.localeCompare(right.label))
    case 'bill_id':
      return procurementOpenBills(catalog, values)
        .map((item) => {
          const id = String(resolvePath(item, 'header.id') || '')
          const number = String(resolvePath(item, 'header.number') || id)
          const vendorName = String(resolvePath(item, 'body.payload.vendor_name') || '')
          const balance = toNumber(resolvePath(item, 'body.payload.balance_due_amount'))
          return {
            value: id,
            label: `${number}${vendorName ? ` - ${vendorName}` : ''}${balance > 0 ? ` (${balance})` : ''}`,
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
    case 'finished_item_code':
      return Object.values(catalog.itemsByCode)
        .map((item) => {
          const code = String(resolvePath(item, 'values.sku') || '')
          const name = String(resolvePath(item, 'values.name') || '')
          const variantLabel = String(resolvePath(item, 'values.variant_label') || '')
          return { value: code, label: [code, name, variantLabel].filter(Boolean).join(' - ') }
        })
        .filter((option) => option.value)
        .sort((left, right) => left.label.localeCompare(right.label))
    default:
      return []
  }
}

function resolveCommercialColumnOptions(
  key: string,
  _widget: string,
  row: Record<string, unknown>,
  catalog?: CommercialFormCatalog,
  values?: FormState,
  fallback?: Array<{ value: string; label: string }>,
): Array<{ value: string; label: string }> {
  if (!catalog) return fallback || []
  if (key === 'variant_signature') {
    return variantSignatureOptions(String(row.product_code || ''), catalog)
  }
  return fallback || commercialSelectOptions(key, catalog, values)
}

function variantSignatureOptions(productCode: string, catalog?: CommercialFormCatalog): Array<{ value: string; label: string }> {
  if (!catalog || !productCode) return []
  return Object.values(catalog.itemsByCode)
    .filter((item) => String(resolvePath(item, 'values.product_code') || '') === productCode)
    .map((item) => ({
      value: String(resolvePath(item, 'values.variant_signature') || ''),
      label: String(resolvePath(item, 'values.variant_label') || resolvePath(item, 'values.variant_signature') || resolvePath(item, 'values.sku') || ''),
    }))
    .filter((option) => option.value)
    .sort((left, right) => left.label.localeCompare(right.label))
}

function resolveVariantItemCodeFromRow(row: Record<string, unknown>, catalog?: CommercialFormCatalog): string {
  if (!catalog) return String(row.item_code || '')
  const productCode = String(row.product_code || '')
  const variantSignature = String(row.variant_signature || '')
  if (!productCode || !variantSignature) return String(row.item_code || '')
  const match = Object.values(catalog.itemsByCode).find((item) =>
    String(resolvePath(item, 'values.product_code') || '') === productCode &&
    String(resolvePath(item, 'values.variant_signature') || '') === variantSignature,
  )
  return String(resolvePath(match, 'values.sku') || row.item_code || '')
}

function parseDimensionCodes(value: unknown): string[] {
  return String(value || '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function hasUnresolvedVariantSelections(values: FormState): boolean {
  const collections = ['lines', 'allocations', 'refund_allocations']
  for (const key of collections) {
    const rows = asRecordList(resolvePath(values, key))
    if (rows.some((row) => String(row.product_code || '') !== '' && String(row.item_code || '') === '')) {
      return true
    }
  }
  return false
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

function procurementOpenBills(catalog?: CommercialFormCatalog, values?: FormState): Array<Record<string, unknown>> {
  if (!catalog) return []
  const vendorID = String(resolvePath(values || {}, 'vendor_id') || '')
  return Object.values(catalog.billsByID)
    .filter((item) => {
      const balance = toNumber(resolvePath(item, 'body.payload.balance_due_amount'))
      if (balance <= 0) return false
      if (!vendorID) return true
      return String(resolvePath(item, 'body.payload.vendor_id') || '') === vendorID
    })
    .sort((left, right) => {
      const leftDue = String(resolvePath(left, 'body.payload.due_date') || '')
      const rightDue = String(resolvePath(right, 'body.payload.due_date') || '')
      if (leftDue !== rightDue) return leftDue.localeCompare(rightDue)
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

function buildProcurementAllocationRows(bills: Array<Record<string, unknown>>, requestedAmount: number): { rows: Array<Record<string, unknown>>; allocatedAmount: number } {
  let remaining = roundMoney(requestedAmount)
  const allocateAll = remaining <= 0
  const rows: Array<Record<string, unknown>> = []
  let allocatedAmount = 0
  for (const bill of bills) {
    const balance = roundMoney(toNumber(resolvePath(bill, 'body.payload.balance_due_amount')))
    if (balance <= 0) continue
    const allocationAmount = allocateAll ? balance : roundMoney(Math.min(balance, remaining))
    if (allocationAmount <= 0) continue
    rows.push({
      bill_id: String(resolvePath(bill, 'header.id') || ''),
      bill_number: String(resolvePath(bill, 'header.number') || ''),
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
    case 'procurement_lines': {
      const rowsWithDefaults = rows.map((row) => ({
        ...row,
        tax_code: row.tax_code || String(resolvePath(current, 'default_tax_code') || ''),
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
    case 'procurement_receipt_lines': {
      const normalizedRows = rows.map((row) => {
        const orderedQty = toNumber(row.ordered_qty || row.quantity)
        const receivedQty = Math.max(toNumber(row.received_qty), 0)
        const previouslyReceived = Math.max(toNumber(row.previously_received_qty), 0)
        return {
          ...row,
          ordered_qty: orderedQty,
          received_qty: receivedQty,
          cumulative_received_qty: roundMoney(previouslyReceived + receivedQty),
        }
      })
      next = assignPathValue(next, path, normalizedRows)
      return next
    }
    case 'procurement_allocations': {
      const normalizedRows = rows.map((row) => ({ ...row, amount: toNumber(row.amount) }))
      const currentAmountPaid = toNumber(resolvePath(current, 'amount_paid'))
      const appliedAmount = normalizedRows.reduce((sum, row) => sum + toNumber(row.amount), 0)
      const amountPaid = currentAmountPaid > 0 ? currentAmountPaid : roundMoney(appliedAmount)
      next = assignPathValue(next, path, normalizedRows)
      next = assignPathValue(next, 'amount_paid', amountPaid)
      next = assignPathValue(next, 'unapplied_amount', roundMoney(Math.max(amountPaid - appliedAmount, 0)))
      return next
    }
    case 'fulfillment_lines': {
      const normalizedRows = normalizeCommercialRows(rows, widget, catalog, current)
      next = assignPathValue(next, path, normalizedRows)
      next = assignPathValue(next, 'reserved_quantity_total', roundMoney(normalizedRows.reduce((sum, row) => sum + toNumber(row.quantity), 0)))
      return next
    }
    case 'delivery_lines': {
      const normalizedRows = normalizeCommercialRows(rows, widget, catalog, current)
      next = assignPathValue(next, path, normalizedRows)
      next = assignPathValue(next, 'delivered_quantity_total', roundMoney(normalizedRows.reduce((sum, row) => sum + toNumber(row.quantity), 0)))
      return next
    }
    case 'return_lines': {
      const normalizedRows = normalizeCommercialRows(rows, widget, catalog, current)
      next = assignPathValue(next, path, normalizedRows)
      next = assignPathValue(next, 'total_quantity', roundMoney(normalizedRows.reduce((sum, row) => sum + toNumber(row.quantity), 0)))
      return next
    }
    case 'supplier_return_lines': {
      const normalizedRows = normalizeCommercialRows(rows, widget, catalog, current)
      next = assignPathValue(next, path, normalizedRows)
      next = assignPathValue(next, 'total_quantity', roundMoney(normalizedRows.reduce((sum, row) => sum + toNumber(row.quantity), 0)))
      return next
    }
    case 'inventory_lines': {
      const normalizedRows = normalizeCommercialRows(rows, widget, catalog, current)
      next = assignPathValue(next, path, normalizedRows)
      next = assignPathValue(next, 'total_quantity', roundMoney(normalizedRows.reduce((sum, row) => sum + toNumber(row.quantity), 0)))
      return next
    }
    case 'production_component_lines': {
      const normalizedRows = normalizeCommercialRows(rows, widget, catalog, current)
      next = assignPathValue(next, path, normalizedRows)
      next = assignPathValue(next, 'reserved_quantity_total', roundMoney(normalizedRows.reduce((sum, row) => sum + toNumber(row.reserved_quantity), 0)))
      next = assignPathValue(next, 'shortage_quantity_total', roundMoney(normalizedRows.reduce((sum, row) => sum + toNumber(row.shortage_quantity), 0)))
      return next
    }
    case 'production_issue_lines': {
      const normalizedRows = normalizeCommercialRows(rows, widget, catalog, current)
      next = assignPathValue(next, path, normalizedRows)
      next = assignPathValue(next, 'total_quantity', roundMoney(normalizedRows.reduce((sum, row) => sum + toNumber(row.quantity), 0)))
      return next
    }
    case 'production_stage_lines': {
      const normalizedRows = normalizeCommercialRows(rows, widget, catalog, current)
      next = assignPathValue(next, path, normalizedRows)
      return next
    }
    case 'trace_batches': {
      const normalizedRows = normalizeCommercialRows(rows, widget, catalog, current)
      next = assignPathValue(next, path, normalizedRows)
      return next
    }
    case 'inventory_transfer_lines': {
      const normalizedRows = normalizeCommercialRows(rows, widget, catalog, current)
      next = assignPathValue(next, path, normalizedRows)
      next = assignPathValue(next, 'total_quantity', roundMoney(normalizedRows.reduce((sum, row) => sum + toNumber(row.quantity), 0)))
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
  if (path === 'product_code') {
    const productCode = String(value || '')
    const product = catalog?.productsByCode?.[productCode]
    let withProduct = next
    const inheritedFields = ['category_code', 'uom_code', 'currency_code', 'tax_code', 'revenue_account_code', 'inventory_tracking_mode', 'default_issue_strategy']
    for (const fieldKey of inheritedFields) {
      const inheritedValue = resolvePath(product, `values.${fieldKey}`)
      if ((typeof inheritedValue === 'string' && inheritedValue && !String(resolvePath(withProduct, fieldKey) || '')) || (typeof inheritedValue === 'number' && !Number.isNaN(inheritedValue as number) && !resolvePath(withProduct, fieldKey))) {
        withProduct = assignPathValue(withProduct, fieldKey, inheritedValue)
      }
    }
    for (const fieldKey of ['inventory_enabled', 'expiry_tracking_enabled', 'allow_negative_stock']) {
      const inheritedValue = resolvePath(product, `values.${fieldKey}`)
      if (typeof inheritedValue === 'boolean' && !resolvePath(withProduct, fieldKey)) {
        withProduct = assignPathValue(withProduct, fieldKey, inheritedValue)
      }
    }
    if (!String(resolvePath(withProduct, 'name') || '')) {
      const productName = String(resolvePath(product, 'values.name') || '')
      if (productName) withProduct = assignPathValue(withProduct, 'name', productName)
    }
    return withProduct
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
  if (path === 'vendor_id') {
    const vendorID = String(value || '')
    const vendor = catalog?.vendorsByID?.[vendorID]
    let withVendor = next
    const vendorName = String(resolvePath(vendor, 'values.vendor_name') || '')
    const currencyCode = String(resolvePath(vendor, 'values.currency_code') || '')
    const taxProfileCode = String(resolvePath(vendor, 'values.tax_profile_code') || '')
    const paymentTermDays = resolvePath(vendor, 'values.payment_term_days')
    const payableAccount = String(resolvePath(vendor, 'values.payable_account_code') || '')
    const expenseAccount = String(resolvePath(vendor, 'values.expense_account_code') || '')
    const defaultPaymentMethod = String(resolvePath(vendor, 'values.default_payment_method_code') || '')
    if (vendorName) withVendor = assignPathValue(withVendor, 'vendor_name', vendorName)
    if (currencyCode && !String(resolvePath(withVendor, 'currency_code') || '')) {
      withVendor = assignPathValue(withVendor, 'currency_code', currencyCode)
    }
    if (taxProfileCode && !String(resolvePath(withVendor, 'tax_profile_code') || '')) {
      withVendor = assignPathValue(withVendor, 'tax_profile_code', taxProfileCode)
    }
    if (paymentTermDays != null && paymentTermDays !== '' && !toNumber(resolvePath(withVendor, 'payment_term_days'))) {
      withVendor = assignPathValue(withVendor, 'payment_term_days', toNumber(paymentTermDays))
      const baseDate = String(resolvePath(withVendor, 'bill_date') || resolvePath(withVendor, 'order_date') || '')
      if (baseDate) {
        const dueDate = addDaysToDate(baseDate, toNumber(paymentTermDays))
        if (dueDate) {
          withVendor = assignPathValue(withVendor, 'due_date', dueDate)
        }
      }
    }
    if (payableAccount && !String(resolvePath(withVendor, 'payable_account_code') || '')) {
      withVendor = assignPathValue(withVendor, 'payable_account_code', payableAccount)
    }
    if (expenseAccount && !String(resolvePath(withVendor, 'expense_account_code') || '')) {
      withVendor = assignPathValue(withVendor, 'expense_account_code', expenseAccount)
    }
    if (defaultPaymentMethod && !String(resolvePath(withVendor, 'payment_method_code') || '')) {
      withVendor = applyFieldUpdate(withVendor, 'payment_method_code', defaultPaymentMethod, catalog)
    }
    const lines = asRecordList(resolvePath(withVendor, 'lines'))
    if (lines.length && String(resolvePath(withVendor, 'source_purchase_order_id') || '') !== '') {
      return applyCommercialArrayUpdate(withVendor, 'lines', 'procurement_receipt_lines', lines, catalog)
    }
    if (lines.length) {
      return applyCommercialArrayUpdate(withVendor, 'lines', 'procurement_lines', lines, catalog)
    }
    return withVendor
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
  if (path === 'amount_paid') {
    const allocations = asRecordList(resolvePath(next, 'allocations'))
    if (allocations.length) {
      return applyCommercialArrayUpdate(next, 'allocations', 'procurement_allocations', allocations, catalog)
    }
    return assignPathValue(next, 'unapplied_amount', roundMoney(Math.max(toNumber(value), 0)))
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
  const vendorID = String(resolvePath(next, 'vendor_id') || '')
  const vendor = catalog.vendorsByID[vendorID]
  if (vendor) {
    const vendorName = String(resolvePath(vendor, 'values.vendor_name') || '')
    const currencyCode = String(resolvePath(vendor, 'values.currency_code') || '')
    const taxProfileCode = String(resolvePath(vendor, 'values.tax_profile_code') || '')
    const paymentTermDays = toNumber(resolvePath(vendor, 'values.payment_term_days'))
    const payableAccount = String(resolvePath(vendor, 'values.payable_account_code') || '')
    const expenseAccount = String(resolvePath(vendor, 'values.expense_account_code') || '')
    const defaultPaymentMethod = String(resolvePath(vendor, 'values.default_payment_method_code') || '')
    if (!resolvePath(next, 'vendor_name') && vendorName) next = assignPathValue(next, 'vendor_name', vendorName)
    if (!resolvePath(next, 'currency_code') && currencyCode) next = assignPathValue(next, 'currency_code', currencyCode)
    if (!resolvePath(next, 'tax_profile_code') && taxProfileCode) next = assignPathValue(next, 'tax_profile_code', taxProfileCode)
    if (!resolvePath(next, 'payment_term_days') && paymentTermDays > 0) next = assignPathValue(next, 'payment_term_days', paymentTermDays)
    if (!resolvePath(next, 'payable_account_code') && payableAccount) next = assignPathValue(next, 'payable_account_code', payableAccount)
    if (!resolvePath(next, 'expense_account_code') && expenseAccount) next = assignPathValue(next, 'expense_account_code', expenseAccount)
    if (!resolvePath(next, 'payment_method_code') && defaultPaymentMethod) next = applyFieldUpdate(next, 'payment_method_code', defaultPaymentMethod, catalog)
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
  if (documentType === 'purchase_request' || documentType === 'purchase_order' || documentType === 'vendor_bill' || documentType === 'vendor_credit_note') {
    const baseDate = String(resolvePath(next, 'bill_date') || resolvePath(next, 'order_date') || resolvePath(next, 'request_date') || '')
    const paymentTermDays = toNumber(resolvePath(next, 'payment_term_days'))
    if (baseDate && paymentTermDays > 0 && !resolvePath(next, 'due_date')) {
      next = assignPathValue(next, 'due_date', addDaysToDate(baseDate, paymentTermDays))
    }
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) {
      return applyCommercialArrayUpdate(next, 'lines', 'procurement_lines', lines, catalog)
    }
    return next
  }
  if (documentType === 'goods_receipt') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) {
      return applyCommercialArrayUpdate(next, 'lines', 'procurement_receipt_lines', lines, catalog)
    }
    return next
  }
  if (documentType === 'sales_fulfillment') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) {
      return applyCommercialArrayUpdate(next, 'lines', 'fulfillment_lines', lines, catalog)
    }
    return next
  }
  if (documentType === 'delivery_order') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) {
      return applyCommercialArrayUpdate(next, 'lines', 'delivery_lines', lines, catalog)
    }
    return next
  }
  if (documentType === 'sales_return' || documentType === 'return_receipt') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) {
      return applyCommercialArrayUpdate(next, 'lines', 'return_lines', lines, catalog)
    }
    return next
  }
  if (documentType === 'supplier_return') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) {
      return applyCommercialArrayUpdate(next, 'lines', 'supplier_return_lines', lines, catalog)
    }
    return next
  }
  if (documentType === 'stock_receipt' || documentType === 'stock_issue' || documentType === 'stock_adjustment') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) {
      return applyCommercialArrayUpdate(next, 'lines', 'inventory_lines', lines, catalog)
    }
    return next
  }
  if (documentType === 'stock_transfer') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) {
      return applyCommercialArrayUpdate(next, 'lines', 'inventory_transfer_lines', lines, catalog)
    }
    return next
  }
  if (documentType === 'production_order') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    const stages = asRecordList(resolvePath(next, 'stages'))
    if (lines.length) {
      next = applyCommercialArrayUpdate(next, 'lines', 'production_component_lines', lines, catalog)
    }
    if (stages.length) {
      next = applyCommercialArrayUpdate(next, 'stages', 'production_stage_lines', stages, catalog)
    }
    return next
  }
  if (documentType === 'production_issue') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) {
      return applyCommercialArrayUpdate(next, 'lines', 'production_issue_lines', lines, catalog)
    }
    return next
  }
  if (documentType === 'payment_out') {
    next = applyFieldUpdate(next, 'payment_method_code', resolvePath(next, 'payment_method_code'), catalog)
    const allocations = asRecordList(resolvePath(next, 'allocations'))
    if (allocations.length) {
      return applyCommercialArrayUpdate(next, 'allocations', 'procurement_allocations', allocations, catalog)
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
  if (widget === 'procurement_receipt_lines') {
    return rows.map((row) => {
      const orderedQty = toNumber(row.ordered_qty || row.quantity)
      const receivedQty = Math.max(toNumber(row.received_qty), 0)
      const previouslyReceived = Math.max(toNumber(row.previously_received_qty), 0)
      const variantItemCode = resolveVariantItemCodeFromRow(row, catalog)
      const itemCode = variantItemCode || String(row.item_code || '')
      const item = catalog?.itemsByCode?.[itemCode]
      return {
        ...row,
        product_code: String(row.product_code || resolvePath(item, 'values.product_code') || ''),
        variant_signature: String(row.variant_signature || resolvePath(item, 'values.variant_signature') || ''),
        item_code: itemCode,
        description: String(row.description || resolvePath(item, 'values.description') || resolvePath(item, 'values.name') || ''),
        uom_code: String(row.uom_code || resolvePath(item, 'values.uom_code') || ''),
        ordered_qty: orderedQty,
        received_qty: receivedQty,
        cumulative_received_qty: roundMoney(previouslyReceived + receivedQty),
      }
    })
  }
  if (widget === 'fulfillment_lines') {
    return rows.map((row) => {
      const itemCode = String(row.item_code || '')
      const item = catalog?.itemsByCode?.[itemCode]
      return {
        ...row,
        product_code: String(row.product_code || resolvePath(item, 'values.product_code') || ''),
        variant_signature: String(row.variant_signature || resolvePath(item, 'values.variant_signature') || ''),
        item_code: itemCode,
        description: String(row.description || resolvePath(item, 'values.description') || resolvePath(item, 'values.name') || ''),
        uom_code: String(row.uom_code || resolvePath(item, 'values.uom_code') || ''),
        warehouse_code: String(row.warehouse_code || resolvePath(values || {}, 'warehouse_code') || ''),
        ordered_quantity: toNumber(row.ordered_quantity || row.quantity),
        quantity: toNumber(row.quantity),
        available_quantity: toNumber(row.available_quantity),
      }
    })
  }
  if (widget === 'delivery_lines') {
    return rows.map((row) => {
      const itemCode = String(row.item_code || '')
      const item = catalog?.itemsByCode?.[itemCode]
      return {
        ...row,
        product_code: String(row.product_code || resolvePath(item, 'values.product_code') || ''),
        variant_signature: String(row.variant_signature || resolvePath(item, 'values.variant_signature') || ''),
        item_code: itemCode,
        description: String(row.description || resolvePath(item, 'values.description') || resolvePath(item, 'values.name') || ''),
        warehouse_code: String(row.warehouse_code || resolvePath(values || {}, 'warehouse_code') || ''),
        batch_code: String(row.batch_code || ''),
        expiration_date: String(row.expiration_date || ''),
        uom_code: String(row.uom_code || resolvePath(item, 'values.uom_code') || ''),
        fulfilled_quantity: toNumber(row.fulfilled_quantity),
        quantity: toNumber(row.quantity),
        tracking_number: String(row.tracking_number || resolvePath(values || {}, 'tracking_number') || ''),
        note: String(row.note || ''),
      }
    })
  }
  if (widget === 'return_lines') {
    return rows.map((row) => {
      const itemCode = String(row.item_code || '')
      const item = catalog?.itemsByCode?.[itemCode]
      return {
        ...row,
        product_code: String(row.product_code || resolvePath(item, 'values.product_code') || ''),
        variant_signature: String(row.variant_signature || resolvePath(item, 'values.variant_signature') || ''),
        item_code: itemCode,
        description: String(row.description || resolvePath(item, 'values.description') || resolvePath(item, 'values.name') || ''),
        warehouse_code: String(row.warehouse_code || resolvePath(values || {}, 'warehouse_code') || ''),
        batch_code: String(row.batch_code || ''),
        expiration_date: String(row.expiration_date || ''),
        uom_code: String(row.uom_code || resolvePath(item, 'values.uom_code') || ''),
        fulfilled_quantity: toNumber(row.fulfilled_quantity),
        quantity: toNumber(row.quantity),
        disposition: String(row.disposition || 'restock'),
        note: String(row.note || ''),
      }
    })
  }
  if (widget === 'supplier_return_lines') {
    return rows.map((row) => {
      const itemCode = String(row.item_code || '')
      const item = catalog?.itemsByCode?.[itemCode]
      return {
        ...row,
        item_code: itemCode,
        description: String(row.description || resolvePath(item, 'values.description') || resolvePath(item, 'values.name') || ''),
        warehouse_code: String(row.warehouse_code || resolvePath(values || {}, 'warehouse_code') || ''),
        batch_code: String(row.batch_code || ''),
        expiration_date: String(row.expiration_date || ''),
        uom_code: String(row.uom_code || resolvePath(item, 'values.uom_code') || ''),
        received_quantity: toNumber(row.received_quantity),
        quantity: toNumber(row.quantity),
        note: String(row.note || ''),
      }
    })
  }
  if (widget === 'inventory_lines') {
    return rows.map((row) => {
      const variantItemCode = resolveVariantItemCodeFromRow(row, catalog)
      const itemCode = variantItemCode || String(row.item_code || '')
      const item = catalog?.itemsByCode?.[itemCode]
      const productCode = String(row.product_code || resolvePath(item, 'values.product_code') || '')
      const variantSignature = String(row.variant_signature || resolvePath(item, 'values.variant_signature') || '')
      return {
        ...row,
        product_code: productCode,
        variant_signature: variantSignature,
        item_code: itemCode,
        description: String(row.description || resolvePath(item, 'values.description') || resolvePath(item, 'values.name') || ''),
        uom_code: String(row.uom_code || resolvePath(item, 'values.uom_code') || ''),
        warehouse_code: String(row.warehouse_code || resolvePath(values || {}, 'warehouse_code') || ''),
        quantity: toNumber(row.quantity),
      }
    })
  }
  if (widget === 'inventory_transfer_lines') {
    return rows.map((row) => {
      const variantItemCode = resolveVariantItemCodeFromRow(row, catalog)
      const itemCode = variantItemCode || String(row.item_code || '')
      const item = catalog?.itemsByCode?.[itemCode]
      const productCode = String(row.product_code || resolvePath(item, 'values.product_code') || '')
      const variantSignature = String(row.variant_signature || resolvePath(item, 'values.variant_signature') || '')
      return {
        ...row,
        product_code: productCode,
        variant_signature: variantSignature,
        item_code: itemCode,
        description: String(row.description || resolvePath(item, 'values.description') || resolvePath(item, 'values.name') || ''),
        uom_code: String(row.uom_code || resolvePath(item, 'values.uom_code') || ''),
        quantity: toNumber(row.quantity),
      }
    })
  }
  if (widget === 'production_component_lines') {
    return rows.map((row) => {
      const componentItemCode = String(row.component_item_code || row.item_code || '')
      const actualItemCode = String(row.actual_item_code || componentItemCode || '')
      const item = catalog?.itemsByCode?.[actualItemCode || componentItemCode]
      return {
        ...row,
        component_item_code: componentItemCode,
        actual_item_code: actualItemCode,
        description: String(row.description || resolvePath(item, 'values.description') || resolvePath(item, 'values.name') || ''),
        warehouse_code: String(row.warehouse_code || resolvePath(values || {}, 'warehouse_code') || ''),
        uom_code: String(row.uom_code || resolvePath(item, 'values.uom_code') || ''),
        quantity_per_unit: toNumber(row.quantity_per_unit),
        quantity: toNumber(row.quantity),
        issued_quantity: toNumber(row.issued_quantity),
        reserved_quantity: toNumber(row.reserved_quantity),
        shortage_quantity: toNumber(row.shortage_quantity),
        available_quantity: toNumber(row.available_quantity),
        allowed_substitute_item_codes: Array.isArray(row.allowed_substitute_item_codes) ? row.allowed_substitute_item_codes.map((item) => String(item || '')).filter(Boolean).join(', ') : String(row.allowed_substitute_item_codes || ''),
        reservation_status: String(row.reservation_status || 'unreserved'),
        substitution_status: String(row.substitution_status || (actualItemCode && actualItemCode !== componentItemCode ? 'substituted' : 'planned')),
      }
    })
  }
  if (widget === 'production_issue_lines') {
    return rows.map((row) => {
      const plannedItemCode = String(row.planned_item_code || row.component_item_code || '')
      const actualItemCode = String(row.actual_item_code || row.item_code || plannedItemCode || '')
      const item = catalog?.itemsByCode?.[actualItemCode || plannedItemCode]
      return {
        ...row,
        planned_item_code: plannedItemCode,
        actual_item_code: actualItemCode,
        item_code: actualItemCode,
        description: String(row.description || resolvePath(item, 'values.description') || resolvePath(item, 'values.name') || ''),
        warehouse_code: String(row.warehouse_code || resolvePath(values || {}, 'warehouse_code') || ''),
        batch_code: String(row.batch_code || ''),
        expiration_date: String(row.expiration_date || ''),
        uom_code: String(row.uom_code || resolvePath(item, 'values.uom_code') || ''),
        quantity: toNumber(row.quantity),
        reserved_quantity: toNumber(row.reserved_quantity),
        available_quantity: toNumber(row.available_quantity),
        allowed_substitute_item_codes: Array.isArray(row.allowed_substitute_item_codes) ? row.allowed_substitute_item_codes.map((item) => String(item || '')).filter(Boolean).join(', ') : String(row.allowed_substitute_item_codes || ''),
        substitution_status: String(row.substitution_status || (actualItemCode && actualItemCode !== plannedItemCode ? 'substituted' : 'planned')),
      }
    })
  }
  if (widget === 'production_stage_lines') {
    return rows.map((row, index) => ({
      ...row,
      stage_code: String(row.stage_code || ''),
      stage_name: String(row.stage_name || row.stage_code || ''),
      sequence: toNumber(row.sequence) || index + 1,
      work_center_code: String(row.work_center_code || ''),
      status: String(row.status || 'pending'),
      required: typeof row.required === 'boolean' ? row.required : String(row.required || 'true') !== 'false',
      note: String(row.note || ''),
    }))
  }
  if (widget === 'trace_batches') {
    return rows.map((row) => {
      const batchID = String(row.batch_id || '')
      const batch = catalog?.inventoryBatchesByID?.[batchID]
      return {
        ...row,
        batch_id: batchID,
        item_code: String(row.item_code || resolvePath(batch, 'values.item_code') || ''),
        warehouse_code: String(row.warehouse_code || resolvePath(batch, 'values.warehouse_code') || ''),
        batch_code: String(row.batch_code || resolvePath(batch, 'values.batch_code') || ''),
        expiration_date: String(row.expiration_date || resolvePath(batch, 'values.expiration_date') || ''),
        status: String(row.status || resolvePath(batch, 'values.status') || ''),
        on_hand_quantity: toNumber(row.on_hand_quantity || resolvePath(batch, 'values.on_hand_quantity')),
        reserved_quantity: toNumber(row.reserved_quantity || resolvePath(batch, 'values.reserved_quantity')),
        available_quantity: toNumber(row.available_quantity || resolvePath(batch, 'values.available_quantity')),
      }
    })
  }
  if (widget !== 'commercial_lines' && widget !== 'procurement_lines') return rows
  return rows.map((row) => {
    const variantItemCode = resolveVariantItemCodeFromRow(row, catalog)
    const itemCode = variantItemCode || String(row.item_code || '')
    const item = catalog?.itemsByCode?.[itemCode]
    const productCode = String(row.product_code || resolvePath(item, 'values.product_code') || '')
    const variantSignature = String(row.variant_signature || resolvePath(item, 'values.variant_signature') || '')
    const itemDescription = resolvePath(item, 'values.description') || resolvePath(item, 'values.name')
    const itemUOMCode = resolvePath(item, 'values.uom_code')
    const priceListCode = String(resolvePath(values || {}, 'price_list_code') || '')
    const priceListItem = catalog?.priceListItemsByKey?.[`${priceListCode}|${itemCode}`]
    const itemUnitPrice = resolvePath(priceListItem, 'values.unit_price') ?? resolvePath(item, 'values.base_price') ?? resolvePath(item, 'values.unit_price')
    const itemTaxCode = resolvePath(item, 'values.tax_code')
    const itemRevenueAccount = resolvePath(item, 'values.revenue_account_code')
    const itemExpenseAccount = resolvePath(item, 'values.expense_account_code')
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
      product_code: productCode,
      variant_signature: variantSignature,
      item_code: itemCode,
      description: String(row.description || itemDescription || ''),
      uom_code: String(row.uom_code || itemUOMCode || ''),
      unit_price: unitPrice,
      tax_code: taxCode,
      tax_rate: taxRate,
      tax_mode: taxMode,
      revenue_account_code: row.revenue_account_code || priceListRevenueAccount || itemRevenueAccount || '',
      expense_account_code: row.expense_account_code || itemExpenseAccount || '',
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
      if (normalizedType === 'goods_receipt' && normalizedStatus === 'received') return true
      if (normalizedType === 'vendor_bill' && (normalizedStatus === 'issued' || normalizedStatus === 'partially_paid')) return true
      if (normalizedType === 'payment_out' && normalizedStatus === 'paid') return true
      if (normalizedType === 'vendor_credit_note' && normalizedStatus === 'issued') return true
      if (normalizedType === 'supplier_return' && normalizedStatus === 'approved') return true
      return false
    case 'reopen':
      return normalizedStatus !== 'draft' && normalizedStatus !== 'submitted'
    case 'generate_invoice':
    case 'generate_fulfillment':
    case 'generate_production_order':
      return normalizedStatus === 'confirmed'
    case 'register_delivery':
      return normalizedStatus === 'issued'
    case 'mark_delivered':
      return normalizedStatus === 'dispatched'
    case 'register_payment':
      return normalizedStatus === 'issued' || normalizedStatus === 'partially_paid'
    case 'issue_credit_note':
      if (normalizedType === 'sales_return') return normalizedStatus === 'approved' || normalizedStatus === 'received'
      return normalizedStatus === 'issued' || normalizedStatus === 'partially_paid' || normalizedStatus === 'paid'
    case 'register_refund':
      if (normalizedType === 'sales_return') return normalizedStatus === 'approved' || normalizedStatus === 'received'
      return normalizedStatus === 'issued'
    case 'register_return':
      return normalizedStatus === 'issued'
    case 'register_return_receipt':
      return normalizedStatus === 'approved'
    case 'create_replacement_order':
      return normalizedType === 'sales_return' && (normalizedStatus === 'approved' || normalizedStatus === 'received')
    case 'register_supplier_return':
      if (normalizedType === 'goods_receipt') return normalizedStatus === 'received'
      if (normalizedType === 'vendor_bill') return normalizedStatus === 'issued' || normalizedStatus === 'partially_paid' || normalizedStatus === 'paid'
      return false
    case 'generate_purchase_order':
      return normalizedStatus === 'approved'
    case 'register_receipt':
      return normalizedStatus === 'approved' || normalizedStatus === 'partially_received'
    case 'register_vendor_bill':
      if (normalizedType === 'purchase_order') return normalizedStatus === 'approved' || normalizedStatus === 'partially_received' || normalizedStatus === 'received'
      if (normalizedType === 'goods_receipt') return normalizedStatus === 'received'
      return false
    case 'register_payment_out':
      return normalizedStatus === 'issued' || normalizedStatus === 'partially_paid'
    case 'issue_vendor_credit_note':
      if (normalizedType === 'supplier_return') return normalizedStatus === 'approved'
      return normalizedStatus === 'issued' || normalizedStatus === 'partially_paid' || normalizedStatus === 'paid'
    case 'register_production_issue':
    case 'register_production_output':
      return normalizedStatus === 'approved' || normalizedStatus === 'in_progress'
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

function isProcurementDocumentLocked(documentType: string, status: string): boolean {
  const normalizedType = documentType.toLowerCase()
  const normalizedStatus = status.toLowerCase()
  if (!['purchase_request', 'purchase_order', 'goods_receipt', 'vendor_bill', 'payment_out', 'vendor_credit_note'].includes(normalizedType)) return false
  if (!normalizedStatus) return false
  return normalizedStatus !== 'draft' && normalizedStatus !== 'rejected'
}

function isFulfillmentDocumentLocked(documentType: string, status: string): boolean {
  const normalizedType = documentType.toLowerCase()
  const normalizedStatus = status.toLowerCase()
  if (!['sales_fulfillment', 'delivery_order'].includes(normalizedType)) return false
  if (!normalizedStatus) return false
  return normalizedStatus !== 'draft' && normalizedStatus !== 'rejected'
}

function isReturnsDocumentLocked(documentType: string, status: string): boolean {
  const normalizedType = documentType.toLowerCase()
  const normalizedStatus = status.toLowerCase()
  if (!['sales_return', 'return_receipt'].includes(normalizedType)) return false
  if (!normalizedStatus) return false
  return normalizedStatus !== 'draft' && normalizedStatus !== 'rejected'
}

function isSupplierReturnsDocumentLocked(documentType: string, status: string): boolean {
  const normalizedType = documentType.toLowerCase()
  const normalizedStatus = status.toLowerCase()
  if (!['supplier_return'].includes(normalizedType)) return false
  if (!normalizedStatus) return false
  return normalizedStatus !== 'draft' && normalizedStatus !== 'rejected'
}

function isProductionDocumentLocked(documentType: string, status: string): boolean {
  const normalizedType = documentType.toLowerCase()
  const normalizedStatus = status.toLowerCase()
  if (!['production_order', 'production_issue', 'production_output'].includes(normalizedType)) return false
  if (!normalizedStatus) return false
  return normalizedStatus !== 'draft' && normalizedStatus !== 'rejected'
}

function isRecallDocumentLocked(documentType: string, status: string): boolean {
  const normalizedType = documentType.toLowerCase()
  const normalizedStatus = status.toLowerCase()
  if (!['recall_case', 'recall_action'].includes(normalizedType)) return false
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
