import { FormEvent, useEffect, useMemo, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { PaginationBar } from '@/components/ui/PaginationBar'
import { formatDate } from './adminShared'

type AuditEvent = {
  id: string
  action: string
  target_type: string
  target_id: string
  actor_id: string
  actor_kind?: string
  on_behalf_of_user_id?: string
  delegation_grant_id?: string
  from_state?: string
  to_state?: string
  organization_id?: string
  location_id?: string
  operating_unit_id?: string
  request_id?: string
  occurred_at: string
  change_summary?: Record<string, unknown>
  metadata?: Record<string, unknown>
  correlation_id?: string
}

type AuditTrailPayload = {
  items?: AuditEvent[]
  total?: number
  summary?: { count?: number }
  facets?: {
    actions?: Record<string, number>
    target_types?: Record<string, number>
    actors?: Record<string, number>
  }
}

const filterKeys = [
  'q',
  'action',
  'target_type',
  'target_id',
  'actor_id',
  'actor_kind',
  'on_behalf_of_user_id',
  'correlation_id',
  'request_id',
  'delegation_grant_id',
  'organization_id',
  'location_id',
  'operating_unit_id',
  'from_state',
  'to_state',
  'from',
  'to',
  'metadata_key',
  'metadata_value',
] as const

type FilterKey = (typeof filterKeys)[number]
type FilterState = Record<FilterKey, string>

export function AdminAuditTrailPage({ payload }: { payload: Record<string, unknown> | null }) {
  const location = useLocation()
  const navigate = useNavigate()
  const params = useMemo(() => new URLSearchParams(location.search), [location.search])
  const queryState = useMemo(() => filtersFromParams(params), [params])
  const [filters, setFilters] = useState<FilterState>(queryState)
  const [selected, setSelected] = useState<AuditEvent | null>(null)
  const auditPayload = (payload || {}) as AuditTrailPayload
  const items = Array.isArray(auditPayload.items) ? auditPayload.items : []
  const total = Number(auditPayload.total || 0)
  const page = Number.parseInt(params.get('page') || '1', 10) || 1
  const pageSize = Number.parseInt(params.get('page_size') || '20', 10) || 20
  const sort = params.get('sort') || 'occurred_at'
  const direction = params.get('direction') || 'desc'
  const activeFilterCount = filterKeys.filter((key) => queryState[key]).length

  useEffect(() => setFilters(queryState), [queryState])

  function navigateWith(next: URLSearchParams) {
    navigate(`/audit?${next.toString()}`)
  }

  function applyFilters(event?: FormEvent) {
    event?.preventDefault()
    const next = new URLSearchParams(params)
    for (const key of filterKeys) {
      const value = filters[key].trim()
      if (value) next.set(key, value)
      else next.delete(key)
    }
    next.set('page', '1')
    next.set('page_size', String(pageSize))
    next.set('sort', sort)
    next.set('direction', direction)
    navigateWith(next)
  }

  function clearFilters() {
    const next = new URLSearchParams()
    next.set('page', '1')
    next.set('page_size', String(pageSize))
    next.set('sort', 'occurred_at')
    next.set('direction', 'desc')
    navigateWith(next)
  }

  function setRangePreset(preset: 'today' | '24h' | '7d') {
    const now = new Date()
    const from = new Date(now)
    if (preset === 'today') {
      from.setHours(0, 0, 0, 0)
    } else if (preset === '24h') {
      from.setTime(now.getTime() - 24 * 60 * 60 * 1000)
    } else {
      from.setTime(now.getTime() - 7 * 24 * 60 * 60 * 1000)
    }
    setFilters({
      ...filters,
      from: from.toISOString(),
      to: now.toISOString(),
    })
  }

  function setSort(nextSort: string) {
    const next = new URLSearchParams(params)
    const nextDirection = sort === nextSort && direction === 'desc' ? 'asc' : 'desc'
    next.set('sort', nextSort)
    next.set('direction', nextDirection)
    next.set('page', '1')
    navigateWith(next)
  }

  function setPage(nextPage: number) {
    const next = new URLSearchParams(params)
    next.set('page', String(nextPage))
    next.set('page_size', String(pageSize))
    navigateWith(next)
  }

  function setPageSize(nextPageSize: number) {
    const next = new URLSearchParams(params)
    next.set('page', '1')
    next.set('page_size', String(nextPageSize))
    navigateWith(next)
  }

  function exportHref(format: 'csv' | 'json') {
    const next = new URLSearchParams(params)
    next.set('format', format)
    next.delete('page')
    next.delete('page_size')
    return `/admin/api/audit-events/export?${next.toString()}`
  }

  return (
    <div className="space-y-5">
      <section className="rounded-2xl border border-line bg-surface p-5 shadow-panel">
        <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
          <div>
            <div className="text-xs font-bold uppercase tracking-[0.18em] text-muted">Audit Trail</div>
            <h2 className="mt-2 text-2xl font-black tracking-tight text-body">Auditor event search</h2>
            <p className="mt-2 max-w-3xl text-sm leading-6 text-muted">
              Search security, configuration, document, MCP, ACP, and model activity using the persisted audit event stream.
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <a className="admin-button admin-button-secondary" href={exportHref('csv')}>Export CSV</a>
            <a className="admin-button admin-button-secondary" href={exportHref('json')}>Export JSON</a>
          </div>
        </div>

        <div className="mt-5 grid grid-cols-1 gap-3 md:grid-cols-4">
          <SummaryTile label="Matching events" value={String(total)} />
          <SummaryTile label="Actions" value={String(Object.keys(auditPayload.facets?.actions || {}).length)} />
          <SummaryTile label="Targets" value={String(Object.keys(auditPayload.facets?.target_types || {}).length)} />
          <SummaryTile label="Active filters" value={String(activeFilterCount)} />
        </div>

        <form className="mt-5 space-y-4" onSubmit={applyFilters}>
          <div className="grid grid-cols-1 gap-3 lg:grid-cols-4">
            <AuditInput label="Search" value={filters.q} onChange={(value) => setFilters({ ...filters, q: value })} placeholder="Action, actor, target, request, metadata…" className="lg:col-span-2" />
            <AuditInput label="From" value={filters.from} onChange={(value) => setFilters({ ...filters, from: value })} placeholder="2026-04-07T00:00:00Z" />
            <AuditInput label="To" value={filters.to} onChange={(value) => setFilters({ ...filters, to: value })} placeholder="2026-04-07T23:59:59Z" />
          </div>
          <div className="flex flex-wrap gap-2">
            <button type="button" className="admin-button admin-button-secondary" onClick={() => setRangePreset('today')}>Today</button>
            <button type="button" className="admin-button admin-button-secondary" onClick={() => setRangePreset('24h')}>Last 24h</button>
            <button type="button" className="admin-button admin-button-secondary" onClick={() => setRangePreset('7d')}>Last 7d</button>
          </div>
          <details className="rounded-xl border border-line bg-shell/70 p-4">
            <summary className="cursor-pointer text-sm font-bold text-body">Advanced filters</summary>
            <div className="mt-4 grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
              {([
                ['action', 'Action'],
                ['target_type', 'Target Type'],
                ['target_id', 'Target ID'],
                ['actor_id', 'Actor ID'],
                ['actor_kind', 'Actor Kind'],
                ['on_behalf_of_user_id', 'On Behalf Of'],
                ['correlation_id', 'Correlation ID'],
                ['request_id', 'Request ID'],
                ['delegation_grant_id', 'Delegation Grant'],
                ['organization_id', 'Organization'],
                ['location_id', 'Location'],
                ['operating_unit_id', 'Operating Unit'],
                ['from_state', 'From State'],
                ['to_state', 'To State'],
                ['metadata_key', 'Metadata Key'],
                ['metadata_value', 'Metadata Value'],
              ] as Array<[FilterKey, string]>).map(([key, label]) => (
                <AuditInput key={key} label={label} value={filters[key]} onChange={(value) => setFilters({ ...filters, [key]: value })} />
              ))}
            </div>
          </details>
          <div className="flex flex-wrap gap-2">
            <button type="submit" className="admin-button">Apply Filters</button>
            <button type="button" className="admin-button admin-button-secondary" onClick={clearFilters}>Clear</button>
          </div>
        </form>
      </section>

      <section className="overflow-hidden rounded-2xl border border-line bg-surface shadow-panel">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-line text-sm">
            <thead className="bg-accent-soft/70">
              <tr>
                <SortableHeader label="Occurred" sortKey="occurred_at" currentSort={sort} direction={direction} onSort={setSort} />
                <SortableHeader label="Action" sortKey="action" currentSort={sort} direction={direction} onSort={setSort} />
                <SortableHeader label="Actor" sortKey="actor_id" currentSort={sort} direction={direction} onSort={setSort} />
                <SortableHeader label="Target" sortKey="target_type" currentSort={sort} direction={direction} onSort={setSort} />
                <th className="px-4 py-3 text-left text-xs font-bold uppercase tracking-[0.14em] text-accent-dark">State</th>
                <SortableHeader label="Correlation" sortKey="correlation_id" currentSort={sort} direction={direction} onSort={setSort} />
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {items.length ? items.map((item) => (
                <tr key={item.id} className="align-top">
                  <td className="px-4 py-3 whitespace-nowrap text-body">{formatDate(item.occurred_at)}</td>
                  <td className="px-4 py-3 font-semibold text-body">{item.action}</td>
                  <td className="px-4 py-3 text-body">
                    <div>{item.actor_id || '-'}</div>
                    <div className="text-xs text-muted">{item.actor_kind || '-'}</div>
                  </td>
                  <td className="px-4 py-3 text-body">
                    <div className="font-medium">{item.target_type || '-'}</div>
                    <div className="max-w-[18rem] truncate text-xs text-muted">{item.target_id || '-'}</div>
                  </td>
                  <td className="px-4 py-3 text-body">{stateText(item)}</td>
                  <td className="max-w-[16rem] truncate px-4 py-3 text-body">{item.correlation_id || '-'}</td>
                  <td className="px-4 py-3 text-right">
                    <button type="button" className="admin-button admin-button-secondary" onClick={() => setSelected(item)}>Inspect</button>
                  </td>
                </tr>
              )) : (
                <tr>
                  <td colSpan={7} className="px-4 py-12 text-center text-sm text-muted">No audit events match the current filters.</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
        <div className="p-4">
          <PaginationBar page={page} pageSize={pageSize} total={total} onPageChange={setPage} onPageSizeChange={setPageSize} pageSizeOptions={[20, 50, 100, 200]} />
        </div>
      </section>

      {selected ? <AuditDetailDrawer item={selected} onClose={() => setSelected(null)} /> : null}
    </div>
  )
}

function SummaryTile({ label, value }: { label: string; value: string }) {
  return (
    <article className="rounded-xl border border-line bg-shell/80 p-4">
      <div className="text-xs font-bold uppercase tracking-[0.16em] text-muted">{label}</div>
      <div className="mt-2 text-2xl font-black text-body">{value}</div>
    </article>
  )
}

function AuditInput({ label, value, onChange, placeholder, type = 'text', className = '' }: { label: string; value: string; onChange: (value: string) => void; placeholder?: string; type?: string; className?: string }) {
  return (
    <label className={`space-y-2 text-sm text-body ${className}`.trim()}>
      <span className="block text-xs font-bold uppercase tracking-[0.14em] text-muted">{label}</span>
      <input className="admin-input" type={type} value={value} placeholder={placeholder} onChange={(event) => onChange(event.target.value)} />
    </label>
  )
}

function SortableHeader({ label, sortKey, currentSort, direction, onSort }: { label: string; sortKey: string; currentSort: string; direction: string; onSort: (sortKey: string) => void }) {
  const active = currentSort === sortKey
  return (
    <th className="px-4 py-3 text-left">
      <button type="button" className="text-xs font-bold uppercase tracking-[0.14em] text-accent-dark" onClick={() => onSort(sortKey)}>
        {label}{active ? ` ${direction === 'desc' ? '↓' : '↑'}` : ''}
      </button>
    </th>
  )
}

function AuditDetailDrawer({ item, onClose }: { item: AuditEvent; onClose: () => void }) {
  return (
    <div className="fixed inset-0 z-50 bg-black/30 p-4 backdrop-blur-sm" role="dialog" aria-modal="true">
      <div className="ml-auto flex h-full max-w-3xl flex-col overflow-hidden rounded-2xl border border-line bg-surface shadow-2xl">
        <div className="flex items-start justify-between gap-4 border-b border-line p-5">
          <div>
            <div className="text-xs font-bold uppercase tracking-[0.18em] text-muted">Audit Event</div>
            <h3 className="mt-2 text-xl font-black text-body">{item.action}</h3>
            <p className="mt-1 text-sm text-muted">{item.id}</p>
          </div>
          <button type="button" className="admin-button admin-button-secondary" onClick={onClose}>Close</button>
        </div>
        <div className="flex-1 space-y-4 overflow-auto p-5">
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            <Detail label="Occurred" value={formatDate(item.occurred_at)} />
            <Detail label="Actor" value={`${item.actor_id || '-'} (${item.actor_kind || '-'})`} />
            <Detail label="Target" value={`${item.target_type || '-'} / ${item.target_id || '-'}`} />
            <Detail label="State" value={stateText(item)} />
            <Detail label="Scope" value={[item.organization_id, item.location_id, item.operating_unit_id].filter(Boolean).join(' / ') || '-'} />
            <Detail label="Request" value={item.request_id || '-'} />
            <Detail label="Correlation" value={item.correlation_id || '-'} />
            <Detail label="Delegation" value={item.delegation_grant_id || item.on_behalf_of_user_id || '-'} />
          </div>
          {item.correlation_id ? (
            <a className="admin-button admin-button-secondary inline-flex" href={`/ops/traces/${encodeURIComponent(item.correlation_id)}`} target="_blank" rel="noreferrer">
              View Correlation Trace
            </a>
          ) : null}
          <div className="flex flex-wrap gap-2">
            <CopyButton label="Copy Event ID" value={item.id} />
            {item.correlation_id ? <CopyButton label="Copy Correlation" value={item.correlation_id} /> : null}
            {item.request_id ? <CopyButton label="Copy Request" value={item.request_id} /> : null}
          </div>
          <JSONBlock title="Change Summary" value={item.change_summary || {}} />
          <JSONBlock title="Metadata" value={item.metadata || {}} />
          <JSONBlock title="Raw Event" value={item} />
        </div>
      </div>
    </div>
  )
}

function CopyButton({ label, value }: { label: string; value: string }) {
  return (
    <button
      type="button"
      className="admin-button admin-button-secondary"
      onClick={() => void navigator.clipboard?.writeText(value)}
    >
      {label}
    </button>
  )
}

function Detail({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-line bg-shell/70 p-3">
      <div className="text-xs font-bold uppercase tracking-[0.14em] text-muted">{label}</div>
      <div className="mt-1 break-all text-sm text-body">{value}</div>
    </div>
  )
}

function JSONBlock({ title, value }: { title: string; value: unknown }) {
  return (
    <section className="rounded-xl border border-line bg-shell/70 p-4">
      <div className="mb-3 text-xs font-bold uppercase tracking-[0.16em] text-muted">{title}</div>
      <pre className="max-h-80 overflow-auto whitespace-pre-wrap text-xs leading-5 text-body">{JSON.stringify(value, null, 2)}</pre>
    </section>
  )
}

function filtersFromParams(params: URLSearchParams): FilterState {
  const filters = {} as FilterState
  for (const key of filterKeys) {
    filters[key] = params.get(key) || ''
  }
  return filters
}

function stateText(item: AuditEvent) {
  if (item.from_state || item.to_state) return `${item.from_state || '-'} → ${item.to_state || '-'}`
  return '-'
}
