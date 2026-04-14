import type { CommercialFormCatalog, FormState } from './workspaceFormTypes'
import { asRecordList, humanize, resolvePath, roundMoney, toNumber } from './workspaceShared'

type CommercialOption = { value: string; label: string }
type CommercialColumn = { key: string; label: string; type: 'text' | 'number'; readOnly?: boolean; options?: CommercialOption[] }

export function commercialArrayColumns(widget: string, catalog?: CommercialFormCatalog, values?: FormState): CommercialColumn[] {
  switch (widget) {
    case 'commercial_lines':
      return [
        { key: 'product_code', label: 'Product', type: 'text', options: commercialSelectOptions('product_code', catalog) },
        { key: 'variant_signature', label: 'Variant', type: 'text' },
        { key: 'item_code', label: 'Item', type: 'text', options: commercialSelectOptions('item_code', catalog) },
        { key: 'description', label: 'Description', type: 'text' },
        { key: 'uom_code', label: 'UOM', type: 'text', options: commercialSelectOptions('uom_code', catalog) },
        { key: 'quantity', label: 'Qty', type: 'number' },
        { key: 'unit_price', label: 'Unit Price', type: 'number' },
        { key: 'discount_amount', label: 'Discount', type: 'number' },
        { key: 'tax_code', label: 'Tax Code', type: 'text', options: commercialSelectOptions('tax_code', catalog) },
        { key: 'tax_rate', label: 'Tax Rate %', type: 'number' },
        { key: 'line_subtotal', label: 'Subtotal', type: 'number', readOnly: true },
        { key: 'tax_amount', label: 'Tax', type: 'number', readOnly: true },
        { key: 'line_total', label: 'Total', type: 'number', readOnly: true },
      ]
    case 'commercial_allocations':
      return [
        { key: 'invoice_number', label: 'Invoice', type: 'text', readOnly: true },
        { key: 'invoice_id', label: 'Invoice ID', type: 'text', options: commercialSelectOptions('invoice_id', catalog, values) },
        { key: 'amount', label: 'Amount', type: 'number' },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'commercial_refund_allocations':
      return [
        { key: 'payment_number', label: 'Receipt', type: 'text', readOnly: true },
        { key: 'payment_id', label: 'Receipt ID', type: 'text', options: commercialSelectOptions('source_payment_id', catalog, values) },
        { key: 'amount', label: 'Amount', type: 'number' },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'commercial_journal_lines':
      return [
        { key: 'account_code', label: 'Account', type: 'text' },
        { key: 'description', label: 'Description', type: 'text' },
        { key: 'debit', label: 'Debit', type: 'number' },
        { key: 'credit', label: 'Credit', type: 'number' },
      ]
    case 'procurement_lines':
      return [
        { key: 'product_code', label: 'Product', type: 'text', options: commercialSelectOptions('product_code', catalog) },
        { key: 'variant_signature', label: 'Variant', type: 'text' },
        { key: 'item_code', label: 'Item', type: 'text', options: commercialSelectOptions('item_code', catalog) },
        { key: 'description', label: 'Description', type: 'text' },
        { key: 'uom_code', label: 'UOM', type: 'text', options: commercialSelectOptions('uom_code', catalog) },
        { key: 'quantity', label: 'Qty', type: 'number' },
        { key: 'received_qty', label: 'Received', type: 'number' },
        { key: 'billed_qty', label: 'Billed', type: 'number' },
        { key: 'unit_price', label: 'Unit Price', type: 'number' },
        { key: 'tax_code', label: 'Tax Code', type: 'text', options: commercialSelectOptions('tax_code', catalog) },
        { key: 'line_total', label: 'Total', type: 'number', readOnly: true },
      ]
    case 'procurement_receipt_lines':
      return [
        { key: 'product_code', label: 'Product', type: 'text', readOnly: true },
        { key: 'variant_signature', label: 'Variant', type: 'text', readOnly: true },
        { key: 'item_code', label: 'Item', type: 'text', readOnly: true },
        { key: 'description', label: 'Description', type: 'text', readOnly: true },
        { key: 'uom_code', label: 'UOM', type: 'text', readOnly: true },
        { key: 'ordered_qty', label: 'Ordered', type: 'number', readOnly: true },
        { key: 'received_qty', label: 'Receive Now', type: 'number' },
        { key: 'cumulative_received_qty', label: 'Cumulative', type: 'number', readOnly: true },
        { key: 'warehouse_code', label: 'Warehouse', type: 'text', options: commercialSelectOptions('warehouse_code', catalog) },
        { key: 'batch_code', label: 'Batch', type: 'text' },
        { key: 'expiration_date', label: 'Expiration', type: 'text' },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'procurement_allocations':
      return [
        { key: 'bill_number', label: 'Bill', type: 'text', readOnly: true },
        { key: 'bill_id', label: 'Bill ID', type: 'text', options: commercialSelectOptions('bill_id', catalog, values) },
        { key: 'amount', label: 'Amount', type: 'number' },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'fulfillment_lines':
      return [
        { key: 'product_code', label: 'Product', type: 'text', readOnly: true },
        { key: 'variant_signature', label: 'Variant', type: 'text', readOnly: true },
        { key: 'item_code', label: 'Item', type: 'text', readOnly: true },
        { key: 'description', label: 'Description', type: 'text', readOnly: true },
        { key: 'warehouse_code', label: 'Warehouse', type: 'text', options: commercialSelectOptions('warehouse_code', catalog) },
        { key: 'batch_code', label: 'Batch', type: 'text' },
        { key: 'expiration_date', label: 'Expiration', type: 'text', readOnly: true },
        { key: 'uom_code', label: 'UOM', type: 'text', readOnly: true },
        { key: 'ordered_quantity', label: 'Ordered', type: 'number', readOnly: true },
        { key: 'quantity', label: 'Reserve', type: 'number' },
        { key: 'available_quantity', label: 'Available', type: 'number', readOnly: true },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'delivery_lines':
      return [
        { key: 'product_code', label: 'Product', type: 'text', readOnly: true },
        { key: 'variant_signature', label: 'Variant', type: 'text', readOnly: true },
        { key: 'item_code', label: 'Item', type: 'text', readOnly: true },
        { key: 'description', label: 'Description', type: 'text', readOnly: true },
        { key: 'warehouse_code', label: 'Warehouse', type: 'text', options: commercialSelectOptions('warehouse_code', catalog) },
        { key: 'batch_code', label: 'Batch', type: 'text', readOnly: true },
        { key: 'expiration_date', label: 'Expiration', type: 'text', readOnly: true },
        { key: 'uom_code', label: 'UOM', type: 'text', readOnly: true },
        { key: 'fulfilled_quantity', label: 'Fulfilled', type: 'number', readOnly: true },
        { key: 'quantity', label: 'Deliver Qty', type: 'number' },
        { key: 'tracking_number', label: 'Tracking', type: 'text' },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'return_lines':
      return [
        { key: 'product_code', label: 'Product', type: 'text', readOnly: true },
        { key: 'variant_signature', label: 'Variant', type: 'text', readOnly: true },
        { key: 'item_code', label: 'Item', type: 'text', readOnly: true },
        { key: 'description', label: 'Description', type: 'text', readOnly: true },
        { key: 'warehouse_code', label: 'Warehouse', type: 'text', options: commercialSelectOptions('warehouse_code', catalog) },
        { key: 'batch_code', label: 'Batch', type: 'text' },
        { key: 'expiration_date', label: 'Expiration', type: 'text' },
        { key: 'uom_code', label: 'UOM', type: 'text', readOnly: true },
        { key: 'fulfilled_quantity', label: 'Fulfilled', type: 'number', readOnly: true },
        { key: 'quantity', label: 'Return Qty', type: 'number' },
        { key: 'disposition', label: 'Disposition', type: 'text', options: [{ value: 'restock', label: 'Restock' }, { value: 'quarantine', label: 'Quarantine' }, { value: 'block', label: 'Block' }] },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'supplier_return_lines':
      return [
        { key: 'item_code', label: 'Item', type: 'text', readOnly: true },
        { key: 'description', label: 'Description', type: 'text', readOnly: true },
        { key: 'warehouse_code', label: 'Warehouse', type: 'text', options: commercialSelectOptions('warehouse_code', catalog) },
        { key: 'batch_code', label: 'Batch', type: 'text' },
        { key: 'expiration_date', label: 'Expiration', type: 'text' },
        { key: 'uom_code', label: 'UOM', type: 'text', readOnly: true },
        { key: 'received_quantity', label: 'Received', type: 'number', readOnly: true },
        { key: 'quantity', label: 'Return Qty', type: 'number' },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'inventory_lines':
      return [
        { key: 'product_code', label: 'Product', type: 'text', options: commercialSelectOptions('product_code', catalog) },
        { key: 'variant_signature', label: 'Variant', type: 'text' },
        { key: 'item_code', label: 'Item', type: 'text', options: commercialSelectOptions('item_code', catalog) },
        { key: 'description', label: 'Description', type: 'text' },
        { key: 'warehouse_code', label: 'Warehouse', type: 'text', options: commercialSelectOptions('warehouse_code', catalog) },
        { key: 'batch_code', label: 'Batch', type: 'text' },
        { key: 'expiration_date', label: 'Expiration', type: 'text' },
        { key: 'uom_code', label: 'UOM', type: 'text', options: commercialSelectOptions('uom_code', catalog) },
        { key: 'quantity', label: 'Quantity', type: 'number' },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'inventory_transfer_lines':
      return [
        { key: 'product_code', label: 'Product', type: 'text', options: commercialSelectOptions('product_code', catalog) },
        { key: 'variant_signature', label: 'Variant', type: 'text' },
        { key: 'item_code', label: 'Item', type: 'text', options: commercialSelectOptions('item_code', catalog) },
        { key: 'description', label: 'Description', type: 'text' },
        { key: 'source_warehouse_code', label: 'Source Warehouse', type: 'text', options: commercialSelectOptions('source_warehouse_code', catalog) },
        { key: 'target_warehouse_code', label: 'Target Warehouse', type: 'text', options: commercialSelectOptions('target_warehouse_code', catalog) },
        { key: 'batch_code', label: 'Batch', type: 'text' },
        { key: 'expiration_date', label: 'Expiration', type: 'text' },
        { key: 'uom_code', label: 'UOM', type: 'text', options: commercialSelectOptions('uom_code', catalog) },
        { key: 'quantity', label: 'Quantity', type: 'number' },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'production_component_lines':
      return [
        { key: 'component_item_code', label: 'Component Item', type: 'text', options: commercialSelectOptions('item_code', catalog) },
        { key: 'actual_item_code', label: 'Actual Item', type: 'text', options: commercialSelectOptions('item_code', catalog) },
        { key: 'description', label: 'Description', type: 'text' },
        { key: 'warehouse_code', label: 'Warehouse', type: 'text', options: commercialSelectOptions('warehouse_code', catalog) },
        { key: 'uom_code', label: 'UOM', type: 'text', options: commercialSelectOptions('uom_code', catalog) },
        { key: 'quantity_per_unit', label: 'Qty / Unit', type: 'number' },
        { key: 'quantity', label: 'Planned Qty', type: 'number' },
        { key: 'issued_quantity', label: 'Issued Qty', type: 'number', readOnly: true },
        { key: 'reserved_quantity', label: 'Reserved Qty', type: 'number', readOnly: true },
        { key: 'shortage_quantity', label: 'Shortage Qty', type: 'number', readOnly: true },
        { key: 'available_quantity', label: 'Available', type: 'number', readOnly: true },
        { key: 'allowed_substitute_item_codes', label: 'Allowed Subs', type: 'text' },
        { key: 'reservation_status', label: 'Reservation', type: 'text', readOnly: true },
        { key: 'substitution_status', label: 'Substitution', type: 'text', readOnly: true },
      ]
    case 'production_issue_lines':
      return [
        { key: 'planned_item_code', label: 'Planned Item', type: 'text', readOnly: true },
        { key: 'actual_item_code', label: 'Actual Item', type: 'text', options: commercialSelectOptions('item_code', catalog) },
        { key: 'description', label: 'Description', type: 'text' },
        { key: 'warehouse_code', label: 'Warehouse', type: 'text', options: commercialSelectOptions('warehouse_code', catalog) },
        { key: 'batch_code', label: 'Batch', type: 'text' },
        { key: 'expiration_date', label: 'Expiration', type: 'text' },
        { key: 'uom_code', label: 'UOM', type: 'text', options: commercialSelectOptions('uom_code', catalog) },
        { key: 'quantity', label: 'Issue Qty', type: 'number' },
        { key: 'reserved_quantity', label: 'Reserved Qty', type: 'number', readOnly: true },
        { key: 'available_quantity', label: 'Available', type: 'number', readOnly: true },
        { key: 'allowed_substitute_item_codes', label: 'Allowed Subs', type: 'text', readOnly: true },
        { key: 'substitution_status', label: 'Substitution', type: 'text', readOnly: true },
      ]
    case 'production_stage_lines':
      return [
        { key: 'stage_code', label: 'Stage Code', type: 'text' },
        { key: 'stage_name', label: 'Stage', type: 'text' },
        { key: 'sequence', label: 'Seq', type: 'number' },
        { key: 'work_center_code', label: 'Work Center', type: 'text', options: commercialSelectOptions('work_center_code', catalog) },
        { key: 'status', label: 'Status', type: 'text', options: [{ value: 'pending', label: 'Pending' }, { value: 'ready', label: 'Ready' }, { value: 'in_progress', label: 'In Progress' }, { value: 'completed', label: 'Completed' }, { value: 'skipped', label: 'Skipped' }] },
        { key: 'required', label: 'Required', type: 'text', options: [{ value: 'true', label: 'Yes' }, { value: 'false', label: 'No' }] },
        { key: 'note', label: 'Note', type: 'text' },
      ]
    case 'trace_batches':
      return [
        { key: 'batch_id', label: 'Batch', type: 'text', options: commercialSelectOptions('batch_id', catalog) },
        { key: 'item_code', label: 'Item', type: 'text', readOnly: true },
        { key: 'warehouse_code', label: 'Warehouse', type: 'text', readOnly: true },
        { key: 'batch_code', label: 'Batch Code', type: 'text', readOnly: true },
        { key: 'expiration_date', label: 'Expiration', type: 'text', readOnly: true },
        { key: 'status', label: 'Status', type: 'text', readOnly: true },
        { key: 'available_quantity', label: 'Available', type: 'number', readOnly: true },
      ]
    default:
      return []
  }
}

export function commercialArrayDefaultRow(widget: string): Record<string, unknown> {
  switch (widget) {
    case 'commercial_lines':
      return { product_code: '', variant_signature: '', item_code: '', description: '', uom_code: '', quantity: 1, unit_price: 0, discount_amount: 0, tax_code: '', tax_rate: 0, line_subtotal: 0, tax_amount: 0, line_total: 0 }
    case 'commercial_allocations':
      return { invoice_number: '', invoice_id: '', amount: 0, note: '' }
    case 'commercial_refund_allocations':
      return { payment_number: '', payment_id: '', amount: 0, note: '' }
    case 'commercial_journal_lines':
      return { account_code: '', description: '', debit: 0, credit: 0 }
    case 'procurement_lines':
      return { product_code: '', variant_signature: '', item_code: '', description: '', uom_code: '', quantity: 1, received_qty: 0, billed_qty: 0, unit_price: 0, tax_code: '', tax_rate: 0, line_subtotal: 0, tax_amount: 0, line_total: 0 }
    case 'procurement_receipt_lines':
      return { product_code: '', variant_signature: '', item_code: '', description: '', uom_code: '', ordered_qty: 0, received_qty: 0, cumulative_received_qty: 0, warehouse_code: '', batch_code: '', expiration_date: '', note: '' }
    case 'procurement_allocations':
      return { bill_number: '', bill_id: '', amount: 0, note: '' }
    case 'fulfillment_lines':
      return { product_code: '', variant_signature: '', item_code: '', description: '', warehouse_code: '', batch_code: '', expiration_date: '', uom_code: '', ordered_quantity: 0, quantity: 0, available_quantity: 0, note: '' }
    case 'delivery_lines':
      return { product_code: '', variant_signature: '', item_code: '', description: '', warehouse_code: '', batch_code: '', expiration_date: '', uom_code: '', fulfilled_quantity: 0, quantity: 0, tracking_number: '', note: '' }
    case 'return_lines':
      return { product_code: '', variant_signature: '', item_code: '', description: '', warehouse_code: '', batch_code: '', expiration_date: '', uom_code: '', fulfilled_quantity: 0, quantity: 0, disposition: 'restock', note: '' }
    case 'supplier_return_lines':
      return { item_code: '', description: '', warehouse_code: '', batch_code: '', expiration_date: '', uom_code: '', received_quantity: 0, quantity: 0, note: '' }
    case 'inventory_lines':
      return { product_code: '', variant_signature: '', item_code: '', description: '', warehouse_code: '', batch_code: '', expiration_date: '', uom_code: '', quantity: 0, note: '' }
    case 'inventory_transfer_lines':
      return { product_code: '', variant_signature: '', item_code: '', description: '', source_warehouse_code: '', target_warehouse_code: '', batch_code: '', expiration_date: '', uom_code: '', quantity: 0, note: '' }
    case 'production_component_lines':
      return { component_item_code: '', actual_item_code: '', description: '', warehouse_code: '', uom_code: '', quantity_per_unit: 0, quantity: 0, issued_quantity: 0, reserved_quantity: 0, shortage_quantity: 0, available_quantity: 0, allowed_substitute_item_codes: '', reservation_status: 'unreserved', substitution_status: 'planned' }
    case 'production_issue_lines':
      return { planned_item_code: '', actual_item_code: '', description: '', warehouse_code: '', batch_code: '', expiration_date: '', uom_code: '', quantity: 0, reserved_quantity: 0, available_quantity: 0, allowed_substitute_item_codes: '', substitution_status: 'planned' }
    case 'production_stage_lines':
      return { stage_code: '', stage_name: '', sequence: 1, work_center_code: '', status: 'pending', required: 'true', note: '' }
    case 'trace_batches':
      return { batch_id: '', item_code: '', warehouse_code: '', batch_code: '', expiration_date: '', status: '', on_hand_quantity: 0, reserved_quantity: 0, available_quantity: 0 }
    default:
      return {}
  }
}

export function commercialSelectOptions(path: string, catalog?: CommercialFormCatalog, values?: FormState): CommercialOption[] {
  if (!catalog) return []
  switch (path) {
    case 'product_code':
      return Object.values(catalog.productsByCode).map((item) => {
        const code = String(resolvePath(item, 'values.code') || '')
        const name = String(resolvePath(item, 'values.name') || '')
        return { value: code, label: name ? `${code} - ${name}` : code }
      }).filter((option) => option.value).sort((left, right) => left.label.localeCompare(right.label))
    case 'dimension_code':
      return Object.values(catalog.variantDimensionsByCode).map((item) => {
        const code = String(resolvePath(item, 'values.code') || '')
        const name = String(resolvePath(item, 'values.name') || '')
        return { value: code, label: name ? `${code} - ${name}` : code }
      }).filter((option) => option.value).sort((left, right) => left.label.localeCompare(right.label))
    case 'party_id':
      return Object.entries(catalog.partiesByID).filter(([value]) => value).map(([value, item]) => ({
        value,
        label: String(resolvePath(item, 'values.display_name') || resolvePath(item, 'values.name') || value),
      })).sort((left, right) => left.label.localeCompare(right.label))
    case 'tax_profile_code':
      return Object.values(catalog.taxProfilesByCode).map((item) => ({
        value: String(resolvePath(item, 'values.code') || ''),
        label: `${String(resolvePath(item, 'values.code') || '')} - ${String(resolvePath(item, 'values.name') || resolvePath(item, 'values.title') || '')}`.trim(),
      })).filter((option) => option.value).sort((left, right) => left.label.localeCompare(right.label))
    case 'category_code':
      return Object.values(catalog.itemCategoriesByCode).map((item) => {
        const code = String(resolvePath(item, 'values.code') || '')
        const name = String(resolvePath(item, 'values.name') || '')
        return { value: code, label: name ? `${code} - ${name}` : code }
      }).filter((option) => option.value).sort((left, right) => left.label.localeCompare(right.label))
    case 'uom_code':
      return Object.values(catalog.uomsByCode).map((item) => {
        const code = String(resolvePath(item, 'values.code') || '')
        const name = String(resolvePath(item, 'values.name') || '')
        const symbol = String(resolvePath(item, 'values.symbol') || '')
        return { value: code, label: [code, name, symbol].filter(Boolean).join(' - ') }
      }).filter((option) => option.value).sort((left, right) => left.label.localeCompare(right.label))
    case 'bom_id':
      return Object.values(catalog.bomsByID).map((item) => {
        const id = String(item.id || '')
        const code = String(resolvePath(item, 'values.code') || '')
        const name = String(resolvePath(item, 'values.name') || '')
        return { value: id, label: [code, name].filter(Boolean).join(' - ') || id }
      }).filter((option) => option.value).sort((left, right) => left.label.localeCompare(right.label))
    case 'bom_version_id':
      return Object.values(catalog.bomVersionsByID).map((item) => {
        const id = String(item.id || '')
        const bomCode = String(resolvePath(item, 'values.bom_code') || '')
        const versionCode = String(resolvePath(item, 'values.version_code') || '')
        return { value: id, label: [bomCode, versionCode].filter(Boolean).join(' - ') || id }
      }).filter((option) => option.value).sort((left, right) => left.label.localeCompare(right.label))
    case 'warehouse_code':
    case 'source_warehouse_code':
    case 'target_warehouse_code':
      return Object.values(catalog.warehousesByCode).map((item) => {
        const code = String(resolvePath(item, 'values.code') || '')
        const name = String(resolvePath(item, 'values.name') || '')
        return { value: code, label: name ? `${code} - ${name}` : code }
      }).filter((option) => option.value).sort((left, right) => left.label.localeCompare(right.label))
    case 'work_center_code':
      return Object.values(catalog.workCentersByCode).map((item) => {
        const code = String(resolvePath(item, 'values.code') || '')
        const name = String(resolvePath(item, 'values.name') || '')
        return { value: code, label: name ? `${code} - ${name}` : code }
      }).filter((option) => option.value).sort((left, right) => left.label.localeCompare(right.label))
    case 'batch_id':
      return Object.values(catalog.inventoryBatchesByID).map((item) => {
        const id = String(resolvePath(item, 'id') || '')
        const itemCode = String(resolvePath(item, 'values.item_code') || '')
        const warehouseCode = String(resolvePath(item, 'values.warehouse_code') || '')
        const batchCode = String(resolvePath(item, 'values.batch_code') || '')
        return { value: id, label: [itemCode, warehouseCode, batchCode].filter(Boolean).join(' - ') || id }
      }).filter((option) => option.value).sort((left, right) => left.label.localeCompare(right.label))
    case 'default_price_list_code':
    case 'price_list_code':
      return Object.values(catalog.priceListsByCode).map((item) => {
        const code = String(resolvePath(item, 'values.code') || '')
        const name = String(resolvePath(item, 'values.name') || '')
        return { value: code, label: name ? `${code} - ${name}` : code }
      }).filter((option) => option.value).sort((left, right) => left.label.localeCompare(right.label))
    case 'default_tax_code':
    case 'tax_code':
      return Object.values(catalog.taxCodesByCode).map((item) => {
        const code = String(resolvePath(item, 'values.code') || '')
        const name = String(resolvePath(item, 'values.name') || '')
        return { value: code, label: name ? `${code} - ${name}` : code }
      }).filter((option) => option.value).sort((left, right) => left.label.localeCompare(right.label))
    case 'payment_method_code':
      return Object.values(catalog.paymentMethodsByCode).map((item) => {
        const code = String(resolvePath(item, 'values.code') || '')
        const name = String(resolvePath(item, 'values.name') || '')
        return { value: code, label: name ? `${code} - ${name}` : code }
      }).filter((option) => option.value).sort((left, right) => left.label.localeCompare(right.label))
    case 'source_payment_id':
      return commercialRefundablePayments(catalog, values).map((item) => {
        const id = String(resolvePath(item, 'header.id') || '')
        const number = String(resolvePath(item, 'header.number') || id)
        const method = String(resolvePath(item, 'body.payload.payment_method_code') || '')
        const amount = toNumber(resolvePath(item, 'body.payload.amount_received'))
        const refunded = toNumber(resolvePath(item, 'body.payload.refunded_amount'))
        const remaining = roundMoney(Math.max(amount - refunded, 0))
        return { value: id, label: `${number}${method ? ` - ${humanize(method)}` : ''}${remaining > 0 ? ` (${remaining})` : ''}` }
      }).filter((option) => option.value).sort((left, right) => left.label.localeCompare(right.label))
    case 'vendor_id':
      return Object.entries(catalog.vendorsByID).filter(([value]) => value).map(([value, item]) => ({
        value,
        label: String(resolvePath(item, 'values.vendor_name') || value),
      })).sort((left, right) => left.label.localeCompare(right.label))
    case 'bill_id':
      return procurementOpenBills(catalog, values).map((item) => {
        const id = String(resolvePath(item, 'header.id') || '')
        const number = String(resolvePath(item, 'header.number') || id)
        const vendorName = String(resolvePath(item, 'body.payload.vendor_name') || '')
        const balance = toNumber(resolvePath(item, 'body.payload.balance_due_amount'))
        return { value: id, label: `${number}${vendorName ? ` - ${vendorName}` : ''}${balance > 0 ? ` (${balance})` : ''}` }
      }).filter((option) => option.value).sort((left, right) => left.label.localeCompare(right.label))
    case 'invoice_id':
      return commercialOpenInvoices(catalog, values).map((item) => {
        const id = String(resolvePath(item, 'header.id') || '')
        const number = String(resolvePath(item, 'header.number') || id)
        const partyName = String(resolvePath(item, 'body.payload.party_name') || '')
        const balance = toNumber(resolvePath(item, 'body.payload.balance_due_amount'))
        return { value: id, label: `${number}${partyName ? ` - ${partyName}` : ''}${balance > 0 ? ` (${balance})` : ''}` }
      }).filter((option) => option.value).sort((left, right) => left.label.localeCompare(right.label))
    case 'item_code':
    case 'finished_item_code':
      return Object.values(catalog.itemsByCode).map((item) => {
        const code = String(resolvePath(item, 'values.sku') || '')
        const name = String(resolvePath(item, 'values.name') || '')
        const variantLabel = String(resolvePath(item, 'values.variant_label') || '')
        return { value: code, label: [code, name, variantLabel].filter(Boolean).join(' - ') }
      }).filter((option) => option.value).sort((left, right) => left.label.localeCompare(right.label))
    default:
      return []
  }
}

export function resolveCommercialColumnOptions(
  key: string,
  _widget: string,
  row: Record<string, unknown>,
  catalog?: CommercialFormCatalog,
  values?: FormState,
  fallback?: CommercialOption[],
): CommercialOption[] {
  if (!catalog) return fallback || []
  if (key === 'variant_signature') return variantSignatureOptions(String(row.product_code || ''), catalog)
  return fallback || commercialSelectOptions(key, catalog, values)
}

function variantSignatureOptions(productCode: string, catalog?: CommercialFormCatalog): CommercialOption[] {
  if (!catalog || !productCode) return []
  return Object.values(catalog.itemsByCode)
    .filter((item) => String(resolvePath(item, 'values.product_code') || '') === productCode)
    .map((item) => ({
      value: String(resolvePath(item, 'values.variant_signature') || ''),
      label: String(resolvePath(item, 'values.variant_label') || resolvePath(item, 'values.variant_signature') || resolvePath(item, 'values.sku') || ''),
    }))
    .filter((option) => option.value)
    .sort((left, right) => left.label.localeCompare(right.label))
}

export function resolveVariantItemCodeFromRow(row: Record<string, unknown>, catalog?: CommercialFormCatalog): string {
  if (!catalog) return String(row.item_code || '')
  const productCode = String(row.product_code || '')
  const variantSignature = String(row.variant_signature || '')
  if (!productCode || !variantSignature) return String(row.item_code || '')
  const match = Object.values(catalog.itemsByCode).find((item) =>
    String(resolvePath(item, 'values.product_code') || '') === productCode &&
    String(resolvePath(item, 'values.variant_signature') || '') === variantSignature,
  )
  return String(resolvePath(match, 'values.sku') || row.item_code || '')
}

export function parseDimensionCodes(value: unknown): string[] {
  return String(value || '').split(',').map((item) => item.trim()).filter(Boolean)
}

export function hasUnresolvedVariantSelections(values: FormState): boolean {
  for (const key of ['lines', 'allocations', 'refund_allocations']) {
    const rows = asRecordList(resolvePath(values, key))
    if (rows.some((row) => String(row.product_code || '') !== '' && String(row.item_code || '') === '')) return true
  }
  return false
}

export function commercialOpenInvoices(catalog?: CommercialFormCatalog, values?: FormState): Array<Record<string, unknown>> {
  if (!catalog) return []
  const partyID = String(resolvePath(values || {}, 'party_id') || '')
  return Object.values(catalog.invoicesByID).filter((item) => {
    const balance = toNumber(resolvePath(item, 'body.payload.balance_due_amount'))
    if (balance <= 0) return false
    return !partyID || String(resolvePath(item, 'body.payload.party_id') || '') === partyID
  }).sort((left, right) => {
    const leftDue = String(resolvePath(left, 'body.payload.due_date') || '')
    const rightDue = String(resolvePath(right, 'body.payload.due_date') || '')
    if (leftDue !== rightDue) return leftDue.localeCompare(rightDue)
    return String(resolvePath(left, 'header.number') || '').localeCompare(String(resolvePath(right, 'header.number') || ''))
  })
}

export function commercialRefundablePayments(catalog?: CommercialFormCatalog, values?: FormState): Array<Record<string, unknown>> {
  if (!catalog) return []
  const invoiceID = String(resolvePath(values || {}, 'source_invoice_id') || '')
  if (!invoiceID) return []
  return Object.values(catalog.paymentsByID).filter((item) => {
    if (String(resolvePath(item, 'header.status') || '') !== 'received') return false
    const amount = toNumber(resolvePath(item, 'body.payload.amount_received'))
    const refunded = toNumber(resolvePath(item, 'body.payload.refunded_amount'))
    if (roundMoney(Math.max(amount - refunded, 0)) <= 0) return false
    const links = (resolvePath(item, 'links') as Array<Record<string, unknown>> | undefined) || []
    return links.some((link) => String(link.link_type || '') === 'payment_for' && String(link.linked_document_id || '') === invoiceID)
  }).sort((left, right) => {
    const leftDate = String(resolvePath(left, 'body.payload.receipt_date') || '')
    const rightDate = String(resolvePath(right, 'body.payload.receipt_date') || '')
    if (leftDate !== rightDate) return leftDate.localeCompare(rightDate)
    return String(resolvePath(left, 'header.number') || '').localeCompare(String(resolvePath(right, 'header.number') || ''))
  })
}

export function procurementOpenBills(catalog?: CommercialFormCatalog, values?: FormState): Array<Record<string, unknown>> {
  if (!catalog) return []
  const vendorID = String(resolvePath(values || {}, 'vendor_id') || '')
  return Object.values(catalog.billsByID).filter((item) => {
    const balance = toNumber(resolvePath(item, 'body.payload.balance_due_amount'))
    if (balance <= 0) return false
    return !vendorID || String(resolvePath(item, 'body.payload.vendor_id') || '') === vendorID
  }).sort((left, right) => {
    const leftDue = String(resolvePath(left, 'body.payload.due_date') || '')
    const rightDue = String(resolvePath(right, 'body.payload.due_date') || '')
    if (leftDue !== rightDue) return leftDue.localeCompare(rightDue)
    return String(resolvePath(left, 'header.number') || '').localeCompare(String(resolvePath(right, 'header.number') || ''))
  })
}

export function buildAllocationRows(invoices: Array<Record<string, unknown>>, requestedAmount: number): { rows: Array<Record<string, unknown>>; allocatedAmount: number } {
  let remaining = roundMoney(requestedAmount)
  const allocateAll = remaining <= 0
  const rows: Array<Record<string, unknown>> = []
  let allocatedAmount = 0
  for (const invoice of invoices) {
    const balance = roundMoney(toNumber(resolvePath(invoice, 'body.payload.balance_due_amount')))
    if (balance <= 0) continue
    const allocationAmount = allocateAll ? balance : roundMoney(Math.min(balance, remaining))
    if (allocationAmount <= 0) continue
    rows.push({ invoice_id: String(resolvePath(invoice, 'header.id') || ''), invoice_number: String(resolvePath(invoice, 'header.number') || ''), amount: allocationAmount, note: '' })
    allocatedAmount = roundMoney(allocatedAmount + allocationAmount)
    if (!allocateAll) {
      remaining = roundMoney(Math.max(remaining - allocationAmount, 0))
      if (remaining <= 0) break
    }
  }
  return { rows, allocatedAmount }
}

export function buildRefundAllocationRows(payments: Array<Record<string, unknown>>, requestedAmount: number): { rows: Array<Record<string, unknown>>; allocatedAmount: number } {
  let remaining = roundMoney(requestedAmount)
  const allocateAll = remaining <= 0
  const rows: Array<Record<string, unknown>> = []
  let allocatedAmount = 0
  for (const payment of payments) {
    const amount = toNumber(resolvePath(payment, 'body.payload.amount_received'))
    const refunded = toNumber(resolvePath(payment, 'body.payload.refunded_amount'))
    const balance = roundMoney(Math.max(amount - refunded, 0))
    if (balance <= 0) continue
    const allocationAmount = allocateAll ? balance : roundMoney(Math.min(balance, remaining))
    if (allocationAmount <= 0) continue
    rows.push({ payment_id: String(resolvePath(payment, 'header.id') || ''), payment_number: String(resolvePath(payment, 'header.number') || ''), amount: allocationAmount, note: '' })
    allocatedAmount = roundMoney(allocatedAmount + allocationAmount)
    if (!allocateAll) {
      remaining = roundMoney(Math.max(remaining - allocationAmount, 0))
      if (remaining <= 0) break
    }
  }
  return { rows, allocatedAmount }
}

export function buildProcurementAllocationRows(bills: Array<Record<string, unknown>>, requestedAmount: number): { rows: Array<Record<string, unknown>>; allocatedAmount: number } {
  let remaining = roundMoney(requestedAmount)
  const allocateAll = remaining <= 0
  const rows: Array<Record<string, unknown>> = []
  let allocatedAmount = 0
  for (const bill of bills) {
    const balance = roundMoney(toNumber(resolvePath(bill, 'body.payload.balance_due_amount')))
    if (balance <= 0) continue
    const allocationAmount = allocateAll ? balance : roundMoney(Math.min(balance, remaining))
    if (allocationAmount <= 0) continue
    rows.push({ bill_id: String(resolvePath(bill, 'header.id') || ''), bill_number: String(resolvePath(bill, 'header.number') || ''), amount: allocationAmount, note: '' })
    allocatedAmount = roundMoney(allocatedAmount + allocationAmount)
    if (!allocateAll) {
      remaining = roundMoney(Math.max(remaining - allocationAmount, 0))
      if (remaining <= 0) break
    }
  }
  return { rows, allocatedAmount }
}
