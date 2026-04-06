import type { ReactNode } from 'react'
import { useMemo, useState } from 'react'
import { PaginationBar } from '@/components/ui/PaginationBar'
import type { FormState } from './workspaceFormTypes'

export function DataTable({
  columns,
  rows,
  emptyText,
  renderAction,
  localPageSize,
}: {
  columns: Array<{ key: string; label: string }>
  rows: Array<Record<string, unknown>>
  emptyText: string
  renderAction?: (row: Record<string, unknown>) => ReactNode
  localPageSize?: number
}) {
  const [page, setPage] = useState(1)
  const pageSize = localPageSize && localPageSize > 0 ? localPageSize : 0
  const total = rows.length
  const pagedRows = useMemo(() => {
    if (!pageSize) return rows
    const totalPages = Math.max(1, Math.ceil(total / pageSize))
    const currentPage = Math.min(page, totalPages)
    const start = (currentPage - 1) * pageSize
    return rows.slice(start, start + pageSize)
  }, [page, pageSize, rows, total])

  return (
    <>
      <div className="overflow-hidden rounded-xl border border-line">
        <table className="min-w-full divide-y divide-line">
          <thead className="border-b border-line bg-accent-soft dark:bg-ink/60">
            <tr>
              {columns.map((column) => (
                <th key={column.key} className="px-4 py-3 text-left text-xs font-bold uppercase tracking-[0.14em] text-accent-dark dark:text-body">
                  {column.label}
                </th>
              ))}
              {renderAction ? <th className="px-4 py-3" /> : null}
            </tr>
          </thead>
          <tbody className="divide-y divide-line bg-surface">
            {pagedRows.length ? pagedRows.map((row, index) => (
              <tr key={index}>
                {columns.map((column) => (
                  <td key={column.key} className="px-4 py-3 text-sm text-body">
                    {displayValue(resolvePath(row, column.key))}
                  </td>
                ))}
                {renderAction ? <td className="px-4 py-3 text-right">{renderAction(row)}</td> : null}
              </tr>
            )) : (
              <tr>
                <td colSpan={columns.length + (renderAction ? 1 : 0)} className="px-4 py-10 text-center text-sm text-muted">
                  {emptyText}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      {pageSize && total > 0 ? (
        <PaginationBar page={page} pageSize={pageSize} total={total} onPageChange={setPage} />
      ) : null}
    </>
  )
}

export function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <article className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
      <div className="text-xs font-semibold uppercase tracking-wide text-body">{label}</div>
      <div className="mt-2 text-2xl font-bold text-body">{value}</div>
    </article>
  )
}

export function displayValue(value: unknown): string {
  if (value == null) return ''
  if (typeof value === 'boolean') return value ? 'Yes' : 'No'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

export function asRecordList(value: unknown): Array<Record<string, unknown>> {
  if (!Array.isArray(value)) return []
  return value.map((item) => (item && typeof item === 'object' ? { ...(item as Record<string, unknown>) } : {}))
}

export function toNumber(value: unknown): number {
  const numeric = typeof value === 'number' ? value : Number(value || 0)
  return Number.isFinite(numeric) ? numeric : 0
}

export function roundMoney(value: number): number {
  return Math.round(value * 100) / 100
}

export function addDaysToDate(baseDate: string, days: number): string {
  if (!baseDate || !Number.isFinite(days) || days <= 0) return ''
  const date = new Date(`${baseDate}T00:00:00`)
  if (Number.isNaN(date.getTime())) return ''
  date.setDate(date.getDate() + days)
  return date.toISOString().slice(0, 10)
}

export function resolvePath(payload: unknown, path: string): unknown {
  if (!path || payload == null) return payload
  return path.split('.').reduce<unknown>((current, key) => {
    if (current && typeof current === 'object' && key in (current as Record<string, unknown>)) {
      return (current as Record<string, unknown>)[key]
    }
    return undefined
  }, payload)
}

export function assignPathValue(current: FormState, path: string, value: unknown): FormState {
  const next = structuredClone(current)
  const parts = path.split('.')
  let target: Record<string, unknown> = next
  while (parts.length > 1) {
    const key = parts.shift()!
    target[key] = (target[key] as Record<string, unknown>) || {}
    target = target[key] as Record<string, unknown>
  }
  const finalKey = parts[0]
  if (finalKey) target[finalKey] = value
  return next
}

export function humanize(value: string): string {
  return value.replace(/[_./-]+/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase())
}
