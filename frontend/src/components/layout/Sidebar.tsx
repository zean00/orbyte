import { useEffect, useMemo, useState } from 'react'
import { NavLink } from 'react-router-dom'
import { motion } from 'framer-motion'
import { cn } from '@/lib/utils'
import { useShellStore } from '@/stores/shellStore'
import type { ShellRoute } from '@/services/bootstrap'

export function Sidebar() {
  const { sidebarOpen, toggleSidebar, routes, adminAccess, adminPath, shellKind, uiAccess, uiPath, currentRoute } = useShellStore()
  const groupedRoutes = useMemo(() => groupRoutesByPath(routes), [routes])
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({})

  useEffect(() => {
    setCollapsedGroups((current) => {
      const next = { ...current }
      for (const group of groupedRoutes) {
        if (groupContainsRoute(group, currentRoute)) {
          next[group.key] = false
        } else if (!(group.key in next)) {
          next[group.key] = false
        }
      }
      return next
    })
  }, [currentRoute, groupedRoutes])

  return (
    <motion.aside
      initial={false}
      animate={{ width: sidebarOpen ? 256 : 72 }}
      transition={{ duration: 0.2 }}
      className="fixed left-0 top-0 z-40 flex h-screen flex-col border-r border-line bg-surface"
    >
      <div className="flex h-16 items-center border-b border-line px-4 shrink-0">
        {sidebarOpen && <span className="font-display font-bold text-lg text-accent">Orbyte</span>}
        <button onClick={toggleSidebar} className="ml-auto p-2 text-muted hover:text-body transition-colors">
          <MenuIcon className="w-5 h-5" />
        </button>
      </div>

      <nav className="min-h-0 flex-1 overflow-y-auto p-3">
        {groupedRoutes.map((group) => (
          <div key={group.key} className="mb-4 last:mb-0">
            {sidebarOpen && (
              <button
                type="button"
                onClick={() =>
                  setCollapsedGroups((current) => ({
                    ...current,
                    [group.key]: !current[group.key],
                  }))
                }
                className="flex w-full items-center justify-between px-3 pb-1 text-[11px] font-semibold uppercase tracking-[0.12em] text-muted/70 transition-colors hover:text-body"
              >
                <span>{group.label}</span>
                <ChevronIcon className={cn('h-4 w-4 transition-transform', collapsedGroups[group.key] ? '-rotate-90' : 'rotate-0')} />
              </button>
            )}
            <div className={cn('space-y-1', sidebarOpen && collapsedGroups[group.key] ? 'hidden' : '')}>
              {group.items.map((item) => (
                <NavLink
                  key={item.key}
                  to={item.path}
                  className={({ isActive }) =>
                    cn(
                      'flex items-center gap-3 rounded-lg px-3 py-2 transition-colors',
                      'hover:bg-shell dark:hover:bg-ink/50',
                      isActive ? 'bg-accent-soft text-accent' : 'text-muted'
                    )
                  }
                >
                  <DotIcon className="h-5 w-5 flex-shrink-0" />
                  {sidebarOpen && <span className="text-sm font-medium">{item.label}</span>}
                </NavLink>
              ))}
            </div>
          </div>
        ))}
      </nav>

      <div className="shrink-0 border-t border-line p-3">
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

type RouteGroup = {
  key: string
  label: string
  items: ShellRoute[]
}

function groupRoutesByPath(routes: ShellRoute[]): RouteGroup[] {
  const groups = new Map<string, RouteGroup>()
  const order: string[] = []

  for (const route of routes) {
    const segment = firstPathSegment(route.path)
    const groupKey = segment || 'general'
    if (!groups.has(groupKey)) {
      groups.set(groupKey, {
        key: groupKey,
        label: groupLabelForSegment(groupKey),
        items: [],
      })
      order.push(groupKey)
    }
    groups.get(groupKey)!.items.push(route)
  }

  return order.map((key) => groups.get(key)!).filter((group) => group.items.length > 0)
}

function groupContainsRoute(group: RouteGroup, currentRoute: string): boolean {
  return group.items.some((item) => item.path === currentRoute)
}

function firstPathSegment(path: string): string {
  return path.replace(/^\/+/, '').split('/')[0] || ''
}

function groupLabelForSegment(segment: string): string {
  const labels: Record<string, string> = {
    analytics: 'Analytics',
    clinic: 'Clinic',
    commercial: 'Commercial',
    delivery: 'Delivery',
    documents: 'Documents',
    fulfillment: 'Fulfillment',
    inventory: 'Inventory',
    masterdata: 'Master Data',
    monitoring: 'Monitoring',
    planning: 'Planning',
    procurement: 'Procurement',
    production: 'Production',
    recall: 'Recall',
    returns: 'Returns',
    'supplier-returns': 'Supplier Returns',
  }
  if (labels[segment]) return labels[segment]
  if (!segment) return 'General'
  return segment
    .split(/[-_]/g)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
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

function ChevronIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="m9 5 7 7-7 7" />
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
