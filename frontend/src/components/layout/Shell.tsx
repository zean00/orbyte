import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { Sidebar } from './Sidebar'
import { Header } from './Header'
import { useShellStore } from '@/stores/shellStore'

export interface ShellProps {
  children: ReactNode
}

export function Shell({ children }: ShellProps) {
  const { sidebarOpen } = useShellStore()

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
        <main className="min-h-0 flex-1 overflow-auto px-4 pb-6 pt-4 sm:px-6 sm:pb-8">
          {children}
        </main>
      </div>
    </div>
  )
}
