import {
  getAdminBootstrap,
  getAuthOptions,
  getUIBootstrap,
} from '@/services/generated/client'
import type { AuthOptions } from '@/services/generated/types'

export type ShellKind = 'workspace' | 'admin'

export interface LocalizedText {
  en?: string
  id?: string
  [key: string]: string | undefined
}

export interface MenuDefinition {
  key: string
  label: string
  label_i18n?: LocalizedText
  icon?: string
  action_key: string
  parent_key?: string
  order?: number
}

export interface ColumnDefinition {
  key: string
  label: string
  label_i18n?: LocalizedText
  path: string
}

export interface FilterDefinition {
  key: string
  label: string
  label_i18n?: LocalizedText
  type: string
  options?: string[]
}

export interface FieldDefinition {
  key: string
  label: string
  label_i18n?: LocalizedText
  path: string
  type: string
  widget?: string
  placeholder?: string
  placeholder_i18n?: LocalizedText
  help_text?: string
  help_text_i18n?: LocalizedText
  options?: string[]
  required?: boolean
  min_length?: number
  max_length?: number
  pattern?: string
  min_value?: number
  max_value?: number
  read_only?: boolean
}

export interface SectionDefinition {
  key: string
  title: string
  title_i18n?: LocalizedText
  fields?: FieldDefinition[]
}

export interface TabDefinition {
  key: string
  title: string
  title_i18n?: LocalizedText
  sections?: SectionDefinition[]
}

export interface CardDefinition {
  key: string
  label: string
  label_i18n?: LocalizedText
  path: string
  widget?: string
  action_key?: string
}

export interface RelatedViewDefinition {
  key: string
  title: string
  title_i18n?: LocalizedText
  source: string
  empty_state?: string
  empty_state_i18n?: LocalizedText
}

export interface ActionPlacementDefinition {
  action_key: string
  zone: string
}

export interface ViewDefinition {
  key: string
  title: string
  title_i18n?: LocalizedText
  kind: string
  document_type?: string
  model_key?: string
  projection_key?: string
  dataset_key?: string
  printable?: boolean
  print_purpose?: string
  print_channel?: string
  allowed_actions?: string[]
  columns?: ColumnDefinition[]
  filters?: FilterDefinition[]
  sections?: SectionDefinition[]
  tabs?: TabDefinition[]
  fields?: FieldDefinition[]
  cards?: CardDefinition[]
  related_views?: RelatedViewDefinition[]
  action_placements?: ActionPlacementDefinition[]
  empty_state?: string
  empty_state_i18n?: LocalizedText
  default_page_size?: number
}

export interface CustomEntryDefinition {
  key: string
  title: string
  title_i18n?: LocalizedText
  route_path: string
  bundle_key: string
  component_export: string
  printable?: boolean
  print_target_kind?: string
  print_target_key?: string
  print_purpose?: string
  print_channel?: string
}

export interface DocumentFlowDocumentDefinition {
  key: string
  title: string
  title_i18n?: LocalizedText
  document_type: string
  tabs?: TabDefinition[]
  sections?: SectionDefinition[]
  fields?: FieldDefinition[]
}

export interface DocumentFlowBranchRule {
  path: string
  equals?: string
  in?: string[]
  truthy?: boolean
  next_step_key: string
}

export interface DocumentFlowStepDefinition {
  key: string
  title: string
  title_i18n?: LocalizedText
  documents?: DocumentFlowDocumentDefinition[]
  next_rules?: DocumentFlowBranchRule[]
  next_step_key?: string
}

export interface DocumentFlowDefinition {
  key: string
  title: string
  title_i18n?: LocalizedText
  route_path: string
  primary_document_type: string
  steps?: DocumentFlowStepDefinition[]
}

export interface ActionDefinition {
  key: string
  label: string
  label_i18n?: LocalizedText
  kind: string
  route_path: string
  view_key?: string
  custom_entry_key?: string
  flow_key?: string
  render_mode: 'generic' | 'custom' | 'flow'
}

export interface ShellRoute {
  key: string
  label: string
  path: string
  icon?: string
  parentKey?: string
  order?: number
}

export interface WorkspaceBootstrapResponse {
  shell_kind: 'workspace'
  surface: string
  available_surfaces: string[]
  menus: MenuDefinition[]
  actions: ActionDefinition[]
  views: ViewDefinition[]
  flows: DocumentFlowDefinition[]
  default_path: string
  preferred_path?: string
  fallback_paths?: Record<string, string>
  admin_access: boolean
  admin_path: string
  locale: string
  supported_locales: string[]
  auth_context: {
    actor_user_id: string
    effective_user_id: string
    delegation_active: boolean
    delegation_grant_id?: string
  }
  capabilities?: Record<string, unknown>
}

export interface AdminBootstrapResponse {
  shell_kind: 'admin'
  menus: MenuDefinition[]
  actions: ActionDefinition[]
  views: ViewDefinition[]
  custom_entries: CustomEntryDefinition[]
  default_path: string
  preferred_path?: string
  ui_access: boolean
  ui_path: string
  current_user_id: string
  organization?: Record<string, unknown>
  locations?: Array<Record<string, unknown>>
  operating_units?: Array<Record<string, unknown>>
  roles?: Array<Record<string, unknown>>
  locale: string
  supported_locales: string[]
}

type HttpError = Error & { status?: number }

function createHttpError(message: string, status: number): HttpError {
  return Object.assign(new Error(message), { status })
}

async function unwrapGeneratedResponse<T>(
  request: Promise<{ data?: T; error?: unknown; response: Response }>
): Promise<T> {
  const { data, response } = await request
  if (!response.ok || data === undefined) {
    throw createHttpError(`Request failed: ${response.status}`, response.status)
  }
  return data
}

export function normalizeShellPath(path: string, shellKind: ShellKind): string {
  const trimmed = String(path || '').trim()
  if (!trimmed) return '/'

  let candidate = trimmed
  const hashIndex = candidate.indexOf('#')
  if (hashIndex >= 0) {
    const hashPath = candidate.slice(hashIndex + 1).trim()
    if (hashPath) candidate = hashPath
  } else if (/^https?:\/\//i.test(candidate)) {
    try {
      candidate = new URL(candidate).pathname
    } catch {
      // fall through and normalize the original path string
    }
  }

  if (!candidate.startsWith('/')) candidate = `/${candidate}`
  if (shellKind === 'admin') {
    candidate = candidate.replace(/^\/admin(?=\/|$)/, '') || '/'
  } else {
    candidate = candidate.replace(/^\/ui(?=\/|$)/, '') || '/'
  }
  return candidate.startsWith('/') ? candidate : `/${candidate}`
}

export function toShellHref(path: string, shellKind: ShellKind): string {
  const normalized = normalizeShellPath(path, shellKind)
  return `${shellKind === 'admin' ? '/admin' : '/ui'}${normalized === '/' ? '' : normalized}`
}

export function toShellRoutes(menus: MenuDefinition[], actions: ActionDefinition[], locale = 'en', shellKind: ShellKind = 'workspace'): ShellRoute[] {
  return menus
    .map((menu) => {
      const action = actions.find((item) => item.key === menu.action_key)
      if (!action) return null
      return {
        key: menu.key,
        label: pickText(menu, 'label', locale),
        path: normalizeShellPath(action.route_path, shellKind),
        icon: menu.icon,
        parentKey: menu.parent_key,
        order: menu.order || 0,
      }
    })
    .sort((left, right) => (left?.order || 0) - (right?.order || 0) || left!.label.localeCompare(right!.label))
    .filter((item): item is NonNullable<typeof item> => item !== null)
}

export async function persistLocale(locale: string): Promise<string> {
  const response = await fetch(`/locale?locale=${encodeURIComponent(locale)}`, { credentials: 'include' })
  if (!response.ok) {
    throw new Error(`Locale error: ${response.status}`)
  }
  const payload = (await response.json()) as { locale?: string }
  return payload.locale || 'en'
}

export function pickText<T extends object>(
  item: T | null | undefined,
  key: string,
  locale = 'en'
): string {
  if (!item) return ''
  const source = item as Record<string, unknown>
  const localized = source[`${key}_i18n`] as LocalizedText | undefined
  if (localized) {
    const preferred = localized[locale] || localized.en || localized.id || ''
    if (preferred) return preferred
  }
  const direct = source[key]
  if (typeof direct === 'string' && direct.trim()) return direct
  return ''
}

export async function fetchWorkspaceBootstrap(surface?: string): Promise<WorkspaceBootstrapResponse> {
  return unwrapGeneratedResponse<WorkspaceBootstrapResponse>(getUIBootstrap(surface))
}

export async function fetchAdminBootstrap(): Promise<AdminBootstrapResponse> {
  return unwrapGeneratedResponse<AdminBootstrapResponse>(getAdminBootstrap())
}

export async function fetchAuthOptions(): Promise<AuthOptions> {
  return unwrapGeneratedResponse<AuthOptions>(getAuthOptions())
}
