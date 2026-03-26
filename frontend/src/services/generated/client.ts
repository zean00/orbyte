// Auto-generated client from OpenAPI spec
// This file should be regenerated when the OpenAPI spec changes

import { apiClient } from '../api'

// Helper functions for common operations
export async function getAuthOptions() {
  return apiClient.GET('/auth/options', { parseAs: 'json' })
}

export async function getUIBootstrap() {
  return apiClient.GET('/bootstrap/ui', { parseAs: 'json' })
}

export async function getAdminBootstrap() {
  return apiClient.GET('/bootstrap/admin', { parseAs: 'json' })
}

export async function listDocuments(params?: { location_id?: string; status?: string }) {
  return apiClient.GET('/documents', { params: { query: params }, parseAs: 'json' })
}

export async function getDocument(id: string) {
  return apiClient.GET('/documents/{id}', { params: { path: { id } }, parseAs: 'json' })
}

export async function createDocument(data: {
  organization_id: string
  payload: Record<string, unknown>
  type: 'clinic_encounter' | 'clinic_registration' | 'generic_request'
}) {
  return apiClient.POST('/documents', { body: data, parseAs: 'json' })
}

export async function getDocumentBody(id: string) {
  return apiClient.GET('/documents/{id}/body', { params: { path: { id } }, parseAs: 'json' })
}

export async function updateDocumentBody(id: string, data: { payload: Record<string, unknown> }) {
  return apiClient.PUT('/documents/{id}/body', { params: { path: { id } }, body: data, parseAs: 'json' })
}

export async function performDocumentAction(
  id: string,
  data: { action: 'submit' | 'approve' | 'reject' | 'reopen' | 'cancel' }
) {
  return apiClient.POST('/documents/{id}/actions', { params: { path: { id } }, body: data, parseAs: 'json' })
}

export async function getDocumentAttachments(id: string) {
  return apiClient.GET('/documents/{id}/attachments', { params: { path: { id } }, parseAs: 'json' })
}

export async function listWorkitems(params?: { assigned_to?: string; status?: string }) {
  return apiClient.GET('/workitems', { params: { query: params }, parseAs: 'json' })
}

export async function getWorkitem(id: string) {
  return apiClient.GET('/workitems/{id}', { params: { path: { id } }, parseAs: 'json' })
}

export async function completeWorkitem(id: string) {
  return apiClient.POST('/workitems/{id}/complete', { params: { path: { id } }, parseAs: 'json' })
}

export async function listWorkflows() {
  return apiClient.GET('/workflows', { parseAs: 'json' })
}

export async function getWorkflow(id: string) {
  return apiClient.GET('/workflows/{id}', { params: { path: { id } }, parseAs: 'json' })
}

export async function getCurrentUser() {
  return apiClient.GET('/users/me', { parseAs: 'json' })
}

// Re-export types for convenience
export type { paths } from './types'
