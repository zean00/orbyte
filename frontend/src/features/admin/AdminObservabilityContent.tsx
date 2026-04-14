export function AdminObservabilityContent({
  payload,
  asItems,
  renderSummaryCard,
  renderDataGrid,
}: {
  payload: Record<string, unknown> | null
  asItems: (value: Record<string, unknown> | null | undefined) => Array<Record<string, unknown>>
  renderSummaryCard: (args: { label: string; value: string }) => JSX.Element
  renderDataGrid: (args: {
    columns: Array<{ key: string; label: string }>
    rows: Array<Record<string, unknown>>
  }) => JSX.Element
}) {
  const metrics = asItems(payload?.metrics as Record<string, unknown> | null)
  const logEvents = asItems(payload?.log_events as Record<string, unknown> | null)
  const domainEvents = asItems(payload?.domain_events as Record<string, unknown> | null)
  const statuses = asItems(payload?.contract_status as Record<string, unknown> | null)

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
        {renderSummaryCard({ label: 'Metrics', value: String(metrics.length) })}
        {renderSummaryCard({ label: 'Log Events', value: String(logEvents.length) })}
        {renderSummaryCard({ label: 'Domain Events', value: String(domainEvents.length) })}
        {renderSummaryCard({ label: 'Contracts', value: String(statuses.length) })}
      </div>
      {renderDataGrid({
        columns: [
          { key: 'key', label: 'Metric' },
          { key: 'type', label: 'Type' },
          { key: 'description', label: 'Description' },
        ],
        rows: metrics,
      })}
      {renderDataGrid({
        columns: [
          { key: 'key', label: 'Log Event' },
          { key: 'category', label: 'Category' },
          { key: 'severity', label: 'Severity' },
        ],
        rows: logEvents,
      })}
      {renderDataGrid({
        columns: [
          { key: 'type', label: 'Domain Event' },
          { key: 'role', label: 'Role' },
          { key: 'correlation_required', label: 'Correlation' },
        ],
        rows: domainEvents,
      })}
    </div>
  )
}
