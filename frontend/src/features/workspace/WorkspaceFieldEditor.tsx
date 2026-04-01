import type { Dispatch, ReactNode, SetStateAction } from 'react'
import { pickText, type FieldDefinition } from '@/services/bootstrap'
import type { CommercialFormCatalog, FormState } from './workspaceFormTypes'

export function WorkspaceFieldEditor({
  field,
  locale,
  values,
  onChange,
  model,
  catalog,
  error,
  onBlur,
  normalizeFieldPath,
  resolvePath,
  commercialSelectOptions,
  humanize,
  applyFieldUpdate,
  assignPathValue,
  toNumber,
  asRecordList,
  commercialArrayColumns,
  commercialOpenInvoices,
  procurementOpenBills,
  commercialRefundablePayments,
  roundMoney,
  resolveVariantItemCodeFromRow,
  normalizeCommercialRows,
  commercialArrayDefaultRow,
  buildAllocationRows,
  buildRefundAllocationRows,
  buildProcurementAllocationRows,
  resolveCommercialColumnOptions,
  displayValue,
  applyCommercialArrayUpdate,
}: {
  field: FieldDefinition
  locale: string
  values: FormState
  onChange: Dispatch<SetStateAction<FormState>>
  model: boolean
  catalog: CommercialFormCatalog
  error?: string
  onBlur?: () => void
  normalizeFieldPath: (field: FieldDefinition, model: boolean) => string
  resolvePath: (payload: unknown, path: string) => unknown
  commercialSelectOptions: (path: string, catalog?: CommercialFormCatalog, values?: FormState) => Array<{ value: string; label: string }>
  humanize: (value: string) => string
  applyFieldUpdate: (current: FormState, path: string, value: unknown, catalog?: CommercialFormCatalog) => FormState
  assignPathValue: (current: FormState, path: string, value: unknown) => FormState
  toNumber: (value: unknown) => number
  asRecordList: (value: unknown) => Array<Record<string, unknown>>
  commercialArrayColumns: (widget: string, catalog?: CommercialFormCatalog, values?: FormState) => Array<{ key: string; label: string; type: 'text' | 'number'; readOnly?: boolean; options?: Array<{ value: string; label: string }> }>
  commercialOpenInvoices: (catalog?: CommercialFormCatalog, values?: FormState) => Array<Record<string, unknown>>
  procurementOpenBills: (catalog?: CommercialFormCatalog, values?: FormState) => Array<Record<string, unknown>>
  commercialRefundablePayments: (catalog?: CommercialFormCatalog, values?: FormState) => Array<Record<string, unknown>>
  roundMoney: (value: number) => number
  resolveVariantItemCodeFromRow: (row: Record<string, unknown>, catalog?: CommercialFormCatalog) => string
  normalizeCommercialRows: (rows: Array<Record<string, unknown>>, widget: string, catalog?: CommercialFormCatalog, values?: FormState) => Array<Record<string, unknown>>
  commercialArrayDefaultRow: (widget: string) => Record<string, unknown>
  buildAllocationRows: (invoices: Array<Record<string, unknown>>, requestedAmount: number) => { rows: Array<Record<string, unknown>>; allocatedAmount: number }
  buildRefundAllocationRows: (payments: Array<Record<string, unknown>>, requestedAmount: number) => { rows: Array<Record<string, unknown>>; allocatedAmount: number }
  buildProcurementAllocationRows: (bills: Array<Record<string, unknown>>, requestedAmount: number) => { rows: Array<Record<string, unknown>>; allocatedAmount: number }
  resolveCommercialColumnOptions: (
    key: string,
    widget: string,
    row: Record<string, unknown>,
    catalog?: CommercialFormCatalog,
    values?: FormState,
    fallback?: Array<{ value: string; label: string }>,
  ) => Array<{ value: string; label: string }>
  displayValue: (value: unknown) => string
  applyCommercialArrayUpdate: (current: FormState, path: string, widget: string, rows: Array<Record<string, unknown>>, catalog?: CommercialFormCatalog) => FormState
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
                if (patch.amount_received != null && (patch.replace_amount_received || toNumber(resolvePath(current, 'amount_received')) <= 0)) next = assignPathValue(next, 'amount_received', patch.amount_received)
                if (patch.amount_paid != null && (patch.replace_amount_paid || toNumber(resolvePath(current, 'amount_paid')) <= 0)) next = assignPathValue(next, 'amount_paid', patch.amount_paid)
                if (patch.amount_refunded != null && (patch.replace_amount_refunded || toNumber(resolvePath(current, 'amount_refunded')) <= 0)) next = assignPathValue(next, 'amount_refunded', patch.amount_refunded)
                if (patch.payment_reference && !String(resolvePath(current, 'payment_reference') || '')) next = assignPathValue(next, 'payment_reference', patch.payment_reference)
                if (patch.refund_reference && !String(resolvePath(current, 'refund_reference') || '')) next = assignPathValue(next, 'refund_reference', patch.refund_reference)
                if (patch.party_id && !String(resolvePath(current, 'party_id') || '')) next = applyFieldUpdate(next, 'party_id', patch.party_id, catalog)
                if (patch.vendor_id && !String(resolvePath(current, 'vendor_id') || '')) next = applyFieldUpdate(next, 'vendor_id', patch.vendor_id, catalog)
                if (patch.party_name && !String(resolvePath(current, 'party_name') || '')) next = assignPathValue(next, 'party_name', patch.party_name)
                if (patch.vendor_name && !String(resolvePath(current, 'vendor_name') || '')) next = assignPathValue(next, 'vendor_name', patch.vendor_name)
                if (patch.currency_code && !String(resolvePath(current, 'currency_code') || '')) next = assignPathValue(next, 'currency_code', patch.currency_code)
              }
              return next
            })
          }
          asRecordList={asRecordList}
          commercialArrayColumns={commercialArrayColumns}
          commercialOpenInvoices={commercialOpenInvoices}
          procurementOpenBills={procurementOpenBills}
          commercialRefundablePayments={commercialRefundablePayments}
          toNumber={toNumber}
          resolvePath={resolvePath}
          roundMoney={roundMoney}
          resolveVariantItemCodeFromRow={resolveVariantItemCodeFromRow}
          normalizeCommercialRows={normalizeCommercialRows}
          commercialArrayDefaultRow={commercialArrayDefaultRow}
          buildAllocationRows={buildAllocationRows}
          buildRefundAllocationRows={buildRefundAllocationRows}
          buildProcurementAllocationRows={buildProcurementAllocationRows}
          resolveCommercialColumnOptions={resolveCommercialColumnOptions}
          displayValue={displayValue}
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
        <textarea id={`field-${field.key}`} name={field.key} value={String(value ?? '')} onChange={(e) => update(e.target.value)} onBlur={onBlur} placeholder={placeholder} required={field.required} minLength={field.min_length} maxLength={field.max_length} className={textareaClassName} />
      ) : field.widget === 'select' || field.options?.length || catalogOptions.length ? (
        <select id={`field-${field.key}`} name={field.key} value={String(value ?? '')} onChange={(e) => update(e.target.value)} onBlur={onBlur} required={field.required} className={inputClassName}>
          <option value="">Select an option</option>
          {(catalogOptions.length ? catalogOptions : (field.options || []).map((option) => ({ value: option, label: humanize(option) }))).map((option) => (
            <option key={option.value} value={option.value}>{option.label}</option>
          ))}
        </select>
      ) : field.type === 'bool' ? (
        <input type="checkbox" id={`field-${field.key}`} name={field.key} checked={Boolean(value)} onChange={(e) => update(e.target.checked)} onBlur={onBlur} className="h-4 w-4" />
      ) : (
        <input type={field.type === 'int' || field.type === 'number' ? 'number' : 'text'} id={`field-${field.key}`} name={field.key} value={String(value ?? '')} onChange={(e) => update(field.type === 'int' || field.type === 'number' ? (e.target.value === '' ? '' : Number(e.target.value)) : e.target.value)} onBlur={onBlur} required={field.required} minLength={field.min_length} maxLength={field.max_length} pattern={field.pattern} min={field.min_value} max={field.max_value} className={inputClassName} placeholder={placeholder} />
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
  asRecordList,
  commercialArrayColumns,
  commercialOpenInvoices,
  procurementOpenBills,
  commercialRefundablePayments,
  toNumber,
  resolvePath,
  roundMoney,
  resolveVariantItemCodeFromRow,
  normalizeCommercialRows,
  commercialArrayDefaultRow,
  buildAllocationRows,
  buildRefundAllocationRows,
  buildProcurementAllocationRows,
  resolveCommercialColumnOptions,
  displayValue,
}: {
  fieldKey: string
  widget: string
  value: unknown
  values: FormState
  catalog: CommercialFormCatalog
  onChange: (rows: Array<Record<string, unknown>>, patch?: Record<string, unknown>) => void
  asRecordList: (value: unknown) => Array<Record<string, unknown>>
  commercialArrayColumns: (widget: string, catalog?: CommercialFormCatalog, values?: FormState) => Array<{ key: string; label: string; type: 'text' | 'number'; readOnly?: boolean; options?: Array<{ value: string; label: string }> }>
  commercialOpenInvoices: (catalog?: CommercialFormCatalog, values?: FormState) => Array<Record<string, unknown>>
  procurementOpenBills: (catalog?: CommercialFormCatalog, values?: FormState) => Array<Record<string, unknown>>
  commercialRefundablePayments: (catalog?: CommercialFormCatalog, values?: FormState) => Array<Record<string, unknown>>
  toNumber: (value: unknown) => number
  resolvePath: (payload: unknown, path: string) => unknown
  roundMoney: (value: number) => number
  resolveVariantItemCodeFromRow: (row: Record<string, unknown>, catalog?: CommercialFormCatalog) => string
  normalizeCommercialRows: (rows: Array<Record<string, unknown>>, widget: string, catalog?: CommercialFormCatalog, values?: FormState) => Array<Record<string, unknown>>
  commercialArrayDefaultRow: (widget: string) => Record<string, unknown>
  buildAllocationRows: (invoices: Array<Record<string, unknown>>, requestedAmount: number) => { rows: Array<Record<string, unknown>>; allocatedAmount: number }
  buildRefundAllocationRows: (payments: Array<Record<string, unknown>>, requestedAmount: number) => { rows: Array<Record<string, unknown>>; allocatedAmount: number }
  buildProcurementAllocationRows: (bills: Array<Record<string, unknown>>, requestedAmount: number) => { rows: Array<Record<string, unknown>>; allocatedAmount: number }
  resolveCommercialColumnOptions: (
    key: string,
    widget: string,
    row: Record<string, unknown>,
    catalog?: CommercialFormCatalog,
    values?: FormState,
    fallback?: Array<{ value: string; label: string }>,
  ) => Array<{ value: string; label: string }>
  displayValue: (value: unknown) => string
}) {
  const rows = asRecordList(value)
  const columns = commercialArrayColumns(widget, catalog, values)
  const openInvoices = widget === 'commercial_allocations' ? commercialOpenInvoices(catalog, values) : []
  const openInvoiceBalance = openInvoices.reduce((sum, invoice) => sum + toNumber(resolvePath(invoice, 'body.payload.balance_due_amount')), 0)
  const openBills = widget === 'procurement_allocations' ? procurementOpenBills(catalog, values) : []
  const openBillBalance = openBills.reduce((sum, bill) => sum + toNumber(resolvePath(bill, 'body.payload.balance_due_amount')), 0)
  const refundablePayments = widget === 'commercial_refund_allocations' ? commercialRefundablePayments(catalog, values) : []
  const refundablePaymentBalance = refundablePayments.reduce((sum, payment) => sum + roundMoney(Math.max(toNumber(resolvePath(payment, 'body.payload.amount_received')) - toNumber(resolvePath(payment, 'body.payload.refunded_amount')), 0)), 0)

  function updateRow(index: number, key: string, nextValue: unknown) {
    let patch: Record<string, unknown> | undefined
    const nextRows = rows.map((row, rowIndex) => {
      if (rowIndex !== index) return row
      const updated = { ...row, [key]: nextValue }
      if (key === 'product_code') {
        updated.variant_signature = ''
        updated.item_code = ''
      }
      if (key === 'variant_signature') updated.item_code = resolveVariantItemCodeFromRow(updated, catalog)
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
        patch = { amount_received: openAmount > 0 ? openAmount : undefined, payment_reference: invoiceNumber || undefined, party_id: partyID || undefined, party_name: partyName || undefined, currency_code: currencyCode || undefined }
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
        patch = { amount_paid: openAmount > 0 ? openAmount : undefined, vendor_id: vendorID || undefined, vendor_name: vendorName || undefined, currency_code: currencyCode || undefined }
      }
      if (widget === 'commercial_refund_allocations' && key === 'payment_id') {
        const payment = catalog.paymentsByID[String(nextValue || '')]
        const paymentNumber = String(resolvePath(payment, 'header.number') || '')
        const methodCode = String(resolvePath(payment, 'body.payload.payment_method_code') || '')
        const clearingAccount = String(resolvePath(payment, 'body.payload.clearing_account_code') || '')
        const remainingAmount = roundMoney(Math.max(toNumber(resolvePath(payment, 'body.payload.amount_received')) - toNumber(resolvePath(payment, 'body.payload.refunded_amount')), 0))
        if (paymentNumber) updated.payment_number = paymentNumber
        if (!toNumber(updated.amount) && remainingAmount > 0) updated.amount = remainingAmount
        patch = { amount_refunded: remainingAmount > 0 ? remainingAmount : undefined, refund_reference: paymentNumber || undefined, payment_method_code: methodCode || undefined, clearing_account_code: clearingAccount || undefined }
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
      if (widget === 'production_component_lines' && key === 'component_item_code' && !String(updated.actual_item_code || '')) updated.actual_item_code = String(nextValue || '')
      if (widget === 'production_issue_lines' && key === 'actual_item_code') updated.item_code = String(nextValue || '')
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
    onChange(normalizeCommercialRows(nextRows, widget, catalog), { amount_received: allocatedAmount, replace_amount_received: useFullOpenBalance || targetAmount <= 0 })
  }

  function autoAllocateRefund(useFullRefundableBalance: boolean) {
    if (widget !== 'commercial_refund_allocations') return
    const targetAmount = useFullRefundableBalance ? refundablePaymentBalance : toNumber(resolvePath(values, 'amount_refunded'))
    const { rows: nextRows, allocatedAmount } = buildRefundAllocationRows(refundablePayments, targetAmount)
    onChange(normalizeCommercialRows(nextRows, widget, catalog), { amount_refunded: allocatedAmount, replace_amount_refunded: useFullRefundableBalance || targetAmount <= 0 })
  }

  function autoAllocateProcurement(useFullOpenBalance: boolean) {
    if (widget !== 'procurement_allocations') return
    const targetAmount = useFullOpenBalance ? openBillBalance : toNumber(resolvePath(values, 'amount_paid'))
    const { rows: nextRows, allocatedAmount } = buildProcurementAllocationRows(openBills, targetAmount)
    onChange(normalizeCommercialRows(nextRows, widget, catalog), { amount_paid: allocatedAmount, replace_amount_paid: useFullOpenBalance || targetAmount <= 0 })
  }

  return (
    <div className="space-y-3 rounded-xl border border-line p-3">
      {widget === 'commercial_allocations' ? <div className="flex flex-wrap items-center gap-3 rounded-lg border border-line bg-accent-soft/30 px-3 py-2 text-sm text-body"><span>Open invoices for payer: <strong>{openInvoices.length}</strong></span><span>Open balance: <strong>{roundMoney(openInvoiceBalance)}</strong></span><button type="button" onClick={() => autoAllocate(false)} className="rounded-lg border border-line px-3 py-2 text-body" disabled={!openInvoices.length}>Auto Allocate Receipt</button><button type="button" onClick={() => autoAllocate(true)} className="rounded-lg border border-line px-3 py-2 text-body" disabled={!openInvoices.length}>Use Full Open Balance</button><button type="button" onClick={() => onChange([])} className="rounded-lg border border-line px-3 py-2 text-body" disabled={!rows.length}>Clear Allocations</button></div> : null}
      {widget === 'commercial_refund_allocations' ? <div className="flex flex-wrap items-center gap-3 rounded-lg border border-line bg-accent-soft/30 px-3 py-2 text-sm text-body"><span>Refundable receipts: <strong>{refundablePayments.length}</strong></span><span>Refundable balance: <strong>{roundMoney(refundablePaymentBalance)}</strong></span><button type="button" onClick={() => autoAllocateRefund(false)} className="rounded-lg border border-line px-3 py-2 text-body" disabled={!refundablePayments.length}>Auto Allocate Refund</button><button type="button" onClick={() => autoAllocateRefund(true)} className="rounded-lg border border-line px-3 py-2 text-body" disabled={!refundablePayments.length}>Use Full Refundable Balance</button><button type="button" onClick={() => onChange([])} className="rounded-lg border border-line px-3 py-2 text-body" disabled={!rows.length}>Clear Refund Allocations</button></div> : null}
      {widget === 'procurement_allocations' ? <div className="flex flex-wrap items-center gap-3 rounded-lg border border-line bg-accent-soft/30 px-3 py-2 text-sm text-body"><span>Open bills for vendor: <strong>{openBills.length}</strong></span><span>Open balance: <strong>{roundMoney(openBillBalance)}</strong></span><button type="button" onClick={() => autoAllocateProcurement(false)} className="rounded-lg border border-line px-3 py-2 text-body" disabled={!openBills.length}>Auto Allocate Payment</button><button type="button" onClick={() => autoAllocateProcurement(true)} className="rounded-lg border border-line px-3 py-2 text-body" disabled={!openBills.length}>Use Full Open Balance</button><button type="button" onClick={() => onChange([])} className="rounded-lg border border-line px-3 py-2 text-body" disabled={!rows.length}>Clear Allocations</button></div> : null}
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-line text-sm">
          <thead className="border-b border-line bg-accent-soft dark:bg-ink/60">
            <tr>
              {columns.map((column) => <th key={column.key} className="px-3 py-2 text-left text-xs font-bold uppercase tracking-[0.14em] text-accent-dark dark:text-body">{column.label}</th>)}
              <th className="px-3 py-2" />
            </tr>
          </thead>
          <tbody className="divide-y divide-line bg-surface">
            {rows.length ? rows.map((row, index) => (
              <tr key={`${widget}-${index}`}>
                {columns.map((column) => (
                  <td key={column.key} className="px-3 py-2 align-top">
                    {column.readOnly ? (
                      <div className="h-10 rounded-lg border border-line bg-accent-soft/40 px-3 py-2 text-sm text-body">{displayValue(row[column.key])}</div>
                    ) : resolveCommercialColumnOptions(column.key, widget, row, catalog, values, column.options).length ? (
                      (() => {
                        const options = resolveCommercialColumnOptions(column.key, widget, row, catalog, values, column.options)
                        return <select id={`field-${fieldKey}-${index}-${column.key}`} name={`${fieldKey}[${index}].${column.key}`} className="h-10 w-full rounded-lg border border-line bg-surface px-3 py-2 text-sm text-body" value={String(row[column.key] ?? '')} onChange={(event) => updateRow(index, column.key, event.target.value)}><option value="">Select an option</option>{options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select>
                      })()
                    ) : (
                      <input type={column.type === 'number' ? 'number' : 'text'} id={`field-${fieldKey}-${index}-${column.key}`} name={`${fieldKey}[${index}].${column.key}`} className="h-10 w-full rounded-lg border border-line bg-surface px-3 py-2 text-sm text-body" value={String(row[column.key] ?? '')} onChange={(event) => updateRow(index, column.key, column.type === 'number' ? (event.target.value === '' ? '' : Number(event.target.value)) : event.target.value)} />
                    )}
                  </td>
                ))}
                <td className="px-3 py-2 text-right"><button type="button" onClick={() => removeRow(index)} className="rounded-lg border border-line px-3 py-2 text-body">Remove</button></td>
              </tr>
            )) : <tr><td colSpan={columns.length + 1} className="px-3 py-6 text-center text-sm text-muted">No rows yet.</td></tr>}
          </tbody>
        </table>
      </div>
      <button type="button" onClick={addRow} className="rounded-lg border border-line px-4 py-2 text-body">Add Row</button>
    </div>
  )
}

export function renderWorkspaceDetailFieldValue({
  field,
  value,
  asRecordList,
  commercialArrayColumns,
  displayValue,
}: {
  field: FieldDefinition
  value: unknown
  asRecordList: (value: unknown) => Array<Record<string, unknown>>
  commercialArrayColumns: (widget: string, catalog?: CommercialFormCatalog, values?: FormState) => Array<{ key: string; label: string; type: 'text' | 'number'; readOnly?: boolean; options?: Array<{ value: string; label: string }> }>
  displayValue: (value: unknown) => string
}): ReactNode {
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
    return <div className="overflow-x-auto"><table className="min-w-full divide-y divide-line text-sm"><thead className="border-b border-line bg-accent-soft dark:bg-ink/60"><tr>{columns.map((column) => <th key={column.key} className="px-3 py-2 text-left text-xs font-bold uppercase tracking-[0.14em] text-accent-dark dark:text-body">{column.label}</th>)}</tr></thead><tbody className="divide-y divide-line bg-surface">{rows.map((row, index) => <tr key={`${field.key}-${index}`}>{columns.map((column) => <td key={column.key} className="px-3 py-2 align-top text-body">{displayValue(row[column.key])}</td>)}</tr>)}</tbody></table></div>
  }
  if (field.widget === 'json') return <pre className="overflow-x-auto whitespace-pre-wrap text-xs text-body">{JSON.stringify(value ?? {}, null, 2)}</pre>
  return displayValue(value)
}
