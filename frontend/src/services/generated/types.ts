// Auto-generated types from OpenAPI spec
// This file should be regenerated when the OpenAPI spec changes

export interface AdminBootstrapResponse {
  actions: UIActionDefinition[]
  custom_entries: UICustomEntryDefinition[]
  default_path?: string
  locale?: string
  locations: Location[]
  menus: UIMenuDefinition[]
  organization: Organization
  roles: Role[]
  supported_locales: string[]
  ui_access?: boolean
  ui_path?: string
  views: UIViewDefinition[]
}

export interface AuthOptions {
  google_button_label?: string
  google_enabled?: boolean
  login_subtitle?: string
  login_title?: string
  password_enabled?: boolean
}

export interface CreateDocumentRequest {
  location_id?: string
  organization_id: string
  payload: Record<string, unknown>
  type: 'clinic_encounter' | 'clinic_registration' | 'generic_request'
}

export interface DocumentActionRequest {
  action: 'submit' | 'approve' | 'reject' | 'reopen' | 'cancel'
  expected_etag?: string
  expected_version?: number
}

export interface DocumentAttachment {
  attachment_type?: string
  content_type?: string
  created_at?: string
  document_id?: string
  file_name?: string
  id?: string
  size_bytes?: number
  storage_key?: string
}

export interface DocumentBody {
  content_hash?: string
  document_id?: string
  payload?: Record<string, unknown>
  schema_version?: string
}

export interface DocumentHeader {
  created_at?: string
  created_by?: string
  etag?: string
  id?: string
  location_id?: string
  organization_id?: string
  status?: DocumentStatus
  type?: string
  updated_at?: string
  updated_by?: string
  version?: number
}

export type DocumentStatus =
  | 'draft'
  | 'pending'
  | 'submitted'
  | 'approved'
  | 'rejected'
  | 'cancelled'
  | 'closed'

export interface UIActionDefinition {
  action: string
  handler?: string
  label?: string
  target?: string
}

export interface UICustomEntryDefinition {
  content_type?: string
  file_name?: string
  handler?: string
  icon?: string
  id?: string
  label?: string
  location_id?: string
  location_path?: string
  payload?: Record<string, unknown>
  type: string
}

export interface UIMenuDefinition {
  entries?: UIMenuEntry[]
  id?: string
  label?: string
  location_id?: string
  location_path?: string
}

export interface UIMenuEntry {
  action?: string
  badge?: string
  icon?: string
  id?: string
  label?: string
  location_id?: string
  location_path?: string
  target?: string
  url?: string
}

export interface UIViewDefinition {
  columns?: UIViewColumn[]
  filters?: Record<string, unknown>
  id?: string
  label?: string
  location_id?: string
  location_path?: string
  type?: string
}

export interface UIViewColumn {
  field?: string
  format?: string
  label?: string
  type?: string
  width?: number
}

export interface Location {
  children?: Location[]
  id?: string
  name?: string
  type?: string
}

export interface Organization {
  id?: string
  name?: string
  slug?: string
}

export interface Role {
  description?: string
  id?: string
  name?: string
  permissions?: string[]
}

export interface Workitem {
  id?: string
  document_id?: string
  document_type?: string
  status?: string
  assigned_to?: string
  due_date?: string
  priority?: string
  created_at?: string
  updated_at?: string
}

export interface Workflow {
  id?: string
  name?: string
  description?: string
  status?: string
  version?: number
  created_at?: string
  updated_at?: string
}

export interface User {
  id?: string
  email?: string
  name?: string
  avatar_url?: string
  roles?: string[]
  organization_id?: string
}

// OpenAPI paths interface for openapi-fetch
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
  '/bootstrap/ui': {
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
  '/bootstrap/admin': {
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
  '/documents': {
    get: {
      parameters: {
        query?: {
          location_id?: string
          status?: string
        }
      }
      responses: {
        200: {
          content: {
            'application/json': {
              data: DocumentHeader[]
            }
          }
        }
      }
    }
    post: {
      requestBody: {
        content: {
          'application/json': CreateDocumentRequest
        }
      }
      responses: {
        201: {
          content: {
            'application/json': DocumentHeader
          }
        }
      }
    }
  }
  '/documents/{id}': {
    get: {
      parameters: {
        path: {
          id: string
        }
      }
      responses: {
        200: {
          content: {
            'application/json': {
              data: DocumentHeader
            }
          }
        }
      }
    }
  }
  '/documents/{id}/body': {
    get: {
      parameters: {
        path: {
          id: string
        }
      }
      responses: {
        200: {
          content: {
            'application/json': {
              data: DocumentBody
            }
          }
        }
      }
    }
    put: {
      parameters: {
        path: {
          id: string
        }
      }
      requestBody: {
        content: {
          'application/json': {
            payload: Record<string, unknown>
          }
        }
      }
      responses: {
        200: {
          content: {
            'application/json': DocumentBody
          }
        }
      }
    }
  }
  '/documents/{id}/actions': {
    post: {
      parameters: {
        path: {
          id: string
        }
      }
      requestBody: {
        content: {
          'application/json': DocumentActionRequest
        }
      }
      responses: {
        200: {
          content: {
            'application/json': DocumentHeader
          }
        }
      }
    }
  }
  '/documents/{id}/attachments': {
    get: {
      parameters: {
        path: {
          id: string
        }
      }
      responses: {
        200: {
          content: {
            'application/json': {
              data: DocumentAttachment[]
            }
          }
        }
      }
    }
  }
  '/workitems': {
    get: {
      parameters: {
        query?: {
          assigned_to?: string
          status?: string
        }
      }
      responses: {
        200: {
          content: {
            'application/json': {
              data: Workitem[]
            }
          }
        }
      }
    }
  }
  '/workitems/{id}': {
    get: {
      parameters: {
        path: {
          id: string
        }
      }
      responses: {
        200: {
          content: {
            'application/json': {
              data: Workitem
            }
          }
        }
      }
    }
  }
  '/workitems/{id}/complete': {
    post: {
      parameters: {
        path: {
          id: string
        }
      }
      responses: {
        200: {
          content: {
            'application/json': Workitem
          }
        }
      }
    }
  }
  '/workflows': {
    get: {
      responses: {
        200: {
          content: {
            'application/json': {
              data: Workflow[]
            }
          }
        }
      }
    }
  }
  '/workflows/{id}': {
    get: {
      parameters: {
        path: {
          id: string
        }
      }
      responses: {
        200: {
          content: {
            'application/json': {
              data: Workflow
            }
          }
        }
      }
    }
  }
  '/users/me': {
    get: {
      responses: {
        200: {
          content: {
            'application/json': {
              data: User
            }
          }
        }
      }
    }
  }
}
