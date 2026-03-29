import type { ReactNode } from 'react'
import { Sidebar } from './Sidebar'
import { Header } from './Header'
import { useShellStore } from '@/stores/shellStore'

export interface ShellProps {
  children: ReactNode
}

export function Shell({ children }: ShellProps) {
  const { sidebarOpen } = useShellStore()

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar />
      <div
        className={`flex min-h-0 flex-1 flex-col transition-all duration-200 ${
          sidebarOpen ? 'ml-64' : 'ml-[72px]'
        }`}
      >
        <Header />
        <main className="min-h-0 flex-1 overflow-auto bg-shell p-6 dark:bg-ink">
          {children}
        </main>
      </div>
    </div>
  )
}
