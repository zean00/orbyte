import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { mutateJson } from './adminClient'

type TargetOption = {
  key: string
  title?: string
}

type TemplateTargetCatalog = {
  documents?: TargetOption[]
  reports?: TargetOption[]
}

export function AdminTemplateListPage({
  rows,
  renderDataGrid,
}: {
  rows: Array<Record<string, unknown>>
  renderDataGrid: (args: {
    columns: Array<{ key: string; label: string }>
    rows: Array<Record<string, unknown>>
    actionLabel?: string
    onAction?: (row: Record<string, unknown>) => void
    secondaryActionLabel?: string
    onSecondaryAction?: (row: Record<string, unknown>) => void
  }) => JSX.Element
}) {
  const navigate = useNavigate()
  const [targetCatalog, setTargetCatalog] = useState<TemplateTargetCatalog>({})
  const [draft, setDraft] = useState({
    key: '',
    title: '',
    targetKind: 'document',
    targetKey: '',
    purpose: '',
  })
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')

  useEffect(() => {
    let mounted = true
    async function loadCatalog() {
      try {
        const payload = await fetch('/admin/api/template-targets', {
          credentials: 'include',
        })
        const data = (await payload.json()) as TemplateTargetCatalog
        if (!mounted) return
        setTargetCatalog(data || {})
        setDraft((current) => ({
          ...current,
          targetKey:
            current.targetKey ||
            (current.targetKind === 'report'
              ? data?.reports?.[0]?.key || ''
              : data?.documents?.[0]?.key || ''),
        }))
      } catch {
        if (!mounted) return
      }
    }
    void loadCatalog()
    return () => {
      mounted = false
    }
  }, [])

  const targetOptions = draft.targetKind === 'report' ? targetCatalog.reports || [] : targetCatalog.documents || []

  async function createTemplate() {
    setBusy(true)
    setMessage('')
    try {
      await mutateJson('/admin/api/templates/definitions', {
        method: 'POST',
        body: JSON.stringify({
          key: draft.key,
          title: draft.title,
          target_kind: draft.targetKind,
          target_key: draft.targetKey,
          renderer_kind: 'visual',
          default_format: 'html',
          purpose: draft.purpose,
        }),
      })
      navigate(`/templates/designer?key=${encodeURIComponent(draft.key)}`)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to create template.')
    } finally {
      setBusy(false)
    }
  }

  async function deleteTemplate(row: Record<string, unknown>) {
    const key = String(resolvePath(row, 'key') || '')
    if (!key) return
    setBusy(true)
    setMessage('')
    try {
      await mutateJson(`/admin/api/templates/definitions/${encodeURIComponent(key)}`, {
        method: 'DELETE',
      })
      navigate(0)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to delete template.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-line bg-accent-soft/60 p-4 text-sm text-body">
        Select a template to open the visual designer, preview drafts, and use advanced body or style editing.
      </div>
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <div className="mb-3 text-sm font-semibold text-body">Create New Template</div>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-5">
          <input
            id="template-create-key"
            name="template_create_key"
            className="admin-input"
            placeholder="Key"
            value={draft.key}
            onChange={(event) => setDraft((current) => ({ ...current, key: event.target.value }))}
          />
          <input
            id="template-create-title"
            name="template_create_title"
            className="admin-input"
            placeholder="Title"
            value={draft.title}
            onChange={(event) => setDraft((current) => ({ ...current, title: event.target.value }))}
          />
          <select
            id="template-create-target-kind"
            name="template_create_target_kind"
            className="admin-input"
            value={draft.targetKind}
            onChange={(event) =>
              setDraft((current) => ({
                ...current,
                targetKind: event.target.value,
                targetKey:
                  event.target.value === 'report'
                    ? targetCatalog.reports?.[0]?.key || ''
                    : targetCatalog.documents?.[0]?.key || '',
              }))
            }
          >
            <option value="document">Document</option>
            <option value="report">Report</option>
          </select>
          <select
            id="template-create-target-key"
            name="template_create_target_key"
            className="admin-input"
            value={draft.targetKey}
            onChange={(event) => setDraft((current) => ({ ...current, targetKey: event.target.value }))}
          >
            <option value="">Select target</option>
            {targetOptions.map((item) => (
              <option key={item.key} value={item.key}>
                {item.title || item.key}
              </option>
            ))}
          </select>
          <input
            id="template-create-purpose"
            name="template_create_purpose"
            className="admin-input"
            placeholder="Purpose"
            value={draft.purpose}
            onChange={(event) => setDraft((current) => ({ ...current, purpose: event.target.value }))}
          />
        </div>
        <div className="mt-3 flex items-center gap-3">
          <button
            type="button"
            className="admin-button"
            disabled={busy || !draft.key || !draft.title || !draft.targetKey}
            onClick={() => void createTemplate()}
          >
            Create New Template
          </button>
          {message ? <div className="text-sm text-body">{message}</div> : null}
        </div>
      </section>
      {renderDataGrid({
        columns: [
          { key: 'key', label: 'Template' },
          { key: 'title', label: 'Title' },
          { key: 'target_kind', label: 'Target' },
          { key: 'default_format', label: 'Format' },
          { key: 'purpose', label: 'Purpose' },
        ],
        rows,
        actionLabel: 'Open Designer',
        onAction: (row) =>
          navigate(`/templates/designer?key=${encodeURIComponent(String(resolvePath(row, 'key') || ''))}`),
        secondaryActionLabel: 'Delete',
        onSecondaryAction: (row) => void deleteTemplate(row),
      })}
    </div>
  )
}

function resolvePath(payload: Record<string, unknown>, path: string): unknown {
  return path.split('.').reduce<unknown>((current, key) => {
    if (current && typeof current === 'object' && key in (current as Record<string, unknown>)) {
      return (current as Record<string, unknown>)[key]
    }
    return undefined
  }, payload)
}
