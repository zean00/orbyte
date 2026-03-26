import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

export interface EmptyProps {
  title?: string
  description?: string
  icon?: ReactNode
  action?: ReactNode
  className?: string
}

export function Empty({
  title = 'No data',
  description,
  icon,
  action,
  className
}: EmptyProps) {
  return (
    <div className={cn('flex flex-col items-center justify-center p-8 text-center', className)}>
      {icon && <div className="mb-4 text-muted">{icon}</div>}
      <h3 className="text-lg font-medium text-body mb-1">{title}</h3>
      {description && <p className="text-sm text-muted mb-4 max-w-sm">{description}</p>}
      {action}
    </div>
  )
}
