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
    <div className="flex h-full">
      <Sidebar />
      <div
        className={`flex flex-col flex-1 transition-all duration-200 ${
          sidebarOpen ? 'ml-64' : 'ml-[72px]'
        }`}
      >
        <Header />
        <main className="flex-1 overflow-auto bg-shell dark:bg-ink p-6">
          {children}
        </main>
      </div>
    </div>
  )
}
