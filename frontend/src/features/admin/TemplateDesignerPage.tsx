import type { CSSProperties, ReactNode } from 'react'
import { cloneElement, isValidElement, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { fetchJson, formatDateTime, mutateJson } from './adminClient'

type TemplateDefinition = {
  key: string
  title?: string
  target_kind?: string
  target_key?: string
  renderer_kind?: string
  default_format?: string
  purpose?: string
  channel?: string
  related_sources?: RelatedSourceDraft[]
}

type TemplateVersion = {
  template_key: string
  version: number
  status: string
  renderer_kind?: string
  body: string
  style?: string
  change_note?: string
  updated_at?: string
  updated_by?: string
  published_at?: string
  published_by?: string
}

type TemplateBinding = {
  id: string
  scope_type: string
  scope_id?: string
  target_kind: string
  target_key: string
  purpose?: string
  channel?: string
  is_default?: boolean
  is_official?: boolean
}

type BindingDraft = {
  id?: string
  scope_type: string
  scope_id: string
  target_kind: string
  target_key: string
  purpose: string
  channel: string
  is_default: boolean
  is_official: boolean
}

type RelatedSourceDraft = {
  key: string
  label?: string
  source_kind?: string
  target_kind: string
  target_key: string
  relation_mode?: string
  max_depth?: number
  document_id_path?: string
}

type TargetOption = {
  key: string
  title?: string
}

type TemplateTargetCatalog = {
  documents?: TargetOption[]
  reports?: TargetOption[]
}

type TemplateFixture = {
  fixture_key: string
  name?: string
  source_type?: string
}

type TemplateColumn = {
  label: string
  path: string
}

type TemplateBlock = {
  id: string
  type: string
  label?: string
  text?: string
  path?: string
  rows_path?: string
  columns?: TemplateColumn[]
  align?: string
  font_size?: string
  emphasis?: string
  visible_if?: string
  value?: string
  image_url?: string
  alt?: string
  format?: string
}

type TemplateCell = {
  id: string
  span: number
  width?: string
  height?: string
  align_x?: 'start' | 'center' | 'end' | 'stretch' | ''
  align_y?: 'start' | 'center' | 'end' | 'stretch' | ''
  content_align_x?: 'start' | 'center' | 'end' | ''
  content_align_y?: 'start' | 'center' | 'end' | 'stretch' | ''
  blocks: TemplateBlock[]
}

type TemplateRow = {
  id: string
  width?: string
  height?: string
  align_x?: 'start' | 'center' | 'end' | 'stretch' | ''
  align_y?: 'start' | 'center' | 'end' | 'stretch' | ''
  content_align_x?: 'start' | 'center' | 'end' | ''
  content_align_y?: 'start' | 'center' | 'end' | 'stretch' | ''
  columns: TemplateCell[]
}

type TemplateSection = {
  id: string
  title: string
  kind?: string
  rows: TemplateRow[]
}

type TemplateLayout = {
  schema_version?: string
  title?: string
  settings?: {
    paper_preset?: string
    orientation?: string
    density?: string
    margins?: string
    show_grid?: boolean
  }
  sections: TemplateSection[]
}

type TemplatePreview = {
  outputs?: Array<{
    html?: string
    status?: string
    format?: string
  }>
  warnings?: Array<{ message?: string; code?: string }>
  issues?: Array<{ message?: string; code?: string }>
  binding_resolution?: {
    definition_key?: string
    version?: number
    scope_path?: Array<{ scope_type: string; scope_id?: string }>
    matched_binding?: { template_key?: string; scope_type?: string; scope_id?: string }
  }
}

type TemplateCompare = {
  template_key: string
  changed_fields: string[]
  has_differences: boolean
}

const PALETTE: Array<{ type: string; label: string }> = [
  { type: 'text', label: 'Text' },
  { type: 'field', label: 'Field' },
  { type: 'table', label: 'Table' },
  { type: 'totals', label: 'Totals' },
  { type: 'divider', label: 'Divider' },
  { type: 'image', label: 'Image' },
  { type: 'barcode', label: 'Barcode' },
  { type: 'signature', label: 'Signature' },
]

const WIDTH_PRESETS = [
  { value: '', label: 'Auto / Span-based' },
  { value: '100%', label: 'Full' },
  { value: '50%', label: 'Half' },
  { value: '33.333%', label: 'Third' },
  { value: '25%', label: 'Quarter' },
]

const HEIGHT_PRESETS = [
  { value: '', label: 'Auto' },
  { value: '120px', label: 'Compact' },
  { value: '180px', label: 'Medium' },
  { value: '240px', label: 'Tall' },
]

function nextDesignerID(prefix: string): string {
  return `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 7)}`
}

function createTemplateBlock(type: string): TemplateBlock {
  const id = nextDesignerID('block')
  switch (type) {
    case 'field':
      return { id, type, label: 'Field', path: 'document.header.number' }
    case 'table':
      return { id, type, label: 'Rows', rows_path: 'document.lines', columns: [{ label: 'Label', path: 'label' }, { label: 'Amount', path: 'amount' }] }
    case 'totals':
      return { id, type, label: 'Total', rows_path: 'document.lines', path: 'amount' }
    case 'divider':
      return { id, type }
    case 'image':
      return { id, type, label: 'Image', image_url: '' }
    case 'barcode':
      return { id, type, label: 'Barcode', path: 'document.header.number' }
    case 'signature':
      return { id, type, label: 'Signature' }
    default:
      return { id, type: 'text', text: 'New text block' }
  }
}

function normalizeAlignX(value: unknown): TemplateRow['align_x'] {
  switch (String(value || '').trim().toLowerCase()) {
    case 'center':
      return 'center'
    case 'end':
      return 'end'
    case 'stretch':
      return 'stretch'
    case 'start':
      return 'start'
    default:
      return ''
  }
}

function normalizeAlignY(value: unknown): TemplateRow['align_y'] {
  switch (String(value || '').trim().toLowerCase()) {
    case 'center':
      return 'center'
    case 'end':
      return 'end'
    case 'stretch':
      return 'stretch'
    case 'start':
      return 'start'
    default:
      return ''
  }
}

function normalizeContentAlignX(value: unknown): TemplateRow['content_align_x'] {
  switch (String(value || '').trim().toLowerCase()) {
    case 'center':
      return 'center'
    case 'end':
      return 'end'
    case 'start':
      return 'start'
    default:
      return ''
  }
}

function normalizeContentAlignY(value: unknown): TemplateRow['content_align_y'] {
  switch (String(value || '').trim().toLowerCase()) {
    case 'center':
      return 'center'
    case 'end':
      return 'end'
    case 'stretch':
      return 'stretch'
    case 'start':
      return 'start'
    default:
      return ''
  }
}

function normalizeLength(value: unknown): string {
  return String(value || '').trim()
}

function cssPosition(value: string, fallback: 'start' | 'stretch' = 'start'): CSSProperties['justifyContent'] {
  switch (value) {
    case 'center':
      return 'center'
    case 'end':
      return 'flex-end'
    case 'stretch':
      return fallback === 'stretch' ? 'stretch' : 'flex-start'
    default:
      return fallback === 'stretch' ? 'stretch' : 'flex-start'
  }
}

function cssItemPosition(value: string, fallback: 'start' | 'stretch' = 'start'): 'start' | 'center' | 'end' | 'stretch' {
  switch (value) {
    case 'center':
      return 'center'
    case 'end':
      return 'end'
    case 'stretch':
      return 'stretch'
    default:
      return fallback
  }
}

function rowShellStyle(row: TemplateRow): CSSProperties {
  return {
    display: 'flex',
    width: '100%',
    minHeight: row.height || undefined,
    justifyContent: cssPosition(row.align_x || ''),
    alignItems: cssPosition(row.align_y || '', 'stretch') as CSSProperties['alignItems'],
  }
}

function rowCanvasStyle(row: TemplateRow): CSSProperties {
  return {
    display: 'grid',
    gridTemplateColumns: 'repeat(12, minmax(0, 1fr))',
    gap: '12px',
    width: row.width || '100%',
    minHeight: row.height || undefined,
    justifyItems: cssItemPosition(row.content_align_x || '', 'stretch'),
    alignItems: cssItemPosition(row.content_align_y || '', 'stretch'),
  }
}

function cellCanvasStyle(cell: TemplateCell): CSSProperties {
  const style: CSSProperties = {
    display: 'flex',
    flexDirection: 'column',
    gap: '12px',
    minHeight: cell.height || undefined,
    justifyContent: cssPosition(cell.content_align_y || '') as CSSProperties['justifyContent'],
    alignItems: cssItemPosition(cell.content_align_x || '', 'stretch'),
    alignSelf: cssItemPosition(cell.align_y || '', 'stretch'),
  }
  if (cell.width) {
    style.gridColumn = 'auto'
    style.width = cell.width
    style.justifySelf = cssItemPosition(cell.align_x || '')
  } else {
    style.gridColumn = `span ${cell.span} / span ${cell.span}`
    style.width = '100%'
    style.justifySelf = 'stretch'
  }
  return style
}

function templateDefaultLayout(definition: TemplateDefinition | null): TemplateLayout {
  const title = definition?.title || definition?.key || 'Template'
  const bodyBlock =
    definition?.target_kind === 'report'
      ? { id: 'body-main', type: 'table', rows_path: 'report.rows', columns: [{ label: 'Label', path: 'label' }, { label: 'Total', path: 'total' }] }
      : { id: 'body-main', type: 'field', label: 'Document Number', path: 'document.header.number' }
  return {
    schema_version: 'visual-grid/v1',
    title,
    settings: { paper_preset: 'a4', orientation: 'portrait', density: 'comfortable' },
    sections: [
      { id: 'header', title: 'Header', kind: 'header', rows: [{ id: 'header-row-1', columns: [{ id: 'header-row-1-cell-1', span: 12, blocks: [{ id: 'header-title', type: 'text', text: title, font_size: 'xl', emphasis: 'strong' }] }] }] },
      { id: 'body', title: 'Body', kind: 'body', rows: [{ id: 'body-row-1', columns: [{ id: 'body-row-1-cell-1', span: 12, blocks: [bodyBlock] }] }] },
      { id: 'footer', title: 'Footer', kind: 'footer', rows: [{ id: 'footer-row-1', columns: [{ id: 'footer-row-1-cell-1', span: 12, blocks: [{ id: 'footer-note', type: 'text', text: 'Prepared by Orbyte', align: 'right', emphasis: 'muted' }] }] }] },
    ],
  }
}

function normalizeLayout(layout: unknown, definition: TemplateDefinition | null): TemplateLayout {
  const fallback = templateDefaultLayout(definition)
  const base = layout && typeof layout === 'object' ? (layout as Partial<TemplateLayout>) : fallback
  const sections = Array.isArray(base.sections) && base.sections.length ? base.sections : fallback.sections
  return {
    schema_version: base.schema_version || 'visual-grid/v1',
    title: base.title || fallback.title,
    settings: {
      paper_preset: base.settings?.paper_preset || 'a4',
      orientation: base.settings?.orientation || 'portrait',
      density: base.settings?.density || 'comfortable',
      margins: base.settings?.margins || '',
      show_grid: Boolean(base.settings?.show_grid),
    },
    sections: sections.map((section, sectionIndex) => {
      const sectionID = section.id || ['header', 'body', 'footer'][sectionIndex] || `section-${sectionIndex + 1}`
      return {
        id: sectionID,
        title: section.title || ['Header', 'Body', 'Footer'][sectionIndex] || `Section ${sectionIndex + 1}`,
        kind: section.kind || section.id || 'body',
        rows: (Array.isArray(section.rows) && section.rows.length ? section.rows : [{ id: '', columns: [{ id: '', span: 12, blocks: [] }] }]).map((row, rowIndex) => {
          const rowID = row.id || `${sectionID}-row-${rowIndex + 1}`
          return {
            id: rowID,
            width: normalizeLength((row as TemplateRow).width),
            height: normalizeLength((row as TemplateRow).height),
            align_x: normalizeAlignX((row as TemplateRow).align_x),
            align_y: normalizeAlignY((row as TemplateRow).align_y),
            content_align_x: normalizeContentAlignX((row as TemplateRow).content_align_x),
            content_align_y: normalizeContentAlignY((row as TemplateRow).content_align_y),
            columns: (Array.isArray(row.columns) && row.columns.length ? row.columns : [{ id: '', span: 12, blocks: [] }]).map((column, columnIndex) => {
              const columnID = column.id || `${rowID}-cell-${columnIndex + 1}`
              return {
                id: columnID,
                span: Math.min(12, Math.max(1, Number(column.span) || 12)),
                width: normalizeLength((column as TemplateCell).width),
                height: normalizeLength((column as TemplateCell).height),
                align_x: normalizeAlignX((column as TemplateCell).align_x),
                align_y: normalizeAlignY((column as TemplateCell).align_y),
                content_align_x: normalizeContentAlignX((column as TemplateCell).content_align_x),
                content_align_y: normalizeContentAlignY((column as TemplateCell).content_align_y),
                blocks: Array.isArray(column.blocks)
                  ? column.blocks.map((block, blockIndex) => ({
                      id: block.id || `${columnID}-block-${blockIndex + 1}`,
                      type: block.type || 'text',
                      label: block.label || '',
                      text: block.text || '',
                      path: block.path || '',
                      rows_path: block.rows_path || '',
                      columns: Array.isArray(block.columns) ? block.columns : [],
                      align: block.align || '',
                      font_size: block.font_size || '',
                      emphasis: block.emphasis || '',
                      visible_if: block.visible_if || '',
                      value: block.value || '',
                      image_url: block.image_url || '',
                      alt: block.alt || '',
                      format: block.format || '',
                    }))
                  : [],
              }
            }),
          }
        }),
      }
    }),
  }
}

function parseTemplateBody(definition: TemplateDefinition | null, body: string): TemplateLayout | null {
  if ((definition?.renderer_kind || '').toLowerCase() !== 'visual') return null
  try {
    return normalizeLayout(JSON.parse(body || '{}'), definition)
  } catch {
    return templateDefaultLayout(definition)
  }
}

function cloneLayout(layout: TemplateLayout): TemplateLayout {
  return JSON.parse(JSON.stringify(layout)) as TemplateLayout
}

function withCurrentTarget(options: TargetOption[], currentKey: string): TargetOption[] {
  if (!currentKey) return options
  if (options.some((item) => item.key === currentKey)) return options
  return [{ key: currentKey, title: currentKey }, ...options]
}

export function TemplateDesignerPage() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const [definitions, setDefinitions] = useState<TemplateDefinition[]>([])
  const [selectedKey, setSelectedKey] = useState(searchParams.get('key') || '')
  const [versions, setVersions] = useState<TemplateVersion[]>([])
  const [selectedVersion, setSelectedVersion] = useState<number | null>(null)
  const [draft, setDraft] = useState<TemplateVersion | null>(null)
  const [layout, setLayout] = useState<TemplateLayout | null>(null)
  const [selectedSectionID, setSelectedSectionID] = useState('body')
  const [selectedRowID, setSelectedRowID] = useState('')
  const [selectedCellID, setSelectedCellID] = useState('')
  const [selectedBlockID, setSelectedBlockID] = useState('')
  const [activeSelection, setActiveSelection] = useState<'canvas' | 'row' | 'cell' | 'block'>('canvas')
  const [bindings, setBindings] = useState<TemplateBinding[]>([])
  const [targetCatalog, setTargetCatalog] = useState<TemplateTargetCatalog>({})
  const [definitionDraft, setDefinitionDraft] = useState({
    target_kind: 'document',
    target_key: '',
    purpose: '',
    channel: '',
    related_sources: [] as RelatedSourceDraft[],
  })
  const [bindingDraft, setBindingDraft] = useState<BindingDraft>({
    scope_type: 'deployment',
    scope_id: '',
    target_kind: '',
    target_key: '',
    purpose: '',
    channel: '',
    is_default: false,
    is_official: false,
  })
  const [fixtures, setFixtures] = useState<TemplateFixture[]>([])
  const [fixtureKey, setFixtureKey] = useState('')
  const [preview, setPreview] = useState<TemplatePreview | null>(null)
  const [comparison, setComparison] = useState<TemplateCompare | null>(null)
  const [compareLeft, setCompareLeft] = useState<number | null>(null)
  const [compareRight, setCompareRight] = useState<number | null>(null)
  const [changeNote, setChangeNote] = useState('')
  const [message, setMessage] = useState('')
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [activePanel, setActivePanel] = useState<'design' | 'preview' | 'advanced'>('design')

  function messageForError(error: unknown): string {
    return error instanceof Error && error.message ? error.message : 'Request failed.'
  }

  useEffect(() => {
    let mounted = true
    async function loadDefinitions() {
      try {
        const [payload, catalog] = await Promise.all([
          fetchJson<{ items: TemplateDefinition[] }>('/admin/api/templates/definitions'),
          fetchJson<TemplateTargetCatalog>('/admin/api/template-targets'),
        ])
        if (!mounted) return
        setDefinitions(payload.items || [])
        setTargetCatalog(catalog || {})
        setSelectedKey((current) => current || searchParams.get('key') || payload.items?.[0]?.key || '')
      } catch (error) {
        if (!mounted) return
        setMessage(messageForError(error))
      } finally {
        if (mounted) setLoading(false)
      }
    }
    void loadDefinitions()
    return () => {
      mounted = false
    }
  }, [searchParams])

  useEffect(() => {
    const key = searchParams.get('key') || ''
    if (key && key !== selectedKey) {
      setSelectedKey(key)
    }
  }, [searchParams, selectedKey])

  const selectedDefinition = useMemo(
    () => definitions.find((item) => item.key === selectedKey) || null,
    [definitions, selectedKey],
  )

  const availableTargetOptions = useMemo(
    () =>
      withCurrentTarget(
        definitionDraft.target_kind === 'report' ? targetCatalog.reports || [] : targetCatalog.documents || [],
        definitionDraft.target_key,
      ),
    [definitionDraft.target_key, definitionDraft.target_kind, targetCatalog.documents, targetCatalog.reports],
  )

  const bindingTargetOptions = useMemo(
    () =>
      withCurrentTarget(
        bindingDraft.target_kind === 'report' ? targetCatalog.reports || [] : targetCatalog.documents || [],
        bindingDraft.target_key,
      ),
    [bindingDraft.target_key, bindingDraft.target_kind, targetCatalog.documents, targetCatalog.reports],
  )

  useEffect(() => {
    let mounted = true
    async function loadContext() {
      if (!selectedKey || !selectedDefinition) return
      try {
        const [versionsPayload, bindingsPayload, fixturesPayload] = await Promise.all([
          fetchJson<{ items: TemplateVersion[] }>(`/admin/api/templates/versions?template_key=${encodeURIComponent(selectedKey)}`),
          fetchJson<{ items: TemplateBinding[] }>(`/admin/api/template-bindings?template_key=${encodeURIComponent(selectedKey)}`),
          fetchJson<{ items: TemplateFixture[] }>(
            `/admin/api/template-fixtures?template_key=${encodeURIComponent(selectedKey)}&target_kind=${encodeURIComponent(selectedDefinition.target_kind || '')}`,
          ),
        ])
        if (!mounted) return
        const orderedVersions = versionsPayload.items || []
        const preferredDraft = orderedVersions.find((item) => item.status === 'draft') || orderedVersions[orderedVersions.length - 1] || null
        setVersions(orderedVersions)
        setBindings(bindingsPayload.items || [])
        setBindingDraft({
          scope_type: 'deployment',
          scope_id: '',
          target_kind: selectedDefinition.target_kind || '',
          target_key: selectedDefinition.target_key || '',
          purpose: selectedDefinition.purpose || '',
          channel: selectedDefinition.channel || '',
          is_default: false,
          is_official: false,
        })
        setDefinitionDraft({
          target_kind: selectedDefinition.target_kind || 'document',
          target_key: selectedDefinition.target_key || '',
          purpose: selectedDefinition.purpose || '',
          channel: selectedDefinition.channel || '',
          related_sources: (selectedDefinition.related_sources || []).map((item, index) => ({
            key: item.key || `related_${index + 1}`,
            label: item.label || '',
            source_kind: item.source_kind || ((selectedDefinition.target_kind || '').toLowerCase() === 'report' ? 'report_row_document' : 'document_link'),
            target_kind: item.target_kind || 'document',
            target_key: item.target_key || '',
            relation_mode: item.relation_mode || 'direct',
            max_depth: item.max_depth || 1,
            document_id_path: item.document_id_path || 'document_id',
          })),
        })
        setFixtures(fixturesPayload.items || [])
        setFixtureKey((current) => current || fixturesPayload.items?.[0]?.fixture_key || '')
        setDraft(preferredDraft)
        setSelectedVersion(preferredDraft?.version || orderedVersions[orderedVersions.length - 1]?.version || null)
        const nextLayout = preferredDraft ? parseTemplateBody(selectedDefinition, preferredDraft.body) : templateDefaultLayout(selectedDefinition)
        setLayout(nextLayout)
        setSelectedSectionID('body')
        const defaultSection = nextLayout?.sections.find((item) => item.id === 'body') || nextLayout?.sections[0]
        const defaultRow = defaultSection?.rows[0]
        const defaultCell = defaultRow?.columns[0]
        setSelectedRowID(defaultRow?.id || '')
        setSelectedCellID(defaultCell?.id || '')
        setSelectedBlockID('')
        setActiveSelection('canvas')
        setPreview(null)
        setComparison(null)
        setCompareLeft(orderedVersions[0]?.version || null)
        setCompareRight(preferredDraft?.version || orderedVersions[orderedVersions.length - 1]?.version || null)
        setChangeNote(preferredDraft?.change_note || '')
        setMessage('')
      } catch (error) {
        if (!mounted) return
        setMessage(messageForError(error))
      }
    }
    void loadContext()
    return () => {
      mounted = false
    }
  }, [selectedDefinition, selectedKey])

  const activeVersion = useMemo(
    () => versions.find((item) => item.version === selectedVersion) || draft,
    [draft, selectedVersion, versions],
  )

  const bodyValue = useMemo(() => {
    if ((selectedDefinition?.renderer_kind || '').toLowerCase() === 'visual' && layout) {
      return JSON.stringify(layout, null, 2)
    }
    return draft?.body || ''
  }, [draft?.body, layout, selectedDefinition?.renderer_kind])

  const styleValue = draft?.style || ''

  const selectedSection = useMemo(
    () => layout?.sections.find((item) => item.id === selectedSectionID) || layout?.sections[0] || null,
    [layout, selectedSectionID],
  )

  const selectedRow = useMemo(
    () => selectedSection?.rows.find((item) => item.id === selectedRowID) || selectedSection?.rows[0] || null,
    [selectedRowID, selectedSection],
  )

  const selectedCell = useMemo(
    () => selectedRow?.columns.find((item) => item.id === selectedCellID) || selectedRow?.columns[0] || null,
    [selectedCellID, selectedRow],
  )

  const selectedBlockRef = useMemo(() => {
    if (!layout || !selectedBlockID) return null
    for (const section of layout.sections) {
      for (const row of section.rows) {
        for (const column of row.columns) {
          for (const block of column.blocks) {
            if (block.id === selectedBlockID) {
              return { section, row, column, block }
            }
          }
        }
      }
    }
    return null
  }, [layout, selectedBlockID])

  useEffect(() => {
    if (!selectedSection) return
    const nextRow = selectedSection.rows.find((item) => item.id === selectedRowID) || selectedSection.rows[0] || null
    const nextCell = nextRow?.columns.find((item) => item.id === selectedCellID) || nextRow?.columns[0] || null
    if ((nextRow?.id || '') !== selectedRowID) setSelectedRowID(nextRow?.id || '')
    if ((nextCell?.id || '') !== selectedCellID) setSelectedCellID(nextCell?.id || '')
  }, [selectedCellID, selectedRowID, selectedSection])

  useEffect(() => {
    if (!selectedBlockRef) return
    setSelectedSectionID(selectedBlockRef.section.id)
    setSelectedRowID(selectedBlockRef.row.id)
    setSelectedCellID(selectedBlockRef.column.id)
  }, [selectedBlockRef])

  useEffect(() => {
    if (activeSelection === 'block' && !selectedBlockRef) {
      setActiveSelection(selectedCell ? 'cell' : selectedRow ? 'row' : 'canvas')
    }
    if (activeSelection === 'cell' && !selectedCell) {
      setActiveSelection(selectedRow ? 'row' : 'canvas')
    }
    if (activeSelection === 'row' && !selectedRow) {
      setActiveSelection('canvas')
    }
  }, [activeSelection, selectedBlockRef, selectedCell, selectedRow])

  function updateLayout(mutator: (next: TemplateLayout) => void) {
    if (!layout) return
    const next = cloneLayout(layout)
    mutator(next)
    setLayout(next)
  }

  function setDraftBody(nextBody: string) {
    setDraft((current) => (current ? { ...current, body: nextBody } : null))
  }

  function setDraftStyle(nextStyle: string) {
    setDraft((current) => (current ? { ...current, style: nextStyle } : current))
  }

  function syncVisualBody(nextLayout: TemplateLayout) {
    setLayout(nextLayout)
    setDraft((current) => (current ? { ...current, body: JSON.stringify(nextLayout, null, 2) } : current))
  }

  async function saveDraft() {
    if (!selectedDefinition) return
    setBusy(true)
    try {
      const payload = await mutateJson<{ version: TemplateVersion }>(`/admin/api/templates/${encodeURIComponent(selectedDefinition.key)}/actions/draft`, {
        method: 'PUT',
        body: JSON.stringify({
          body: bodyValue,
          style: styleValue,
          change_note: changeNote,
        }),
      })
      setDraft(payload.version)
      setSelectedVersion(payload.version.version)
      setVersions((current) => {
        const items = current.filter((item) => item.version !== payload.version.version)
        return [...items, payload.version].sort((left, right) => left.version - right.version)
      })
      setMessage('Draft saved.')
    } catch (error) {
      setMessage(messageForError(error))
    } finally {
      setBusy(false)
    }
  }

  async function duplicateDraft() {
    if (!selectedDefinition) return
    setBusy(true)
    try {
      const payload = await mutateJson<{ version: TemplateVersion }>(`/admin/api/templates/${encodeURIComponent(selectedDefinition.key)}/actions/duplicate-draft`, {
        method: 'POST',
        body: JSON.stringify({ from_version: activeVersion?.version || 0 }),
      })
      setDraft(payload.version)
      setSelectedVersion(payload.version.version)
      setLayout(parseTemplateBody(selectedDefinition, payload.version.body))
      setVersions((current) => [...current.filter((item) => item.version !== payload.version.version), payload.version].sort((left, right) => left.version - right.version))
      setMessage('Draft duplicated.')
    } catch (error) {
      setMessage(messageForError(error))
    } finally {
      setBusy(false)
    }
  }

  async function resetDraft() {
    if (!selectedDefinition) return
    setBusy(true)
    try {
      const payload = await mutateJson<{ version: TemplateVersion }>(`/admin/api/templates/${encodeURIComponent(selectedDefinition.key)}/actions/reset-draft`, {
        method: 'POST',
      })
      setDraft(payload.version)
      setSelectedVersion(payload.version.version)
      setLayout(parseTemplateBody(selectedDefinition, payload.version.body))
      setMessage('Draft reset to the published version.')
    } catch (error) {
      setMessage(messageForError(error))
    } finally {
      setBusy(false)
    }
  }

  async function validateDraft() {
    if (!selectedDefinition) return
    setBusy(true)
    try {
      const payload = await mutateJson<{ valid: boolean; issues?: Array<{ message?: string; code?: string }> }>('/admin/api/templates/validate', {
        method: 'POST',
        body: JSON.stringify({
          template_key: selectedDefinition.key,
          body: bodyValue,
          style: styleValue,
          target_kind: selectedDefinition.target_kind,
          target_key: selectedDefinition.target_key,
          fixture_key: fixtureKey,
          draft: true,
        }),
      })
      setMessage(payload.valid ? 'Template is valid.' : (payload.issues || []).map((item) => item.message || item.code).filter(Boolean).join('; ') || 'Validation failed.')
    } catch (error) {
      setMessage(messageForError(error))
    } finally {
      setBusy(false)
    }
  }

  async function previewDraft() {
    if (!selectedDefinition) return
    setBusy(true)
    try {
      const payload = await mutateJson<TemplatePreview>('/admin/api/templates/preview', {
        method: 'POST',
        body: JSON.stringify({
          template_key: selectedDefinition.key,
          body: bodyValue,
          style: styleValue,
          target_kind: selectedDefinition.target_kind,
          target_key: selectedDefinition.target_key,
          fixture_key: fixtureKey,
          sample: selectedDefinition.target_kind === 'report' && !fixtureKey,
          format: selectedDefinition.default_format || 'html',
          draft: true,
        }),
      })
      setPreview(payload)
      setActivePanel('preview')
      setMessage('Preview generated.')
    } catch (error) {
      setMessage(messageForError(error))
    } finally {
      setBusy(false)
    }
  }

  async function compareVersions() {
    if (!selectedDefinition || !compareLeft || !compareRight) return
    setBusy(true)
    try {
      const payload = await fetchJson<{ comparison: TemplateCompare }>(
        `/admin/api/templates/compare?template_key=${encodeURIComponent(selectedDefinition.key)}&left=${compareLeft}&right=${compareRight}`,
      )
      setComparison(payload.comparison)
      setActivePanel('advanced')
      setMessage('Version comparison loaded.')
    } catch (error) {
      setMessage(messageForError(error))
    } finally {
      setBusy(false)
    }
  }

  async function publishVersion() {
    if (!selectedDefinition || !activeVersion) return
    setBusy(true)
    try {
      const payload = await mutateJson<{ version: TemplateVersion }>(
        `/admin/api/templates/${encodeURIComponent(selectedDefinition.key)}/versions/${activeVersion.version}/publish`,
        { method: 'POST' },
      )
      setVersions((current) => current.map((item) => (item.version === payload.version.version ? payload.version : item)))
      setMessage(`Published version ${payload.version.version}.`)
    } catch (error) {
      setMessage(messageForError(error))
    } finally {
      setBusy(false)
    }
  }

  async function saveDefinitionSettings() {
    if (!selectedDefinition) return
    setBusy(true)
    try {
      const payload = await mutateJson<{ definition: TemplateDefinition }>(`/admin/api/templates/definitions/${encodeURIComponent(selectedDefinition.key)}`, {
        method: 'PUT',
        body: JSON.stringify({
          title: selectedDefinition.title,
          target_kind: definitionDraft.target_kind,
          target_key: definitionDraft.target_key,
          renderer_kind: selectedDefinition.renderer_kind,
          default_format: selectedDefinition.default_format,
          purpose: definitionDraft.purpose,
          channel: definitionDraft.channel,
          related_sources: definitionDraft.related_sources,
        }),
      })
      setDefinitions((current) => current.map((item) => (item.key === payload.definition.key ? payload.definition : item)))
      setBindingDraft((current) => ({
        ...current,
        target_kind: payload.definition.target_kind || current.target_kind,
        target_key: payload.definition.target_key || current.target_key,
        purpose: payload.definition.purpose || '',
        channel: payload.definition.channel || '',
      }))
      setMessage('Template data sources saved.')
    } catch (error) {
      setMessage(messageForError(error))
    } finally {
      setBusy(false)
    }
  }

  function addRelatedSource() {
    setDefinitionDraft((current) => ({
      ...current,
      related_sources: [
        ...current.related_sources,
        {
          key: `related_${current.related_sources.length + 1}`,
          label: '',
          source_kind: current.target_kind === 'report' ? 'report_row_document' : 'document_link',
          target_kind: 'document',
          target_key: targetCatalog.documents?.[0]?.key || '',
          relation_mode: 'direct',
          max_depth: 1,
          document_id_path: 'document_id',
        },
      ],
    }))
  }

  function updateRelatedSource(index: number, updater: (item: RelatedSourceDraft) => RelatedSourceDraft) {
    setDefinitionDraft((current) => ({
      ...current,
      related_sources: current.related_sources.map((item, itemIndex) => (itemIndex === index ? updater(item) : item)),
    }))
  }

  function removeRelatedSource(index: number) {
    setDefinitionDraft((current) => ({
      ...current,
      related_sources: current.related_sources.filter((_, itemIndex) => itemIndex !== index),
    }))
  }

  async function saveBinding() {
    if (!selectedDefinition) return
    setBusy(true)
    try {
      const payload = await mutateJson<{ binding: TemplateBinding }>('/admin/api/template-bindings', {
        method: 'PUT',
        body: JSON.stringify({
          id: bindingDraft.id,
          template_key: selectedDefinition.key,
          scope_type: bindingDraft.scope_type,
          scope_id: bindingDraft.scope_id,
          target_kind: bindingDraft.target_kind,
          target_key: bindingDraft.target_key,
          purpose: bindingDraft.purpose,
          channel: bindingDraft.channel,
          is_default: bindingDraft.is_default,
          is_official: bindingDraft.is_official,
        }),
      })
      setBindings((current) => {
        const next = current.filter((item) => item.id !== payload.binding.id)
        return [payload.binding, ...next]
      })
      setBindingDraft({
        id: '',
        scope_type: 'deployment',
        scope_id: '',
        target_kind: definitionDraft.target_kind || selectedDefinition.target_kind || '',
        target_key: definitionDraft.target_key || selectedDefinition.target_key || '',
        purpose: definitionDraft.purpose || selectedDefinition.purpose || '',
        channel: definitionDraft.channel || selectedDefinition.channel || '',
        is_default: false,
        is_official: false,
      })
      setMessage('Binding saved.')
    } catch (error) {
      setMessage(messageForError(error))
    } finally {
      setBusy(false)
    }
  }

  async function deleteBinding(id: string) {
    setBusy(true)
    try {
      await mutateJson<{ deleted: boolean }>(`/admin/api/template-bindings/${encodeURIComponent(id)}`, {
        method: 'DELETE',
      })
      setBindings((current) => current.filter((item) => item.id !== id))
      setMessage('Binding deleted.')
    } catch (error) {
      setMessage(messageForError(error))
    } finally {
      setBusy(false)
    }
  }

  function addBlock(type: string) {
    if (!selectedSection) return
    updateLayout((next) => {
      const section = next.sections.find((item) => item.id === selectedSection.id)
      const row = section?.rows.find((item) => item.id === selectedRow?.id) || section?.rows[0]
      const cell = row?.columns.find((item) => item.id === selectedCell?.id) || row?.columns[0]
      if (!cell) return
      const block = createTemplateBlock(type)
      cell.blocks.push(block)
      setSelectedRowID(row?.id || '')
      setSelectedCellID(cell.id)
      setSelectedBlockID(block.id)
      setActiveSelection('block')
    })
  }

  function addRow() {
    if (!selectedSection) return
    updateLayout((next) => {
      const section = next.sections.find((item) => item.id === selectedSection.id)
      if (!section) return
      const rowID = nextDesignerID('row')
      const cellID = nextDesignerID('cell')
      section.rows.push({
        id: rowID,
        columns: [{ id: cellID, span: 12, blocks: [] }],
      })
      setSelectedRowID(rowID)
      setSelectedCellID(cellID)
      setSelectedBlockID('')
      setActiveSelection('row')
    })
  }

  function addColumn(rowID: string) {
    updateLayout((next) => {
      for (const section of next.sections) {
        const row = section.rows.find((item) => item.id === rowID)
        if (!row) continue
        const nextCount = row.columns.length + 1
        const nextSpan = Math.max(2, Math.floor(12 / nextCount))
        row.columns = row.columns.map((column) => ({ ...column, span: nextSpan }))
        const cellID = nextDesignerID('cell')
        row.columns.push({ id: cellID, span: nextSpan, blocks: [] })
        setSelectedRowID(row.id)
        setSelectedCellID(cellID)
        setSelectedBlockID('')
        setActiveSelection('cell')
      }
    })
  }

  function removeColumn(cellID: string) {
    updateLayout((next) => {
      for (const section of next.sections) {
        for (const row of section.rows) {
          const index = row.columns.findIndex((item) => item.id === cellID)
          if (index < 0 || row.columns.length === 1) continue
          row.columns.splice(index, 1)
          const nextSpan = Math.max(2, Math.floor(12 / row.columns.length))
          row.columns = row.columns.map((column) => ({ ...column, span: nextSpan }))
          const fallbackCell = row.columns[Math.min(index, row.columns.length - 1)]
          if (selectedCellID === cellID) {
            setSelectedRowID(row.id)
            setSelectedCellID(fallbackCell?.id || '')
            setSelectedBlockID('')
            setActiveSelection(fallbackCell ? 'cell' : 'row')
          }
        }
      }
    })
  }

  function moveRow(sectionID: string, rowID: string, direction: -1 | 1) {
    updateLayout((next) => {
      const section = next.sections.find((item) => item.id === sectionID)
      if (!section) return
      const index = section.rows.findIndex((item) => item.id === rowID)
      const target = index + direction
      if (index < 0 || target < 0 || target >= section.rows.length) return
      const [row] = section.rows.splice(index, 1)
      if (!row) return
      section.rows.splice(target, 0, row)
    })
  }

  function deleteRow(sectionID: string, rowID: string) {
    updateLayout((next) => {
      const section = next.sections.find((item) => item.id === sectionID)
      if (!section || section.rows.length <= 1) return
      const deletedRowIndex = section.rows.findIndex((item) => item.id === rowID)
      section.rows = section.rows.filter((item) => item.id !== rowID)
      if (selectedRowID === rowID) {
        const fallbackRow = section.rows[Math.min(deletedRowIndex, section.rows.length - 1)] || section.rows[0]
        setSelectedRowID(fallbackRow?.id || '')
        setSelectedCellID(fallbackRow?.columns[0]?.id || '')
        setSelectedBlockID('')
        setActiveSelection(fallbackRow ? 'row' : 'canvas')
      }
    })
  }

  function updateBlock(updater: (draftBlock: TemplateBlock, cell: TemplateCell) => void) {
    if (!selectedBlockRef) return
    updateLayout((next) => {
      const section = next.sections.find((item) => item.id === selectedBlockRef.section.id)
      const row = section?.rows.find((item) => item.id === selectedBlockRef.row.id)
      const column = row?.columns.find((item) => item.id === selectedBlockRef.column.id)
      const block = column?.blocks.find((item) => item.id === selectedBlockRef.block.id)
      if (!column || !block) return
      updater(block, column)
    })
  }

  function updateRow(updater: (row: TemplateRow) => void) {
    if (!selectedSection || !selectedRow) return
    updateLayout((next) => {
      const section = next.sections.find((item) => item.id === selectedSection.id)
      const row = section?.rows.find((item) => item.id === selectedRow.id)
      if (!row) return
      updater(row)
    })
  }

  function updateCell(updater: (cell: TemplateCell) => void) {
    if (!selectedSection || !selectedRow || !selectedCell) return
    updateLayout((next) => {
      const section = next.sections.find((item) => item.id === selectedSection.id)
      const row = section?.rows.find((item) => item.id === selectedRow.id)
      const cell = row?.columns.find((item) => item.id === selectedCell.id)
      if (!cell) return
      updater(cell)
    })
  }

  useEffect(() => {
    if (!layout) return
    syncVisualBody(layout)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [layout])

  if (loading) {
    return <div className="rounded-xl border border-line bg-surface p-6 text-sm text-muted">Loading templates…</div>
  }

  const bindingResolution = preview?.binding_resolution
  const rowWidthPreset = WIDTH_PRESETS.some((item) => item.value === (selectedRow?.width || '')) ? (selectedRow?.width || '') : '__custom__'
  const rowHeightPreset = HEIGHT_PRESETS.some((item) => item.value === (selectedRow?.height || '')) ? (selectedRow?.height || '') : '__custom__'
  const cellWidthPreset = WIDTH_PRESETS.some((item) => item.value === (selectedCell?.width || '')) ? (selectedCell?.width || '') : '__custom__'
  const cellHeightPreset = HEIGHT_PRESETS.some((item) => item.value === (selectedCell?.height || '')) ? (selectedCell?.height || '') : '__custom__'

  return (
    <div className="space-y-6">
      <section className="rounded-2xl border border-line bg-surface p-6 shadow-panel">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h2 className="text-xl font-bold text-body">Template Designer</h2>
            <p className="mt-1 text-sm text-muted">Edit visual layouts, preview bindings, compare versions, and publish drafts from the current admin APIs.</p>
          </div>
          {message ? <div className="rounded-lg border border-line bg-accent-soft px-4 py-2 text-sm text-body">{message}</div> : null}
        </div>

        <div className="mt-6 grid grid-cols-1 gap-4 xl:grid-cols-[1.3fr_1fr_1fr_auto]">
          <Field label="Template">
            <select
              className="admin-input"
              value={selectedKey}
              onChange={(event) => {
                const next = event.target.value
                setSelectedKey(next)
                setSearchParams(next ? { key: next } : {})
              }}
            >
              {definitions.map((item) => (
                <option key={item.key} value={item.key}>
                  {item.title || item.key}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Version">
            <select
              className="admin-input"
              value={selectedVersion || ''}
              onChange={(event) => setSelectedVersion(Number(event.target.value) || null)}
            >
              {versions.map((item) => (
                <option key={item.version} value={item.version}>
                  v{item.version} · {item.status}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Fixture">
            <select className="admin-input" value={fixtureKey} onChange={(event) => setFixtureKey(event.target.value)}>
              <option value="">None</option>
              {fixtures.map((item) => (
                <option key={item.fixture_key} value={item.fixture_key}>
                  {item.name || item.fixture_key}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Actions">
            <div className="flex flex-wrap gap-2">
              <button type="button" className="admin-button admin-button-secondary" onClick={() => navigate('/templates')}>
                Back to List
              </button>
              <button type="button" className="admin-button" disabled={busy} onClick={() => void saveDraft()}>
                Save Draft
              </button>
              <button type="button" className="admin-button admin-button-secondary" disabled={busy} onClick={() => void duplicateDraft()}>
                Duplicate
              </button>
              <button type="button" className="admin-button admin-button-secondary" disabled={busy} onClick={() => void resetDraft()}>
                Reset
              </button>
              <button type="button" className="admin-button admin-button-secondary" disabled={busy} onClick={() => void validateDraft()}>
                Validate
              </button>
              <button type="button" className="admin-button admin-button-secondary" disabled={busy} onClick={() => void previewDraft()}>
                Preview
              </button>
              <button type="button" className="admin-button" disabled={busy || !activeVersion} onClick={() => void publishVersion()}>
                Publish
              </button>
            </div>
          </Field>
        </div>

        <div className="mt-6 grid grid-cols-1 gap-4 md:grid-cols-4">
          <MetricCard label="Renderer" value={selectedDefinition?.renderer_kind || '-'} />
          <MetricCard label="Target" value={[selectedDefinition?.target_kind, selectedDefinition?.target_key].filter(Boolean).join(' · ') || '-'} />
          <MetricCard label="Updated" value={formatDateTime(activeVersion?.updated_at)} />
          <MetricCard label="Published" value={formatDateTime(activeVersion?.published_at)} />
        </div>

        <div className="mt-6 flex flex-wrap gap-2">
          <button
            type="button"
            className={`admin-button ${activePanel === 'design' ? '' : 'admin-button-secondary'}`}
            onClick={() => setActivePanel('design')}
          >
            Design
          </button>
          <button
            type="button"
            className={`admin-button ${activePanel === 'preview' ? '' : 'admin-button-secondary'}`}
            onClick={() => setActivePanel('preview')}
          >
            Preview
          </button>
          <button
            type="button"
            className={`admin-button ${activePanel === 'advanced' ? '' : 'admin-button-secondary'}`}
            onClick={() => setActivePanel('advanced')}
          >
            Advanced
          </button>
        </div>
      </section>

      {activePanel === 'design' ? (
      <div className="grid grid-cols-1 gap-6 2xl:grid-cols-[220px_minmax(0,1fr)_340px]">
        <section className="rounded-2xl border border-line bg-surface p-5 shadow-panel">
          <h3 className="text-sm font-semibold uppercase tracking-[0.14em] text-body">Palette</h3>
          <div className="mt-4 flex flex-col gap-2">
            {PALETTE.map((item) => (
              <button key={item.type} type="button" className="admin-button admin-button-secondary justify-start" onClick={() => addBlock(item.type)}>
                {item.label}
              </button>
            ))}
          </div>
          <div className="mt-6 border-t border-line pt-4">
            <h4 className="text-xs font-semibold uppercase tracking-[0.14em] text-muted">Sections</h4>
            <div className="mt-3 flex flex-col gap-2">
              {(layout?.sections || []).map((section) => (
                <button
                  key={section.id}
                  type="button"
                  className={`rounded-xl border px-3 py-2 text-left text-sm ${selectedSectionID === section.id ? 'border-accent bg-accent-soft text-accent-dark' : 'border-line bg-surface text-body'}`}
                  onClick={() => {
                    setSelectedSectionID(section.id)
                    setSelectedRowID(section.rows[0]?.id || '')
                    setSelectedCellID(section.rows[0]?.columns[0]?.id || '')
                    setSelectedBlockID('')
                    setActiveSelection('canvas')
                  }}
                >
                  {section.title}
                </button>
              ))}
            </div>
          </div>
        </section>

        <section className="rounded-2xl border border-line bg-surface p-5 shadow-panel">
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <div>
              <h3 className="text-lg font-bold text-body">{selectedSection?.title || 'Canvas'}</h3>
              <p className="text-sm text-muted">Visual blocks round-trip into the existing template body JSON.</p>
            </div>
            <div className="flex gap-2">
              <button
                type="button"
                className="admin-button admin-button-secondary"
                onClick={() => {
                  setSelectedBlockID('')
                  setActiveSelection('canvas')
                }}
              >
                Canvas Settings
              </button>
              <button type="button" className="admin-button admin-button-secondary" onClick={addRow}>
                Add Row
              </button>
            </div>
          </div>

          <div className="space-y-4 rounded-2xl border border-dashed border-line bg-shell p-4">
            {(selectedSection?.rows || []).map((row, rowIndex) => (
              <div
                key={row.id}
                data-template-row-id={row.id}
                role="button"
                tabIndex={0}
                className={`space-y-3 rounded-2xl border p-3 ${selectedRowID === row.id ? 'border-accent bg-accent-soft/40' : 'border-line bg-surface'}`}
                onClick={() => {
                  setSelectedRowID(row.id)
                  setSelectedCellID(row.columns[0]?.id || '')
                  setSelectedBlockID('')
                  setActiveSelection('row')
                }}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault()
                    setSelectedRowID(row.id)
                    setSelectedCellID(row.columns[0]?.id || '')
                    setSelectedBlockID('')
                    setActiveSelection('row')
                  }
                }}
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <button
                    type="button"
                    className={`text-xs font-semibold uppercase tracking-[0.14em] ${selectedRowID === row.id ? 'text-accent-dark' : 'text-muted'}`}
                    onClick={(event) => {
                      event.stopPropagation()
                      setSelectedRowID(row.id)
                      setSelectedCellID(row.columns[0]?.id || '')
                      setSelectedBlockID('')
                      setActiveSelection('row')
                    }}
                  >
                    Row {rowIndex + 1}
                  </button>
                  <div className="flex gap-2">
                    <button type="button" className="admin-button admin-button-secondary" onClick={(event) => { event.stopPropagation(); moveRow(selectedSection?.id || '', row.id, -1) }}>
                      Up
                    </button>
                    <button type="button" className="admin-button admin-button-secondary" onClick={(event) => { event.stopPropagation(); moveRow(selectedSection?.id || '', row.id, 1) }}>
                      Down
                    </button>
                    <button type="button" className="admin-button admin-button-secondary" onClick={(event) => { event.stopPropagation(); addColumn(row.id) }}>
                      Add Column
                    </button>
                    <button type="button" className="admin-button admin-button-secondary" onClick={(event) => { event.stopPropagation(); deleteRow(selectedSection?.id || '', row.id) }}>
                      Delete Row
                    </button>
                  </div>
                </div>
                <div style={rowShellStyle(row)}>
                  <div style={rowCanvasStyle(row)}>
                    {row.columns.map((column, columnIndex) => (
                      <div
                        key={column.id}
                        data-template-cell-id={column.id}
                        role="button"
                        tabIndex={0}
                        className={`space-y-3 rounded-xl border p-3 text-left ${selectedCellID === column.id ? 'border-accent bg-accent-soft/30 shadow-panel' : 'border-line bg-surface'}`}
                        style={cellCanvasStyle(column)}
                        onClick={(event) => {
                          event.stopPropagation()
                          setSelectedRowID(row.id)
                          setSelectedCellID(column.id)
                          setSelectedBlockID('')
                          setActiveSelection('cell')
                        }}
                        onKeyDown={(event) => {
                          if (event.key === 'Enter' || event.key === ' ') {
                            event.preventDefault()
                            event.stopPropagation()
                            setSelectedRowID(row.id)
                            setSelectedCellID(column.id)
                            setSelectedBlockID('')
                            setActiveSelection('cell')
                          }
                        }}
                      >
                        <div className="flex items-center justify-between gap-2">
                          <button
                            type="button"
                            className={`text-xs font-semibold uppercase tracking-[0.14em] ${selectedCellID === column.id ? 'text-accent-dark' : 'text-muted'}`}
                            onClick={(event) => {
                              event.stopPropagation()
                              setSelectedRowID(row.id)
                              setSelectedCellID(column.id)
                              setSelectedBlockID('')
                              setActiveSelection('cell')
                            }}
                          >
                            Column {columnIndex + 1} · {column.width ? `Width ${column.width}` : `Span ${column.span}/12`}
                          </button>
                          {row.columns.length > 1 ? (
                            <button
                              type="button"
                              className="text-xs font-semibold uppercase tracking-[0.14em] text-accent-dark"
                              onClick={(event) => {
                                event.stopPropagation()
                                removeColumn(column.id)
                              }}
                            >
                              Remove
                            </button>
                          ) : null}
                        </div>
                        <div className="space-y-2">
                          {column.blocks.length ? (
                            column.blocks.map((block) => (
                              <button
                                key={block.id}
                                type="button"
                                className={`block w-full rounded-xl border p-3 text-left ${selectedBlockID === block.id ? 'border-line bg-shell shadow-sm' : 'border-line bg-surface'}`}
                                onClick={(event) => {
                                  event.stopPropagation()
                                  if (selectedCellID !== column.id) {
                                    setSelectedRowID(row.id)
                                    setSelectedCellID(column.id)
                                    setSelectedBlockID('')
                                    setActiveSelection('cell')
                                    return
                                  }
                                  setSelectedRowID(row.id)
                                  setSelectedCellID(column.id)
                                  setSelectedBlockID(block.id)
                                  setActiveSelection('block')
                                }}
                              >
                                <div className="text-sm font-semibold text-body">{block.label || block.text || block.type}</div>
                                <div className="mt-1 text-xs uppercase tracking-[0.14em] text-muted">{[block.type, block.path || block.rows_path].filter(Boolean).join(' · ')}</div>
                              </button>
                            ))
                          ) : (
                            <div className="rounded-xl border border-dashed border-line px-3 py-5 text-center text-sm text-muted">No blocks in this column yet.</div>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </section>

        <section className="space-y-6">
          <Panel title="Inspector" subtitle="Tune row and column layout, then refine individual blocks.">
            <div className="space-y-6">
              {activeSelection === 'row' && selectedRow ? (
                <article className="space-y-3 rounded-xl border border-line bg-accent-soft/30 p-4">
                  <div className="text-sm font-semibold text-body">Row Layout</div>
                  <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                    <Field label="Width Preset">
                      <select
                        className="admin-input"
                        value={rowWidthPreset}
                        onChange={(event) =>
                          updateRow((row) => {
                            row.width = event.target.value === '__custom__' ? row.width || '50%' : event.target.value
                          })
                        }
                      >
                        {WIDTH_PRESETS.map((item) => (
                          <option key={item.label} value={item.value}>
                            {item.label}
                          </option>
                        ))}
                        <option value="__custom__">Custom</option>
                      </select>
                    </Field>
                    <Field label="Custom Width">
                      <input className="admin-input" placeholder="e.g. 72%, 640px" value={selectedRow.width || ''} onChange={(event) => updateRow((row) => { row.width = event.target.value.trim() })} />
                    </Field>
                    <Field label="Height Preset">
                      <select
                        className="admin-input"
                        value={rowHeightPreset}
                        onChange={(event) =>
                          updateRow((row) => {
                            row.height = event.target.value === '__custom__' ? row.height || '180px' : event.target.value
                          })
                        }
                      >
                        {HEIGHT_PRESETS.map((item) => (
                          <option key={item.label} value={item.value}>
                            {item.label}
                          </option>
                        ))}
                        <option value="__custom__">Custom</option>
                      </select>
                    </Field>
                    <Field label="Custom Height">
                      <input className="admin-input" placeholder="e.g. 180px" value={selectedRow.height || ''} onChange={(event) => updateRow((row) => { row.height = event.target.value.trim() })} />
                    </Field>
                    <Field label="Container Horizontal">
                      <select className="admin-input" value={selectedRow.align_x || ''} onChange={(event) => updateRow((row) => { row.align_x = normalizeAlignX(event.target.value) })}>
                        <option value="">Start</option>
                        <option value="center">Center</option>
                        <option value="end">End</option>
                        <option value="stretch">Stretch</option>
                      </select>
                    </Field>
                    <Field label="Container Vertical">
                      <select className="admin-input" value={selectedRow.align_y || ''} onChange={(event) => updateRow((row) => { row.align_y = normalizeAlignY(event.target.value) })}>
                        <option value="">Start</option>
                        <option value="center">Center</option>
                        <option value="end">End</option>
                        <option value="stretch">Stretch</option>
                      </select>
                    </Field>
                    <Field label="Content Horizontal">
                      <select className="admin-input" value={selectedRow.content_align_x || ''} onChange={(event) => updateRow((row) => { row.content_align_x = normalizeContentAlignX(event.target.value) })}>
                        <option value="">Start</option>
                        <option value="center">Center</option>
                        <option value="end">End</option>
                      </select>
                    </Field>
                    <Field label="Content Vertical">
                      <select className="admin-input" value={selectedRow.content_align_y || ''} onChange={(event) => updateRow((row) => { row.content_align_y = normalizeContentAlignY(event.target.value) })}>
                        <option value="">Start</option>
                        <option value="center">Center</option>
                        <option value="end">End</option>
                        <option value="stretch">Stretch</option>
                      </select>
                    </Field>
                  </div>
                  <div className="flex justify-end">
                    <button
                      type="button"
                      className="admin-button admin-button-secondary"
                      onClick={() =>
                        updateRow((row) => {
                          row.width = ''
                          row.height = ''
                          row.align_x = ''
                          row.align_y = ''
                          row.content_align_x = ''
                          row.content_align_y = ''
                        })
                      }
                    >
                      Reset Row
                    </button>
                  </div>
                </article>
              ) : null}

              {activeSelection === 'cell' && selectedCell ? (
                <article className="space-y-3 rounded-xl border border-line bg-surface p-4">
                  <div className="text-sm font-semibold text-body">Column Layout</div>
                  <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                    <Field label="Span">
                      <input
                        className="admin-input"
                        type="number"
                        min={1}
                        max={12}
                        value={selectedCell.span}
                        onChange={(event) =>
                          updateCell((cell) => {
                            cell.span = Math.min(12, Math.max(1, Number(event.target.value) || 12))
                          })
                        }
                      />
                    </Field>
                    <Field label="Width Preset">
                      <select
                        className="admin-input"
                        value={cellWidthPreset}
                        onChange={(event) =>
                          updateCell((cell) => {
                            cell.width = event.target.value === '__custom__' ? cell.width || '50%' : event.target.value
                          })
                        }
                      >
                        {WIDTH_PRESETS.map((item) => (
                          <option key={item.label} value={item.value}>
                            {item.label}
                          </option>
                        ))}
                        <option value="__custom__">Custom</option>
                      </select>
                    </Field>
                    <Field label="Custom Width">
                      <input className="admin-input" placeholder="Leave blank to use span" value={selectedCell.width || ''} onChange={(event) => updateCell((cell) => { cell.width = event.target.value.trim() })} />
                    </Field>
                    <Field label="Height Preset">
                      <select
                        className="admin-input"
                        value={cellHeightPreset}
                        onChange={(event) =>
                          updateCell((cell) => {
                            cell.height = event.target.value === '__custom__' ? cell.height || '180px' : event.target.value
                          })
                        }
                      >
                        {HEIGHT_PRESETS.map((item) => (
                          <option key={item.label} value={item.value}>
                            {item.label}
                          </option>
                        ))}
                        <option value="__custom__">Custom</option>
                      </select>
                    </Field>
                    <Field label="Custom Height">
                      <input className="admin-input" placeholder="e.g. 160px" value={selectedCell.height || ''} onChange={(event) => updateCell((cell) => { cell.height = event.target.value.trim() })} />
                    </Field>
                    <Field label="Container Horizontal">
                      <select className="admin-input" value={selectedCell.align_x || ''} onChange={(event) => updateCell((cell) => { cell.align_x = normalizeAlignX(event.target.value) })}>
                        <option value="">Start</option>
                        <option value="center">Center</option>
                        <option value="end">End</option>
                        <option value="stretch">Stretch</option>
                      </select>
                    </Field>
                    <Field label="Container Vertical">
                      <select className="admin-input" value={selectedCell.align_y || ''} onChange={(event) => updateCell((cell) => { cell.align_y = normalizeAlignY(event.target.value) })}>
                        <option value="">Start</option>
                        <option value="center">Center</option>
                        <option value="end">End</option>
                        <option value="stretch">Stretch</option>
                      </select>
                    </Field>
                    <Field label="Content Horizontal">
                      <select className="admin-input" value={selectedCell.content_align_x || ''} onChange={(event) => updateCell((cell) => { cell.content_align_x = normalizeContentAlignX(event.target.value) })}>
                        <option value="">Start</option>
                        <option value="center">Center</option>
                        <option value="end">End</option>
                      </select>
                    </Field>
                    <Field label="Content Vertical">
                      <select className="admin-input" value={selectedCell.content_align_y || ''} onChange={(event) => updateCell((cell) => { cell.content_align_y = normalizeContentAlignY(event.target.value) })}>
                        <option value="">Start</option>
                        <option value="center">Center</option>
                        <option value="end">End</option>
                        <option value="stretch">Stretch</option>
                      </select>
                    </Field>
                  </div>
                  <div className="flex justify-end">
                    <button
                      type="button"
                      className="admin-button admin-button-secondary"
                      onClick={() =>
                        updateCell((cell) => {
                          cell.width = ''
                          cell.height = ''
                          cell.align_x = ''
                          cell.align_y = ''
                          cell.content_align_x = ''
                          cell.content_align_y = ''
                        })
                      }
                    >
                      Reset Column
                    </button>
                  </div>
                </article>
              ) : null}

              {activeSelection === 'block' && selectedBlockRef ? (
                <article className="space-y-3 rounded-xl border border-line bg-surface p-4">
                  <div className="text-sm font-semibold text-body">Block</div>
                  <Field label="Label">
                    <input className="admin-input" value={selectedBlockRef.block.label || ''} onChange={(event) => updateBlock((block) => { block.label = event.target.value })} />
                  </Field>
                  {selectedBlockRef.block.type === 'text' ? (
                    <Field label="Text">
                      <textarea className="admin-input min-h-24" value={selectedBlockRef.block.text || ''} onChange={(event) => updateBlock((block) => { block.text = event.target.value })} />
                    </Field>
                  ) : null}
                  {['field', 'barcode', 'totals'].includes(selectedBlockRef.block.type) ? (
                    <Field label="Path">
                      <input className="admin-input" value={selectedBlockRef.block.path || ''} onChange={(event) => updateBlock((block) => { block.path = event.target.value })} />
                    </Field>
                  ) : null}
                  {['table', 'totals'].includes(selectedBlockRef.block.type) ? (
                    <Field label="Rows Path">
                      <input className="admin-input" value={selectedBlockRef.block.rows_path || ''} onChange={(event) => updateBlock((block) => { block.rows_path = event.target.value })} />
                    </Field>
                  ) : null}
                  {selectedBlockRef.block.type === 'image' ? (
                    <Field label="Image URL">
                      <input className="admin-input" value={selectedBlockRef.block.image_url || ''} onChange={(event) => updateBlock((block) => { block.image_url = event.target.value })} />
                    </Field>
                  ) : null}
                  {selectedBlockRef.block.type === 'table' ? (
                    <div className="space-y-2">
                      <div className="text-xs font-semibold uppercase tracking-[0.14em] text-muted">Columns</div>
                      {(selectedBlockRef.block.columns || []).map((column, index) => (
                        <div key={`${column.label}-${index}`} className="grid grid-cols-2 gap-2">
                          <input
                            className="admin-input"
                            placeholder="Label"
                            value={column.label}
                            onChange={(event) =>
                              updateBlock((block) => {
                                block.columns = [...(block.columns || [])]
                                block.columns[index] = { label: event.target.value, path: block.columns[index]?.path || '' }
                              })
                            }
                          />
                          <input
                            className="admin-input"
                            placeholder="Path"
                            value={column.path}
                            onChange={(event) =>
                              updateBlock((block) => {
                                block.columns = [...(block.columns || [])]
                                block.columns[index] = { label: block.columns[index]?.label || '', path: event.target.value }
                              })
                            }
                          />
                        </div>
                      ))}
                      <button
                        type="button"
                        className="admin-button admin-button-secondary"
                        onClick={() =>
                          updateBlock((block) => {
                            block.columns = [...(block.columns || []), { label: 'Column', path: '' }]
                          })
                        }
                      >
                        Add Column
                      </button>
                    </div>
                  ) : null}
                  <div className="flex gap-2">
                    <button
                      type="button"
                      className="admin-button admin-button-secondary"
                      onClick={() =>
                        updateBlock((block, cell) => {
                          const copy = JSON.parse(JSON.stringify(block)) as TemplateBlock
                          copy.id = nextDesignerID('block')
                          cell.blocks.push(copy)
                          setSelectedBlockID(copy.id)
                        })
                      }
                    >
                      Duplicate
                    </button>
                    <button
                      type="button"
                      className="admin-button admin-button-secondary"
                      onClick={() =>
                        updateBlock((block, cell) => {
                          cell.blocks = cell.blocks.filter((item) => item.id !== block.id)
                          setSelectedBlockID('')
                        })
                      }
                    >
                      Delete
                    </button>
                  </div>
                </article>
              ) : null}

              {activeSelection === 'canvas' ? (
              <article className="space-y-3 rounded-xl border border-line bg-surface p-4">
                <div className="text-sm font-semibold text-body">Canvas Defaults</div>
                <Field label="Paper Preset">
                  <select
                    className="admin-input"
                    value={layout?.settings?.paper_preset || 'a4'}
                    onChange={(event) =>
                      updateLayout((next) => {
                        next.settings = next.settings || {}
                        next.settings.paper_preset = event.target.value
                        next.settings.orientation = event.target.value === 'a4-landscape' ? 'landscape' : 'portrait'
                        if (event.target.value === 'a4-landscape') next.settings.paper_preset = 'a4'
                      })
                    }
                  >
                    <option value="a4">A4 Portrait</option>
                    <option value="a4-landscape">A4 Landscape</option>
                    <option value="receipt-80">Receipt 80mm</option>
                    <option value="receipt-58">Receipt 58mm</option>
                  </select>
                </Field>
                <Field label="Density">
                  <select
                    className="admin-input"
                    value={layout?.settings?.density || 'comfortable'}
                    onChange={(event) =>
                      updateLayout((next) => {
                        next.settings = next.settings || {}
                        next.settings.density = event.target.value
                      })
                    }
                  >
                    <option value="comfortable">Comfortable</option>
                    <option value="compact">Compact</option>
                  </select>
                </Field>
              </article>
              ) : null}
            </div>
          </Panel>

          <Panel title="Data Sources" subtitle="Choose the primary target and related documents the template can render.">
            <div className="space-y-4">
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                <Field label="Main Target Type">
                  <select
                    className="admin-input"
                    value={definitionDraft.target_kind}
                    onChange={(event) =>
                      setDefinitionDraft((current) => ({
                        ...current,
                        target_kind: event.target.value,
                        target_key: event.target.value === 'report' ? targetCatalog.reports?.[0]?.key || '' : targetCatalog.documents?.[0]?.key || '',
                        related_sources: current.related_sources.map((item) => ({
                          ...item,
                          source_kind: event.target.value === 'report' ? 'report_row_document' : 'document_link',
                        })),
                      }))
                    }
                  >
                    <option value="document">Document</option>
                    <option value="report">Report</option>
                  </select>
                </Field>
                <Field label="Main Target">
                  <select className="admin-input" value={definitionDraft.target_key} onChange={(event) => setDefinitionDraft((current) => ({ ...current, target_key: event.target.value }))}>
                    <option value="">Select target</option>
                    {availableTargetOptions.map((item) => (
                      <option key={item.key} value={item.key}>
                        {item.title || item.key}
                      </option>
                    ))}
                  </select>
                </Field>
                <Field label="Purpose">
                  <input className="admin-input" value={definitionDraft.purpose} onChange={(event) => setDefinitionDraft((current) => ({ ...current, purpose: event.target.value }))} />
                </Field>
                <Field label="Channel">
                  <input className="admin-input" value={definitionDraft.channel} onChange={(event) => setDefinitionDraft((current) => ({ ...current, channel: event.target.value }))} />
                </Field>
              </div>
              <div className="rounded-xl border border-line bg-accent-soft/40 p-3 text-sm text-body">
                Related document sources become available in template paths under <code>related_documents.&lt;alias&gt;</code> and <code>related_document.&lt;alias&gt;</code>.
              </div>
              <div className="space-y-3">
                <div className="flex items-center justify-between gap-3">
                  <div className="text-sm font-semibold text-body">Related Documents</div>
                  <button type="button" className="admin-button admin-button-secondary" onClick={addRelatedSource}>
                    Add Related Source
                  </button>
                </div>
                {definitionDraft.related_sources.length ? (
                  definitionDraft.related_sources.map((item, index) => (
                    <article key={`${item.key}-${index}`} className="rounded-xl border border-line bg-surface p-3">
                      <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
                        <Field label="Alias">
                          <input className="admin-input" value={item.key} onChange={(event) => updateRelatedSource(index, (current) => ({ ...current, key: event.target.value }))} />
                        </Field>
                        <Field label="Label">
                          <input className="admin-input" value={item.label || ''} onChange={(event) => updateRelatedSource(index, (current) => ({ ...current, label: event.target.value }))} />
                        </Field>
                        <Field label="Document">
                          <select className="admin-input" value={item.target_key} onChange={(event) => updateRelatedSource(index, (current) => ({ ...current, target_kind: 'document', target_key: event.target.value }))}>
                            <option value="">Select related document</option>
                            {(targetCatalog.documents || []).map((option) => (
                              <option key={option.key} value={option.key}>
                                {option.title || option.key}
                              </option>
                            ))}
                          </select>
                        </Field>
                        <Field label="Relation">
                          <select className="admin-input" value={item.relation_mode || 'direct'} onChange={(event) => updateRelatedSource(index, (current) => ({ ...current, relation_mode: event.target.value }))}>
                            <option value="direct">Direct Link</option>
                            <option value="indirect">Direct + Indirect</option>
                          </select>
                        </Field>
                        <Field label="Max Depth">
                          <input
                            className="admin-input"
                            type="number"
                            min={1}
                            max={5}
                            value={item.max_depth || 1}
                            onChange={(event) => updateRelatedSource(index, (current) => ({ ...current, max_depth: Number(event.target.value) || 1 }))}
                          />
                        </Field>
                        {definitionDraft.target_kind === 'report' ? (
                          <Field label="Report Row Document ID Path">
                            <input
                              className="admin-input"
                              value={item.document_id_path || 'document_id'}
                              onChange={(event) => updateRelatedSource(index, (current) => ({ ...current, document_id_path: event.target.value }))}
                            />
                          </Field>
                        ) : null}
                      </div>
                      <div className="mt-3 flex justify-end">
                        <button type="button" className="admin-button admin-button-secondary" onClick={() => removeRelatedSource(index)}>
                          Remove
                        </button>
                      </div>
                    </article>
                  ))
                ) : (
                  <div className="rounded-xl border border-dashed border-line p-4 text-sm text-muted">No related documents configured yet.</div>
                )}
              </div>
              <div className="flex justify-end">
                <button type="button" className="admin-button admin-button-secondary" disabled={busy || !definitionDraft.target_key} onClick={() => void saveDefinitionSettings()}>
                  Save Data Sources
                </button>
              </div>
            </div>
          </Panel>

          <Panel title="Bindings" subtitle="Effective binding resolution for the selected template.">
            <div className="space-y-4">
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                <Field label="Scope Type">
                  <select className="admin-input" value={bindingDraft.scope_type} onChange={(event) => setBindingDraft((current) => ({ ...current, scope_type: event.target.value }))}>
                    <option value="deployment">Deployment</option>
                    <option value="organization">Organization</option>
                    <option value="location">Location</option>
                  </select>
                </Field>
                <Field label="Scope ID">
                  <input className="admin-input" value={bindingDraft.scope_id} onChange={(event) => setBindingDraft((current) => ({ ...current, scope_id: event.target.value }))} />
                </Field>
                <Field label="Target Kind">
                  <select className="admin-input" value={bindingDraft.target_kind} onChange={(event) => setBindingDraft((current) => ({ ...current, target_kind: event.target.value }))}>
                    <option value="document">Document</option>
                    <option value="report">Report</option>
                  </select>
                </Field>
                <Field label="Target Key">
                  <select className="admin-input" value={bindingDraft.target_key} onChange={(event) => setBindingDraft((current) => ({ ...current, target_key: event.target.value }))}>
                    <option value="">Select target</option>
                    {bindingTargetOptions.map((item) => (
                      <option key={item.key} value={item.key}>
                        {item.title || item.key}
                      </option>
                    ))}
                  </select>
                </Field>
                <Field label="Purpose">
                  <input className="admin-input" value={bindingDraft.purpose} onChange={(event) => setBindingDraft((current) => ({ ...current, purpose: event.target.value }))} />
                </Field>
                <Field label="Channel">
                  <input className="admin-input" value={bindingDraft.channel} onChange={(event) => setBindingDraft((current) => ({ ...current, channel: event.target.value }))} />
                </Field>
              </div>
              <div className="flex flex-wrap gap-3">
                <label className="flex items-center gap-2 text-sm text-body">
                  <input type="checkbox" checked={bindingDraft.is_default} onChange={(event) => setBindingDraft((current) => ({ ...current, is_default: event.target.checked }))} />
                  Default
                </label>
                <label className="flex items-center gap-2 text-sm text-body">
                  <input type="checkbox" checked={bindingDraft.is_official} onChange={(event) => setBindingDraft((current) => ({ ...current, is_official: event.target.checked }))} />
                  Official
                </label>
                <button type="button" className="admin-button admin-button-secondary" disabled={busy || !bindingDraft.target_kind || !bindingDraft.target_key} onClick={() => void saveBinding()}>
                  Save Binding
                </button>
              </div>
            <div className="space-y-2">
              {bindings.length ? (
                bindings.map((item) => (
                  <article key={item.id} className="rounded-xl border border-line bg-accent-soft/60 p-3">
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <div className="text-sm font-semibold text-body">
                          {item.scope_type}
                          {item.scope_id ? `:${item.scope_id}` : ''}
                        </div>
                        <div className="mt-1 text-sm text-muted">
                          {[item.target_kind, item.target_key, item.purpose, item.channel].filter(Boolean).join(' · ')}
                        </div>
                      </div>
                      <div className="flex gap-2">
                        <button
                          type="button"
                          className="admin-button admin-button-secondary"
                          onClick={() =>
                            setBindingDraft({
                              id: item.id,
                              scope_type: item.scope_type,
                              scope_id: item.scope_id || '',
                              target_kind: item.target_kind,
                              target_key: item.target_key,
                              purpose: item.purpose || '',
                              channel: item.channel || '',
                              is_default: Boolean(item.is_default),
                              is_official: Boolean(item.is_official),
                            })
                          }
                        >
                          Edit
                        </button>
                        <button type="button" className="admin-button admin-button-secondary" onClick={() => void deleteBinding(item.id)}>
                          Delete
                        </button>
                      </div>
                    </div>
                  </article>
                ))
              ) : (
                <div className="rounded-xl border border-dashed border-line p-4 text-sm text-muted">No explicit bindings. The module default will be used.</div>
              )}
            </div>
            </div>
          </Panel>
        </section>
      </div>
      ) : null}

      {activePanel !== 'design' ? (
      <div className={`grid grid-cols-1 gap-6 ${activePanel === 'advanced' ? '' : 'xl:grid-cols-2'}`}>
        {activePanel === 'preview' ? (
        <Panel title="Preview & Diagnostics" subtitle="Render the current draft against a fixture without publishing it.">
          <div className="space-y-4">
            {preview?.outputs?.[0]?.html ? (
              <div className="overflow-hidden rounded-xl border border-line bg-white">
                <div className="max-h-[480px] overflow-auto p-4" dangerouslySetInnerHTML={{ __html: preview.outputs[0].html || '' }} />
              </div>
            ) : (
              <div className="rounded-xl border border-dashed border-line p-6 text-sm text-muted">Run Preview to inspect rendered output.</div>
            )}
            <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
              <InfoListCard title="Warnings" items={(preview?.warnings || []).map((item) => item.message || item.code || '-')} />
              <InfoListCard title="Issues" items={(preview?.issues || []).map((item) => item.message || item.code || '-')} />
              <InfoListCard
                title="Binding"
                items={[
                  bindingResolution?.definition_key ? `Definition: ${bindingResolution.definition_key}` : 'No binding resolution yet.',
                  bindingResolution?.version ? `Version: v${bindingResolution.version}` : '',
                  (bindingResolution?.scope_path || []).length
                    ? `Path: ${(bindingResolution?.scope_path || [])
                        .map((item) => `${item.scope_type}${item.scope_id ? `:${item.scope_id}` : ''}`)
                        .join(' -> ')}`
                    : '',
                ].filter(Boolean)}
              />
            </div>
          </div>
        </Panel>
        ) : null}

        {activePanel === 'advanced' ? (
        <Panel title="Compare & Advanced" subtitle="Keep the raw body and style editors available for edge cases.">
          <div className="space-y-4">
            <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
              <Field label="Compare Left">
                <select className="admin-input" value={compareLeft || ''} onChange={(event) => setCompareLeft(Number(event.target.value) || null)}>
                  {versions.map((item) => (
                    <option key={`left-${item.version}`} value={item.version}>
                      v{item.version}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="Compare Right">
                <select className="admin-input" value={compareRight || ''} onChange={(event) => setCompareRight(Number(event.target.value) || null)}>
                  {versions.map((item) => (
                    <option key={`right-${item.version}`} value={item.version}>
                      v{item.version}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="Run Compare">
                <button type="button" className="admin-button admin-button-secondary" disabled={busy} onClick={() => void compareVersions()}>
                  Compare Versions
                </button>
              </Field>
            </div>
            {comparison ? (
              <div className="rounded-xl border border-line bg-accent-soft/60 p-4">
                <div className="text-sm font-semibold text-body">{comparison.has_differences ? 'Differences detected' : 'Versions are identical'}</div>
                <div className="mt-2 text-sm text-muted">{comparison.changed_fields.length ? comparison.changed_fields.join(', ') : 'No changed fields.'}</div>
              </div>
            ) : null}
            <Field label="Change Note">
              <input className="admin-input" value={changeNote} onChange={(event) => setChangeNote(event.target.value)} />
            </Field>
            <Field label="Template Body">
              <textarea className="admin-input min-h-64 font-mono text-xs" value={bodyValue} onChange={(event) => {
                setDraftBody(event.target.value)
                setLayout(parseTemplateBody(selectedDefinition, event.target.value))
              }} />
            </Field>
            <Field label="Template Style">
              <textarea className="admin-input min-h-40 font-mono text-xs" value={styleValue} onChange={(event) => setDraftStyle(event.target.value)} />
            </Field>
          </div>
        </Panel>
        ) : null}
      </div>
      ) : null}
    </div>
  )
}

function Panel({ title, subtitle, children }: { title: string; subtitle: string; children: ReactNode }) {
  return (
    <section className="rounded-2xl border border-line bg-surface p-5 shadow-panel">
      <div className="mb-4">
        <h3 className="text-lg font-bold text-body">{title}</h3>
        <p className="mt-1 text-sm text-muted">{subtitle}</p>
      </div>
      {children}
    </section>
  )
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  const token = label.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
  let control = children
  if (isValidElement(children) && typeof children.type === 'string' && ['input', 'select', 'textarea'].includes(children.type)) {
    control = cloneElement(children, {
      id: children.props.id || `admin-field-${token}`,
      name: children.props.name || token || 'field',
    })
  }
  return (
    <label className="block">
      <span className="mb-2 block text-xs font-semibold uppercase tracking-[0.14em] text-muted">{label}</span>
      {control}
    </label>
  )
}

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <article className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
      <div className="text-xs font-semibold uppercase tracking-wide text-body">{label}</div>
      <div className="mt-2 text-lg font-bold text-body">{value}</div>
    </article>
  )
}

function InfoListCard({ title, items }: { title: string; items: string[] }) {
  return (
    <article className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
      <div className="text-xs font-semibold uppercase tracking-wide text-body">{title}</div>
      <div className="mt-3 space-y-2 text-sm text-body">
        {items.length ? items.map((item, index) => <div key={`${title}-${index}`}>{item}</div>) : <div className="text-muted">-</div>}
      </div>
    </article>
  )
}
