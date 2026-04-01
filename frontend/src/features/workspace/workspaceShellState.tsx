import type { ReactNode } from 'react'
import { PageSection } from '@/components/layout/PageSection'
import type { RouteResolution } from './workspaceTypes'

type WorkspacePanelProps = {
  title: string
  status?: string
  children?: ReactNode
}

export function WorkspacePanel({ title, status, children }: WorkspacePanelProps) {
  return (
    <PageSection title={title} status={status}>
      {children}
    </PageSection>
  )
}

type WorkspaceRecoveryPanelProps = {
  route: RouteResolution
  onDefault: () => void
  onSwitchSurface: () => void
}

export function WorkspaceRecoveryPanel({
  route,
  onDefault,
  onSwitchSurface,
}: WorkspaceRecoveryPanelProps) {
  return (
    <WorkspacePanel
      title={route.status === 'forbidden' ? 'Route forbidden' : 'Route unavailable'}
      status={route.message}
    >
      <div className="flex gap-3">
        {route.status === 'surface_mismatch' ? (
          <button onClick={onSwitchSurface} className="rounded-lg bg-accent px-4 py-2 text-white">
            Switch surface
          </button>
        ) : null}
        <button onClick={onDefault} className="rounded-lg border border-line px-4 py-2 text-body">
          Go to default
        </button>
      </div>
    </WorkspacePanel>
  )
}
