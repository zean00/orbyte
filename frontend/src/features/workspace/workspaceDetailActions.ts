import type { ActionDefinition } from '@/services/bootstrap'

type ToastVariant = 'default' | 'success' | 'warning' | 'error'

type DetailActionContext = {
  header: Record<string, unknown>
  actionKey: string
  routeActions: ActionDefinition[]
  onNavigate: (target: string) => void
  onToast: (message: string, variant?: ToastVariant) => void
  onReload: () => void
  onRequireStepUp: (actionKey: string) => void
  resolvePath: (payload: Record<string, unknown>, path: string) => unknown
  routeForDocument: (documentType: string, kind: 'detail' | 'form', actions: ActionDefinition[], currentPath?: string) => string
  invokeCommercialAction: (url: string) => Promise<Record<string, unknown>>
  invokeDocumentAction: (documentID: string, action: string) => Promise<void>
}

type GeneratedActionConfig = {
  type: string
  actionKey: string
  path: (id: string) => string
  successMessage: string | ((created: Record<string, unknown>) => string)
  errorMessage: string
  targetDocumentType: string
  targetBasePath: string
  createdIDPath?: string
  collectionItemsPath?: string
}

const GENERATED_ACTIONS: GeneratedActionConfig[] = [
  {
    type: 'sales_fulfillment',
    actionKey: 'register_return',
    path: (id) => `/returns/fulfillments/${encodeURIComponent(id)}/register-return`,
    successMessage: 'Return draft generated.',
    errorMessage: 'Return generation failed',
    targetDocumentType: 'sales_return',
    targetBasePath: '/returns/returns',
  },
  {
    type: 'sales_fulfillment',
    actionKey: 'register_delivery',
    path: (id) => `/delivery/fulfillments/${encodeURIComponent(id)}/register-delivery`,
    successMessage: 'Delivery draft generated.',
    errorMessage: 'Delivery generation failed',
    targetDocumentType: 'delivery_order',
    targetBasePath: '/delivery/orders',
  },
  {
    type: 'sales_order',
    actionKey: 'generate_fulfillment',
    path: (id) => `/commercial/orders/${encodeURIComponent(id)}/generate-fulfillment`,
    successMessage: 'Fulfillment draft generated.',
    errorMessage: 'Fulfillment generation failed',
    targetDocumentType: 'sales_fulfillment',
    targetBasePath: '/fulfillment/fulfillments',
  },
  {
    type: 'sales_order',
    actionKey: 'generate_production_order',
    path: (id) => `/commercial/orders/${encodeURIComponent(id)}/generate-production-order`,
    successMessage: (created) => {
      const items = asRecordList(created.items)
      return items.length > 1 ? `${items.length} production orders generated.` : 'Production order draft generated.'
    },
    errorMessage: 'Production order generation failed',
    targetDocumentType: 'production_order',
    targetBasePath: '/production/orders',
    collectionItemsPath: 'items',
  },
  {
    type: 'sales_order',
    actionKey: 'generate_invoice',
    path: (id) => `/commercial/orders/${encodeURIComponent(id)}/generate-invoice`,
    successMessage: 'Invoice generated.',
    errorMessage: 'Invoice generation failed',
    targetDocumentType: 'invoice',
    targetBasePath: '/commercial/invoices',
  },
  {
    type: 'invoice',
    actionKey: 'register_payment',
    path: (id) => `/commercial/invoices/${encodeURIComponent(id)}/register-payment`,
    successMessage: 'Payment draft generated.',
    errorMessage: 'Payment registration failed',
    targetDocumentType: 'payment_receipt',
    targetBasePath: '/commercial/payments',
  },
  {
    type: 'invoice',
    actionKey: 'issue_credit_note',
    path: (id) => `/commercial/invoices/${encodeURIComponent(id)}/issue-credit-note`,
    successMessage: 'Credit note draft generated.',
    errorMessage: 'Credit note generation failed',
    targetDocumentType: 'credit_note',
    targetBasePath: '/commercial/credit-notes',
  },
  {
    type: 'credit_note',
    actionKey: 'register_refund',
    path: (id) => `/commercial/credit-notes/${encodeURIComponent(id)}/register-refund`,
    successMessage: 'Refund draft generated.',
    errorMessage: 'Refund generation failed',
    targetDocumentType: 'payment_refund',
    targetBasePath: '/commercial/refunds',
  },
  {
    type: 'sales_return',
    actionKey: 'register_return_receipt',
    path: (id) => `/returns/returns/${encodeURIComponent(id)}/register-receipt`,
    successMessage: 'Return receipt draft generated.',
    errorMessage: 'Return receipt generation failed',
    targetDocumentType: 'return_receipt',
    targetBasePath: '/returns/receipts',
  },
  {
    type: 'sales_return',
    actionKey: 'issue_credit_note',
    path: (id) => `/returns/returns/${encodeURIComponent(id)}/issue-credit-note`,
    successMessage: 'Credit note draft generated.',
    errorMessage: 'Credit note generation failed',
    targetDocumentType: 'credit_note',
    targetBasePath: '/commercial/credit-notes',
  },
  {
    type: 'sales_return',
    actionKey: 'register_refund',
    path: (id) => `/returns/returns/${encodeURIComponent(id)}/register-refund`,
    successMessage: 'Refund draft generated.',
    errorMessage: 'Refund generation failed',
    targetDocumentType: 'payment_refund',
    targetBasePath: '/commercial/refunds',
  },
  {
    type: 'sales_return',
    actionKey: 'create_replacement_order',
    path: (id) => `/returns/returns/${encodeURIComponent(id)}/create-replacement-order`,
    successMessage: 'Replacement order draft generated.',
    errorMessage: 'Replacement order generation failed',
    targetDocumentType: 'sales_order',
    targetBasePath: '/commercial/orders',
  },
  {
    type: 'purchase_request',
    actionKey: 'generate_purchase_order',
    path: (id) => `/procurement/requests/${encodeURIComponent(id)}/generate-purchase-order`,
    successMessage: 'Purchase order draft generated.',
    errorMessage: 'Purchase order generation failed',
    targetDocumentType: 'purchase_order',
    targetBasePath: '/procurement/orders',
  },
  {
    type: 'purchase_order',
    actionKey: 'register_receipt',
    path: (id) => `/procurement/orders/${encodeURIComponent(id)}/register-receipt`,
    successMessage: 'Goods receipt draft generated.',
    errorMessage: 'Receipt registration failed',
    targetDocumentType: 'goods_receipt',
    targetBasePath: '/procurement/receipts',
  },
  {
    type: 'production_order',
    actionKey: 'register_production_issue',
    path: (id) => `/production/orders/${encodeURIComponent(id)}/register-issue`,
    successMessage: 'Production issue draft generated.',
    errorMessage: 'Production issue generation failed',
    targetDocumentType: 'production_issue',
    targetBasePath: '/production/issues',
  },
  {
    type: 'production_order',
    actionKey: 'register_production_output',
    path: (id) => `/production/orders/${encodeURIComponent(id)}/register-output`,
    successMessage: 'Production output draft generated.',
    errorMessage: 'Production output generation failed',
    targetDocumentType: 'production_output',
    targetBasePath: '/production/outputs',
  },
  {
    type: 'purchase_order',
    actionKey: 'register_vendor_bill',
    path: (id) => `/procurement/orders/${encodeURIComponent(id)}/register-vendor-bill`,
    successMessage: 'Vendor bill draft generated.',
    errorMessage: 'Vendor bill generation failed',
    targetDocumentType: 'vendor_bill',
    targetBasePath: '/procurement/bills',
  },
  {
    type: 'goods_receipt',
    actionKey: 'register_vendor_bill',
    path: (id) => `/procurement/receipts/${encodeURIComponent(id)}/register-vendor-bill`,
    successMessage: 'Vendor bill draft generated.',
    errorMessage: 'Vendor bill generation failed',
    targetDocumentType: 'vendor_bill',
    targetBasePath: '/procurement/bills',
  },
  {
    type: 'goods_receipt',
    actionKey: 'register_supplier_return',
    path: (id) => `/procurement/receipts/${encodeURIComponent(id)}/register-supplier-return`,
    successMessage: 'Supplier return draft generated.',
    errorMessage: 'Supplier return generation failed',
    targetDocumentType: 'supplier_return',
    targetBasePath: '/supplier-returns/returns',
  },
  {
    type: 'vendor_bill',
    actionKey: 'register_payment_out',
    path: (id) => `/procurement/bills/${encodeURIComponent(id)}/register-payment`,
    successMessage: 'Payment-out draft generated.',
    errorMessage: 'Payment registration failed',
    targetDocumentType: 'payment_out',
    targetBasePath: '/procurement/payments',
  },
  {
    type: 'vendor_bill',
    actionKey: 'issue_vendor_credit_note',
    path: (id) => `/procurement/bills/${encodeURIComponent(id)}/issue-credit-note`,
    successMessage: 'Vendor credit draft generated.',
    errorMessage: 'Vendor credit generation failed',
    targetDocumentType: 'vendor_credit_note',
    targetBasePath: '/procurement/credits',
  },
  {
    type: 'vendor_bill',
    actionKey: 'register_supplier_return',
    path: (id) => `/procurement/bills/${encodeURIComponent(id)}/register-supplier-return`,
    successMessage: 'Supplier return draft generated.',
    errorMessage: 'Supplier return generation failed',
    targetDocumentType: 'supplier_return',
    targetBasePath: '/supplier-returns/returns',
  },
  {
    type: 'supplier_return',
    actionKey: 'issue_vendor_credit_note',
    path: (id) => `/supplier-returns/returns/${encodeURIComponent(id)}/issue-vendor-credit`,
    successMessage: 'Vendor credit draft generated.',
    errorMessage: 'Vendor credit generation failed',
    targetDocumentType: 'vendor_credit_note',
    targetBasePath: '/procurement/credits',
  },
]

export async function handleWorkspaceDetailAction(context: DetailActionContext): Promise<void> {
  const documentType = String(context.header.type || '')
  const documentID = String(context.header.id || '')

  const generatedConfig = GENERATED_ACTIONS.find(
    (item) => item.type === documentType && item.actionKey === context.actionKey,
  )

  if (generatedConfig && documentID) {
    try {
      const created = await context.invokeCommercialAction(generatedConfig.path(documentID))
      const message =
        typeof generatedConfig.successMessage === 'function'
          ? generatedConfig.successMessage(created)
          : generatedConfig.successMessage
      context.onToast(message, 'success')

      const target = context.routeForDocument(
        generatedConfig.targetDocumentType,
        'detail',
        context.routeActions,
        generatedConfig.targetBasePath,
      )
      const createdID = generatedConfig.collectionItemsPath
        ? context.resolvePath(asRecordList(context.resolvePath(created, generatedConfig.collectionItemsPath))[0] || {}, 'header.id')
        : context.resolvePath(created, generatedConfig.createdIDPath || 'header.id')
      if (target && createdID) {
        context.onNavigate(`${target}?id=${encodeURIComponent(String(createdID))}`)
        return
      }
      context.onReload()
    } catch (error) {
      context.onToast(error instanceof Error ? error.message : generatedConfig.errorMessage, 'error')
    }
    return
  }

  try {
    await context.invokeDocumentAction(documentID, context.actionKey)
    context.onToast(`Action ${context.actionKey} applied`, 'success')
    context.onReload()
  } catch (error) {
    const status = typeof error === 'object' && error && 'status' in error ? Number((error as { status?: number }).status || 0) : 0
    const message = error instanceof Error ? error.message : 'Action failed'
    if (status === 403 && /step-up verification required/i.test(message) && (context.actionKey === 'approve' || context.actionKey === 'reject')) {
      context.onRequireStepUp(context.actionKey)
      return
    }
    context.onToast(message, 'error')
  }
}

function asRecordList(value: unknown): Array<Record<string, unknown>> {
  return Array.isArray(value) ? (value as Array<Record<string, unknown>>) : []
}
