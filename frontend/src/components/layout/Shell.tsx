import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { Sidebar } from './Sidebar'
import { Header } from './Header'
import { useShellStore } from '@/stores/shellStore'

export interface ShellProps {
  children: ReactNode
  loading?: boolean
  loadingLabel?: string
}

export function Shell({ children, loading = false, loadingLabel }: ShellProps) {
  const { sidebarOpen, navigationPendingKind } = useShellStore()
  const switchingSurface = navigationPendingKind === 'surface'
  const showLoading =
    loading ||
    navigationPendingKind === 'workspace_data' ||
    navigationPendingKind === 'admin_data'

  if (switchingSurface) {
    return (
      <div className="flex min-h-screen bg-shell dark:bg-ink">
        <div className="flex min-h-screen flex-1 flex-col">
          <div className="sticky top-0 z-20 border-b border-line bg-surface/92 px-4 py-3 shadow-[0_12px_32px_rgba(15,23,42,0.08)] backdrop-blur sm:px-6">
            <div className="overflow-hidden rounded-2xl border border-accent/15 bg-surface/92">
              <div className="h-1 w-full overflow-hidden bg-accent-soft/60">
                <div className="h-full w-1/3 animate-[shell-progress_1.2s_ease-in-out_infinite] rounded-full bg-accent" />
              </div>
              <div className="flex items-center gap-3 px-4 py-3 text-sm text-body">
                <span className="inline-flex h-2.5 w-2.5 animate-pulse rounded-full bg-accent" />
                <span>Switching workspace surface.</span>
              </div>
            </div>
          </div>
          <main className="min-h-0 flex-1 overflow-auto px-4 pb-6 pt-4 sm:px-6 sm:pb-8">
            {children}
          </main>
        </div>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen bg-shell dark:bg-ink">
      <Sidebar />
      <div
        className={cn(
          'flex min-h-screen flex-1 flex-col transition-[margin] duration-200',
          sidebarOpen ? 'md:ml-72' : 'md:ml-24'
        )}
      >
        <Header />
        {showLoading ? (
          <div className="sticky top-16 z-20 px-4 pt-3 sm:px-6">
            <div className="overflow-hidden rounded-2xl border border-accent/15 bg-surface/92 shadow-[0_12px_32px_rgba(15,23,42,0.08)] backdrop-blur">
              <div className="h-1 w-full overflow-hidden bg-accent-soft/60">
                <div className="h-full w-1/3 animate-[shell-progress_1.2s_ease-in-out_infinite] rounded-full bg-accent" />
              </div>
              <div className="flex items-center gap-3 px-4 py-3 text-sm text-body">
                <span className="inline-flex h-2.5 w-2.5 animate-pulse rounded-full bg-accent" />
                <span>{loadingLabel || "Loading data from the server."}</span>
              </div>
            </div>
          </div>
        ) : null}
        <main className="min-h-0 flex-1 overflow-auto px-4 pb-6 pt-4 sm:px-6 sm:pb-8">
          {children}
        </main>
      </div>
    </div>
  )
}
