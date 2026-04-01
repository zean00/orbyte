import { useMemo } from 'react'
import { pickText, type ActionDefinition, type ViewDefinition } from '@/services/bootstrap'
import type { RouteResolution } from './workspaceTypes'

export function WorkspaceRouteContent({
  route,
  locale,
  actions,
  currentPath,
  onNavigate,
  onToast,
  renderQueueView,
  renderListView,
  renderDetailView,
  renderDashboardView,
  renderFormView,
  renderUnsupported,
}: {
  route: RouteResolution
  locale: string
  actions: ActionDefinition[]
  currentPath: string
  onNavigate: (target: string) => void
  onToast: (message: string, variant?: 'default' | 'success' | 'warning' | 'error') => void
  renderQueueView: (args: { view: ViewDefinition; locale: string; routeActions: ActionDefinition[]; onNavigate: (target: string) => void }) => JSX.Element
  renderListView: (args: { view: ViewDefinition; locale: string; routeActions: ActionDefinition[]; currentPath: string; onNavigate: (target: string) => void }) => JSX.Element
  renderDetailView: (args: {
    view: ViewDefinition
    locale: string
    routeActions: ActionDefinition[]
    currentPath: string
    onNavigate: (target: string) => void
    onToast: (message: string, variant?: 'default' | 'success' | 'warning' | 'error') => void
  }) => JSX.Element
  renderDashboardView: (args: {
    view: ViewDefinition
    locale: string
    onNavigate: (target: string) => void
    routeActions: ActionDefinition[]
    onToast: (message: string, variant?: 'default' | 'success' | 'warning' | 'error') => void
  }) => JSX.Element
  renderFormView: (args: {
    view: ViewDefinition
    locale: string
    currentPath: string
    searchParams: URLSearchParams
    onNavigate: (target: string) => void
    onToast: (message: string, variant?: 'default' | 'success' | 'warning' | 'error') => void
  }) => JSX.Element
  renderUnsupported: (args: { title: string; status: string }) => JSX.Element
}) {
  const view = route.view!
  const searchParams = useMemo(() => new URLSearchParams(window.location.search), [route.requested_path, window.location.search])

  if (view.kind === 'queue') {
    return renderQueueView({ view, locale, routeActions: actions, onNavigate })
  }
  if (view.kind === 'list') {
    return renderListView({ view, locale, routeActions: actions, currentPath, onNavigate })
  }
  if (view.kind === 'detail') {
    return renderDetailView({ view, locale, routeActions: actions, currentPath, onNavigate, onToast })
  }
  if (view.kind === 'dashboard') {
    return renderDashboardView({ view, locale, onNavigate, routeActions: actions, onToast })
  }
  if (view.kind === 'form') {
    return renderFormView({ view, locale, currentPath, searchParams, onNavigate, onToast })
  }
  return renderUnsupported({
    title: pickText(view, 'title', locale) || route.requested_path,
    status: 'View kind is not yet supported.',
  })
}
