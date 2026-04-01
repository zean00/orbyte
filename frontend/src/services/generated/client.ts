// Auto-generated client from OpenAPI spec
// This file should be regenerated when the OpenAPI spec changes

import { apiClient } from '../api'

export async function getAuthOptions() {
  return apiClient.GET('/auth/options', { parseAs: 'json' })
}

export async function getUIBootstrap(surface?: string) {
  return apiClient.GET('/ui/bootstrap', {
    params: surface ? { query: { surface } } : undefined,
    parseAs: 'json',
  })
}

export async function getAdminBootstrap() {
  return apiClient.GET('/admin/api/bootstrap', { parseAs: 'json' })
}

// Re-export types for convenience
export type { paths } from './types'
