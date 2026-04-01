import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

type PageSectionProps = {
  title: string
  status?: string
  actions?: ReactNode
  children?: ReactNode
  className?: string
}

export function PageSection({
  title,
  status,
  actions,
  children,
  className,
}: PageSectionProps) {
  return (
    <section className={cn('rounded-[1.75rem] border border-line bg-surface p-6 shadow-panel sm:p-7', className)}>
      <div className="mb-5 flex flex-col gap-3 border-b border-line/70 pb-4 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-body sm:text-[1.75rem]">{title}</h1>
          {status ? <p className="mt-1 text-sm leading-6 text-muted">{status}</p> : null}
        </div>
        {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
      </div>
      {children}
    </section>
  )
}
