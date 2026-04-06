import { useEffect, useMemo, useState } from 'react'
import { pickText, type ActionDefinition, type ViewDefinition } from '@/services/bootstrap'
import { PaginationBar } from '@/components/ui/PaginationBar'
import { normalizeWorkspaceRoute } from './workspaceRouteHelpers'
import {
  clearWorkspaceListCache,
  clearWorkspaceListInFlight,
  readWorkspaceListCache,
  readWorkspaceListInFlight,
  writeWorkspaceListCache,
  writeWorkspaceListInFlight,
} from './workspaceCache'

type ListPayload = { items: Array<Record<string, unknown>>; total?: number }

export function WorkspaceListView({
  view,
  locale,
  routeActions,
  currentPath,
  onNavigate,
  fetchJSON,
  documentListNeedsPayload,
  routeForModel,
  routeForCreate,
  humanize,
  resolvePath,
  routeForDocument,
  renderPanel,
  renderDataTable,
}: {
  view: ViewDefinition
  locale: string
  routeActions: ActionDefinition[]
  currentPath: string
  onNavigate: (target: string) => void
  fetchJSON: <T>(url: string) => Promise<T>
  documentListNeedsPayload: (view: ViewDefinition) => boolean
  routeForModel: (modelKey: string, mode: 'form' | 'detail', routeActions: ActionDefinition[], currentPath: string) => string
  routeForCreate: (currentPath: string, routeActions: ActionDefinition[]) => string
  humanize: (value: string) => string
  resolvePath: (payload: Record<string, unknown>, path: string) => unknown
  routeForDocument: (documentType: string, mode: 'detail', routeActions: ActionDefinition[], currentPath: string) => string
  renderPanel: (args: { title: string; status: string; children: JSX.Element }) => JSX.Element
  renderDataTable: (args: {
    columns: Array<{ key: string; label: string }>
    rows: Array<Record<string, unknown>>
    emptyText: string
    renderAction: (row: Record<string, unknown>) => JSX.Element | null
  }) => JSX.Element
}) {
  const searchParams = useMemo(() => new URLSearchParams(window.location.search), [window.location.search])
  const createTarget = view.model_key ? routeForModel(view.model_key, 'form', routeActions, currentPath) : routeForCreate(currentPath, routeActions)
  const activeSearch = searchParams.get('name') || ''
  const activeSort = searchParams.get('sort') || ''
  const activePageSize = searchParams.get('page_size') || ''
  const currentPage = Number.parseInt(searchParams.get('page') || '1', 10) || 1
  const defaultPageSize = view.default_page_size && view.default_page_size > 0 ? view.default_page_size : 20
  const resolvedPageSize = Number.parseInt(activePageSize || String(defaultPageSize), 10) || defaultPageSize
  const requestURL = useMemo(() => {
    const query = new URLSearchParams()
    if (view.document_type) query.set('type', view.document_type)
    if (view.model_key) query.set('model', view.model_key)
    if (!view.model_key && documentListNeedsPayload(view)) {
      query.set('include_payload', '1')
    }
    query.set('page', String(currentPage))
    query.set('page_size', String(resolvedPageSize))
    const queryKeys = new Set(['name', 'sort', 'page', 'page_size'])
    for (const filter of view.filters || []) {
      queryKeys.add(filter.key)
    }
    for (const key of queryKeys) {
      const value = searchParams.get(key)
      if (value) query.set(key, value)
    }
    const base = view.model_key ? '/ui/data/models' : '/ui/data/documents'
    return `${base}?${query}`
  }, [
    currentPage,
    documentListNeedsPayload,
    resolvedPageSize,
    searchParams,
    view.document_type,
    view.filters,
    view.model_key,
    view,
  ])
  const cachedPayload = useMemo(() => readWorkspaceListCache(requestURL), [requestURL])
  const [payload, setPayload] = useState<ListPayload | null>(cachedPayload)
  const [refreshing, setRefreshing] = useState(false)
  const [refreshNonce, setRefreshNonce] = useState(0)

  useEffect(() => {
    let mounted = true
    async function load() {
      const cached = readWorkspaceListCache(requestURL)
      if (cached) {
        if (!mounted) return
        setPayload(cached)
      }
      setRefreshing(true)
      const request =
        readWorkspaceListInFlight(requestURL) ||
        fetchJSON<ListPayload>(requestURL).then((result) => {
          writeWorkspaceListCache(requestURL, result)
          clearWorkspaceListInFlight(requestURL)
          return result
        }).catch((error) => {
          clearWorkspaceListInFlight(requestURL)
          throw error
        })
      writeWorkspaceListInFlight(requestURL, request)
      try {
        const result = await request
        if (!mounted) return
        setPayload(result)
      } finally {
        if (mounted) setRefreshing(false)
      }
    }
    void load()
    return () => {
      mounted = false
    }
  }, [fetchJSON, refreshNonce, requestURL])

  useEffect(() => {
    setPayload(cachedPayload)
  }, [cachedPayload, requestURL])

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
  const totalPages = Math.max(1, Math.ceil(totalItems / resolvedPageSize))
  const effectivePage = Math.min(currentPage, totalPages)
  const currentRows = payload?.items?.length ?? 0
  const currentStart = totalItems === 0 || currentRows === 0 ? 0 : (effectivePage - 1) * resolvedPageSize + 1
  const currentEnd = totalItems === 0 || currentRows === 0 ? 0 : Math.min(totalItems, currentStart + currentRows - 1)
  const listStatus = totalItems > 0
    ? `Showing ${currentStart}-${currentEnd} of ${totalItems} item${totalItems === 1 ? '' : 's'}.`
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

  return renderPanel({
    title: pickText(view, 'title', locale) || 'List',
    status: listStatus,
    children: (
      <>
        <div className="mb-4 space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3 text-sm text-muted">
              <span>Items {totalItems}</span>
              {refreshing ? <span className="text-accent">Refreshing…</span> : null}
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => {
                  clearWorkspaceListCache(requestURL)
                  setRefreshing(true)
                  setRefreshNonce((current) => current + 1)
                }}
                className="rounded-lg border border-line px-3 py-2 text-sm text-body"
              >
                Refresh
              </button>
              {createTarget ? (
                <button onClick={() => onNavigate(createTarget)} className="rounded-lg bg-accent px-4 py-2 text-sm text-white">
                  New
                </button>
              ) : null}
            </div>
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
          </div>
        </div>
        {renderDataTable({
          columns: columns.map((column) => ({ key: column.path, label: pickText(column, 'label', locale) || column.key })),
          rows: payload?.items || [],
          emptyText: pickText(view, 'empty_state', locale) || 'No records.',
          renderAction: (row) => {
            const id = String(view.model_key ? row.id || '' : resolvePath(row, 'header.id') || '')
            const directPath = String(row.open_path || '')
            if (directPath) {
              return (
                <button
                  onClick={() => onNavigate(normalizeWorkspaceRoute(directPath))}
                  className="rounded-lg border border-line px-3 py-1.5 text-sm text-body"
                >
                  Open
                </button>
              )
            }
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
          },
        })}
        <PaginationBar
          page={currentPage}
          pageSize={resolvedPageSize}
          total={totalItems}
          onPageChange={(page) => {
            const next = new URLSearchParams(searchParams)
            next.set('page', String(page))
            if (!next.get('page_size')) {
              next.set('page_size', String(defaultPageSize))
            }
            onNavigate(`${currentPath}?${next.toString()}`)
          }}
          onPageSizeChange={(pageSize) => applyListQuery({ page_size: String(pageSize) })}
        />
      </>
    ),
  })
}
