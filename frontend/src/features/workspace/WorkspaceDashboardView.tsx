import { useEffect, useState } from 'react'
import { pickText, type ActionDefinition, type ViewDefinition } from '@/services/bootstrap'

type ToastVariant = 'default' | 'success' | 'warning' | 'error'

export function WorkspaceDashboardView({
  view,
  locale,
  onNavigate,
  routeActions,
  onToast,
  fetchJSON,
  resolvePath,
  asRecordList,
  displayValue,
  readCookie,
  buildError,
  routeForDocument,
  renderPanel,
  renderMetricCard,
  renderDataTable,
}: {
  view: ViewDefinition
  locale: string
  onNavigate: (target: string) => void
  routeActions: ActionDefinition[]
  onToast: (message: string, variant?: ToastVariant) => void
  fetchJSON: <T>(url: string) => Promise<T>
  resolvePath: (payload: Record<string, unknown> | null, path: string) => unknown
  asRecordList: (value: unknown) => Array<Record<string, unknown>>
  displayValue: (value: unknown) => string
  readCookie: (name: string) => string
  buildError: (response: Response) => Promise<Error>
  routeForDocument: (documentType: string, kind: 'detail' | 'form', actions: ActionDefinition[], currentPath?: string) => string
  renderPanel: (args: { title: string; children: JSX.Element }) => JSX.Element
  renderMetricCard: (args: { label: string; value: string }) => JSX.Element
  renderDataTable: (args: {
    columns: Array<{ key: string; label: string }>
    rows: Array<Record<string, unknown>>
    emptyText: string
    renderAction?: (row: Record<string, unknown>) => JSX.Element | null
  }) => JSX.Element
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
  }, [fetchJSON, searchKey, view.dataset_key, view.projection_key])

  const statementSearch = new URLSearchParams(window.location.search)
  const partyID = statementSearch.get('party_id') || ''
  const vendorID = statementSearch.get('vendor_id') || ''
  const receivablesItems = view.projection_key === 'commercial.receivables.summary' ? asRecordList(resolvePath(payload, 'items')) : []
  const payablesItems = view.projection_key === 'procurement.payables.summary' ? asRecordList(resolvePath(payload, 'items')) : []
  const inventoryBatches = view.projection_key === 'inventory.summary' ? asRecordList(resolvePath(payload, 'batches')) : []
  const planningRuns = view.projection_key === 'planning.runs.summary' ? asRecordList(resolvePath(payload, 'items')) : []
  const replenishmentItems = view.projection_key === 'planning.replenishment.summary' ? asRecordList(resolvePath(payload, 'items')) : []
  const planningProposals = view.projection_key === 'planning.proposals.summary' ? asRecordList(resolvePath(payload, 'items')) : []

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

  return renderPanel({
    title: pickText(view, 'title', locale) || 'Dashboard',
    children: (
      <>
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
            {renderDataTable({
              columns: [
                { key: 'number', label: 'Invoice' },
                { key: 'party_name', label: 'Customer' },
                { key: 'due_date', label: 'Due Date' },
                { key: 'paid_amount', label: 'Paid' },
                { key: 'credited', label: 'Credited' },
                { key: 'refunded', label: 'Refunded' },
                { key: 'balance_due', label: 'Open Balance' },
                { key: 'aging_bucket', label: 'Aging' },
              ],
              rows: receivablesItems,
              emptyText: 'No receivables.',
              renderAction: (row) => {
                const detailPath = routeForDocument('invoice', 'detail', routeActions, '/commercial/invoices')
                const documentID = String(row.id || '')
                if (!detailPath || !documentID) return null
                return (
                  <button onClick={() => onNavigate(`${detailPath}?id=${encodeURIComponent(documentID)}`)} className="rounded-lg border border-line px-3 py-1.5 text-sm text-body">
                    Open
                  </button>
                )
              },
            })}
          </section>
        ) : null}
        {payablesItems.length ? (
          <section className="mt-6 rounded-xl border border-line p-4">
            <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Payables Aging</h2>
            {renderDataTable({
              columns: [
                { key: 'number', label: 'Bill' },
                { key: 'vendor_name', label: 'Vendor' },
                { key: 'due_date', label: 'Due Date' },
                { key: 'paid_amount', label: 'Paid' },
                { key: 'credited', label: 'Credited' },
                { key: 'balance_due', label: 'Open Balance' },
                { key: 'aging_bucket', label: 'Aging' },
              ],
              rows: payablesItems,
              emptyText: 'No payables.',
              renderAction: (row) => {
                const detailPath = routeForDocument('vendor_bill', 'detail', routeActions, '/procurement/bills')
                const recordID = String(row.id || '')
                if (!detailPath || !recordID) return null
                return (
                  <button onClick={() => onNavigate(`${detailPath}?id=${encodeURIComponent(recordID)}`)} className="rounded-lg border border-line px-3 py-1.5 text-sm text-body">
                    Open
                  </button>
                )
              },
            })}
          </section>
        ) : null}
        {inventoryBatches.length ? (
          <section className="mt-6 rounded-xl border border-line p-4">
            <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Batch Stock</h2>
            {renderDataTable({
              columns: [
                { key: 'item_code', label: 'Item' },
                { key: 'warehouse_code', label: 'Warehouse' },
                { key: 'batch_code', label: 'Batch' },
                { key: 'expiration_date', label: 'Expiration' },
                { key: 'status', label: 'Status' },
                { key: 'on_hand_quantity', label: 'On Hand' },
                { key: 'available_quantity', label: 'Available' },
              ],
              rows: inventoryBatches,
              emptyText: 'No tracked batches.',
            })}
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
                              setSelectedReplenishmentKeys((existing) => event.target.checked ? [...existing, rowKey] : existing.filter((value) => value !== rowKey))
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
                <button onClick={() => onNavigate('/planning/runs')} className="rounded-lg border border-line px-3 py-1.5 text-sm text-body">
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
                              setSelectedProposalIDs((existing) => event.target.checked ? [...existing, proposalID] : existing.filter((value) => value !== proposalID))
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
        {view.projection_key === 'planning.runs.summary' ? (
          <section className="mt-6 rounded-xl border border-line p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <h2 className="text-sm font-semibold uppercase tracking-wide text-muted">Saved Planning Runs</h2>
              <button onClick={() => onNavigate('/planning/replenishment')} className="rounded-lg border border-line px-3 py-1.5 text-sm text-body">
                Open Replenishment
              </button>
            </div>
            {renderDataTable({
              columns: [
                { key: 'run_date', label: 'Run Date' },
                { key: 'warehouse_code', label: 'Warehouse' },
                { key: 'proposal_count', label: 'Proposals' },
                { key: 'projected_shortage_item_count', label: 'Shortages' },
                { key: 'total_forecast_demand_quantity', label: 'Forecast Demand' },
                { key: 'due_soon_count', label: 'Due Soon' },
                { key: 'status', label: 'Status' },
              ],
              rows: planningRuns,
              emptyText: 'No planning runs.',
              renderAction: (row) => {
                const runID = String(row.id || '')
                if (!runID) return null
                return (
                  <button onClick={() => onNavigate(`/planning/proposals?run_id=${encodeURIComponent(runID)}`)} className="rounded-lg border border-line px-3 py-1.5 text-sm text-body">
                    Open
                  </button>
                )
              },
            })}
          </section>
        ) : null}
        {view.projection_key === 'commercial.party_statement' ? (
          partyID ? (
            <section className="mt-6">
              <PartyCommercialSummaryPanel
                summary={payload || {}}
                routeActions={routeActions}
                onNavigate={onNavigate}
                resolvePath={resolvePath}
                asRecordList={asRecordList}
                displayValue={displayValue}
                routeForDocument={routeForDocument}
                renderMetricCard={renderMetricCard}
                renderDataTable={renderDataTable}
              />
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
              <VendorProcurementSummaryPanel
                summary={payload || {}}
                routeActions={routeActions}
                onNavigate={onNavigate}
                resolvePath={resolvePath}
                asRecordList={asRecordList}
                displayValue={displayValue}
                routeForDocument={routeForDocument}
                renderMetricCard={renderMetricCard}
                renderDataTable={renderDataTable}
              />
            </section>
          ) : (
            <section className="mt-6 rounded-xl border border-line p-4 text-sm text-muted">
              Select a vendor from vendor detail to open a statement.
            </section>
          )
        ) : null}
      </>
    ),
  })
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

export function PartyCommercialSummaryPanel({
  summary,
  routeActions,
  onNavigate,
  resolvePath,
  asRecordList,
  displayValue,
  routeForDocument,
  renderMetricCard,
  renderDataTable,
}: {
  summary: Record<string, unknown>
  routeActions: ActionDefinition[]
  onNavigate: (target: string) => void
  resolvePath: (payload: Record<string, unknown>, path: string) => unknown
  asRecordList: (value: unknown) => Array<Record<string, unknown>>
  displayValue: (value: unknown) => string
  routeForDocument: (documentType: string, kind: 'detail' | 'form', actions: ActionDefinition[], currentPath?: string) => string
  renderMetricCard: (args: { label: string; value: string }) => JSX.Element
  renderDataTable: (args: {
    columns: Array<{ key: string; label: string }>
    rows: Array<Record<string, unknown>>
    emptyText: string
    renderAction?: (row: Record<string, unknown>) => JSX.Element | null
  }) => JSX.Element
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
        {renderMetricCard({ label: 'Open Invoices', value: displayValue(resolvePath(summary, 'open_invoice_count')) })}
        {renderMetricCard({ label: 'Open Balance', value: displayValue(resolvePath(summary, 'open_balance_total')) })}
        {renderMetricCard({ label: 'Collected', value: displayValue(resolvePath(summary, 'paid_amount_total')) })}
        {renderMetricCard({ label: 'Refunded', value: displayValue(resolvePath(summary, 'refunded_amount_total')) })}
      </div>
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <section>
          <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Open Invoices</h3>
          {renderDataTable({
            columns: [
              { key: 'number', label: 'Invoice' },
              { key: 'due_date', label: 'Due Date' },
              { key: 'paid_amount', label: 'Paid' },
              { key: 'credited', label: 'Credited' },
              { key: 'refunded', label: 'Refunded' },
              { key: 'balance_due', label: 'Open Balance' },
            ],
            rows: openInvoices,
            emptyText: 'No open invoices.',
            renderAction: (row) => {
              const documentID = String(row.id || '')
              if (!invoiceDetailPath || !documentID) return null
              return (
                <button onClick={() => onNavigate(`${invoiceDetailPath}?id=${encodeURIComponent(documentID)}`)} className="rounded-lg border border-line px-3 py-1.5 text-sm text-body">
                  Open
                </button>
              )
            },
          })}
        </section>
        <section>
          <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Recent Activity</h3>
          {renderDataTable({
            columns: [
              { key: 'date', label: 'Date' },
              { key: 'type', label: 'Type' },
              { key: 'number', label: 'Number' },
              { key: 'counter', label: 'Counterparty Doc' },
              { key: 'amount', label: 'Amount' },
              { key: 'status', label: 'Status' },
            ],
            rows: activities,
            emptyText: 'No commercial activity yet.',
          })}
        </section>
      </div>
    </section>
  )
}

export function VendorProcurementSummaryPanel({
  summary,
  routeActions,
  onNavigate,
  resolvePath,
  asRecordList,
  displayValue,
  routeForDocument,
  renderMetricCard,
  renderDataTable,
}: {
  summary: Record<string, unknown>
  routeActions: ActionDefinition[]
  onNavigate: (target: string) => void
  resolvePath: (payload: Record<string, unknown>, path: string) => unknown
  asRecordList: (value: unknown) => Array<Record<string, unknown>>
  displayValue: (value: unknown) => string
  routeForDocument: (documentType: string, kind: 'detail' | 'form', actions: ActionDefinition[], currentPath?: string) => string
  renderMetricCard: (args: { label: string; value: string }) => JSX.Element
  renderDataTable: (args: {
    columns: Array<{ key: string; label: string }>
    rows: Array<Record<string, unknown>>
    emptyText: string
    renderAction?: (row: Record<string, unknown>) => JSX.Element | null
  }) => JSX.Element
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
        {renderMetricCard({ label: 'Open Bills', value: displayValue(resolvePath(summary, 'open_bill_count')) })}
        {renderMetricCard({ label: 'Open Balance', value: displayValue(resolvePath(summary, 'open_balance_total')) })}
        {renderMetricCard({ label: 'Paid', value: displayValue(resolvePath(summary, 'paid_amount_total')) })}
        {renderMetricCard({ label: 'Credited', value: displayValue(resolvePath(summary, 'credited_amount_total')) })}
      </div>
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <section>
          <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Open Bills</h3>
          {renderDataTable({
            columns: [
              { key: 'number', label: 'Bill' },
              { key: 'due_date', label: 'Due Date' },
              { key: 'paid_amount', label: 'Paid' },
              { key: 'credited', label: 'Credited' },
              { key: 'balance_due', label: 'Open Balance' },
            ],
            rows: openBills,
            emptyText: 'No open bills.',
            renderAction: (row) => {
              const recordID = String(row.id || '')
              if (!billDetailPath || !recordID) return null
              return (
                <button onClick={() => onNavigate(`${billDetailPath}?id=${encodeURIComponent(recordID)}`)} className="rounded-lg border border-line px-3 py-1.5 text-sm text-body">
                  Open
                </button>
              )
            },
          })}
        </section>
        <section>
          <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Recent Activity</h3>
          {renderDataTable({
            columns: [
              { key: 'date', label: 'Date' },
              { key: 'type', label: 'Type' },
              { key: 'number', label: 'Number' },
              { key: 'counter', label: 'Counterparty Doc' },
              { key: 'amount', label: 'Amount' },
              { key: 'status', label: 'Status' },
            ],
            rows: activities,
            emptyText: 'No procurement activity yet.',
          })}
        </section>
      </div>
    </section>
  )
}
