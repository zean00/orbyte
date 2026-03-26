import { NavLink } from 'react-router-dom'
import { motion } from 'framer-motion'
import { cn } from '@/lib/utils'
import { useShellStore } from '@/stores/shellStore'

export function Sidebar() {
  const { sidebarOpen, toggleSidebar, routes, adminAccess, adminPath, shellKind, uiAccess, uiPath } = useShellStore()

  return (
    <motion.aside
      initial={false}
      animate={{ width: sidebarOpen ? 256 : 72 }}
      transition={{ duration: 0.2 }}
      className="fixed left-0 top-0 h-full bg-surface border-r border-line z-40"
    >
      <div className="flex items-center h-16 px-4 border-b border-line">
        {sidebarOpen && <span className="font-display font-bold text-lg text-accent">Orbyte</span>}
        <button onClick={toggleSidebar} className="ml-auto p-2 text-muted hover:text-body transition-colors">
          <MenuIcon className="w-5 h-5" />
        </button>
      </div>

      <nav className="p-3 space-y-1">
        {routes.map((item) => (
          <NavLink
            key={item.key}
            to={item.path}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-3 px-3 py-2 rounded-lg transition-colors',
                'hover:bg-shell dark:hover:bg-ink/50',
                isActive ? 'bg-accent-soft text-accent' : 'text-muted'
              )
            }
          >
            <DotIcon className="w-5 h-5 flex-shrink-0" />
            {sidebarOpen && <span className="text-sm font-medium">{item.label}</span>}
          </NavLink>
        ))}
      </nav>

      <div className="absolute bottom-0 left-0 right-0 p-3 border-t border-line">
        {shellKind === 'workspace' && adminAccess && (
          <button
            type="button"
            onClick={() => {
              window.location.href = `/admin${adminPath === '/' ? '' : adminPath}`
            }}
            className="flex w-full items-center gap-3 rounded-lg bg-transparent px-3 py-2 text-left text-muted hover:bg-shell dark:hover:bg-ink/50"
          >
            <AdminIcon className="w-5 h-5 flex-shrink-0" />
            {sidebarOpen && <span className="text-sm font-medium">Admin</span>}
          </button>
        )}
        {shellKind === 'admin' && uiAccess && (
          <button
            type="button"
            onClick={() => {
              window.location.href = `/ui${uiPath === '/' ? '' : uiPath}`
            }}
            className="flex w-full items-center gap-3 rounded-lg bg-transparent px-3 py-2 text-left text-muted hover:bg-shell dark:hover:bg-ink/50"
          >
            <AdminIcon className="w-5 h-5 flex-shrink-0" />
            {sidebarOpen && <span className="text-sm font-medium">Workspace</span>}
          </button>
        )}
      </div>
    </motion.aside>
  )
}

function MenuIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M4 6h16M4 12h16M4 18h16" />
    </svg>
  )
}

function DotIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor">
      <circle cx="12" cy="12" r="5" />
    </svg>
  )
}

function AdminIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
    </svg>
  )
}
