import { normalizeShellPath } from '@/services/bootstrap'

export function endpointForAdminPath(path: string): string {
  const normalizedPath = normalizeShellPath(path, 'admin')
  if (normalizedPath.startsWith('/modules/')) {
    const moduleKey = normalizedPath.slice('/modules/'.length)
    return moduleKey ? `/admin/api/modules/${encodeURIComponent(moduleKey)}/console` : ''
  }

  switch (normalizedPath) {
    case '/modules':
      return '/admin/api/modules'
    case '/config':
      return '/admin/api/config/definitions'
    case '/auth':
      return '/admin/api/auth/settings'
    case '/mcp':
      return '/admin/api/mcp'
    case '/acp':
      return '/admin/api/acp'
    case '/finance':
      return '/admin/api/config/definitions'
    case '/definitions':
      return '/admin/api/templates/definitions'
    case '/security':
      return '/admin/api/security/policy-hooks'
    case '/audit':
      return '/admin/api/audit-events'
    case '/observability':
      return '/admin/api/observability/contracts'
    case '/dashboards':
      return '/admin/api/dashboards'
    case '/templates':
      return '/admin/api/templates/definitions'
    case '/templates/designer':
      return '/admin/api/templates/definitions'
    case '/workflows':
      return '/admin/api/workflows'
    case '/workflows/designer':
      return '/admin/api/workflows'
    default:
      return ''
  }
}

export function adminPathSupportsPagination(path: string): boolean {
  const normalizedPath = normalizeShellPath(path, 'admin')
  switch (normalizedPath) {
    case '/modules':
    case '/config':
    case '/definitions':
    case '/security':
    case '/audit':
    case '/dashboards':
    case '/templates':
    case '/workflows':
      return true
    default:
      return false
  }
}

export function titleForAdminPath(path: string): string {
  const normalizedPath = normalizeShellPath(path, 'admin')
  if (normalizedPath.startsWith('/modules/')) {
    const moduleKey = normalizedPath.slice('/modules/'.length)
    return moduleKey ? `${startCase(moduleKey)} Console` : 'Module Console'
  }

  switch (normalizedPath) {
    case '/modules':
      return 'Modules'
    case '/config':
      return 'Configuration'
    case '/auth':
      return 'Authentication'
    case '/mcp':
      return 'MCP'
    case '/acp':
      return 'ACP'
    case '/finance':
      return 'Finance'
    case '/definitions':
      return 'Definitions'
    case '/security':
      return 'Security'
    case '/audit':
      return 'Audit Trail'
    case '/observability':
      return 'Observability'
    case '/dashboards':
      return 'Dashboards'
    case '/templates':
      return 'Templates'
    case '/templates/designer':
      return 'Template Designer'
    case '/workflows':
      return 'Workflows'
    case '/workflows/designer':
      return 'Workflow Designer'
    case '/org':
      return 'Organization'
    default:
      return 'Admin'
  }
}

function startCase(value: string): string {
  return value
    .split(/[_-]+/g)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}
