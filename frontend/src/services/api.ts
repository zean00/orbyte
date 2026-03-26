import createClient from 'openapi-fetch'
import type { paths } from './generated/types'

const baseUrl = '/api/v1'

export const apiClient = createClient<paths>({
  baseUrl,
  credentials: 'include',
})

export async function fetchWithAuth<T>(
  url: string,
  options?: RequestInit
): Promise<T> {
  const response = await fetch(`${baseUrl}${url}`, {
    ...options,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  })

  if (!response.ok) {
    throw new Error(`API error: ${response.status}`)
  }

  return response.json()
}
