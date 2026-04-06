type PaginationBarProps = {
  page: number
  pageSize: number
  total: number
  onPageChange: (page: number) => void
  onPageSizeChange?: (pageSize: number) => void
  pageSizeOptions?: number[]
  className?: string
}

export function PaginationBar({
  page,
  pageSize,
  total,
  onPageChange,
  onPageSizeChange,
  pageSizeOptions = [10, 20, 50, 100],
  className = '',
}: PaginationBarProps) {
  const safePage = page > 0 ? page : 1
  const safePageSize = pageSize > 0 ? pageSize : 20
  const totalPages = Math.max(1, Math.ceil(total / safePageSize))
  const currentPage = Math.min(safePage, totalPages)
  const start = total === 0 ? 0 : (currentPage - 1) * safePageSize + 1
  const end = total === 0 ? 0 : Math.min(total, currentPage * safePageSize)

  return (
    <div className={`mt-4 flex flex-col gap-3 rounded-xl border border-line bg-surface px-4 py-3 text-sm text-body md:flex-row md:items-center md:justify-between ${className}`.trim()}>
      <div className="flex flex-wrap items-center gap-3 text-muted">
        <span>
          Page {currentPage} of {totalPages}
        </span>
        <span>
          Showing {start}-{end} of {total}
        </span>
        {onPageSizeChange ? (
          <label className="flex items-center gap-2 text-body">
            <span className="text-muted">Page size</span>
            <select
              className="rounded-lg border border-line bg-surface px-2 py-1 text-sm text-body"
              value={safePageSize}
              onChange={(event) => onPageSizeChange(Number(event.target.value || safePageSize))}
            >
              {pageSizeOptions.map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </select>
          </label>
        ) : null}
      </div>
      <div className="flex items-center gap-2">
        <button
          type="button"
          className="rounded-lg border border-line px-3 py-1.5 text-sm text-body disabled:opacity-50"
          disabled={currentPage <= 1}
          onClick={() => onPageChange(currentPage - 1)}
        >
          Previous
        </button>
        <button
          type="button"
          className="rounded-lg border border-line px-3 py-1.5 text-sm text-body disabled:opacity-50"
          disabled={currentPage >= totalPages}
          onClick={() => onPageChange(currentPage + 1)}
        >
          Next
        </button>
      </div>
    </div>
  )
}
