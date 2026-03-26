import { useEffect, useMemo, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { Shell } from '@/components/layout/Shell'
import { useShellStore } from '@/stores/shellStore'
import { normalizeShellPath } from '@/services/bootstrap'
import { mutateJson } from './adminClient'
import { TemplateDesignerPage } from './TemplateDesignerPage'
import { WorkflowDesignerPage } from './WorkflowDesignerPage'

type TargetOption = {
  key: string
  title?: string
}

type TemplateTargetCatalog = {
  documents?: TargetOption[]
  reports?: TargetOption[]
}

export default function AdminWorkspacePage() {
  const location = useLocation()
  const navigate = useNavigate()
  const bootstrap = useShellStore((state) => state.adminBootstrap)
  const defaultPath = useShellStore((state) => state.defaultPath)
  const routes = useShellStore((state) => state.routes)
  const [payload, setPayload] = useState<Record<string, unknown> | null>(null)
  const path = normalizeShellPath(location.pathname || '/', 'admin')

  useEffect(() => {
    if (path === '/' && defaultPath && defaultPath !== '/') {
      navigate(defaultPath, { replace: true })
    }
  }, [defaultPath, navigate, path])

  useEffect(() => {
    let mounted = true
    async function load() {
      if (!bootstrap) return
      const target = endpointForPath(path)
      if (!target) {
        setPayload(null)
        return
      }
      const response = await fetch(target, { credentials: 'include' })
      const data = await response.json()
      if (!mounted) return
      setPayload(data)
    }
    void load()
    return () => {
      mounted = false
    }
  }, [bootstrap, path])

  const title = useMemo(() => {
    return routes.find((item) => item.path === path)?.label || titleForPath(path)
  }, [path, routes])

  return (
    <Shell>
      <section className="rounded-2xl border border-line bg-surface p-6 shadow-panel">
        <div className="mb-4">
          <h1 className="text-2xl font-bold text-body">{title}</h1>
          <p className="mt-1 text-sm text-muted">Admin data rendered from the existing server APIs.</p>
        </div>

        {path === '/org' && bootstrap ? (
          <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
            <SummaryCard label="Organization" value={String((bootstrap.organization as Record<string, unknown> | undefined)?.name || 'Root')} />
            <SummaryCard label="Locations" value={String(bootstrap.locations?.length || 0)} />
            <SummaryCard label="Operating Units" value={String(bootstrap.operating_units?.length || 0)} />
          </div>
        ) : null}

        <div className="mt-4">
          <AdminContent path={path} payload={payload} bootstrap={bootstrap} />
        </div>
      </section>
    </Shell>
  )
}

function AdminContent({
  path,
  payload,
  bootstrap,
}: {
  path: string
  payload: Record<string, unknown> | null
  bootstrap: ReturnType<typeof useShellStore.getState>['adminBootstrap']
}) {
  if (path === '/modules') {
    const items = asItems(payload)
    return (
      <DataGrid
        columns={[
          { key: 'manifest.key', label: 'Module' },
          { key: 'manifest.name', label: 'Name' },
          { key: 'manifest.version', label: 'Version' },
          { key: 'installed.enabled', label: 'Enabled' },
          { key: 'lifecycle_state', label: 'Lifecycle' },
        ]}
        rows={items}
      />
    )
  }

  if (path === '/auth') {
    const definition = (payload?.definition || {}) as Record<string, unknown>
    const entry = (payload?.entry || {}) as Record<string, unknown>
    const fields = Array.isArray(definition.fields) ? (definition.fields as Array<Record<string, unknown>>) : []
    const settings = entry.value && typeof entry.value === 'object' ? (entry.value as Record<string, unknown>) : {}
    return (
      <div className="space-y-4">
        <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
          <SummaryCard label="Key" value={String(definition.key || 'identity.auth')} />
          <SummaryCard label="Scope" value={String(entry.source_scope || entry.scope || 'deployment')} />
          <SummaryCard label="Resolved At" value={formatDate(entry.resolved_at)} />
        </div>
        <FieldGrid label="Current Settings" fields={fields} values={settings} />
      </div>
    )
  }

  if (path === '/config') {
    return (
      <DataGrid
        columns={[
          { key: 'key', label: 'Key' },
          { key: 'module_key', label: 'Module' },
          { key: 'type', label: 'Type' },
          { key: 'allowed_scopes', label: 'Scopes' },
        ]}
        rows={asItems(payload)}
      />
    )
  }

  if (path === '/definitions' || path === '/templates') {
    if (path === '/templates') {
      return <TemplateListPage rows={asItems(payload)} />
    }
    return (
      <DataGrid
        columns={[
          { key: 'key', label: 'Template' },
          { key: 'title', label: 'Title' },
          { key: 'target_kind', label: 'Target' },
          { key: 'default_format', label: 'Format' },
          { key: 'purpose', label: 'Purpose' },
        ]}
        rows={asItems(payload)}
      />
    )
  }

  if (path === '/templates/designer') {
    return <TemplateDesignerPage />
  }

  if (path === '/workflows') {
    return <WorkflowListPage rows={asItems(payload)} />
  }

  if (path === '/workflows/designer') {
    return <WorkflowDesignerPage />
  }

  if (path === '/security') {
    return (
      <DataGrid
        columns={[
          { key: 'definition.key', label: 'Hook' },
          { key: 'definition.kind', label: 'Kind' },
          { key: 'definition.target', label: 'Target' },
          { key: 'rule.source_scope', label: 'Scope' },
          { key: 'engine', label: 'Engine' },
          { key: 'eval_valid', label: 'Valid' },
        ]}
        rows={asItems(payload)}
      />
    )
  }

  if (path === '/observability') {
    const metrics = asItems(payload?.metrics as Record<string, unknown> | null)
    const logEvents = asItems(payload?.log_events as Record<string, unknown> | null)
    const domainEvents = asItems(payload?.domain_events as Record<string, unknown> | null)
    const statuses = asItems(payload?.contract_status as Record<string, unknown> | null)
    return (
      <div className="space-y-4">
        <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
          <SummaryCard label="Metrics" value={String(metrics.length)} />
          <SummaryCard label="Log Events" value={String(logEvents.length)} />
          <SummaryCard label="Domain Events" value={String(domainEvents.length)} />
          <SummaryCard label="Contracts" value={String(statuses.length)} />
        </div>
        <DataGrid columns={[{ key: 'key', label: 'Metric' }, { key: 'type', label: 'Type' }, { key: 'description', label: 'Description' }]} rows={metrics} />
        <DataGrid columns={[{ key: 'key', label: 'Log Event' }, { key: 'category', label: 'Category' }, { key: 'severity', label: 'Severity' }]} rows={logEvents} />
        <DataGrid columns={[{ key: 'type', label: 'Domain Event' }, { key: 'role', label: 'Role' }, { key: 'correlation_required', label: 'Correlation' }]} rows={domainEvents} />
      </div>
    )
  }

  return <ValueCard label="Raw payload" value={payload ?? bootstrapSummary(path, bootstrap)} />
}

function endpointForPath(path: string): string {
  switch (normalizeShellPath(path, 'admin')) {
    case '/modules':
      return '/admin/api/modules'
    case '/config':
      return '/admin/api/config/definitions'
    case '/auth':
      return '/admin/api/auth/settings'
    case '/definitions':
      return '/admin/api/templates/definitions'
    case '/security':
      return '/admin/api/security/policy-hooks'
    case '/observability':
      return '/admin/api/observability/contracts'
    case '/templates':
      return '/admin/api/templates/definitions'
    case '/templates/designer':
      return '/admin/api/templates/definitions'
    case '/workflows':
      return '/admin/api/workflows'
    case '/workflows/designer':
      return '/admin/api/workflows'
    default:
      return ''
  }
}

function titleForPath(path: string): string {
  switch (normalizeShellPath(path, 'admin')) {
    case '/modules':
      return 'Modules'
    case '/config':
      return 'Configuration'
    case '/auth':
      return 'Authentication'
    case '/definitions':
      return 'Definitions'
    case '/security':
      return 'Security'
    case '/observability':
      return 'Observability'
    case '/templates':
      return 'Templates'
    case '/templates/designer':
      return 'Template Designer'
    case '/workflows':
      return 'Workflows'
    case '/workflows/designer':
      return 'Workflow Designer'
    case '/org':
      return 'Organization'
    default:
      return 'Admin'
  }
}

function bootstrapSummary(path: string, bootstrap: ReturnType<typeof useShellStore.getState>['adminBootstrap']) {
  const normalizedPath = normalizeShellPath(path, 'admin')
  if (!bootstrap) return {}
  if (normalizedPath === '/' || normalizedPath === '') {
    return {
      menus: bootstrap.menus,
      actions: bootstrap.actions,
      default_path: bootstrap.default_path,
    }
  }
  if (normalizedPath === '/org') {
    return {
      organization: bootstrap.organization,
      locations: bootstrap.locations,
      operating_units: bootstrap.operating_units,
      roles: bootstrap.roles,
    }
  }
  return bootstrap
}

function SummaryCard({ label, value }: { label: string; value: string }) {
  return (
    <article className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
      <div className="text-xs font-semibold uppercase tracking-wide text-body">{label}</div>
      <div className="mt-2 text-2xl font-bold text-body">{value}</div>
    </article>
  )
}

function TemplateListPage({ rows }: { rows: Array<Record<string, unknown>> }) {
  const navigate = useNavigate()
  const [targetCatalog, setTargetCatalog] = useState<TemplateTargetCatalog>({})
  const [draft, setDraft] = useState({ key: '', title: '', targetKind: 'document', targetKey: '', purpose: '' })
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')

  useEffect(() => {
    let mounted = true
    async function loadCatalog() {
      try {
        const payload = await fetch('/admin/api/template-targets', { credentials: 'include' })
        const data = (await payload.json()) as TemplateTargetCatalog
        if (!mounted) return
        setTargetCatalog(data || {})
        setDraft((current) => ({
          ...current,
          targetKey: current.targetKey || (current.targetKind === 'report' ? data?.reports?.[0]?.key || '' : data?.documents?.[0]?.key || ''),
        }))
      } catch {
        if (!mounted) return
      }
    }
    void loadCatalog()
    return () => {
      mounted = false
    }
  }, [])

  const targetOptions = draft.targetKind === 'report' ? targetCatalog.reports || [] : targetCatalog.documents || []

  async function createTemplate() {
    setBusy(true)
    setMessage('')
    try {
      await mutateJson('/admin/api/templates/definitions', {
        method: 'POST',
        body: JSON.stringify({
          key: draft.key,
          title: draft.title,
          target_kind: draft.targetKind,
          target_key: draft.targetKey,
          renderer_kind: 'visual',
          default_format: 'html',
          purpose: draft.purpose,
        }),
      })
      navigate(`/templates/designer?key=${encodeURIComponent(draft.key)}`)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to create template.')
    } finally {
      setBusy(false)
    }
  }

  async function deleteTemplate(row: Record<string, unknown>) {
    const key = String(resolvePath(row, 'key') || '')
    if (!key) return
    setBusy(true)
    setMessage('')
    try {
      await mutateJson(`/admin/api/templates/definitions/${encodeURIComponent(key)}`, {
        method: 'DELETE',
      })
      navigate(0)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to delete template.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-line bg-accent-soft/60 p-4 text-sm text-body">
        Select a template to open the visual designer, preview drafts, and use advanced body or style editing.
      </div>
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <div className="mb-3 text-sm font-semibold text-body">Create New Template</div>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-5">
          <input className="admin-input" placeholder="Key" value={draft.key} onChange={(event) => setDraft((current) => ({ ...current, key: event.target.value }))} />
          <input className="admin-input" placeholder="Title" value={draft.title} onChange={(event) => setDraft((current) => ({ ...current, title: event.target.value }))} />
          <select
            className="admin-input"
            value={draft.targetKind}
            onChange={(event) =>
              setDraft((current) => ({
                ...current,
                targetKind: event.target.value,
                targetKey: event.target.value === 'report' ? targetCatalog.reports?.[0]?.key || '' : targetCatalog.documents?.[0]?.key || '',
              }))
            }
          >
            <option value="document">Document</option>
            <option value="report">Report</option>
          </select>
          <select
            className="admin-input"
            value={draft.targetKey}
            onChange={(event) => setDraft((current) => ({ ...current, targetKey: event.target.value }))}
          >
            <option value="">Select target</option>
            {targetOptions.map((item) => (
              <option key={item.key} value={item.key}>
                {item.title || item.key}
              </option>
            ))}
          </select>
          <input className="admin-input" placeholder="Purpose" value={draft.purpose} onChange={(event) => setDraft((current) => ({ ...current, purpose: event.target.value }))} />
        </div>
        <div className="mt-3 flex items-center gap-3">
          <button type="button" className="admin-button" disabled={busy || !draft.key || !draft.title || !draft.targetKey} onClick={() => void createTemplate()}>
            Create New Template
          </button>
          {message ? <div className="text-sm text-body">{message}</div> : null}
        </div>
      </section>
      <DataGrid
        columns={[
          { key: 'key', label: 'Template' },
          { key: 'title', label: 'Title' },
          { key: 'target_kind', label: 'Target' },
          { key: 'default_format', label: 'Format' },
          { key: 'purpose', label: 'Purpose' },
        ]}
        rows={rows}
        actionLabel="Open Designer"
        onAction={(row) => navigate(`/templates/designer?key=${encodeURIComponent(String(resolvePath(row, 'key') || ''))}`)}
        secondaryActionLabel="Delete"
        onSecondaryAction={(row) => void deleteTemplate(row)}
      />
    </div>
  )
}

function WorkflowListPage({ rows }: { rows: Array<Record<string, unknown>> }) {
  const navigate = useNavigate()
  const [key, setKey] = useState('')
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')

  async function createWorkflow() {
    setBusy(true)
    setMessage('')
    try {
      await mutateJson('/admin/api/workflows', {
        method: 'POST',
        body: JSON.stringify({ key }),
      })
      navigate(`/workflows/designer?key=${encodeURIComponent(key)}`)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to create workflow.')
    } finally {
      setBusy(false)
    }
  }

  async function deleteWorkflow(row: Record<string, unknown>) {
    const key = String(resolvePath(row, 'key') || '')
    if (!key) return
    setBusy(true)
    setMessage('')
    try {
      await mutateJson(`/admin/api/workflows/${encodeURIComponent(key)}`, {
        method: 'DELETE',
      })
      navigate(0)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to delete workflow.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-line bg-accent-soft/60 p-4 text-sm text-body">
        Select a workflow to open the visual designer, manage versions, validate transitions, and simulate routing.
      </div>
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <div className="mb-3 text-sm font-semibold text-body">Create New Workflow</div>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-[minmax(0,1fr)_auto]">
          <input className="admin-input" placeholder="Workflow Key" value={key} onChange={(event) => setKey(event.target.value)} />
          <button type="button" className="admin-button" disabled={busy || !key} onClick={() => void createWorkflow()}>
            Create New Workflow
          </button>
        </div>
        {message ? <div className="mt-3 text-sm text-body">{message}</div> : null}
      </section>
      <DataGrid
        columns={[{ key: 'key', label: 'Workflow' }]}
        rows={rows}
        actionLabel="Open Designer"
        onAction={(row) => navigate(`/workflows/designer?key=${encodeURIComponent(String(resolvePath(row, 'key') || ''))}`)}
        secondaryActionLabel="Delete"
        onSecondaryAction={(row) => void deleteWorkflow(row)}
      />
    </div>
  )
}

function DataGrid({
  columns,
  rows,
  actionLabel,
  onAction,
  secondaryActionLabel,
  onSecondaryAction,
}: {
  columns: Array<{ key: string; label: string }>
  rows: Array<Record<string, unknown>>
  actionLabel?: string
  onAction?: (row: Record<string, unknown>) => void
  secondaryActionLabel?: string
  onSecondaryAction?: (row: Record<string, unknown>) => void
}) {
  if (!rows.length) {
    return <div className="rounded-xl border border-dashed border-line p-6 text-sm text-muted">No data available.</div>
  }

  const rowKeyColumn = columns[0]?.key

  return (
    <div className="overflow-hidden rounded-xl border border-line">
      <table className="min-w-full divide-y divide-line text-sm">
        <thead className="border-b border-line bg-accent-soft dark:bg-ink/60">
          <tr>
            {columns.map((column) => (
              <th key={column.key} className="px-4 py-3 text-left text-xs font-bold uppercase tracking-[0.14em] text-accent-dark dark:text-body">
                {column.label}
              </th>
            ))}
            {actionLabel || secondaryActionLabel ? <th className="px-4 py-3" /> : null}
          </tr>
        </thead>
        <tbody className="divide-y divide-line bg-surface">
          {rows.map((row, index) => (
            <tr key={`${index}-${String((rowKeyColumn && resolvePath(row, rowKeyColumn)) || index)}`}>
              {columns.map((column) => (
                <td key={column.key} className="px-4 py-3 align-top text-body">
                  {displayValue(resolvePath(row, column.key))}
                </td>
              ))}
              {actionLabel || secondaryActionLabel ? (
                <td className="px-4 py-3 text-right">
                  <div className="flex justify-end gap-2">
                    {actionLabel ? (
                      <button type="button" className="admin-button admin-button-secondary" onClick={() => onAction?.(row)}>
                        {actionLabel}
                      </button>
                    ) : null}
                    {secondaryActionLabel ? (
                      <button type="button" className="admin-button admin-button-secondary" onClick={() => onSecondaryAction?.(row)}>
                        {secondaryActionLabel}
                      </button>
                    ) : null}
                  </div>
                </td>
              ) : null}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function ValueCard({ label, value }: { label: string; value: unknown }) {
  return (
    <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
      <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-body">{label}</div>
      <pre className="overflow-auto text-xs text-body">{JSON.stringify(value ?? {}, null, 2)}</pre>
    </section>
  )
}

function FieldGrid({
  label,
  fields,
  values,
}: {
  label: string
  fields: Array<Record<string, unknown>>
  values: Record<string, unknown>
}) {
  const orderedFields = fields.filter((field) => typeof field.key === 'string')
  const knownKeys = new Set(orderedFields.map((field) => String(field.key)))
  const extraFields = Object.keys(values)
    .filter((key) => !knownKeys.has(key))
    .sort()
    .map((key) => ({ key, label: startCase(key) }))
  const visibleFields = [...orderedFields, ...extraFields]

  if (!visibleFields.length) {
    return <ValueCard label={label} value={values} />
  }

  return (
    <section className="rounded-xl border border-line bg-surface dark:bg-ink/60">
      <div className="border-b border-line px-4 py-3 text-xs font-semibold uppercase tracking-wide text-body">{label}</div>
      <dl className="grid grid-cols-1 divide-y divide-line md:grid-cols-2 md:divide-x md:divide-y-0">
        {visibleFields.map((field) => {
          const key = String(field.key)
          const fieldLabel = String(field.label || startCase(key))
          return (
            <div key={key} className="space-y-2 p-4">
              <dt className="text-xs font-semibold uppercase tracking-wide text-muted">{fieldLabel}</dt>
              <dd className="text-sm text-body">{displayValue(values[key]) || 'Not set'}</dd>
            </div>
          )
        })}
      </dl>
    </section>
  )
}

function asItems(payload: Record<string, unknown> | null): Array<Record<string, unknown>> {
  const items = payload?.items
  return Array.isArray(items) ? (items as Array<Record<string, unknown>>) : []
}

function resolvePath(payload: Record<string, unknown>, path: string): unknown {
  return path.split('.').reduce<unknown>((current, key) => {
    if (current && typeof current === 'object' && key in (current as Record<string, unknown>)) {
      return (current as Record<string, unknown>)[key]
    }
    return undefined
  }, payload)
}

function displayValue(value: unknown): string {
  if (value == null) return ''
  if (typeof value === 'boolean') return value ? 'Yes' : 'No'
  if (Array.isArray(value)) return value.map((item) => displayValue(item)).filter(Boolean).join(', ')
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function formatDate(value: unknown): string {
  if (typeof value !== 'string' || !value) return 'Unknown'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

function startCase(value: string): string {
  return value
    .replace(/[_-]+/g, ' ')
    .replace(/\b\w/g, (character) => character.toUpperCase())
}
