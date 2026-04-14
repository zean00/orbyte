// Auto-generated types from OpenAPI spec.
// This file is intentionally kept in sync with contracts/openapi/latest/openapi.json.

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

export interface AuthOptions {
  google_button_label?: string
  google_enabled?: boolean
  login_subtitle?: string
  login_title?: string
  password_enabled?: boolean
  totp_enabled?: boolean
}

export interface UIBootstrapResponse {
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

export interface paths {
  '/auth/options': {
    get: {
      responses: {
        200: {
          content: {
            'application/json': AuthOptions
          }
        }
      }
    }
  }
  '/ui/bootstrap': {
    get: {
      parameters: {
        query?: {
          surface?: string
        }
      }
      responses: {
        200: {
          content: {
            'application/json': UIBootstrapResponse
          }
        }
      }
    }
  }
  '/admin/api/bootstrap': {
    get: {
      responses: {
        200: {
          content: {
            'application/json': AdminBootstrapResponse
          }
        }
      }
    }
  }
}
