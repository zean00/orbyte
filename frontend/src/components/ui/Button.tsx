import { forwardRef, type ButtonHTMLAttributes } from 'react'
import { cn } from '@/lib/utils'

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger'
  size?: 'sm' | 'md' | 'lg'
  isLoading?: boolean
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = 'primary', size = 'md', isLoading, disabled, children, ...props }, ref) => {
    const baseStyles = 'inline-flex items-center justify-center font-medium rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50'

    const getVariantStyles = () => {
      switch (variant) {
        case 'primary':
          return { backgroundColor: 'var(--color-accent)', color: '#ffffff' }
        case 'secondary':
          return { backgroundColor: 'var(--color-surface)', color: 'var(--color-body)', border: '1px solid var(--color-line)' }
        case 'ghost':
          return { backgroundColor: 'transparent', color: 'var(--color-body)' }
        case 'danger':
          return { backgroundColor: 'var(--color-warn)', color: '#ffffff' }
        default:
          return {}
      }
    }

    const getSizeStyles = () => {
      switch (size) {
        case 'sm':
          return { height: '2rem', padding: '0 0.75rem', fontSize: '0.875rem' }
        case 'md':
          return { height: '2.5rem', padding: '0 1rem', fontSize: '0.875rem' }
        case 'lg':
          return { height: '3rem', padding: '0 1.5rem', fontSize: '1rem' }
        default:
          return {}
      }
    }

    return (
      <button
        ref={ref}
        className={cn(baseStyles, className)}
        style={{ ...getVariantStyles(), ...getSizeStyles() }}
        disabled={disabled || isLoading}
        {...props}
      >
        {isLoading && (
          <svg className="mr-2 h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" style={{ animation: 'spin 1s linear infinite' }}>
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
          </svg>
        )}
        {children}
      </button>
    )
  }
)

Button.displayName = 'Button'
