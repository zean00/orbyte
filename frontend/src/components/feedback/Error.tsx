import { Button } from '@/components/ui/Button'

export interface ErrorProps {
  message?: string
  onRetry?: () => void
  className?: string
}

export function Error({ message = 'Something went wrong', onRetry, className }: ErrorProps) {
  return (
    <div className={`flex flex-col items-center justify-center p-8 text-center ${className}`}>
      <div className="w-12 h-12 mb-4 text-warn">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <circle cx="12" cy="12" r="10" />
          <path d="M12 8v4M12 16h.01" />
        </svg>
      </div>
      <p className="text-body mb-4">{message}</p>
      {onRetry && (
        <Button variant="secondary" size="sm" onClick={onRetry}>
          Try again
        </Button>
      )}
    </div>
  )
}
