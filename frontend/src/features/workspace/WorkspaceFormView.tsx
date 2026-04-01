import { useEffect, useState } from 'react'
import { pickText, type ActionDefinition, type FieldDefinition, type ViewDefinition } from '@/services/bootstrap'
import type { CommercialFormCatalog, FormState, ValidationErrors } from './workspaceFormTypes'
import { emptyCommercialFormCatalog } from './workspaceFormTypes'

type ToastVariant = 'default' | 'success' | 'warning' | 'error'

export function WorkspaceFormView({
  view,
  locale,
  currentPath,
  searchParams,
  actions,
  onNavigate,
  onToast,
  fetchJSON,
  resolvePath,
  readCookie,
  buildError,
  routeForModel,
  routeForDocument,
  stripEditorSuffix,
  resolveSections,
  validateFieldCollection,
  validateFieldInput,
  normalizeFieldPath,
  hasUnresolvedVariantSelections,
  normalizeCommercialFormState,
  isCommercialDocumentLocked,
  isProcurementDocumentLocked,
  isFulfillmentDocumentLocked,
  isReturnsDocumentLocked,
  isSupplierReturnsDocumentLocked,
  isProductionDocumentLocked,
  isRecallDocumentLocked,
  renderPanel,
  renderFieldEditor,
}: {
  view: ViewDefinition
  locale: string
  currentPath: string
  searchParams: URLSearchParams
  actions: ActionDefinition[]
  onNavigate: (target: string) => void
  onToast: (message: string, variant?: ToastVariant) => void
  fetchJSON: <T>(url: string) => Promise<T>
  resolvePath: (payload: unknown, path: string) => unknown
  readCookie: (name: string) => string
  buildError: (response: Response) => Promise<Error>
  routeForModel: (modelKey: string, kind: 'detail' | 'form', actions: ActionDefinition[], currentPath?: string) => string
  routeForDocument: (documentType: string, kind: 'detail' | 'form', actions: ActionDefinition[], currentPath?: string) => string
  stripEditorSuffix: (path: string) => string
  resolveSections: (view: Pick<ViewDefinition, 'sections' | 'tabs' | 'fields'>) => Array<{ key: string; title?: string; fields?: FieldDefinition[] }>
  validateFieldCollection: (fields: FieldDefinition[], values: FormState, model: boolean, locale: string, scope?: string) => ValidationErrors
  validateFieldInput: (field: FieldDefinition, value: unknown, locale: string) => string
  normalizeFieldPath: (field: FieldDefinition, model: boolean) => string
  hasUnresolvedVariantSelections: (values: FormState) => boolean
  normalizeCommercialFormState: (current: FormState, documentType: string, catalog: CommercialFormCatalog) => FormState
  isCommercialDocumentLocked: (documentType: string, status: string) => boolean
  isProcurementDocumentLocked: (documentType: string, status: string) => boolean
  isFulfillmentDocumentLocked: (documentType: string, status: string) => boolean
  isReturnsDocumentLocked: (documentType: string, status: string) => boolean
  isSupplierReturnsDocumentLocked: (documentType: string, status: string) => boolean
  isProductionDocumentLocked: (documentType: string, status: string) => boolean
  isRecallDocumentLocked: (documentType: string, status: string) => boolean
  renderPanel: (args: { title: string; status?: string; children: JSX.Element }) => JSX.Element
  renderFieldEditor: (args: {
    field: FieldDefinition
    locale: string
    values: FormState
    onChange: React.Dispatch<React.SetStateAction<FormState>>
    model: boolean
    catalog: CommercialFormCatalog
    error?: string
    onBlur?: () => void
  }) => JSX.Element
}) {
  const targetID = searchParams.get('id') || ''
  const [values, setValues] = useState<FormState>({})
  const [version, setVersion] = useState(0)
  const [etag, setETag] = useState('')
  const [recordStatus, setRecordStatus] = useState('')
  const [errors, setErrors] = useState<ValidationErrors>({})
  const [catalog, setCatalog] = useState<CommercialFormCatalog>(emptyCommercialFormCatalog)

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
        return
      }
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
    void load()
    return () => {
      mounted = false
    }
  }, [fetchJSON, targetID, view.model_key])

  useEffect(() => {
    let mounted = true
    async function loadCatalog() {
      const documentType = String(view.document_type || '')
      const modelKey = String(view.model_key || '')
      const needsCatalog =
        ['sales_order', 'invoice', 'credit_note', 'payment_receipt', 'payment_refund', 'purchase_request', 'purchase_order', 'goods_receipt', 'vendor_bill', 'payment_out', 'vendor_credit_note', 'sales_fulfillment', 'delivery_order', 'sales_return', 'return_receipt', 'supplier_return', 'stock_receipt', 'stock_issue', 'stock_adjustment', 'stock_transfer', 'production_order', 'production_issue', 'production_output', 'recall_case', 'recall_action'].includes(documentType) ||
        ['party', 'vendor_profile', 'commercial_product', 'commercial_item', 'commercial_variant_dimension', 'commercial_variant_value', 'commercial_price_list_item', 'inventory_batch', 'warehouse', 'production_bom', 'production_bom_version', 'production_work_center'].includes(modelKey)
      if (!needsCatalog) {
        if (mounted) setCatalog(emptyCommercialFormCatalog())
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
        const invoiceDetails = await Promise.all(openInvoiceIDs.map(async (id) => {
          try {
            return await fetchJSON<Record<string, unknown>>(`/ui/data/documents/${encodeURIComponent(id)}`)
          } catch {
            return null
          }
        }))
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
        const billDetails = await Promise.all(openBillIDs.map(async (id) => {
          try {
            return await fetchJSON<Record<string, unknown>>(`/ui/data/documents/${encodeURIComponent(id)}`)
          } catch {
            return null
          }
        }))
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
        const paymentDetails = await Promise.all(paymentIDs.map(async (id) => {
          try {
            return await fetchJSON<Record<string, unknown>>(`/ui/data/documents/${encodeURIComponent(id)}`)
          } catch {
            return null
          }
        }))
        const paymentsByID = Object.fromEntries(
          paymentDetails
            .map((detail) => (detail?.record || detail) as Record<string, unknown> | null)
            .filter((detail): detail is Record<string, unknown> => !!detail)
            .map((detail) => [String(resolvePath(detail, 'header.id') || ''), detail]),
        )
        if (!mounted) return
        const nextCatalog: CommercialFormCatalog = {
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
        }
        setCatalog(nextCatalog)
        setValues((current) => normalizeCommercialFormState(current, documentType, nextCatalog))
      } catch {
        if (mounted) setCatalog(emptyCommercialFormCatalog())
      }
    }
    void loadCatalog()
    return () => {
      mounted = false
    }
  }, [fetchJSON, normalizeCommercialFormState, resolvePath, view.document_type, view.model_key])

  const sections = resolveSections(view)
  const validationFields = sections.flatMap((section) => section.fields || [])
  const cancelTarget = targetID
    ? (view.model_key
        ? routeForModel(view.model_key, 'detail', actions, currentPath)
        : routeForDocument(view.document_type || '', 'detail', actions, currentPath))
    : stripEditorSuffix(currentPath) || '/'
  const formLocked = !view.model_key && targetID && (
    isCommercialDocumentLocked(String(view.document_type || ''), recordStatus) ||
    isProcurementDocumentLocked(String(view.document_type || ''), recordStatus) ||
    isFulfillmentDocumentLocked(String(view.document_type || ''), recordStatus) ||
    isReturnsDocumentLocked(String(view.document_type || ''), recordStatus) ||
    isSupplierReturnsDocumentLocked(String(view.document_type || ''), recordStatus) ||
    isProductionDocumentLocked(String(view.document_type || ''), recordStatus) ||
    isRecallDocumentLocked(String(view.document_type || ''), recordStatus)
  )

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
        const target = routeForModel(view.model_key, 'detail', actions, currentPath)
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
    const target = routeForDocument(view.document_type || '', 'detail', actions, currentPath)
    if (target && created?.header?.id) onNavigate(`${target}?id=${encodeURIComponent(created.header.id)}`)
  }

  if (formLocked) {
    return renderPanel({
      title: pickText(view, 'title', locale) || 'Editor',
      status: `Editing is unavailable while this record is ${recordStatus || 'locked'}.`,
      children: (
        <div className="mt-2 flex gap-3">
          <button
            onClick={() => onNavigate(cancelTarget ? `${cancelTarget}?id=${encodeURIComponent(targetID)}` : stripEditorSuffix(currentPath) || '/')}
            className="rounded-lg border border-line px-4 py-2 text-body"
          >
            Back to detail
          </button>
        </div>
      ),
    })
  }

  return renderPanel({
    title: pickText(view, 'title', locale) || 'Editor',
    children: (
      <>
        <div className="space-y-4">
          {sections.map((section) => (
            <section key={section.key} className="rounded-xl border border-line p-4">
              <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">{pickText(section, 'title', locale) || section.key}</h2>
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                {(section.fields || []).map((field) => renderFieldEditor({
                  field,
                  locale,
                  values,
                  onChange: setValues,
                  model: !!view.model_key,
                  catalog,
                  error: errors[field.key],
                  onBlur: () =>
                    setErrors((current) => {
                      const message = validateFieldInput(field, resolvePath(values, normalizeFieldPath(field, !!view.model_key)), locale)
                      if (!message && !current[field.key]) return current
                      const next = { ...current }
                      if (message) next[field.key] = message
                      else delete next[field.key]
                      return next
                    }),
                }))}
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
      </>
    ),
  })
}
