import { useEffect, useMemo, useRef, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { Shell } from '@/components/layout/Shell'
import { useShellStore } from '@/stores/shellStore'
import { fetchWorkspaceBootstrap, normalizeShellPath, pickText, toShellRoutes, type ActionDefinition, type CustomEntryDefinition, type ViewDefinition } from '@/services/bootstrap'
import { useToast } from '@/components/ui/Toast'
import SettingsPage from '@/features/SettingsPage'
import { WorkspaceDetailView } from './WorkspaceDetailView'
import { WorkspaceDashboardView } from './WorkspaceDashboardView'
import { renderWorkspaceDetailFieldValue, WorkspaceFieldEditor } from './WorkspaceFieldEditor'
import { WorkspaceFlowRouteView } from './WorkspaceFlowRouteView'
import { WorkspaceFormView } from './WorkspaceFormView'
import { buildAllocationRows, buildProcurementAllocationRows, buildRefundAllocationRows, commercialArrayColumns, commercialArrayDefaultRow, commercialOpenInvoices, commercialRefundablePayments, commercialSelectOptions, hasUnresolvedVariantSelections, parseDimensionCodes, procurementOpenBills, resolveCommercialColumnOptions, resolveVariantItemCodeFromRow } from './workspaceCommercial'
import { WorkspaceListView } from './WorkspaceListView'
import { WorkspacePanel, WorkspaceRecoveryPanel } from './workspaceShellState'
import { applyCommercialArrayUpdate, applyFieldUpdate, normalizeCommercialFormState, normalizeCommercialRows } from './workspaceCommercialState'
import { asRecordList, assignPathValue, DataTable, displayValue, humanize, MetricCard, resolvePath, roundMoney, toNumber } from './workspaceShared'
import { WorkspaceRouteContent } from './WorkspaceRouteContent'
import { useWorkspaceRouteResolution } from './useWorkspaceRouteResolution'
import type { RouteResolution } from './workspaceTypes'
import {
  actionVisibleForStatus,
  collectFlowFields,
  isCommercialDocumentLocked,
  isFulfillmentDocumentLocked,
  isProcurementDocumentLocked,
  isProductionDocumentLocked,
  isRecallDocumentLocked,
  isReturnsDocumentLocked,
  isSupplierReturnsDocumentLocked,
  normalizeFieldPath,
  normalizeLegacyWorkspacePath,
  readCookie,
  resolveFlowSequence,
  resolveSections,
  routeForCreate,
  routeForDocument,
  routeForEdit,
  routeForModel,
  routeForWorkItem,
  stripEditorSuffix,
  validateFieldCollection,
  validateFieldInput,
  validationFieldKey,
} from './workspaceRouteHelpers'

const Panel = WorkspacePanel

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
  const pathname = normalizeLegacyWorkspacePath(location.pathname || '/')
  const { route, loading } = useWorkspaceRouteResolution({
    pathname,
    currentSurface,
  })

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
    if (loading) return <WorkspacePanel title="Loading" status="Resolving route contract." />
    if (!route) return <WorkspacePanel title="Unavailable" status="Route could not be resolved." />
    if (route.status !== 'ok') {
      return (
        <WorkspaceRecoveryPanel
          route={route}
          onDefault={() => navigate(route.fallback_path || defaultPath || '/', { replace: true })}
          onSwitchSurface={() => void handleSwitchSurface(route.suggested_surface || currentSurface, route.fallback_path)}
        />
      )
    }
    if (route.render_mode === 'flow' && route.flow) {
      return (
        <WorkspaceFlowRouteView
          route={route}
          locale={locale}
          fetchJSON={fetchJSON}
          readCookie={readCookie}
          buildError={buildError}
          routeForDocument={routeForDocument}
          stripEditorSuffix={stripEditorSuffix}
          resolvePath={resolvePath}
          collectFlowFields={collectFlowFields}
          validateFieldCollection={validateFieldCollection}
          validateFieldInput={validateFieldInput}
          normalizeFieldPath={normalizeFieldPath}
          resolveFlowSequence={resolveFlowSequence}
          validationFieldKey={validationFieldKey}
          renderPanel={({ title, status, children }) => (
            <WorkspacePanel title={title} status={status}>
              {children}
            </WorkspacePanel>
          )}
          renderFieldEditor={(props) => (
            <WorkspaceFieldEditor
              key={props.field.key}
              {...props}
              normalizeFieldPath={normalizeFieldPath}
              resolvePath={resolvePath}
              commercialSelectOptions={commercialSelectOptions}
              humanize={humanize}
              applyFieldUpdate={applyFieldUpdate}
              assignPathValue={assignPathValue}
              toNumber={toNumber}
              asRecordList={asRecordList}
              commercialArrayColumns={commercialArrayColumns}
              commercialOpenInvoices={commercialOpenInvoices}
              procurementOpenBills={procurementOpenBills}
              commercialRefundablePayments={commercialRefundablePayments}
              roundMoney={roundMoney}
              resolveVariantItemCodeFromRow={resolveVariantItemCodeFromRow}
              normalizeCommercialRows={normalizeCommercialRows}
              commercialArrayDefaultRow={commercialArrayDefaultRow}
              buildAllocationRows={buildAllocationRows}
              buildRefundAllocationRows={buildRefundAllocationRows}
              buildProcurementAllocationRows={buildProcurementAllocationRows}
              resolveCommercialColumnOptions={resolveCommercialColumnOptions}
              displayValue={displayValue}
              applyCommercialArrayUpdate={applyCommercialArrayUpdate}
            />
          )}
        />
      )
    }
    if (route.render_mode === 'custom' && route.custom_entry) {
      return <CustomRouteView entry={route.custom_entry} route={route} locale={locale} />
    }
    if (pathname === '/notifications') {
      return <NotificationsView />
    }
    if (route.view) {
      return (
        <WorkspaceRouteContent
          route={route}
          locale={locale}
          actions={actions}
          currentPath={pathname}
          onNavigate={(target) => navigate(target)}
          onToast={(message, variant = 'default') => addToast({ message, variant })}
          renderQueueView={(props) => <QueueView {...props} />}
          renderListView={(props) => <ListView {...props} />}
          renderDetailView={(props) => <DetailView {...props} />}
          renderDashboardView={(props) => <DashboardView {...props} />}
          renderFormView={(props) => <FormView {...props} />}
          renderUnsupported={({ title, status }) => <WorkspacePanel title={title} status={status} />}
        />
      )
    }
    return <WorkspacePanel title="Unavailable" status="No renderer available for this route." />
  }, [actions, addToast, currentSurface, defaultPath, loading, locale, navigate, pathname, route])

  return (
    <Shell>
      {shellKind === 'workspace' ? content : <WorkspacePanel title="Unavailable" status="Workspace shell is not active." />}
    </Shell>
  )
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
    <WorkspacePanel title={title} status={pickText(view, 'empty_state', locale) || 'Operational queue for workflow work.'}>
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
    </WorkspacePanel>
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
  return (
    <WorkspaceListView
      view={view}
      locale={locale}
      routeActions={routeActions}
      currentPath={currentPath}
      onNavigate={onNavigate}
      fetchJSON={fetchJSON}
      documentListNeedsPayload={documentListNeedsPayload}
      routeForModel={routeForModel}
      routeForCreate={routeForCreate}
      humanize={humanize}
      resolvePath={resolvePath}
      routeForDocument={routeForDocument}
      renderPanel={({ title, status, children }) => (
        <Panel title={title} status={status}>
          {children}
        </Panel>
      )}
      renderDataTable={({ columns, rows, emptyText, renderAction }) => (
        <DataTable
          columns={columns}
          rows={rows}
          emptyText={emptyText}
          renderAction={renderAction}
        />
      )}
    />
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
  return (
    <WorkspaceDetailView
      view={view}
      locale={locale}
      routeActions={routeActions}
      currentPath={currentPath}
      onNavigate={onNavigate}
      onToast={onToast}
      fetchJSON={fetchJSON}
      resolvePath={resolvePath}
      readCookie={readCookie}
      buildError={buildError}
      routeForDocument={routeForDocument}
      routeForModel={routeForModel}
      routeForEdit={routeForEdit}
      stripEditorSuffix={stripEditorSuffix}
      resolveSections={resolveSections}
      actionVisibleForStatus={actionVisibleForStatus}
      isCommercialDocumentLocked={isCommercialDocumentLocked}
      isProcurementDocumentLocked={isProcurementDocumentLocked}
      isFulfillmentDocumentLocked={isFulfillmentDocumentLocked}
      isReturnsDocumentLocked={isReturnsDocumentLocked}
      isSupplierReturnsDocumentLocked={isSupplierReturnsDocumentLocked}
      isProductionDocumentLocked={isProductionDocumentLocked}
      invokeCommercialAction={invokeCommercialAction}
      invokeDocumentAction={invokeDocumentAction}
      renderDetailFieldValue={(field, value) => renderWorkspaceDetailFieldValue({
        field,
        value,
        asRecordList,
        commercialArrayColumns,
        displayValue,
      })}
      humanize={humanize}
      parseDimensionCodes={parseDimensionCodes}
      toNumber={toNumber}
      displayValue={displayValue}
      asRecordList={asRecordList}
      renderPanel={({ title, status, children }) => (
        <Panel title={title} status={status}>
          {children}
        </Panel>
      )}
      renderMetricCard={({ label, value }) => <MetricCard label={label} value={value} />}
      renderDataTable={({ columns, rows, emptyText, renderAction }) => (
        <DataTable columns={columns} rows={rows} emptyText={emptyText} renderAction={renderAction} />
      )}
    />
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
  return (
    <WorkspaceDashboardView
      view={view}
      locale={locale}
      onNavigate={onNavigate}
      routeActions={routeActions}
      onToast={onToast}
      fetchJSON={fetchJSON}
      resolvePath={resolvePath}
      asRecordList={asRecordList}
      displayValue={displayValue}
      readCookie={readCookie}
      buildError={buildError}
      routeForDocument={routeForDocument}
      renderPanel={({ title, children }) => <Panel title={title}>{children}</Panel>}
      renderMetricCard={({ label, value }) => <MetricCard label={label} value={value} />}
      renderDataTable={({ columns, rows, emptyText, renderAction }) => (
        <DataTable
          columns={columns}
          rows={rows}
          emptyText={emptyText}
          renderAction={renderAction}
        />
      )}
    />
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
  return (
    <WorkspaceFormView
      view={view}
      locale={locale}
      currentPath={currentPath}
      searchParams={searchParams}
      actions={useShellStore.getState().actions}
      onNavigate={onNavigate}
      onToast={onToast}
      fetchJSON={fetchJSON}
      resolvePath={resolvePath}
      readCookie={readCookie}
      buildError={buildError}
      routeForModel={routeForModel}
      routeForDocument={routeForDocument}
      stripEditorSuffix={stripEditorSuffix}
      resolveSections={resolveSections}
      validateFieldCollection={validateFieldCollection}
      validateFieldInput={validateFieldInput}
      normalizeFieldPath={normalizeFieldPath}
      hasUnresolvedVariantSelections={hasUnresolvedVariantSelections}
      normalizeCommercialFormState={normalizeCommercialFormState}
      isCommercialDocumentLocked={isCommercialDocumentLocked}
      isProcurementDocumentLocked={isProcurementDocumentLocked}
      isFulfillmentDocumentLocked={isFulfillmentDocumentLocked}
      isReturnsDocumentLocked={isReturnsDocumentLocked}
      isSupplierReturnsDocumentLocked={isSupplierReturnsDocumentLocked}
      isProductionDocumentLocked={isProductionDocumentLocked}
      isRecallDocumentLocked={isRecallDocumentLocked}
      renderPanel={({ title, status, children }) => (
        <Panel title={title} status={status}>
          {children}
        </Panel>
      )}
      renderFieldEditor={(props) => (
        <WorkspaceFieldEditor
          key={props.field.key}
          {...props}
          normalizeFieldPath={normalizeFieldPath}
          resolvePath={resolvePath}
          commercialSelectOptions={commercialSelectOptions}
          humanize={humanize}
          applyFieldUpdate={applyFieldUpdate}
          assignPathValue={assignPathValue}
          toNumber={toNumber}
          asRecordList={asRecordList}
          commercialArrayColumns={commercialArrayColumns}
          commercialOpenInvoices={commercialOpenInvoices}
          procurementOpenBills={procurementOpenBills}
          commercialRefundablePayments={commercialRefundablePayments}
          roundMoney={roundMoney}
          resolveVariantItemCodeFromRow={resolveVariantItemCodeFromRow}
          normalizeCommercialRows={normalizeCommercialRows}
          commercialArrayDefaultRow={commercialArrayDefaultRow}
          buildAllocationRows={buildAllocationRows}
          buildRefundAllocationRows={buildRefundAllocationRows}
          buildProcurementAllocationRows={buildProcurementAllocationRows}
          resolveCommercialColumnOptions={resolveCommercialColumnOptions}
          displayValue={displayValue}
          applyCommercialArrayUpdate={applyCommercialArrayUpdate}
        />
      )}
    />
  )
}

function documentListNeedsPayload(view: ViewDefinition): boolean {
  return (view.columns || []).some((column) => {
    const path = String(column.path || '')
    return path.startsWith('body.') || path.startsWith('lines') || path.startsWith('links') || path.startsWith('attachments') || path === 'header.number'
  })
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
