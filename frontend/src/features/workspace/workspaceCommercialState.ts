import { resolveVariantItemCodeFromRow } from './workspaceCommercial'
import type { CommercialFormCatalog, FormState } from './workspaceFormTypes'
import { addDaysToDate, asRecordList, assignPathValue, resolvePath, roundMoney, toNumber } from './workspaceShared'

export function applyCommercialArrayUpdate(current: FormState, path: string, widget: string, rows: Array<Record<string, unknown>>, catalog?: CommercialFormCatalog): FormState {
  let next = assignPathValue(current, path, rows)
  switch (widget) {
    case 'commercial_lines': {
      const defaultTaxCode = String(resolvePath(current, 'default_tax_code') || '')
      const rowsWithDefaults = rows.map((row) => ({ ...row, tax_code: row.tax_code || defaultTaxCode }))
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
      next = assignPathValue(next, 'unapplied_amount', roundMoney(Math.max(amountReceived - appliedAmount, 0)))
      return next
    }
    case 'commercial_refund_allocations': {
      const normalizedRows = rows.map((row) => ({ ...row, amount: toNumber(row.amount) }))
      const refundedAmount = normalizedRows.reduce((sum, row) => sum + toNumber(row.amount), 0)
      next = assignPathValue(next, path, normalizedRows)
      next = assignPathValue(next, 'amount_refunded', roundMoney(refundedAmount))
      if (normalizedRows.length === 1) {
        const firstRow = normalizedRows[0] as Record<string, unknown> | undefined
        next = assignPathValue(next, 'source_payment_id', String(firstRow?.payment_id || ''))
        next = assignPathValue(next, 'source_payment_number', String(firstRow?.payment_number || ''))
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
      const rowsWithDefaults = rows.map((row) => ({ ...row, tax_code: row.tax_code || String(resolvePath(current, 'default_tax_code') || '') }))
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
        return { ...row, ordered_qty: orderedQty, received_qty: receivedQty, cumulative_received_qty: roundMoney(previouslyReceived + receivedQty) }
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
    case 'return_lines':
    case 'supplier_return_lines':
    case 'inventory_lines':
    case 'inventory_transfer_lines': {
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
    case 'production_stage_lines':
    case 'trace_batches': {
      const normalizedRows = normalizeCommercialRows(rows, widget, catalog, current)
      next = assignPathValue(next, path, normalizedRows)
      return next
    }
    default:
      return next
  }
}

export function applyFieldUpdate(current: FormState, path: string, value: unknown, catalog?: CommercialFormCatalog): FormState {
  const next = assignPathValue(current, path, value)
  if (path === 'default_tax_code') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) return applyCommercialArrayUpdate(next, 'lines', 'commercial_lines', lines, catalog)
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
      if (typeof inheritedValue === 'boolean' && !resolvePath(withProduct, fieldKey)) withProduct = assignPathValue(withProduct, fieldKey, inheritedValue)
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
    if (partyName) withParty = assignPathValue(withParty, 'party_name', partyName)
    if (currencyCode && !String(resolvePath(withParty, 'currency_code') || '')) withParty = assignPathValue(withParty, 'currency_code', currencyCode)
    if (taxProfileCode && !String(resolvePath(withParty, 'tax_profile_code') || '')) withParty = assignPathValue(withParty, 'tax_profile_code', taxProfileCode)
    if (priceListCode && !String(resolvePath(withParty, 'price_list_code') || '')) withParty = assignPathValue(withParty, 'price_list_code', priceListCode)
    if (profileDefaultTaxCode && !String(resolvePath(withParty, 'default_tax_code') || '')) withParty = assignPathValue(withParty, 'default_tax_code', profileDefaultTaxCode)
    const resolvedPaymentTermDays = toNumber(paymentTermDays) || profilePaymentTermDays
    if (resolvedPaymentTermDays > 0 && !toNumber(resolvePath(withParty, 'payment_term_days'))) {
      withParty = assignPathValue(withParty, 'payment_term_days', resolvedPaymentTermDays)
      const baseDate = String(resolvePath(withParty, 'invoice_date') || resolvePath(withParty, 'order_date') || '')
      if (baseDate) {
        const dueDate = addDaysToDate(baseDate, resolvedPaymentTermDays)
        if (dueDate) withParty = assignPathValue(withParty, 'due_date', dueDate)
      }
    }
    const lines = asRecordList(resolvePath(withParty, 'lines'))
    if (lines.length) return applyCommercialArrayUpdate(withParty, 'lines', 'commercial_lines', lines, catalog)
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
    if (currencyCode && !String(resolvePath(withVendor, 'currency_code') || '')) withVendor = assignPathValue(withVendor, 'currency_code', currencyCode)
    if (taxProfileCode && !String(resolvePath(withVendor, 'tax_profile_code') || '')) withVendor = assignPathValue(withVendor, 'tax_profile_code', taxProfileCode)
    if (paymentTermDays != null && paymentTermDays !== '' && !toNumber(resolvePath(withVendor, 'payment_term_days'))) {
      withVendor = assignPathValue(withVendor, 'payment_term_days', toNumber(paymentTermDays))
      const baseDate = String(resolvePath(withVendor, 'bill_date') || resolvePath(withVendor, 'order_date') || '')
      if (baseDate) {
        const dueDate = addDaysToDate(baseDate, toNumber(paymentTermDays))
        if (dueDate) withVendor = assignPathValue(withVendor, 'due_date', dueDate)
      }
    }
    if (payableAccount && !String(resolvePath(withVendor, 'payable_account_code') || '')) withVendor = assignPathValue(withVendor, 'payable_account_code', payableAccount)
    if (expenseAccount && !String(resolvePath(withVendor, 'expense_account_code') || '')) withVendor = assignPathValue(withVendor, 'expense_account_code', expenseAccount)
    if (defaultPaymentMethod && !String(resolvePath(withVendor, 'payment_method_code') || '')) withVendor = applyFieldUpdate(withVendor, 'payment_method_code', defaultPaymentMethod, catalog)
    const lines = asRecordList(resolvePath(withVendor, 'lines'))
    if (lines.length && String(resolvePath(withVendor, 'source_purchase_order_id') || '') !== '') return applyCommercialArrayUpdate(withVendor, 'lines', 'procurement_receipt_lines', lines, catalog)
    if (lines.length) return applyCommercialArrayUpdate(withVendor, 'lines', 'procurement_lines', lines, catalog)
    return withVendor
  }
  if (path === 'price_list_code') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) return applyCommercialArrayUpdate(next, 'lines', 'commercial_lines', lines, catalog)
    return next
  }
  if (path === 'tax_profile_code') {
    const profileCode = String(value || '')
    const profile = catalog?.taxProfilesByCode?.[profileCode]
    let withProfile = next
    const defaultTaxCode = resolvePath(profile, 'values.default_tax_code')
    const paymentTermDays = resolvePath(profile, 'values.payment_term_days')
    if (typeof defaultTaxCode === 'string' && defaultTaxCode) withProfile = assignPathValue(withProfile, 'default_tax_code', defaultTaxCode)
    if (paymentTermDays != null && paymentTermDays !== '') {
      withProfile = assignPathValue(withProfile, 'payment_term_days', toNumber(paymentTermDays))
      const baseDate = String(resolvePath(withProfile, 'invoice_date') || resolvePath(withProfile, 'order_date') || '')
      if (baseDate) withProfile = assignPathValue(withProfile, 'due_date', addDaysToDate(baseDate, toNumber(paymentTermDays)))
    }
    const lines = asRecordList(resolvePath(withProfile, 'lines'))
    if (lines.length) return applyCommercialArrayUpdate(withProfile, 'lines', 'commercial_lines', lines, catalog)
    return withProfile
  }
  if ((path === 'invoice_date' || path === 'order_date') && toNumber(resolvePath(next, 'payment_term_days')) > 0) {
    const dueDate = addDaysToDate(String(value || ''), toNumber(resolvePath(next, 'payment_term_days')))
    if (dueDate) return assignPathValue(next, 'due_date', dueDate)
  }
  if (path === 'payment_term_days') {
    const baseDate = String(resolvePath(next, 'invoice_date') || resolvePath(next, 'order_date') || '')
    const dueDate = addDaysToDate(baseDate, toNumber(value))
    if (dueDate) return assignPathValue(next, 'due_date', dueDate)
  }
  if (path === 'payment_method_code') {
    const methodCode = String(value || '')
    const clearingAccount = resolvePath(catalog?.paymentMethodsByCode?.[methodCode], 'values.clearing_account_code')
    if (typeof clearingAccount === 'string' && clearingAccount) return assignPathValue(next, 'clearing_account_code', clearingAccount)
  }
  if (path === 'amount_paid') {
    const allocations = asRecordList(resolvePath(next, 'allocations'))
    if (allocations.length) return applyCommercialArrayUpdate(next, 'allocations', 'procurement_allocations', allocations, catalog)
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
    if (paymentNumber && !String(resolvePath(updated, 'refund_reference') || '')) updated = assignPathValue(updated, 'refund_reference', paymentNumber)
    const currentRefund = toNumber(resolvePath(updated, 'amount_refunded'))
    if ((currentRefund <= 0 || currentRefund > remainingAmount) && remainingAmount > 0) updated = assignPathValue(updated, 'amount_refunded', remainingAmount)
    return updated
  }
  if (path === 'amount_refunded') {
    const refundAllocations = asRecordList(resolvePath(next, 'refund_allocations'))
    if (refundAllocations.length) return applyCommercialArrayUpdate(next, 'refund_allocations', 'commercial_refund_allocations', refundAllocations, catalog)
  }
  if (path === 'amount_received') {
    const allocations = asRecordList(resolvePath(next, 'allocations'))
    if (allocations.length) return applyCommercialArrayUpdate(next, 'allocations', 'commercial_allocations', allocations, catalog)
    return assignPathValue(next, 'unapplied_amount', roundMoney(Math.max(toNumber(value), 0)))
  }
  return next
}

export function normalizeCommercialFormState(current: FormState, documentType: string, catalog: CommercialFormCatalog): FormState {
  let next = current
  const partyID = String(resolvePath(next, 'party_id') || '')
  const party = catalog.partiesByID[partyID]
  if (party) {
    const partyName = String(resolvePath(party, 'values.display_name') || resolvePath(party, 'values.name') || '')
    const currencyCode = String(resolvePath(party, 'values.currency_code') || '')
    const taxProfileCode = String(resolvePath(party, 'values.tax_profile_code') || '')
    const priceListCode = String(resolvePath(party, 'values.default_price_list_code') || '')
    const paymentTermDays = toNumber(resolvePath(party, 'values.payment_term_days'))
    if (!resolvePath(next, 'party_name') && partyName) next = assignPathValue(next, 'party_name', partyName)
    if (!resolvePath(next, 'currency_code') && currencyCode) next = assignPathValue(next, 'currency_code', currencyCode)
    if (!resolvePath(next, 'tax_profile_code') && taxProfileCode) next = assignPathValue(next, 'tax_profile_code', taxProfileCode)
    if (!resolvePath(next, 'price_list_code') && priceListCode) next = assignPathValue(next, 'price_list_code', priceListCode)
    if (!resolvePath(next, 'payment_term_days') && paymentTermDays > 0) next = assignPathValue(next, 'payment_term_days', paymentTermDays)
  }
  const profileCode = String(resolvePath(next, 'tax_profile_code') || '')
  const profile = catalog.taxProfilesByCode[profileCode]
  if (profile) {
    const defaultTaxCode = String(resolvePath(profile, 'values.default_tax_code') || '')
    if (!resolvePath(next, 'default_tax_code') && defaultTaxCode) next = assignPathValue(next, 'default_tax_code', defaultTaxCode)
    const paymentTermDays = toNumber(resolvePath(profile, 'values.payment_term_days'))
    if (!resolvePath(next, 'payment_term_days') && paymentTermDays > 0) next = assignPathValue(next, 'payment_term_days', paymentTermDays)
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
    if (baseDate && paymentTermDays > 0 && !resolvePath(next, 'due_date')) next = assignPathValue(next, 'due_date', addDaysToDate(baseDate, paymentTermDays))
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) return applyCommercialArrayUpdate(next, 'lines', 'commercial_lines', lines, catalog)
    return next
  }
  if (documentType === 'purchase_request' || documentType === 'purchase_order' || documentType === 'vendor_bill' || documentType === 'vendor_credit_note') {
    const baseDate = String(resolvePath(next, 'bill_date') || resolvePath(next, 'order_date') || resolvePath(next, 'request_date') || '')
    const paymentTermDays = toNumber(resolvePath(next, 'payment_term_days'))
    if (baseDate && paymentTermDays > 0 && !resolvePath(next, 'due_date')) next = assignPathValue(next, 'due_date', addDaysToDate(baseDate, paymentTermDays))
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) return applyCommercialArrayUpdate(next, 'lines', 'procurement_lines', lines, catalog)
    return next
  }
  if (documentType === 'goods_receipt') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) return applyCommercialArrayUpdate(next, 'lines', 'procurement_receipt_lines', lines, catalog)
    return next
  }
  if (documentType === 'sales_fulfillment') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) return applyCommercialArrayUpdate(next, 'lines', 'fulfillment_lines', lines, catalog)
    return next
  }
  if (documentType === 'delivery_order') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) return applyCommercialArrayUpdate(next, 'lines', 'delivery_lines', lines, catalog)
    return next
  }
  if (documentType === 'sales_return' || documentType === 'return_receipt') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) return applyCommercialArrayUpdate(next, 'lines', 'return_lines', lines, catalog)
    return next
  }
  if (documentType === 'supplier_return') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) return applyCommercialArrayUpdate(next, 'lines', 'supplier_return_lines', lines, catalog)
    return next
  }
  if (documentType === 'stock_receipt' || documentType === 'stock_issue' || documentType === 'stock_adjustment') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) return applyCommercialArrayUpdate(next, 'lines', 'inventory_lines', lines, catalog)
    return next
  }
  if (documentType === 'stock_transfer') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) return applyCommercialArrayUpdate(next, 'lines', 'inventory_transfer_lines', lines, catalog)
    return next
  }
  if (documentType === 'production_order') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    const stages = asRecordList(resolvePath(next, 'stages'))
    if (lines.length) next = applyCommercialArrayUpdate(next, 'lines', 'production_component_lines', lines, catalog)
    if (stages.length) next = applyCommercialArrayUpdate(next, 'stages', 'production_stage_lines', stages, catalog)
    return next
  }
  if (documentType === 'production_issue') {
    const lines = asRecordList(resolvePath(next, 'lines'))
    if (lines.length) return applyCommercialArrayUpdate(next, 'lines', 'production_issue_lines', lines, catalog)
    return next
  }
  if (documentType === 'payment_out') {
    next = applyFieldUpdate(next, 'payment_method_code', resolvePath(next, 'payment_method_code'), catalog)
    const allocations = asRecordList(resolvePath(next, 'allocations'))
    if (allocations.length) return applyCommercialArrayUpdate(next, 'allocations', 'procurement_allocations', allocations, catalog)
    return next
  }
  if (documentType === 'payment_receipt') {
    next = applyFieldUpdate(next, 'payment_method_code', resolvePath(next, 'payment_method_code'), catalog)
    const allocations = asRecordList(resolvePath(next, 'allocations'))
    if (allocations.length) return applyCommercialArrayUpdate(next, 'allocations', 'commercial_allocations', allocations, catalog)
    return next
  }
  if (documentType === 'payment_refund') {
    const refundAllocations = asRecordList(resolvePath(next, 'refund_allocations'))
    if (refundAllocations.length) next = applyCommercialArrayUpdate(next, 'refund_allocations', 'commercial_refund_allocations', refundAllocations, catalog)
    next = applyFieldUpdate(next, 'source_payment_id', resolvePath(next, 'source_payment_id'), catalog)
    next = applyFieldUpdate(next, 'payment_method_code', resolvePath(next, 'payment_method_code'), catalog)
    return next
  }
  return next
}

export function normalizeCommercialRows(rows: Array<Record<string, unknown>>, widget: string, catalog?: CommercialFormCatalog, values?: FormState): Array<Record<string, unknown>> {
  if (widget === 'commercial_refund_allocations') {
    return rows.map((row) => {
      const paymentID = String(row.payment_id || '')
      const payment = catalog?.paymentsByID?.[paymentID]
      const paymentNumber = String(row.payment_number || resolvePath(payment, 'header.number') || '')
      const amountReceived = toNumber(resolvePath(payment, 'body.payload.amount_received'))
      const refundedAmount = toNumber(resolvePath(payment, 'body.payload.refunded_amount'))
      const remainingAmount = roundMoney(Math.max(amountReceived - refundedAmount, 0))
      let amount = toNumber(row.amount)
      if (remainingAmount > 0 && (amount <= 0 || amount > remainingAmount)) amount = remainingAmount
      return { ...row, payment_number: paymentNumber, amount }
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
  if (mode === 'exempt') return { subtotal: roundMoney(grossAmount), tax: 0, total: roundMoney(grossAmount) }
  const subtotal = roundMoney(grossAmount)
  const tax = roundMoney(subtotal * taxRate / 100)
  return { subtotal, tax, total: roundMoney(subtotal + tax) }
}
