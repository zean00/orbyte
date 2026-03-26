import { forwardRef, type InputHTMLAttributes } from 'react'
import { cn } from '@/lib/utils'

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
  hint?: string
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className, label, error, hint, id, name, ...props }, ref) => {
    const inputId = id || label?.toLowerCase().replace(/\s+/g, '-')
    const inputName = name || inputId

    return (
      <div className="space-y-1.5">
        {label && (
          <label htmlFor={inputId} className="block text-sm font-medium" style={{ color: 'var(--color-body)' }}>
            {label}
          </label>
        )}
        <input
          ref={ref}
          id={inputId}
          name={inputName}
          className={cn(
            'flex h-10 w-full rounded-lg border px-3 py-2 text-sm transition-colors',
            'focus:outline-none focus:ring-2 focus:ring-accent focus:border-transparent',
            'disabled:cursor-not-allowed disabled:opacity-50',
            className
          )}
          style={{
            backgroundColor: 'var(--color-surface)',
            borderColor: error ? 'var(--color-warn)' : 'var(--color-line)',
            color: 'var(--color-body)',
          }}
          {...props}
        />
        {error && <p className="text-sm" style={{ color: 'var(--color-warn)' }}>{error}</p>}
        {hint && !error && <p className="text-sm" style={{ color: 'var(--color-muted)' }}>{hint}</p>}
      </div>
    )
  }
)

Input.displayName = 'Input'
