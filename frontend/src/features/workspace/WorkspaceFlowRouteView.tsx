import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useToast } from '@/components/ui/Toast'
import { pickText, type DocumentFlowDefinition, type DocumentFlowStepDefinition, type FieldDefinition, type SectionDefinition } from '@/services/bootstrap'
import { useShellStore } from '@/stores/shellStore'
import { emptyCommercialFormCatalog, type FormState, type ValidationErrors } from './workspaceFormTypes'
import type { RouteResolution } from './workspaceTypes'

export function WorkspaceFlowRouteView({
  route,
  locale,
  fetchJSON,
  readCookie,
  buildError,
  routeForDocument,
  stripEditorSuffix,
  resolvePath,
  collectFlowFields,
  validateFieldCollection,
  validateFieldInput,
  normalizeFieldPath,
  resolveFlowSequence,
  renderPanel,
  renderFieldEditor,
  validationFieldKey,
}: {
  route: RouteResolution
  locale: string
  fetchJSON: <T>(url: string) => Promise<T>
  readCookie: (name: string) => string
  buildError: (response: Response) => Promise<Error>
  routeForDocument: (documentType: string, kind: 'detail' | 'form', actions: ReturnType<typeof useShellStore.getState>['actions'], currentPath?: string) => string
  stripEditorSuffix: (path: string) => string
  resolvePath: (payload: unknown, path: string) => unknown
  collectFlowFields: (doc: { fields?: FieldDefinition[]; sections?: SectionDefinition[]; tabs?: Array<{ sections?: SectionDefinition[] }> }) => FieldDefinition[]
  validateFieldCollection: (fields: FieldDefinition[], values: FormState, model: boolean, locale: string, scope?: string) => ValidationErrors
  validateFieldInput: (field: FieldDefinition, value: unknown, locale: string) => string
  normalizeFieldPath: (field: FieldDefinition, model: boolean) => string
  resolveFlowSequence: (flow: DocumentFlowDefinition, draft: Record<string, { payload: FormState }>) => DocumentFlowStepDefinition[]
  renderPanel: (args: { title: string; status?: string; children: JSX.Element }) => JSX.Element
  renderFieldEditor: (args: {
    field: FieldDefinition
    locale: string
    values: FormState
    onChange: React.Dispatch<React.SetStateAction<FormState>>
    model: boolean
    catalog: ReturnType<typeof emptyCommercialFormCatalog>
    error?: string
    onBlur?: () => void
  }) => JSX.Element
  validationFieldKey: (scope: string, fieldKey: string) => string
}) {
  const flow = route.flow!
  const navigate = useNavigate()
  const { addToast } = useToast()
  const documentID = new URLSearchParams(window.location.search).get('id') || ''
  const activeParam = new URLSearchParams(window.location.search).get('document_key') || ''
  const [draft, setDraft] = useState<Record<string, { payload: FormState }>>({})
  const [stepIndex, setStepIndex] = useState(0)
  const [activeDocKey, setActiveDocKey] = useState(activeParam)
  const [errors, setErrors] = useState<ValidationErrors>({})

  useEffect(() => {
    let mounted = true
    async function load() {
      if (!documentID) return
      const payload = await fetchJSON<Record<string, unknown>>(`/ui/data/documents/${encodeURIComponent(documentID)}`)
      if (!mounted) return
      const instance = payload.flow_instance as Record<string, unknown> | undefined
      const nextDraft: Record<string, { payload: FormState }> = {}
      const items = (instance?.items || []) as Array<Record<string, unknown>>
      for (const item of items) {
        const definition = item.definition as Record<string, unknown>
        const record = item.record as Record<string, unknown>
        const body = (record?.body || {}) as Record<string, unknown>
        nextDraft[String(definition?.key || '')] = { payload: ((body.payload as FormState) || {}) }
      }
      setDraft(nextDraft)
      setActiveDocKey(String(instance?.active_document_key || activeParam || ''))
      setErrors({})
    }
    void load()
    return () => {
      mounted = false
    }
  }, [activeParam, documentID, fetchJSON])

  const steps = useMemo(() => resolveFlowSequence(flow, draft), [draft, flow, resolveFlowSequence])
  const currentStep = steps[stepIndex] || steps[0]
  const currentDocKey = activeDocKey || currentStep?.documents?.[0]?.key || ''

  function validateCurrentStep(): boolean {
    const nextErrors: ValidationErrors = {}
    for (const doc of currentStep?.documents || []) {
      const docErrors = validateFieldCollection(collectFlowFields(doc), draft[doc.key]?.payload || {}, false, locale, doc.key)
      Object.assign(nextErrors, docErrors)
    }
    setErrors(nextErrors)
    return Object.keys(nextErrors).length === 0
  }

  async function handleCommit() {
    const response = await fetch(`/document-flows/${encodeURIComponent(flow.key)}/commit`, {
      method: 'POST',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': readCookie('orbyte_csrf'),
      },
      body: JSON.stringify({
        organization_id: 'org_default',
        primary_document_id: documentID,
        documents: Object.fromEntries(Object.entries(draft).map(([key, value]) => [key, value.payload])),
      }),
    })
    if (!response.ok) throw await buildError(response)
    const payload = await response.json()
    const target = routeForDocument(payload.primary_document_type, 'detail', useShellStore.getState().actions, route.requested_path)
    if (target && payload.primary_document_id) {
      navigate(`${target}?id=${encodeURIComponent(payload.primary_document_id)}`, { replace: true })
    }
  }

  if (!currentStep) {
    return renderPanel({
      title: pickText(flow, 'title', locale) || 'Flow',
      status: 'No flow steps are available.',
      children: <></>,
    })
  }

  const cancelTarget = documentID
    ? `${routeForDocument(flow.primary_document_type, 'detail', useShellStore.getState().actions, route.requested_path)}?id=${encodeURIComponent(documentID)}`
    : stripEditorSuffix(route.requested_path) || route.fallback_path || '/documents'

  return renderPanel({
    title: pickText(flow, 'title', locale) || 'Flow',
    status: pickText(currentStep, 'title', locale),
    children: (
      <>
        <div className="mb-4 flex flex-wrap gap-2">
          {steps.map((step, index) => {
            return (
              <button
                key={step.key}
                onClick={() => setStepIndex(index)}
                className={`rounded-lg px-3 py-2 text-sm ${index === stepIndex ? 'bg-accent text-white' : 'border border-line text-body'}`}
              >
                {pickText(step, 'title', locale) || step.key}
              </button>
            )
          })}
        </div>
        {currentStep.documents && currentStep.documents.length > 1 ? (
          <div className="mb-4 flex gap-2">
            {currentStep.documents.map((doc) => (
              <button
                key={doc.key}
                onClick={() => setActiveDocKey(doc.key)}
                className={`rounded-lg px-3 py-2 text-sm ${currentDocKey === doc.key ? 'bg-accent text-white' : 'border border-line text-body'}`}
              >
                {pickText(doc, 'title', locale) || doc.key}
              </button>
            ))}
          </div>
        ) : null}
        <div className="space-y-4">
          {(currentStep.documents || [])
            .filter((doc) => doc.key === currentDocKey)
            .map((doc) => (
              <section key={doc.key} className="rounded-xl border border-line p-4">
                <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">{pickText(doc, 'title', locale) || doc.key}</h2>
                <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                  {collectFlowFields(doc).map((field) => renderFieldEditor({
                    field,
                    locale,
                    values: draft[doc.key]?.payload || {},
                    onChange: (updater) =>
                      setDraft((current) => ({
                        ...current,
                        [doc.key]: {
                          payload: typeof updater === 'function'
                            ? (updater as (state: FormState) => FormState)(current[doc.key]?.payload || {})
                            : updater,
                        },
                      })),
                    model: false,
                    catalog: emptyCommercialFormCatalog(),
                    error: errors[validationFieldKey(doc.key, field.key)],
                    onBlur: () =>
                      setErrors((current) => {
                        const message = validateFieldInput(field, resolvePath(draft[doc.key]?.payload || {}, normalizeFieldPath(field, false)), locale)
                        const key = validationFieldKey(doc.key, field.key)
                        if (!message && !current[key]) return current
                        const next = { ...current }
                        if (message) next[key] = message
                        else delete next[key]
                        return next
                      }),
                  }))}
                </div>
              </section>
            ))}
        </div>
        <div className="mt-6 flex gap-3">
          <button onClick={() => navigate(cancelTarget, { replace: true })} className="rounded-lg border border-line px-4 py-2 text-body">
            Cancel
          </button>
          <button
            disabled={stepIndex === 0}
            onClick={() => setStepIndex((current) => Math.max(0, current - 1))}
            className="rounded-lg border border-line px-4 py-2 text-body disabled:opacity-50"
          >
            Previous
          </button>
          <button
            onClick={() => {
              if (!validateCurrentStep()) {
                addToast({ message: 'Please fix the highlighted fields.', variant: 'warning' })
                return
              }
              if (stepIndex >= steps.length - 1) {
                void handleCommit().catch((error) => addToast({ message: error instanceof Error ? error.message : 'Save failed', variant: 'error' }))
                return
              }
              setStepIndex((current) => Math.min(steps.length - 1, current + 1))
            }}
            className="rounded-lg bg-accent px-4 py-2 text-white"
          >
            {stepIndex >= steps.length - 1 ? (documentID ? 'Save' : 'Create') : 'Next'}
          </button>
        </div>
      </>
    ),
  })
}
