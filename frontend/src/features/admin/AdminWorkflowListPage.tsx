import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { mutateJson } from './adminClient'

export function AdminWorkflowListPage({
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
  const [key, setKey] = useState('')
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')

  async function createWorkflow() {
    setBusy(true)
    setMessage('')
    try {
      await mutateJson('/admin/api/workflows', {
        method: 'POST',
        body: JSON.stringify({ key }),
      })
      navigate(`/workflows/designer?key=${encodeURIComponent(key)}`)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to create workflow.')
    } finally {
      setBusy(false)
    }
  }

  async function deleteWorkflow(row: Record<string, unknown>) {
    const nextKey = String(resolvePath(row, 'key') || '')
    if (!nextKey) return
    setBusy(true)
    setMessage('')
    try {
      await mutateJson(`/admin/api/workflows/${encodeURIComponent(nextKey)}`, {
        method: 'DELETE',
      })
      navigate(0)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to delete workflow.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-line bg-accent-soft/60 p-4 text-sm text-body">
        Select a workflow to open the visual designer, manage versions, validate transitions, and simulate routing.
      </div>
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <div className="mb-3 text-sm font-semibold text-body">Create New Workflow</div>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-[minmax(0,1fr)_auto]">
          <input
            id="workflow-create-key"
            name="workflow_create_key"
            className="admin-input"
            placeholder="Workflow Key"
            value={key}
            onChange={(event) => setKey(event.target.value)}
          />
          <button type="button" className="admin-button" disabled={busy || !key} onClick={() => void createWorkflow()}>
            Create New Workflow
          </button>
        </div>
        {message ? <div className="mt-3 text-sm text-body">{message}</div> : null}
      </section>
      {renderDataGrid({
        columns: [{ key: 'key', label: 'Workflow' }],
        rows,
        actionLabel: 'Open Designer',
        onAction: (row) =>
          navigate(`/workflows/designer?key=${encodeURIComponent(String(resolvePath(row, 'key') || ''))}`),
        secondaryActionLabel: 'Delete',
        onSecondaryAction: (row) => void deleteWorkflow(row),
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
