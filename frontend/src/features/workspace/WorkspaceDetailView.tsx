import { useEffect, useMemo, useState } from 'react'
import { Modal } from '@/components/ui/Modal'
import { pickText, type ActionDefinition, type FieldDefinition, type SectionDefinition, type ViewDefinition } from '@/services/bootstrap'
import { PartyCommercialSummaryPanel, VendorProcurementSummaryPanel } from './WorkspaceDashboardView'
import { handleWorkspaceDetailAction } from './workspaceDetailActions'

type ToastVariant = 'default' | 'success' | 'warning' | 'error'

export function WorkspaceDetailView({
  view,
  locale,
  routeActions,
  currentPath,
  onNavigate,
  onToast,
  fetchJSON,
  resolvePath,
  readCookie,
  buildError,
  routeForDocument,
  routeForModel,
  routeForEdit,
  stripEditorSuffix,
  resolveSections,
  actionVisibleForStatus,
  isCommercialDocumentLocked,
  isProcurementDocumentLocked,
  isFulfillmentDocumentLocked,
  isReturnsDocumentLocked,
  isSupplierReturnsDocumentLocked,
  isProductionDocumentLocked,
  invokeCommercialAction,
  invokeDocumentAction,
  renderDetailFieldValue,
  humanize,
  parseDimensionCodes,
  toNumber,
  displayValue,
  asRecordList,
  renderPanel,
  renderMetricCard,
  renderDataTable,
}: {
  view: ViewDefinition
  locale: string
  routeActions: ActionDefinition[]
  currentPath: string
  onNavigate: (target: string) => void
  onToast: (message: string, variant?: ToastVariant) => void
  fetchJSON: <T>(url: string) => Promise<T>
  resolvePath: (payload: unknown, path: string) => unknown
  readCookie: (name: string) => string
  buildError: (response: Response) => Promise<Error>
  routeForDocument: (documentType: string, kind: 'detail' | 'form', actions: ActionDefinition[], currentPath?: string) => string
  routeForModel: (modelKey: string, kind: 'detail' | 'form', actions: ActionDefinition[], currentPath?: string) => string
  routeForEdit: (currentPath: string, documentType: string, actions: ActionDefinition[]) => string
  stripEditorSuffix: (path: string) => string
  resolveSections: (view: Pick<ViewDefinition, 'sections' | 'tabs' | 'fields'>) => SectionDefinition[]
  actionVisibleForStatus: (actionKey: string, status: string, documentType: string) => boolean
  isCommercialDocumentLocked: (documentType: string, status: string) => boolean
  isProcurementDocumentLocked: (documentType: string, status: string) => boolean
  isFulfillmentDocumentLocked: (documentType: string, status: string) => boolean
  isReturnsDocumentLocked: (documentType: string, status: string) => boolean
  isSupplierReturnsDocumentLocked: (documentType: string, status: string) => boolean
  isProductionDocumentLocked: (documentType: string, status: string) => boolean
  invokeCommercialAction: (url: string) => Promise<Record<string, unknown>>
  invokeDocumentAction: (documentID: string, action: string) => Promise<void>
  renderDetailFieldValue: (field: FieldDefinition, value: unknown) => React.ReactNode
  humanize: (value: string) => string
  parseDimensionCodes: (value: unknown) => string[]
  toNumber: (value: unknown) => number
  displayValue: (value: unknown) => string
  asRecordList: (value: unknown) => Array<Record<string, unknown>>
  renderPanel: (args: { title: string; status?: string; children: JSX.Element }) => JSX.Element
  renderMetricCard: (args: { label: string; value: string }) => JSX.Element
  renderDataTable: (args: {
    columns: Array<{ key: string; label: string }>
    rows: Array<Record<string, unknown>>
    emptyText: string
    renderAction?: (row: Record<string, unknown>) => JSX.Element | null
  }) => JSX.Element
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
  }, [documentID, fetchJSON, reloadKey, view.model_key])

  if (!documentID) {
    return renderPanel({
      title: pickText(view, 'title', locale) || 'Detail',
      status: 'Select a record from a list to inspect it.',
      children: <></>,
    })
  }

  const record = (payload?.record || payload) as Record<string, unknown> | null
  const header = (record?.header || {}) as Record<string, unknown>
  const sections = resolveSections(view)
  const editTarget = view.model_key ? routeForModel(view.model_key, 'form', routeActions, currentPath) : routeForEdit(currentPath, view.document_type || String(header.type || ''), routeActions)
  const cancelTarget = stripEditorSuffix(currentPath) || '/documents'
  const visibleActions = (view.allowed_actions || []).filter((actionKey) =>
    actionVisibleForStatus(actionKey, String(header.status || ''), String(header.type || view.document_type || '')),
  )
  const hasDocumentCancelAction = visibleActions.some((actionKey) => actionKey.toLowerCase() === 'cancel')
  const canEdit =
    !!editTarget &&
    !isCommercialDocumentLocked(String(header.type || ''), String(header.status || '')) &&
    !isProcurementDocumentLocked(String(header.type || ''), String(header.status || '')) &&
    !isFulfillmentDocumentLocked(String(header.type || ''), String(header.status || '')) &&
    !isReturnsDocumentLocked(String(header.type || ''), String(header.status || '')) &&
    !isSupplierReturnsDocumentLocked(String(header.type || ''), String(header.status || '')) &&
    !isProductionDocumentLocked(String(header.type || ''), String(header.status || ''))

  async function handleAction(actionKey: string) {
    await handleWorkspaceDetailAction({
      header,
      actionKey,
      routeActions,
      onNavigate,
      onToast,
      onReload: () => setReloadKey((current) => current + 1),
      onRequireStepUp: (nextActionKey) => {
        setPendingAction(nextActionKey)
        setStepUpCode('')
        setStepUpError('')
        setStepUpOpen(true)
      },
      resolvePath,
      routeForDocument,
      invokeCommercialAction,
      invokeDocumentAction,
    })
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

  return renderPanel({
    title: pickText(view, 'title', locale) || 'Detail',
    status: String(header.status || ''),
    children: (
      <>
        <div className="mb-4 flex flex-wrap gap-3">
          {!hasDocumentCancelAction ? (
            <button onClick={() => onNavigate(cancelTarget)} className="rounded-lg border border-line px-4 py-2 text-sm text-body">
              Cancel
            </button>
          ) : null}
          {visibleActions.map((actionKey) => (
            <button key={actionKey} onClick={() => void handleAction(actionKey)} className="rounded-lg border border-line px-4 py-2 text-sm text-body">
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
            <PartyCommercialSummaryPanel
              summary={commercialSummary}
              routeActions={routeActions}
              onNavigate={onNavigate}
              resolvePath={resolvePath}
              asRecordList={asRecordList}
              displayValue={displayValue}
              routeForDocument={routeForDocument}
              renderMetricCard={renderMetricCard}
              renderDataTable={renderDataTable}
            />
          ) : null}
          {view.model_key === 'vendor_profile' && procurementSummary ? (
            <VendorProcurementSummaryPanel
              summary={procurementSummary}
              routeActions={routeActions}
              onNavigate={onNavigate}
              resolvePath={resolvePath}
              asRecordList={asRecordList}
              displayValue={displayValue}
              routeForDocument={routeForDocument}
              renderMetricCard={renderMetricCard}
              renderDataTable={renderDataTable}
            />
          ) : null}
          {view.model_key === 'commercial_product' ? (
            <ProductVariantsPanel
              product={record || {}}
              routeActions={routeActions}
              onNavigate={onNavigate}
              onToast={onToast}
              fetchJSON={fetchJSON}
              resolvePath={resolvePath}
              readCookie={readCookie}
              buildError={buildError}
              routeForModel={routeForModel}
              parseDimensionCodes={parseDimensionCodes}
              toNumber={toNumber}
              displayValue={displayValue}
            />
          ) : null}
          {view.model_key === 'inventory_batch' && record ? (
            <>
              <InventoryBatchControlPanel
                batch={record}
                onToast={onToast}
                onChanged={() => setReloadKey((current) => current + 1)}
                resolvePath={resolvePath}
                readCookie={readCookie}
                buildError={buildError}
                humanize={humanize}
              />
              <InventoryBatchTracePanel
                batch={record}
                onToast={onToast}
                fetchJSON={fetchJSON}
                resolvePath={resolvePath}
                asRecordList={asRecordList}
                displayValue={displayValue}
                renderMetricCard={renderMetricCard}
                renderDataTable={renderDataTable}
              />
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
                      if (pendingAction) await handleAction(pendingAction)
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
      </>
    ),
  })
}

function InventoryBatchControlPanel({
  batch,
  onToast,
  onChanged,
  resolvePath,
  readCookie,
  buildError,
  humanize,
}: {
  batch: Record<string, unknown>
  onToast: (message: string, variant?: ToastVariant) => void
  onChanged: () => void
  resolvePath: (payload: unknown, path: string) => unknown
  readCookie: (name: string) => string
  buildError: (response: Response) => Promise<Error>
  humanize: (value: string) => string
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
        body: JSON.stringify({ action, reason: action }),
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
          <button key={action} type="button" disabled={submitting !== '' && submitting !== action} onClick={() => void handleAction(action)} className="rounded-lg border border-line px-3 py-2 text-sm text-body disabled:opacity-50">
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
  fetchJSON,
  resolvePath,
  asRecordList,
  displayValue,
  renderMetricCard,
  renderDataTable,
}: {
  batch: Record<string, unknown>
  onToast: (message: string, variant?: ToastVariant) => void
  fetchJSON: <T>(url: string) => Promise<T>
  resolvePath: (payload: unknown, path: string) => unknown
  asRecordList: (value: unknown) => Array<Record<string, unknown>>
  displayValue: (value: unknown) => string
  renderMetricCard: (args: { label: string; value: string }) => JSX.Element
  renderDataTable: (args: {
    columns: Array<{ key: string; label: string }>
    rows: Array<Record<string, unknown>>
    emptyText: string
    renderAction?: (row: Record<string, unknown>) => JSX.Element | null
  }) => JSX.Element
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
  }, [batchID, fetchJSON, onToast])

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
            {renderMetricCard({ label: 'On Hand', value: displayValue(resolvePath(summary, 'on_hand_quantity')) })}
            {renderMetricCard({ label: 'Available', value: displayValue(resolvePath(summary, 'available_quantity')) })}
            {renderMetricCard({ label: 'Movements', value: displayValue(resolvePath(summary, 'movement_count')) })}
            {renderMetricCard({ label: 'Linked Docs', value: displayValue(resolvePath(summary, 'document_node_count')) })}
          </div>
          <section>
            <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Document Chain</h3>
            {renderDataTable({
              columns: [
                { key: 'date', label: 'Date' },
                { key: 'type', label: 'Type' },
                { key: 'number', label: 'Number' },
                { key: 'movement_reason', label: 'Reason' },
                { key: 'quantity_delta', label: 'Delta' },
                { key: 'status', label: 'Status' },
              ],
              rows: nodes,
              emptyText: 'No trace nodes.',
            })}
          </section>
          <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
            <section>
              <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Produced Into</h3>
              {renderDataTable({
                columns: [
                  { key: 'production_order_number', label: 'Production Order' },
                  { key: 'production_output_number', label: 'Output' },
                  { key: 'item_code', label: 'Item' },
                  { key: 'batch_code', label: 'Batch' },
                  { key: 'output_quantity', label: 'Quantity' },
                ],
                rows: producedInto,
                emptyText: 'This batch has not produced downstream lots.',
              })}
            </section>
            <section>
              <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Consumed From</h3>
              {renderDataTable({
                columns: [
                  { key: 'production_order_number', label: 'Production Order' },
                  { key: 'production_issue_number', label: 'Issue' },
                  { key: 'item_code', label: 'Item' },
                  { key: 'batch_code', label: 'Batch' },
                  { key: 'quantity', label: 'Quantity' },
                ],
                rows: consumedFrom,
                emptyText: 'No upstream production consumption links.',
              })}
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
  fetchJSON,
  resolvePath,
  readCookie,
  buildError,
  routeForModel,
  parseDimensionCodes,
  toNumber,
  displayValue,
}: {
  product: Record<string, unknown>
  routeActions: ActionDefinition[]
  onNavigate: (target: string) => void
  onToast: (message: string, variant?: ToastVariant) => void
  fetchJSON: <T>(url: string) => Promise<T>
  resolvePath: (payload: unknown, path: string) => unknown
  readCookie: (name: string) => string
  buildError: (response: Response) => Promise<Error>
  routeForModel: (modelKey: string, kind: 'detail' | 'form', actions: ActionDefinition[], currentPath?: string) => string
  parseDimensionCodes: (value: unknown) => string[]
  toNumber: (value: unknown) => number
  displayValue: (value: unknown) => string
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
  }, [dimensionCodes, fetchJSON, productCode, resolvePath])

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
  }, [dimensionCodes, resolvePath, toNumber, variantValues])

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
                  <div className="mb-2 text-sm font-medium text-body">{String(resolvePath(dimension, 'values.name') || dimensionCode)}</div>
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
