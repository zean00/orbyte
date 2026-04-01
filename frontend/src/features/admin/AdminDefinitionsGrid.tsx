export function AdminDefinitionsGrid({
  rows,
  renderDataGrid,
}: {
  rows: Array<Record<string, unknown>>
  renderDataGrid: (args: {
    columns: Array<{ key: string; label: string }>
    rows: Array<Record<string, unknown>>
  }) => JSX.Element
}) {
  return renderDataGrid({
    columns: [
      { key: 'key', label: 'Template' },
      { key: 'title', label: 'Title' },
      { key: 'target_kind', label: 'Target' },
      { key: 'default_format', label: 'Format' },
      { key: 'purpose', label: 'Purpose' },
    ],
    rows,
  })
}
