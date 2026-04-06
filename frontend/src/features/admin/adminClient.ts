export function readCookie(name: string): string {
  const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const match = document.cookie.match(new RegExp(`(?:^|; )${escaped}=([^;]*)`))
  return match?.[1] ? decodeURIComponent(match[1]) : ''
}

export async function fetchJson<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    credentials: 'include',
    ...init,
    headers: {
      ...(init?.headers || {}),
    },
  })
  if (!response.ok) {
    const text = await response.text().catch(() => '')
    throw new Error(text || `${response.status} ${response.statusText}`)
  }
  return (await response.json()) as T
}

export async function mutateJson<T>(url: string, init?: RequestInit): Promise<T> {
  return fetchJson<T>(url, {
    ...init,
    headers: {
      'X-CSRF-Token': readCookie('orbyte_csrf'),
      ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
      ...(init?.headers || {}),
    },
  })
}

export async function fetchAllPagedItems<T>(
  baseUrl: string,
  pageSize = 200,
): Promise<T[]> {
  const items: T[] = []
  let page = 1
  while (true) {
    const separator = baseUrl.includes('?') ? '&' : '?'
    const payload = await fetchJson<{ items?: T[]; total?: number }>(
      `${baseUrl}${separator}page=${page}&page_size=${pageSize}`,
    )
    const batch = Array.isArray(payload.items) ? payload.items : []
    items.push(...batch)
    const total = typeof payload.total === 'number' ? payload.total : undefined
    if (batch.length === 0) break
    if (total !== undefined && items.length >= total) break
    if (batch.length < pageSize) break
    page += 1
  }
  return items
}

export function formatDateTime(value: string | undefined): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

export function startCase(value: string): string {
  return String(value || '')
    .replace(/[_-]+/g, ' ')
    .replace(/\b\w/g, (character) => character.toUpperCase())
}
