import { useEffect, useMemo, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { Shell } from '@/components/layout/Shell'
import { useShellStore } from '@/stores/shellStore'
import { normalizeShellPath } from '@/services/bootstrap'
import { fetchJson, mutateJson } from './adminClient'
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

type ModuleConsoleSection = {
  key: string
  title?: string
  description?: string
  kind: string
  config_key?: string
  definition?: Record<string, unknown>
  entry?: Record<string, unknown>
  editable?: boolean
  links?: Array<Record<string, unknown>>
}

type ModuleDependencyNode = {
  module_key: string
  name?: string
  version?: string
  enabled?: boolean
  lifecycle_state?: string
  role?: string
  domain_family?: string
  category?: string
  status?: string
  console_path?: string
}

type ModuleDependencyEdge = {
  source_module_key: string
  target_module_key: string
  kind?: string
  version_range?: string
  status?: string
  reason?: string
}

type ModuleDependencyGraph = {
  nodes?: ModuleDependencyNode[]
  edges?: ModuleDependencyEdge[]
  summary?: {
    total_modules?: number
    enabled_modules?: number
    unhealthy_modules?: number
    total_edges?: number
  }
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
    return <ModuleManagementPage payload={payload} />
  }

  if (path.startsWith('/modules/')) {
    return <ModuleConsolePage payload={payload} />
  }

  if (path === '/auth') {
    return <AuthSettingsPage payload={payload} />
  }

  if (path === '/config') {
    return <ConfigManagementPage definitions={asItems(payload)} />
  }

  if (path === '/finance') {
    return <FinanceSettingsPage />
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
    return <SecurityHooksPage rows={asItems(payload)} />
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
  const normalizedPath = normalizeShellPath(path, 'admin')
  if (normalizedPath.startsWith('/modules/')) {
    const moduleKey = normalizedPath.slice('/modules/'.length)
    return moduleKey ? `/admin/api/modules/${encodeURIComponent(moduleKey)}/console` : ''
  }
  switch (normalizedPath) {
    case '/modules':
      return '/admin/api/modules'
    case '/config':
      return '/admin/api/config/definitions'
    case '/auth':
      return '/admin/api/auth/settings'
    case '/finance':
      return '/admin/api/config/definitions'
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
  const normalizedPath = normalizeShellPath(path, 'admin')
  if (normalizedPath.startsWith('/modules/')) {
    const moduleKey = normalizedPath.slice('/modules/'.length)
    return moduleKey ? `${startCase(moduleKey)} Console` : 'Module Console'
  }
  switch (normalizedPath) {
    case '/modules':
      return 'Modules'
    case '/config':
      return 'Configuration'
    case '/auth':
      return 'Authentication'
    case '/finance':
      return 'Finance'
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

function ModuleManagementPage({ payload }: { payload: Record<string, unknown> | null }) {
  const navigate = useNavigate()
  const rows = asItems(payload)
  const [items, setItems] = useState(rows)
  const [busyKey, setBusyKey] = useState('')
  const [message, setMessage] = useState('')

  useEffect(() => {
    setItems(rows)
  }, [rows])

  const graph = buildClientModuleDependencyGraph(items)

  async function toggleModule(row: Record<string, unknown>) {
    const key = String(resolvePath(row, 'manifest.key') || '')
    const enabled = Boolean(resolvePath(row, 'installed.enabled'))
    if (!key) return
    setBusyKey(key)
    setMessage('')
    try {
      const updated = await mutateJson<Record<string, unknown>>(`/admin/api/modules/${encodeURIComponent(key)}/actions/${enabled ? 'disable' : 'enable'}`, {
        method: 'POST',
      })
      setItems((current) =>
        current.map((item) =>
          String(resolvePath(item, 'manifest.key') || '') === key
            ? {
                ...item,
                installed: {
                  ...(((item.installed as Record<string, unknown>) || {}) as Record<string, unknown>),
                  enabled: Boolean(updated.enabled),
                  updated_at: updated.updated_at || resolvePath(item, 'installed.updated_at'),
                  updated_by: updated.updated_by || resolvePath(item, 'installed.updated_by'),
                },
                lifecycle_state: updated.lifecycle_state || item.lifecycle_state,
              }
            : item,
        ),
      )
      setMessage(`Module ${enabled ? 'disabled' : 'enabled'}.`)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to update module.')
    } finally {
      setBusyKey('')
    }
  }

  return (
    <div className="space-y-4">
      {message ? <div className="rounded-xl border border-line bg-accent-soft/60 p-4 text-sm text-body">{message}</div> : null}
      <ModuleDependencyGraphPanel
        title="Dependency Tree"
        description="Visualize module dependencies, dependency health, and direct navigation to each module console."
        graph={graph}
        onSelectModule={(moduleKey) => navigate(`/modules/${encodeURIComponent(moduleKey)}`)}
      />
      <DataGrid
        columns={[
          { key: 'manifest.key', label: 'Module' },
          { key: 'manifest.name', label: 'Name' },
          { key: 'manifest.version', label: 'Version' },
          { key: 'installed.enabled', label: 'Enabled' },
          { key: 'lifecycle_state', label: 'Lifecycle' },
        ]}
        rows={items}
        actionLabel="Open Console"
        onAction={(row) => navigate(`/modules/${encodeURIComponent(String(resolvePath(row, 'manifest.key') || ''))}`)}
        actionDisabledForRow={(row) => !Array.isArray(resolvePath(row, 'manifest.admin_console.sections') as unknown[] | undefined) || !((resolvePath(row, 'manifest.admin_console.sections') as unknown[] | undefined)?.length)}
        secondaryActionLabel="Toggle"
        secondaryActionLabelForRow={(row) => (Boolean(resolvePath(row, 'installed.enabled')) ? 'Disable' : 'Enable')}
        onSecondaryAction={(row) => void toggleModule(row)}
        secondaryActionDisabledForRow={(row) => busyKey === String(resolvePath(row, 'manifest.key') || '')}
      />
    </div>
  )
}

function AuthSettingsPage({ payload }: { payload: Record<string, unknown> | null }) {
  const definition = (payload?.definition || {}) as Record<string, unknown>
  const entry = (payload?.entry || {}) as Record<string, unknown>
  const fields = Array.isArray(definition.fields) ? (definition.fields as Array<Record<string, unknown>>) : []
  const settings = entry.value && typeof entry.value === 'object' ? (entry.value as Record<string, unknown>) : {}
  const [draft, setDraft] = useState<Record<string, unknown>>(settings)
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    setDraft(settings)
  }, [payload])

  async function save() {
    setBusy(true)
    setMessage('')
    try {
      const response = await mutateJson<{ entry: { value: Record<string, unknown> } }>('/admin/api/auth/settings', {
        method: 'PUT',
        body: JSON.stringify({
          scope: String(entry.source_scope || entry.scope || 'deployment'),
          scope_id: String(entry.source_scope_id || entry.scope_id || ''),
          value: normalizeEditorPayload(fields, draft),
        }),
      })
      setDraft((response.entry?.value as Record<string, unknown>) || draft)
      setMessage('Authentication settings updated.')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to save authentication settings.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <SummaryCard label="Key" value={String(definition.key || 'identity.auth')} />
        <SummaryCard label="Scope" value={String(entry.source_scope || entry.scope || 'deployment')} />
        <SummaryCard label="Resolved At" value={formatDate(entry.resolved_at)} />
      </div>
      <EditableFieldSection
        label="Authentication Settings"
        fields={fields}
        values={draft}
        onChange={setDraft}
      />
      <div className="flex items-center gap-3">
        <button type="button" className="admin-button" disabled={busy} onClick={() => void save()}>
          Save Settings
        </button>
        {message ? <div className="text-sm text-body">{message}</div> : null}
      </div>
    </div>
  )
}

function ConfigManagementPage({ definitions }: { definitions: Array<Record<string, unknown>> }) {
  const [effective, setEffective] = useState<Array<Record<string, unknown>>>([])
  const [selectedKey, setSelectedKey] = useState('')
  const [draft, setDraft] = useState<Record<string, unknown>>({})
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let mounted = true
    async function load() {
      try {
        const payload = await fetchJson<{ items: Array<Record<string, unknown>> }>('/admin/api/config/effective')
        if (!mounted) return
        setEffective(payload.items || [])
      } catch {
        if (!mounted) return
      }
    }
    void load()
    return () => {
      mounted = false
    }
  }, [])

  const selectedDefinition = definitions.find((item) => String(item.key || '') === selectedKey) || null
  const selectedEffective = effective.find((item) => String(item.key || '') === selectedKey) || null
  const selectedFields = Array.isArray(selectedDefinition?.fields) ? (selectedDefinition?.fields as Array<Record<string, unknown>>) : []

  useEffect(() => {
    if (selectedKey || definitions.length === 0) {
      return
    }
    const first = definitions[0]
    if (!first) {
      return
    }
    const key = String(first.key || '')
    const current = effective.find((item) => String(item.key || '') === key)
    setSelectedKey(key)
    setDraft(((current?.value as Record<string, unknown>) || (first.default_value as Record<string, unknown>) || {}) as Record<string, unknown>)
  }, [definitions, effective, selectedKey])

  function openEditor(row: Record<string, unknown>) {
    const key = String(resolvePath(row, 'key') || '')
    const current = effective.find((item) => String(item.key || '') === key)
    setSelectedKey(key)
    setDraft(((current?.value as Record<string, unknown>) || (row.default_value as Record<string, unknown>) || {}) as Record<string, unknown>)
    setMessage('')
  }

  async function save() {
    if (!selectedDefinition || !selectedKey) return
    setBusy(true)
    setMessage('')
    try {
      const response = await mutateJson<Record<string, unknown>>(`/admin/api/config/entries/${encodeURIComponent(selectedKey)}/value`, {
        method: 'PUT',
        body: JSON.stringify({
          scope: normalizeEditorScope(selectedEffective?.source_scope),
          scope_id: normalizeEditorScopeID(selectedEffective?.source_scope, selectedEffective?.source_scope_id),
          value: normalizeEditorPayload(selectedFields, draft),
        }),
      })
      setEffective((current) => {
        const next = current.filter((item) => String(item.key || '') !== selectedKey)
        next.push(response)
        return next.sort((left, right) => String(left.key || '').localeCompare(String(right.key || '')))
      })
      setDraft((response.value as Record<string, unknown>) || draft)
      setMessage('Configuration updated.')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to update configuration.')
    } finally {
      setBusy(false)
    }
  }

  const rows = definitions.map((item) => {
    const current = effective.find((value) => String(value.key || '') === String(item.key || ''))
    return {
      ...item,
      current_scope: current?.source_scope || 'default',
      current_value: displayValue(current?.value),
    }
  })

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-line bg-accent-soft/60 p-4 text-sm text-body">
        Configuration values are editable. Select a row to load its form, then use <span className="font-semibold">Save Configuration</span>.
      </div>
      <DataGrid
        columns={[
          { key: 'key', label: 'Key' },
          { key: 'module_key', label: 'Module' },
          { key: 'current_scope', label: 'Current Scope' },
          { key: 'current_value', label: 'Current Value' },
        ]}
        rows={rows}
        actionLabel="Edit"
        onAction={openEditor}
      />
      {selectedDefinition ? (
        <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
          <div className="mb-3 text-sm font-semibold text-body">Edit {String(selectedDefinition.key || '')}</div>
          <EditableFieldSection label="Value" fields={selectedFields} values={draft} onChange={setDraft} />
          <div className="mt-4 flex items-center gap-3">
            <button type="button" className="admin-button" disabled={busy} onClick={() => void save()}>
              Save Configuration
            </button>
            {message ? <div className="text-sm text-body">{message}</div> : null}
          </div>
        </section>
      ) : null}
    </div>
  )
}

function FinanceSettingsPage() {
  const [definition, setDefinition] = useState<Record<string, unknown> | null>(null)
  const [effective, setEffective] = useState<Record<string, unknown> | null>(null)
  const [draft, setDraft] = useState<Record<string, unknown>>({})
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let mounted = true
    async function load() {
      const [definitionsPayload, effectivePayload] = await Promise.all([
        fetchJson<{ items: Array<Record<string, unknown>> }>('/admin/api/config/definitions'),
        fetchJson<{ items: Array<Record<string, unknown>> }>('/admin/api/config/effective'),
      ])
      if (!mounted) return
      const financeDefinition = (definitionsPayload.items || []).find((item) => String(item.key || '') === 'commercial.posting') || null
      const financeEffective = (effectivePayload.items || []).find((item) => String(item.key || '') === 'commercial.posting') || null
      setDefinition(financeDefinition)
      setEffective(financeEffective)
      setDraft(((financeEffective?.value as Record<string, unknown>) || (financeDefinition?.default_value as Record<string, unknown>) || {}) as Record<string, unknown>)
    }
    void load()
    return () => {
      mounted = false
    }
  }, [])

  const fields = Array.isArray(definition?.fields) ? (definition?.fields as Array<Record<string, unknown>>) : []

  async function save() {
    if (!definition) return
    setBusy(true)
    setMessage('')
    try {
      const response = await mutateJson<Record<string, unknown>>('/admin/api/config/entries/commercial.posting/value', {
        method: 'PUT',
        body: JSON.stringify({
          scope: normalizeEditorScope(effective?.source_scope),
          scope_id: normalizeEditorScopeID(effective?.source_scope, effective?.source_scope_id),
          value: normalizeEditorPayload(fields, draft),
        }),
      })
      setEffective(response)
      setDraft((response.value as Record<string, unknown>) || draft)
      setMessage('Finance posting defaults updated.')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to update finance settings.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <SummaryCard label="Config Key" value="commercial.posting" />
        <SummaryCard label="Scope" value={String(effective?.source_scope || effective?.scope || 'deployment')} />
        <SummaryCard label="Purpose" value="Posting Defaults" />
      </div>
      <div className="rounded-xl border border-line bg-accent-soft/60 p-4 text-sm text-body">
        Set the default receivable, revenue, tax, and clearing accounts used by commercial invoice and payment postings when the document or catalog does not override them.
      </div>
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <EditableFieldSection label="Posting Defaults" fields={fields} values={draft} onChange={setDraft} />
        <div className="mt-4 flex items-center gap-3">
          <button type="button" className="admin-button" disabled={busy} onClick={() => void save()}>
            Save Finance Settings
          </button>
          {message ? <div className="text-sm text-body">{message}</div> : null}
        </div>
      </section>
    </div>
  )
}

function ModuleConsolePage({ payload }: { payload: Record<string, unknown> | null }) {
  const navigate = useNavigate()
  const consolePayload = ((payload?.console as Record<string, unknown> | null) || {}) as Record<string, unknown>
  const moduleDetail = ((payload?.module as Record<string, unknown> | null) || {}) as Record<string, unknown>
  const dependencyGraph = ((payload?.dependency_graph as ModuleDependencyGraph | null) || {}) as ModuleDependencyGraph
  const sections = Array.isArray(consolePayload.sections) ? (consolePayload.sections as ModuleConsoleSection[]) : []
  const title = String(consolePayload.title || resolvePath(moduleDetail, 'manifest.name') || 'Module Console')
  const description = String(consolePayload.description || '')

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-line bg-accent-soft/60 p-4 text-sm text-body">
        <div className="font-semibold text-body">{title}</div>
        {description ? <div className="mt-1 text-sm text-body">{description}</div> : null}
      </div>
      <ModuleDependencyGraphPanel
        title="Dependency Tree"
        description="This focused view shows the current module, its direct dependencies, and modules that depend on it."
        graph={dependencyGraph}
        compact
        onSelectModule={(moduleKey) => navigate(`/modules/${encodeURIComponent(moduleKey)}`)}
      />
      {sections.length ? (
        sections.map((section) => {
          if (section.kind === 'settings_form') {
            return <ModuleConsoleSettingsSection key={section.key} section={section} />
          }
          return <ModuleConsoleLinkSection key={section.key} section={section} />
        })
      ) : (
        <ValueCard label="Console Payload" value={payload ?? {}} />
      )}
    </div>
  )
}

function ModuleDependencyGraphPanel({
  title,
  description,
  graph,
  onSelectModule,
  compact = false,
}: {
  title: string
  description?: string
  graph: ModuleDependencyGraph
  onSelectModule: (moduleKey: string) => void
  compact?: boolean
}) {
  const nodes = Array.isArray(graph.nodes) ? graph.nodes : []
  const edges = Array.isArray(graph.edges) ? graph.edges : []
  const summary = graph.summary || {}

  if (!nodes.length) {
    return (
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <div className="text-sm font-semibold text-body">{title}</div>
        {description ? <div className="mt-1 text-sm text-muted">{description}</div> : null}
        <div className="mt-4 rounded-xl border border-dashed border-line p-4 text-sm text-muted">No dependency graph is available.</div>
      </section>
    )
  }

  const incoming = new Map<string, number>()
  const outgoing = new Map<string, number>()
  for (const edge of edges) {
    outgoing.set(edge.source_module_key, (outgoing.get(edge.source_module_key) || 0) + 1)
    incoming.set(edge.target_module_key, (incoming.get(edge.target_module_key) || 0) + 1)
  }

  const sortedNodes = [...nodes].sort((left, right) => {
    const leftWeight = (incoming.get(left.module_key) || 0) + (outgoing.get(left.module_key) || 0)
    const rightWeight = (incoming.get(right.module_key) || 0) + (outgoing.get(right.module_key) || 0)
    if (leftWeight === rightWeight) return String(left.name || left.module_key).localeCompare(String(right.name || right.module_key))
    return rightWeight - leftWeight
  })

  const maxRows = compact ? 12 : 24
  const visibleNodes = sortedNodes.slice(0, maxRows)
  const visibleKeys = new Set(visibleNodes.map((node) => node.module_key))
  const visibleEdges = edges.filter((edge) => visibleKeys.has(edge.source_module_key) && visibleKeys.has(edge.target_module_key))
  const layout = buildModuleGraphLayout(visibleNodes, visibleEdges)

  return (
    <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="text-sm font-semibold text-body">{title}</div>
          {description ? <div className="mt-1 text-sm text-muted">{description}</div> : null}
        </div>
        <div className="grid grid-cols-2 gap-2 text-xs text-body md:grid-cols-4">
          <MiniStat label="Modules" value={String(summary.total_modules || nodes.length)} />
          <MiniStat label="Enabled" value={String(summary.enabled_modules || nodes.filter((node) => node.enabled).length)} />
          <MiniStat label="Warnings" value={String(summary.unhealthy_modules || nodes.filter((node) => node.status !== 'healthy').length)} />
          <MiniStat label="Edges" value={String(summary.total_edges || edges.length)} />
        </div>
      </div>
      <div className="overflow-auto rounded-xl border border-line bg-shell/40 p-3 dark:bg-ink/70">
        <svg width={layout.width} height={layout.height} className="min-w-full">
          <defs>
            <marker id="module-edge-arrow" markerWidth="10" markerHeight="10" refX="9" refY="3" orient="auto" markerUnits="strokeWidth">
              <path d="M0,0 L0,6 L9,3 z" fill="#6b7280" />
            </marker>
          </defs>
          {visibleEdges.map((edge) => {
            const source = layout.positions.get(edge.source_module_key)
            const target = layout.positions.get(edge.target_module_key)
            if (!source || !target) return null
            const startX = source.x + layout.nodeWidth
            const startY = source.y + layout.nodeHeight / 2
            const endX = target.x
            const endY = target.y + layout.nodeHeight / 2
            const delta = Math.max(40, (endX - startX) / 2)
            const path = `M ${startX} ${startY} C ${startX + delta} ${startY}, ${endX - delta} ${endY}, ${endX} ${endY}`
            return (
              <path
                key={`${edge.source_module_key}:${edge.target_module_key}:${edge.kind}`}
                d={path}
                fill="none"
                stroke={moduleEdgeColor(edge.status)}
                strokeWidth={2}
                markerEnd="url(#module-edge-arrow)"
                opacity={0.9}
              />
            )
          })}
          {visibleNodes.map((node) => {
            const position = layout.positions.get(node.module_key)
            if (!position) return null
            return (
              <g
                key={node.module_key}
                transform={`translate(${position.x}, ${position.y})`}
                className="cursor-pointer"
                onClick={() => onSelectModule(node.module_key)}
              >
                <rect
                  width={layout.nodeWidth}
                  height={layout.nodeHeight}
                  rx={14}
                  fill={moduleNodeFill(node.status)}
                  stroke={moduleNodeStroke(node.status)}
                  strokeWidth={2}
                />
                <text x={14} y={24} fontSize="13" fontWeight="700" fill="#111827">
                  {String(node.name || node.module_key).slice(0, 28)}
                </text>
                <text x={14} y={42} fontSize="11" fill="#374151">
                  {node.module_key}
                </text>
                <text x={14} y={60} fontSize="11" fill="#4b5563">
                  {node.version || '-'} • {node.role || 'module'}
                </text>
                <text x={14} y={78} fontSize="11" fill="#4b5563">
                  {node.lifecycle_state || node.status || '-'}
                </text>
              </g>
            )
          })}
        </svg>
      </div>
      {visibleNodes.length < nodes.length ? (
        <div className="mt-3 text-xs text-muted">
          Showing {visibleNodes.length} of {nodes.length} modules. Open a module console for a focused local dependency view.
        </div>
      ) : null}
    </section>
  )
}

function MiniStat({ label, value }: { label: string; value: string }) {
  return (
    <article className="rounded-lg border border-line bg-surface px-3 py-2 dark:bg-ink/60">
      <div className="text-[10px] font-semibold uppercase tracking-[0.14em] text-muted">{label}</div>
      <div className="mt-1 text-sm font-semibold text-body">{value}</div>
    </article>
  )
}

function buildClientModuleDependencyGraph(rows: Array<Record<string, unknown>>): ModuleDependencyGraph {
  const nodes = rows
    .map((row): ModuleDependencyNode | null => {
      const moduleKey = String(resolvePath(row, 'manifest.key') || '')
      if (!moduleKey) return null
      const lifecycleState = String(resolvePath(row, 'lifecycle_state') || '')
      const enabled = Boolean(resolvePath(row, 'installed.enabled'))
      return {
        module_key: moduleKey,
        name: String(resolvePath(row, 'manifest.name') || moduleKey),
        version: String(resolvePath(row, 'manifest.version') || ''),
        enabled,
        lifecycle_state: lifecycleState,
        role: String(resolvePath(row, 'manifest.role') || ''),
        domain_family: String(resolvePath(row, 'manifest.domain_family') || ''),
        category: String(resolvePath(row, 'manifest.category') || ''),
        status: !enabled || lifecycleState === 'disabled' ? 'disabled' : lifecycleState === 'healthy' ? 'healthy' : 'warning',
        console_path: `/admin/modules/${moduleKey}`,
      }
    })
    .filter((node): node is ModuleDependencyNode => node !== null)

  const detailByKey = new Map(rows.map((row) => [String(resolvePath(row, 'manifest.key') || ''), row]))
  const edges: ModuleDependencyEdge[] = []
  for (const row of rows) {
    const sourceModuleKey = String(resolvePath(row, 'manifest.key') || '')
    if (!sourceModuleKey) continue
    const diagnostics = Array.isArray(resolvePath(row, 'dependency_diagnostics')) ? (resolvePath(row, 'dependency_diagnostics') as Array<Record<string, unknown>>) : []
    const diagnosticsByKey = new Map(diagnostics.map((item) => [String(item.module_key || ''), item]))
    const requirements = clientManifestDependencies(row)
    for (const requirement of requirements) {
      const targetModuleKey = String(requirement.module_key || '')
      if (!targetModuleKey) continue
      const diagnostic = diagnosticsByKey.get(targetModuleKey)
      const dependencyRow = detailByKey.get(targetModuleKey)
      const kind = String(requirement.kind || 'required')
      const enabled = diagnostic ? Boolean(diagnostic.enabled) : Boolean(dependencyRow ? resolvePath(dependencyRow, 'installed.enabled') : false)
      const compatible = diagnostic ? Boolean(diagnostic.compatible) : true
      edges.push({
        source_module_key: sourceModuleKey,
        target_module_key: targetModuleKey,
        kind,
        version_range: String(requirement.version_range || ''),
        status: !diagnostic
          ? 'missing'
          : !enabled
            ? kind === 'optional'
              ? 'optional'
              : 'disabled'
            : !compatible
              ? 'incompatible'
              : 'ok',
        reason: String((diagnostic && diagnostic.reason) || ''),
      })
    }
  }

  return {
    nodes,
    edges,
    summary: {
      total_modules: nodes.length,
      enabled_modules: nodes.filter((node) => node.enabled).length,
      unhealthy_modules: nodes.filter((node) => node.status !== 'healthy').length,
      total_edges: edges.length,
    },
  }
}

function clientManifestDependencies(row: Record<string, unknown>): Array<Record<string, unknown>> {
  const dependencyRequirements = resolvePath(row, 'manifest.dependency_requirements')
  if (Array.isArray(dependencyRequirements) && dependencyRequirements.length > 0) {
    return dependencyRequirements.filter((item): item is Record<string, unknown> => Boolean(item && typeof item === 'object'))
  }
  const dependencies = resolvePath(row, 'manifest.dependencies')
  if (!Array.isArray(dependencies)) return []
  return dependencies.map((moduleKey) => ({
    module_key: String(moduleKey || ''),
    kind: 'required',
    version_range: '',
  }))
}

function buildModuleGraphLayout(nodes: ModuleDependencyNode[], edges: ModuleDependencyEdge[]) {
  const nodeWidth = 240
  const nodeHeight = 92
  const columnGap = 90
  const rowGap = 26
  const incoming = new Map<string, number>()
  const outgoing = new Map<string, number>()
  const dependents = new Map<string, string[]>()
  const indegree = new Map<string, number>()
  for (const node of nodes) {
    incoming.set(node.module_key, 0)
    outgoing.set(node.module_key, 0)
    dependents.set(node.module_key, [])
    indegree.set(node.module_key, 0)
  }
  for (const edge of edges) {
    if (!incoming.has(edge.target_module_key) || !outgoing.has(edge.source_module_key)) continue
    incoming.set(edge.target_module_key, (incoming.get(edge.target_module_key) || 0) + 1)
    outgoing.set(edge.source_module_key, (outgoing.get(edge.source_module_key) || 0) + 1)
    dependents.set(edge.source_module_key, [...(dependents.get(edge.source_module_key) || []), edge.target_module_key])
    indegree.set(edge.target_module_key, (indegree.get(edge.target_module_key) || 0) + 1)
  }

  const levels = new Map<string, number>()
  const queue: string[] = []
  for (const node of nodes) {
    if ((indegree.get(node.module_key) || 0) === 0) {
      queue.push(node.module_key)
      levels.set(node.module_key, 0)
    }
  }
  queue.sort()
  while (queue.length) {
    const current = queue.shift() || ''
    const level = levels.get(current) || 0
    for (const target of dependents.get(current) || []) {
      const nextLevel = Math.max(levels.get(target) || 0, level + 1)
      levels.set(target, nextLevel)
      indegree.set(target, (indegree.get(target) || 0) - 1)
      if ((indegree.get(target) || 0) === 0) queue.push(target)
    }
  }
  for (const node of nodes) {
    if (!levels.has(node.module_key)) levels.set(node.module_key, 0)
  }

  const columns = new Map<number, ModuleDependencyNode[]>()
  for (const node of nodes) {
    const level = levels.get(node.module_key) || 0
    columns.set(level, [...(columns.get(level) || []), node])
  }
  for (const [level, group] of columns.entries()) {
    group.sort((left, right) => {
      const leftWeight = (incoming.get(left.module_key) || 0) + (outgoing.get(left.module_key) || 0)
      const rightWeight = (incoming.get(right.module_key) || 0) + (outgoing.get(right.module_key) || 0)
      if (leftWeight === rightWeight) return String(left.name || left.module_key).localeCompare(String(right.name || right.module_key))
      return rightWeight - leftWeight
    })
    columns.set(level, group)
  }

  const positions = new Map<string, { x: number; y: number }>()
  const maxColumnSize = Math.max(...Array.from(columns.values()).map((group) => group.length), 1)
  const width = Math.max(860, columns.size * (nodeWidth + columnGap) + 80)
  const height = Math.max(240, maxColumnSize * (nodeHeight + rowGap) + 60)
  for (const [level, group] of Array.from(columns.entries()).sort((left, right) => left[0] - right[0])) {
    const totalHeight = group.length * nodeHeight + Math.max(0, group.length-1) * rowGap
    const startY = Math.max(24, Math.round((height - totalHeight) / 2))
    group.forEach((node, index) => {
      positions.set(node.module_key, {
        x: 24 + level * (nodeWidth + columnGap),
        y: startY + index * (nodeHeight + rowGap),
      })
    })
  }
  return { positions, width, height, nodeWidth, nodeHeight }
}

function moduleNodeFill(status: string | undefined) {
  switch (status) {
    case 'disabled':
      return '#f3f4f6'
    case 'warning':
      return '#fff7ed'
    default:
      return '#ecfdf5'
  }
}

function moduleNodeStroke(status: string | undefined) {
  switch (status) {
    case 'disabled':
      return '#9ca3af'
    case 'warning':
      return '#f97316'
    default:
      return '#10b981'
  }
}

function moduleEdgeColor(status: string | undefined) {
  switch (status) {
    case 'disabled':
      return '#9ca3af'
    case 'missing':
    case 'incompatible':
      return '#dc2626'
    case 'optional':
      return '#f59e0b'
    default:
      return '#6b7280'
  }
}

function ModuleConsoleSettingsSection({ section }: { section: ModuleConsoleSection }) {
  const definition = (section.definition || {}) as Record<string, unknown>
  const entry = (section.entry || {}) as Record<string, unknown>
  const fields = Array.isArray(definition.fields) ? (definition.fields as Array<Record<string, unknown>>) : []
  const resolved = entry.value && typeof entry.value === 'object' ? (entry.value as Record<string, unknown>) : {}
  const defaults = (definition.default_value && typeof definition.default_value === 'object' ? (definition.default_value as Record<string, unknown>) : {}) || {}
  const [draft, setDraft] = useState<Record<string, unknown>>({ ...defaults, ...resolved })
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    setDraft({ ...defaults, ...resolved })
  }, [section.config_key, definition.default_value, entry.value])

  async function save() {
    if (!section.config_key || !section.editable) return
    setBusy(true)
    setMessage('')
    try {
      const response = await mutateJson<Record<string, unknown>>(`/admin/api/config/entries/${encodeURIComponent(section.config_key)}/value`, {
        method: 'PUT',
        body: JSON.stringify({
          scope: normalizeEditorScope(entry.source_scope),
          scope_id: normalizeEditorScopeID(entry.source_scope, entry.source_scope_id),
          value: normalizeEditorPayload(fields, draft),
        }),
      })
      setDraft((response.value as Record<string, unknown>) || draft)
      setMessage('Configuration updated.')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to update configuration.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
      <div className="mb-3">
        <div className="text-sm font-semibold text-body">{String(section.title || section.config_key || 'Settings')}</div>
        {section.description ? <div className="mt-1 text-sm text-muted">{section.description}</div> : null}
      </div>
      <EditableFieldSection
        label={String(section.title || section.config_key || 'Settings')}
        fields={fields}
        values={draft}
        onChange={setDraft}
        disabled={!section.editable}
      />
      <div className="mt-4 flex items-center gap-3">
        {section.editable ? (
          <button type="button" className="admin-button" disabled={busy} onClick={() => void save()}>
            Save Settings
          </button>
        ) : (
          <div className="text-sm text-muted">Read-only for your current permissions.</div>
        )}
        {message ? <div className="text-sm text-body">{message}</div> : null}
      </div>
    </section>
  )
}

function ModuleConsoleLinkSection({ section }: { section: ModuleConsoleSection }) {
  const links = Array.isArray(section.links) ? section.links : []

  return (
    <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
      <div className="mb-4">
        <div className="text-sm font-semibold text-body">{String(section.title || 'Links')}</div>
        {section.description ? <div className="mt-1 text-sm text-muted">{section.description}</div> : null}
      </div>
      {links.length ? (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
          {links.map((link) => {
            const routePath = String(link.route_path || '')
            return (
              <article key={String(link.key || routePath)} className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
                <div className="text-sm font-semibold text-body">{String(link.label || routePath)}</div>
                {link.description ? <div className="mt-1 text-sm text-muted">{String(link.description)}</div> : null}
                <div className="mt-4">
                  <button
                    type="button"
                    className="admin-button admin-button-secondary"
                    onClick={() => {
                      if (routePath) {
                        window.location.assign(routePath)
                      }
                    }}
                  >
                    Open
                  </button>
                </div>
              </article>
            )
          })}
        </div>
      ) : (
        <div className="rounded-xl border border-dashed border-line p-4 text-sm text-muted">No links available for your current permissions.</div>
      )}
    </section>
  )
}

function SecurityHooksPage({ rows }: { rows: Array<Record<string, unknown>> }) {
  const [items, setItems] = useState(rows)
  const [selectedKey, setSelectedKey] = useState('')
  const [draft, setDraft] = useState<Record<string, unknown>>({})
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const selected = items.find((row) => String(resolvePath(row, 'definition.key') || '') === selectedKey) || null
  const fields = Array.isArray(selected?.rule_fields) ? (selected?.rule_fields as Array<Record<string, unknown>>) : []

  useEffect(() => {
    setItems(rows)
  }, [rows])

  function openEditor(row: Record<string, unknown>) {
    setSelectedKey(String(resolvePath(row, 'definition.key') || ''))
    setDraft(((resolvePath(row, 'rule.value') as Record<string, unknown>) || {}) as Record<string, unknown>)
    setMessage('')
  }

  async function save() {
    if (!selected || !selectedKey) return
    setBusy(true)
    setMessage('')
    try {
      const response = await mutateJson<Record<string, unknown>>(`/admin/api/security/policy-hooks/${encodeURIComponent(selectedKey)}`, {
        method: 'PUT',
        body: JSON.stringify({
          scope: normalizeEditorScope(resolvePath(selected, 'rule.source_scope')),
          scope_id: normalizeEditorScopeID(resolvePath(selected, 'rule.source_scope'), resolvePath(selected, 'rule.source_scope_id')),
          value: normalizeEditorPayload(fields, draft),
        }),
      })
      setItems((current) =>
        current.map((row) => (String(resolvePath(row, 'definition.key') || '') === selectedKey ? response : row)),
      )
      setDraft(((response.rule as Record<string, unknown>)?.value as Record<string, unknown>) || draft)
      setMessage('Security policy updated.')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to update security policy.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-4">
      <DataGrid
        columns={[
          { key: 'definition.key', label: 'Hook' },
          { key: 'definition.kind', label: 'Kind' },
          { key: 'definition.target', label: 'Target' },
          { key: 'rule.source_scope', label: 'Scope' },
          { key: 'engine', label: 'Engine' },
          { key: 'eval_valid', label: 'Valid' },
        ]}
        rows={items}
        actionLabel="Edit"
        onAction={openEditor}
      />
      {selected ? (
        <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
          <div className="mb-3 text-sm font-semibold text-body">Edit {String(resolvePath(selected, 'definition.key') || '')}</div>
          <EditableFieldSection label="Rule" fields={fields} values={draft} onChange={setDraft} />
          <div className="mt-4 flex items-center gap-3">
            <button type="button" className="admin-button" disabled={busy} onClick={() => void save()}>
              Save Policy
            </button>
            {message ? <div className="text-sm text-body">{message}</div> : null}
          </div>
        </section>
      ) : null}
    </div>
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
          <input
            id="template-create-key"
            name="template_create_key"
            className="admin-input"
            placeholder="Key"
            value={draft.key}
            onChange={(event) => setDraft((current) => ({ ...current, key: event.target.value }))}
          />
          <input
            id="template-create-title"
            name="template_create_title"
            className="admin-input"
            placeholder="Title"
            value={draft.title}
            onChange={(event) => setDraft((current) => ({ ...current, title: event.target.value }))}
          />
          <select
            id="template-create-target-kind"
            name="template_create_target_kind"
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
            id="template-create-target-key"
            name="template_create_target_key"
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
          <input
            id="template-create-purpose"
            name="template_create_purpose"
            className="admin-input"
            placeholder="Purpose"
            value={draft.purpose}
            onChange={(event) => setDraft((current) => ({ ...current, purpose: event.target.value }))}
          />
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
          <input
            id="workflow-create-key"
            name="workflow_create_key"
            className="admin-input"
            placeholder="Workflow Key"
            value={key}
            onChange={(event) => setKey(event.target.value)}
          />
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
  actionLabelForRow,
  actionDisabledForRow,
  secondaryActionLabel,
  secondaryActionLabelForRow,
  onSecondaryAction,
  secondaryActionDisabledForRow,
}: {
  columns: Array<{ key: string; label: string }>
  rows: Array<Record<string, unknown>>
  actionLabel?: string
  onAction?: (row: Record<string, unknown>) => void
  actionLabelForRow?: (row: Record<string, unknown>) => string
  actionDisabledForRow?: (row: Record<string, unknown>) => boolean
  secondaryActionLabel?: string
  secondaryActionLabelForRow?: (row: Record<string, unknown>) => string
  onSecondaryAction?: (row: Record<string, unknown>) => void
  secondaryActionDisabledForRow?: (row: Record<string, unknown>) => boolean
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
                      <button type="button" className="admin-button admin-button-secondary" disabled={actionDisabledForRow?.(row)} onClick={() => onAction?.(row)}>
                        {actionLabelForRow ? actionLabelForRow(row) : actionLabel}
                      </button>
                    ) : null}
                    {secondaryActionLabel ? (
                      <button type="button" className="admin-button admin-button-secondary" disabled={secondaryActionDisabledForRow?.(row)} onClick={() => onSecondaryAction?.(row)}>
                        {secondaryActionLabelForRow ? secondaryActionLabelForRow(row) : secondaryActionLabel}
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

function EditableFieldSection({
  label,
  fields,
  values,
  onChange,
  disabled,
}: {
  label: string
  fields: Array<Record<string, unknown>>
  values: Record<string, unknown>
  onChange: (next: Record<string, unknown>) => void
  disabled?: boolean
}) {
  const visibleFields = fields.filter((field) => typeof field.key === 'string')
  return (
    <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
      <div className="mb-4 text-sm font-semibold text-body">{label}</div>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        {visibleFields.map((field) => {
          const key = String(field.key)
          const type = String(field.type || 'string')
          const labelText = String(field.label || startCase(key))
          const value = values[key]
          const enumValues = Array.isArray(field.enum) ? (field.enum as string[]) : []
          const fieldId = `${label.toLowerCase().replace(/[^a-z0-9]+/g, '-')}-${key}`
          if (type === 'bool') {
            return (
              <label key={key} className="flex items-center gap-3 rounded-xl border border-line p-3 text-sm text-body" htmlFor={fieldId}>
                <input id={fieldId} name={fieldId} type="checkbox" disabled={disabled} checked={Boolean(value)} onChange={(event) => onChange({ ...values, [key]: event.target.checked })} />
                <span>{labelText}</span>
              </label>
            )
          }
          if (enumValues.length > 0) {
            return (
              <label key={key} className="space-y-2 text-sm text-body" htmlFor={fieldId}>
                <span className="block text-xs font-semibold uppercase tracking-wide text-muted">{labelText}</span>
                <select id={fieldId} name={fieldId} className="admin-input" disabled={disabled} value={String(value ?? '')} onChange={(event) => onChange({ ...values, [key]: event.target.value })}>
                  <option value="">Select {labelText}</option>
                  {enumValues.map((item) => (
                    <option key={item} value={item}>
                      {item}
                    </option>
                  ))}
                </select>
              </label>
            )
          }
          if (type === 'string_list') {
            return (
              <label key={key} className="space-y-2 text-sm text-body md:col-span-2" htmlFor={fieldId}>
                <span className="block text-xs font-semibold uppercase tracking-wide text-muted">{labelText}</span>
                <textarea
                  id={fieldId}
                  name={fieldId}
                  className="admin-input min-h-24"
                  disabled={disabled}
                  value={Array.isArray(value) ? value.map(String).join('\n') : ''}
                  onChange={(event) => onChange({ ...values, [key]: event.target.value.split('\n').map((item) => item.trim()).filter(Boolean) })}
                />
              </label>
            )
          }
          return (
            <label key={key} className="space-y-2 text-sm text-body" htmlFor={fieldId}>
              <span className="block text-xs font-semibold uppercase tracking-wide text-muted">{labelText}</span>
              <input
                id={fieldId}
                name={fieldId}
                className="admin-input"
                disabled={disabled}
                type={type === 'int' ? 'number' : 'text'}
                value={type === 'int' ? String(value ?? 0) : String(value ?? '')}
                onChange={(event) => onChange({ ...values, [key]: type === 'int' ? event.target.value : event.target.value })}
              />
            </label>
          )
        })}
      </div>
    </section>
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

function normalizeEditorPayload(fields: Array<Record<string, unknown>>, values: Record<string, unknown>): Record<string, unknown> {
  const fieldTypes = new Map(fields.map((field) => [String(field.key || ''), String(field.type || 'string')]))
  const payload: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(values)) {
    const type = fieldTypes.get(key) || 'string'
    if (type === 'int') {
      payload[key] = typeof value === 'number' ? value : Number.parseInt(String(value || '0'), 10) || 0
      continue
    }
    if (type === 'bool') {
      payload[key] = Boolean(value)
      continue
    }
    if (type === 'string_list') {
      payload[key] = Array.isArray(value) ? value.map((item) => String(item)) : []
      continue
    }
    payload[key] = value
  }
  return payload
}

function normalizeEditorScope(scope: unknown): string {
  const value = String(scope || '').trim()
  if (value === '' || value === 'default') {
    return 'deployment'
  }
  return value
}

function normalizeEditorScopeID(scope: unknown, scopeID: unknown): string {
  return normalizeEditorScope(scope) === 'deployment' ? '' : String(scopeID || '').trim()
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
