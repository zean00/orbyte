import type {
  ActionDefinition,
  CustomEntryDefinition,
  DocumentFlowDefinition,
  ViewDefinition,
} from '@/services/bootstrap'

export type RouteResolution = {
  status: 'ok' | 'not_found' | 'forbidden' | 'surface_mismatch'
  requested_path: string
  fallback_path?: string
  suggested_surface?: string
  message?: string
  render_mode?: 'generic' | 'custom' | 'flow'
  action?: ActionDefinition
  view?: ViewDefinition
  flow?: DocumentFlowDefinition
  custom_entry?: CustomEntryDefinition
}
