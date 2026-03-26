import { type FormEvent, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useShellStore } from '@/stores/shellStore'
import { useAuth } from '@/hooks/useAuth'
import { useDarkMode } from '@/hooks/useDarkMode'
import { usePreferencesStore } from '@/stores/preferencesStore'
import { fetchAdminBootstrap, fetchWorkspaceBootstrap, normalizeShellPath, persistLocale, pickText, toShellRoutes } from '@/services/bootstrap'

const surfaceLabels: Record<string, string> = {
  backoffice: 'Backoffice',
  worklist: 'Worklist',
  self_service: 'Self-Service',
  pos: 'POS',
}

export function Header() {
  const navigate = useNavigate()
  const {
    shellKind,
    availableSurfaces,
    currentSurface,
    locale,
    supportedLocales,
    adminAccess,
    adminPath,
    uiAccess,
    uiPath,
    workspaceBootstrap,
    setWorkspaceBootstrap,
    setAdminBootstrap,
    setLocale,
    setRoutes,
    actions,
    routes,
  } = useShellStore()
  const { setLocale: setPreferredLocale } = usePreferencesStore()
  const { darkMode, toggleDarkMode } = useDarkMode()
  const { user, logout } = useAuth()
  const [command, setCommand] = useState('')

  const commandOptions = useMemo(() => {
    const unique = new Map<string, { label: string; path: string }>()
    for (const route of routes) {
      unique.set(route.path, { label: route.label, path: route.path })
    }
    for (const action of actions) {
      const path = normalizeShellPath(action.route_path, shellKind)
      if (!path) continue
      unique.set(path, { label: pickText(action, 'label', locale) || action.key, path })
    }
    if (shellKind === 'workspace') {
      unique.set('/notifications', { label: 'Notifications', path: '/notifications' })
    }
    return [...unique.values()]
  }, [actions, locale, routes, shellKind])

  const handleSurfaceChange = async (surface: string) => {
    const bootstrap = await fetchWorkspaceBootstrap(surface)
    setRoutes(toShellRoutes(bootstrap.menus, bootstrap.actions, bootstrap.locale, 'workspace'))
    setWorkspaceBootstrap(bootstrap)
    navigate(useShellStore.getState().defaultPath || '/', { replace: true })
  }

  const handleLocaleChange = async (newLocale: string) => {
    const activeLocale = await persistLocale(newLocale)
    setLocale(activeLocale)
    setPreferredLocale(activeLocale)
    if (shellKind === 'workspace') {
      const bootstrap = await fetchWorkspaceBootstrap(currentSurface)
      setWorkspaceBootstrap(bootstrap)
      setRoutes(toShellRoutes(bootstrap.menus, bootstrap.actions, bootstrap.locale, 'workspace'))
      return
    }
    const bootstrap = await fetchAdminBootstrap()
    setAdminBootstrap(bootstrap)
    setRoutes(toShellRoutes(bootstrap.menus, bootstrap.actions, bootstrap.locale, 'admin'))
  }

  const handleCommandSubmit = (event: FormEvent) => {
    event.preventDefault()
    const raw = command.trim()
    if (!raw) return
    const lowered = raw.toLowerCase()
    const match =
      commandOptions.find((item) => item.path === raw || item.label.toLowerCase() === lowered) ||
      commandOptions.find((item) => item.label.toLowerCase().includes(lowered) || item.path.toLowerCase().includes(lowered))
    navigate((match?.path || raw).startsWith('/') ? match?.path || raw : `/${match?.path || raw}`)
    setCommand('')
  }

  return (
    <header className="h-12 bg-surface border-b border-line flex items-center px-3 gap-2 overflow-hidden">
      {shellKind === 'workspace' && availableSurfaces.length > 0 && (
        <div className="flex items-center gap-0.5 shrink-0">
          {availableSurfaces.map((surface) => (
            <button
              key={surface}
              onClick={() => void handleSurfaceChange(surface)}
              className={`px-1.5 py-0.5 text-xs rounded transition-colors whitespace-nowrap ${
                currentSurface === surface ? 'bg-accent text-white' : 'text-muted hover:text-body hover:bg-shell'
              }`}
            >
              {surfaceLabels[surface] || surface}
            </button>
          ))}
        </div>
      )}

      <div className="ml-auto flex items-center gap-1">
        <form onSubmit={handleCommandSubmit} className="flex items-center gap-1">
          <input
            id="shell-command"
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            list="shell-route-options"
            placeholder={shellKind === 'admin' ? 'Search admin sections or jump to a route' : 'Search pages or jump to a route'}
            className="w-40 rounded border border-line bg-surface px-2 py-1 text-xs text-body md:w-64"
            name="shell_command"
          />
          <datalist id="shell-route-options">
            {commandOptions.map((item) => (
              <option key={item.path} value={item.path}>
                {item.label}
              </option>
            ))}
          </datalist>
          <button type="submit" className="rounded border border-line px-2 py-1 text-xs text-body">
            Go
          </button>
        </form>

        {supportedLocales.length > 1 && (
          <select
            id="locale"
            value={locale}
            onChange={(e) => void handleLocaleChange(e.target.value)}
            className="px-1 py-0.5 text-xs bg-surface border border-line rounded text-body"
            name="locale"
          >
            {supportedLocales.map((loc) => (
              <option key={loc} value={loc}>
                {loc.toUpperCase()}
              </option>
            ))}
          </select>
        )}

        {shellKind === 'workspace' && (
          <button
            onClick={() => navigate('/settings')}
            className="p-1 text-muted hover:text-body transition-colors rounded hover:bg-shell"
            title="Settings"
          >
            <SettingsIcon className="w-4 h-4" />
          </button>
        )}

        {shellKind === 'workspace' && (
          <button
            onClick={() => navigate('/notifications')}
            className="p-1 text-muted hover:text-body transition-colors rounded hover:bg-shell"
            title="Notifications"
          >
            <BellIcon className="w-4 h-4" />
          </button>
        )}

        <button
          onClick={toggleDarkMode}
          className="p-1 text-muted hover:text-body transition-colors rounded hover:bg-shell"
          title={darkMode ? 'Light mode' : 'Dark mode'}
        >
          {darkMode ? <SunIcon className="w-4 h-4" /> : <MoonIcon className="w-4 h-4" />}
        </button>

        {user && (
          <div className="flex items-center gap-1 pl-1.5 border-l border-line">
            <span className="text-xs font-medium text-body">{user.name}</span>
            <button
              onClick={() => {
                void (async () => {
                  await logout()
                  if (shellKind === 'admin') {
                    window.location.href = '/ui/login'
                    return
                  }
                  navigate('/login', { replace: true })
                })()
              }}
              className="p-1 text-muted hover:text-warn transition-colors"
              title="Log out"
            >
              <LogoutIcon className="w-4 h-4" />
            </button>
          </div>
        )}

        {shellKind === 'workspace' && adminAccess && workspaceBootstrap && (
          <button
            type="button"
            onClick={() => {
              window.location.href = `/admin${adminPath === '/' ? '' : adminPath}`
            }}
            className="px-1.5 py-0.5 text-xs bg-accent text-white rounded hover:bg-accent-dark transition-colors"
          >
            Admin
          </button>
        )}

        {shellKind === 'admin' && uiAccess && (
          <button
            type="button"
            onClick={() => {
              window.location.href = `/ui${uiPath === '/' ? '' : uiPath}`
            }}
            className="px-1.5 py-0.5 text-xs bg-accent text-white rounded hover:bg-accent-dark transition-colors"
          >
            Workspace
          </button>
        )}
      </div>
    </header>
  )
}

function SunIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
    </svg>
  )
}

function MoonIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
    </svg>
  )
}

function BellIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
    </svg>
  )
}

function LogoutIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
    </svg>
  )
}

function SettingsIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M10.325 4.317a1 1 0 011.35-.936l.821.328a1 1 0 00.748 0l.821-.328a1 1 0 011.35.936l.062.883a1 1 0 00.512.815l.746.43a1 1 0 01.365 1.366l-.43.746a1 1 0 000 .748l.43.746a1 1 0 01-.365 1.366l-.746.43a1 1 0 00-.512.815l-.062.883a1 1 0 01-1.35.936l-.821-.328a1 1 0 00-.748 0l-.821.328a1 1 0 01-1.35-.936l-.062-.883a1 1 0 00-.512-.815l-.746-.43a1 1 0 01-.365-1.366l.43-.746a1 1 0 000-.748l-.43-.746a1 1 0 01.365-1.366l.746-.43a1 1 0 00.512-.815l.062-.883z" />
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 15a3 3 0 100-6 3 3 0 000 6z" />
    </svg>
  )
}
