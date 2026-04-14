import type { Connection, Edge, EdgeMouseHandler, Node, NodeChange, NodeMouseHandler, NodeProps } from '@xyflow/react'
import { Background, ConnectionMode, Controls, Handle, MiniMap, Position as FlowPosition, ReactFlow, applyNodeChanges } from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import type { ReactElement, ReactNode } from 'react'
import { cloneElement, isValidElement, useEffect, useId, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { fetchAllPagedItems, fetchJson, formatDateTime, mutateJson } from './adminClient'

type WorkflowSummary = {
  key: string
}

type WorkflowVersion = {
  version: number
  status: string
}

type WorkflowActionRule = {
  action: string
  from_state: string
  to_state: string
  permission_key?: string
  task_type?: string
  create_approval?: boolean
  assignment_strategy?: string
  assignment_mode?: string
  assignee_role_key?: string
  candidate_role_keys?: string[]
  fallback_role_key?: string
  approval_stage_key?: string
  due_after_seconds?: number
  escalate_after_seconds?: number
  requires_different_actor?: boolean
  step_up_required?: boolean
  link_mode?: string
  link_ttl_seconds?: number
  link_review_only?: boolean
  link_require_step_up?: boolean
  link_allowed_actions?: string[]
}

type WorkflowDefinition = {
  key: string
  version: number
  status: string
  states: string[]
  actions: WorkflowActionRule[]
  updated_at?: string
  updated_by?: string
  published_at?: string
}

type WorkflowSimulationResponse = {
  simulation?: {
    valid?: boolean
    issues?: string[]
    transition?: {
      from_state?: string
      to_state?: string
      action?: string
    }
  }
  routing_preview?: {
    resolved_via?: string
    error?: string
    resolved_assignee_username?: string
    resolved_assignee_user_id?: string
    resolved_candidate_usernames?: string[]
    fallback_role_key?: string
  }
}

type Position = {
  x: number
  y: number
}

type WorkflowNodeData = {
  label: string
  state: string
  detail: string
  isStart: boolean
  isTerminal: boolean
}

type WorkflowEdgeData = {
  action: string
  highlight: boolean
  isLoopback: boolean
}

const nodeTypes = {
  workflowState: WorkflowStateNode,
}

const ASSIGNMENT_STRATEGIES = ['', 'requester_manager', 'previous_approver_manager', 'role_fallback', 'static_role']

const EMPTY_RULE: WorkflowActionRule = {
  action: '',
  from_state: '',
  to_state: '',
  assignment_strategy: '',
  assignee_role_key: '',
  fallback_role_key: '',
  task_type: '',
  approval_stage_key: '',
  create_approval: false,
}

export function WorkflowDesignerPage() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const [definitions, setDefinitions] = useState<WorkflowSummary[]>([])
  const [selectedKey, setSelectedKey] = useState(searchParams.get('key') || '')
  const [versions, setVersions] = useState<WorkflowVersion[]>([])
  const [selectedVersion, setSelectedVersion] = useState<number | null>(null)
  const [draft, setDraft] = useState<WorkflowDefinition | null>(null)
  const [selectedRuleIndex, setSelectedRuleIndex] = useState<number>(0)
  const [selectedStateKey, setSelectedStateKey] = useState('')
  const [nodePositions, setNodePositions] = useState<Record<string, Position>>({})
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const [loading, setLoading] = useState(true)
  const [validationIssues, setValidationIssues] = useState<string[]>([])
  const [simulation, setSimulation] = useState<WorkflowSimulationResponse | null>(null)
  const [simulationInput, setSimulationInput] = useState({
    current_state: '',
    action: '',
    actor_id: '',
    location_id: '',
    requester_user_id: '',
    previous_approver_id: '',
  })

  function messageForError(error: unknown): string {
    return error instanceof Error && error.message ? error.message : 'Request failed.'
  }

  useEffect(() => {
    let mounted = true
    async function loadDefinitions() {
      try {
        const items = await fetchAllPagedItems<WorkflowSummary>('/admin/api/workflows')
        if (!mounted) return
        setDefinitions(items)
        setSelectedKey((current) => current || searchParams.get('key') || items[0]?.key || '')
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

  useEffect(() => {
    let mounted = true
    async function loadVersions() {
      if (!selectedKey) return
      try {
        const payload = await fetchJson<{ items: WorkflowVersion[] }>(`/admin/api/workflows/${encodeURIComponent(selectedKey)}/versions`)
        if (!mounted) return
        const ordered = payload.items || []
        setVersions(ordered)
        const preferred = ordered.find((item) => item.status === 'draft') || ordered[0] || null
        setSelectedVersion(preferred?.version || null)
      } catch (error) {
        if (!mounted) return
        setMessage(messageForError(error))
      }
    }
    void loadVersions()
    return () => {
      mounted = false
    }
  }, [selectedKey])

  useEffect(() => {
    let mounted = true
    async function loadDraft() {
      if (!selectedKey || !selectedVersion) return
      try {
        const payload = await fetchJson<WorkflowDefinition>(`/admin/api/workflows/${encodeURIComponent(selectedKey)}/versions/${selectedVersion}`)
        if (!mounted) return
        setDraft(payload)
        setSelectedRuleIndex(payload.actions?.length ? 0 : -1)
        setSelectedStateKey(payload.states?.[0] || '')
        setNodePositions(computeAutoLayout(payload.states || [], payload.actions || []))
        setValidationIssues([])
        setSimulation(null)
        setSimulationInput({
          current_state: payload.states?.[0] || '',
          action: payload.actions?.[0]?.action || '',
          actor_id: '',
          location_id: '',
          requester_user_id: '',
          previous_approver_id: '',
        })
        setMessage('')
      } catch (error) {
        if (!mounted) return
        setMessage(messageForError(error))
      }
    }
    void loadDraft()
    return () => {
      mounted = false
    }
  }, [selectedKey, selectedVersion])

  const selectedRule = selectedRuleIndex >= 0 ? draft?.actions?.[selectedRuleIndex] || null : null
  const selectedState = selectedStateKey && draft?.states?.includes(selectedStateKey) ? selectedStateKey : ''
  const startState = draft?.states?.[0] || ''

  const stateStats = useMemo(() => {
    const incoming = new Map<string, number>()
    const outgoing = new Map<string, number>()
    for (const state of draft?.states || []) {
      incoming.set(state, 0)
      outgoing.set(state, 0)
    }
    for (const rule of draft?.actions || []) {
      if (outgoing.has(rule.from_state)) outgoing.set(rule.from_state, (outgoing.get(rule.from_state) || 0) + 1)
      if (incoming.has(rule.to_state)) incoming.set(rule.to_state, (incoming.get(rule.to_state) || 0) + 1)
    }
    return { incoming, outgoing }
  }, [draft])

  const terminalStates = useMemo(
    () => (draft?.states || []).filter((state) => (stateStats.outgoing.get(state) || 0) === 0),
    [draft, stateStats],
  )

  function updateDraft(mutator: (next: WorkflowDefinition) => void, options?: { relayout?: boolean }) {
    setDraft((current) => {
      if (!current) return current
      const next = JSON.parse(JSON.stringify(current)) as WorkflowDefinition
      mutator(next)
      if (options?.relayout) {
        setNodePositions(computeAutoLayout(next.states || [], next.actions || []))
      }
      return next
    })
  }

  function updateSelectedRule(mutator: (rule: WorkflowActionRule) => void, options?: { relayout?: boolean }) {
    updateDraft((next) => {
      const rule = next.actions[selectedRuleIndex]
      if (!rule) return
      mutator(rule)
    }, options)
  }

  async function createDraft() {
    if (!selectedKey) return
    setBusy(true)
    try {
      const payload = await mutateJson<WorkflowDefinition>(`/admin/api/workflows/${encodeURIComponent(selectedKey)}/drafts`, {
        method: 'POST',
      })
      setDraft(payload)
      setSelectedVersion(payload.version)
      setSelectedRuleIndex(payload.actions?.length ? 0 : -1)
      setSelectedStateKey(payload.states?.[0] || '')
      setNodePositions(computeAutoLayout(payload.states || [], payload.actions || []))
      setMessage('Draft created.')
      const versionsPayload = await fetchJson<{ items: WorkflowVersion[] }>(`/admin/api/workflows/${encodeURIComponent(selectedKey)}/versions`)
      setVersions(versionsPayload.items || [])
    } catch (error) {
      setMessage(messageForError(error))
    } finally {
      setBusy(false)
    }
  }

  async function saveDraft() {
    if (!draft) return
    setBusy(true)
    try {
      const payload = await mutateJson<WorkflowDefinition>(`/admin/api/workflows/${encodeURIComponent(draft.key)}/versions/${draft.version}`, {
        method: 'PUT',
        body: JSON.stringify(draft),
      })
      setDraft(payload)
      setNodePositions((current) => mergePositions(current, computeAutoLayout(payload.states || [], payload.actions || []), payload.states || []))
      setMessage('Draft saved.')
    } catch (error) {
      setMessage(messageForError(error))
    } finally {
      setBusy(false)
    }
  }

  async function validateDraft() {
    if (!draft) return
    setBusy(true)
    try {
      const payload = await mutateJson<{ valid: boolean; issues?: string[] }>(`/admin/api/workflows/${encodeURIComponent(draft.key)}/versions/${draft.version}/validate`, {
        method: 'POST',
      })
      setValidationIssues(payload.issues || [])
      setMessage(payload.valid ? 'Workflow is valid.' : (payload.issues || []).join('; ') || 'Workflow validation failed.')
    } catch (error) {
      setMessage(messageForError(error))
    } finally {
      setBusy(false)
    }
  }

  async function publishDraft() {
    if (!draft) return
    setBusy(true)
    try {
      const payload = await mutateJson<WorkflowDefinition>(`/admin/api/workflows/${encodeURIComponent(draft.key)}/versions/${draft.version}/publish`, {
        method: 'POST',
      })
      setDraft(payload)
      setNodePositions((current) => mergePositions(current, computeAutoLayout(payload.states || [], payload.actions || []), payload.states || []))
      setMessage(`Published version ${payload.version}.`)
    } catch (error) {
      setMessage(messageForError(error))
    } finally {
      setBusy(false)
    }
  }

  async function simulateRouting() {
    if (!draft) return
    setBusy(true)
    try {
      const payload = await mutateJson<WorkflowSimulationResponse>(`/admin/api/workflows/${encodeURIComponent(draft.key)}/versions/${draft.version}/simulate`, {
        method: 'POST',
        body: JSON.stringify({
          current_state: simulationInput.current_state,
          action: simulationInput.action,
          actor_id: simulationInput.actor_id,
          location_id: simulationInput.location_id,
          additional_input: {
            requester_user_id: simulationInput.requester_user_id,
            previous_approver_id: simulationInput.previous_approver_id,
          },
        }),
      })
      setSimulation(payload)
      setMessage('Routing simulation loaded.')
    } catch (error) {
      setMessage(messageForError(error))
    } finally {
      setBusy(false)
    }
  }

  function addState() {
    updateDraft((next) => {
      const state = nextStateKey(next.states || [])
      next.states = [...(next.states || []), state]
      setSelectedStateKey(state)
      setSelectedRuleIndex(-1)
      setSimulationInput((current) => ({ ...current, current_state: state }))
    }, { relayout: true })
  }

  function deleteSelectedState() {
    if (!draft || !selectedState) return
    updateDraft((next) => {
      next.states = (next.states || []).filter((state) => state !== selectedState)
      next.actions = (next.actions || []).filter((rule) => rule.from_state !== selectedState && rule.to_state !== selectedState)
      const fallbackState = next.states[0] || ''
      setSelectedStateKey(fallbackState)
      setSelectedRuleIndex(next.actions.length ? 0 : -1)
      setSimulationInput((current) => ({
        ...current,
        current_state: current.current_state === selectedState ? fallbackState : current.current_state,
      }))
    }, { relayout: true })
  }

  function addTransition(source: string, target: string) {
    updateDraft((next) => {
      const rule: WorkflowActionRule = {
        ...EMPTY_RULE,
        from_state: source,
        to_state: target,
      }
      next.actions = [...(next.actions || []), rule]
      setSelectedRuleIndex(next.actions.length - 1)
      setSelectedStateKey('')
      setSimulationInput((current) => ({
        ...current,
        current_state: source,
        action: rule.action || current.action,
      }))
    }, { relayout: true })
  }

  function duplicateSelectedTransition() {
    if (!draft || !selectedRule) return
    updateDraft((next) => {
      const copy = JSON.parse(JSON.stringify(next.actions[selectedRuleIndex])) as WorkflowActionRule
      next.actions.splice(selectedRuleIndex + 1, 0, copy)
      setSelectedRuleIndex(selectedRuleIndex + 1)
      setSelectedStateKey('')
    }, { relayout: true })
  }

  function deleteSelectedTransition() {
    if (!draft || selectedRuleIndex < 0) return
    updateDraft((next) => {
      next.actions.splice(selectedRuleIndex, 1)
      setSelectedRuleIndex(next.actions.length ? Math.min(selectedRuleIndex, next.actions.length - 1) : -1)
    }, { relayout: true })
  }

  const nodes = useMemo<Node<WorkflowNodeData>[]>(() => {
    if (!draft) return []
    return (draft.states || []).map((state) => {
      const position = nodePositions[state] || { x: 0, y: 0 }
      const inbound = (draft.actions || []).filter((rule) => rule.to_state === state).length
      const outbound = (draft.actions || []).filter((rule) => rule.from_state === state).length
      return {
        id: state,
        type: 'workflowState',
        position,
        data: {
          label: state,
          state,
          detail: `${inbound} in · ${outbound} out`,
          isStart: state === startState,
          isTerminal: outbound === 0,
        },
        className: selectedState === state ? 'workflow-node workflow-node-selected' : 'workflow-node',
        sourcePosition: FlowPosition.Right,
        targetPosition: FlowPosition.Left,
        style: { width: 220, height: 128 },
        width: 220,
        height: 128,
        initialWidth: 220,
        initialHeight: 128,
        measured: { width: 220, height: 128 },
        handles: [
          { id: 'target', nodeId: state, type: 'target', position: FlowPosition.Left, x: 0, y: 64, width: 10, height: 10 },
          { id: 'source', nodeId: state, type: 'source', position: FlowPosition.Right, x: 220, y: 64, width: 10, height: 10 },
        ],
        draggable: true,
      }
    })
  }, [draft, nodePositions, selectedState])

  const edges = useMemo<Edge<WorkflowEdgeData>[]>(() => {
    if (!draft) return []
    const groupedCounts = new Map<string, number>()
    return (draft.actions || []).map((rule, index) => {
      const pairKey = `${rule.from_state}->${rule.to_state}`
      const parallelIndex = groupedCounts.get(pairKey) || 0
      groupedCounts.set(pairKey, parallelIndex + 1)
      const isLoopback = (draft.states || []).indexOf(rule.to_state) <= (draft.states || []).indexOf(rule.from_state)
      return {
        id: edgeIDForRule(rule, index),
        source: rule.from_state,
        target: rule.to_state,
        label: rule.action || 'New transition',
        animated: Boolean(rule.create_approval || rule.step_up_required || isLoopback),
        type: 'smoothstep',
        pathOptions: { offset: 24 + parallelIndex * 18 },
        className: `${selectedRuleIndex === index ? 'workflow-edge workflow-edge-selected' : 'workflow-edge'}${isLoopback ? ' workflow-edge-loopback' : ''}`,
        style: isLoopback ? { strokeDasharray: '8 4' } : undefined,
        data: {
          action: rule.action || '',
          highlight: Boolean(rule.create_approval || rule.step_up_required),
          isLoopback,
        },
      }
    })
  }, [draft, selectedRuleIndex])

  function handleNodesChange(changes: NodeChange<Node<WorkflowNodeData>>[]) {
    setNodePositions((current) => {
      const nodesForChange = nodes.map((node) => ({
        ...node,
        position: current[node.id] || node.position,
      }))
      const changed = applyNodeChanges(changes, nodesForChange)
      const next = { ...current }
      for (const node of changed) {
        next[node.id] = node.position
      }
      return next
    })
  }

  const handleConnect = (connection: Connection) => {
    if (!connection.source || !connection.target) return
    if (connection.source === connection.target) return
    addTransition(connection.source, connection.target)
  }

  const handleNodeClick: NodeMouseHandler<Node<WorkflowNodeData>> = (_, node) => {
    setSelectedStateKey(node.id)
    setSelectedRuleIndex(-1)
  }

  const handleEdgeClick: EdgeMouseHandler<Edge<WorkflowEdgeData>> = (_, edge) => {
    const edgeIndex = draft?.actions?.findIndex((rule, index) => edgeIDForRule(rule, index) === edge.id) ?? -1
    if (edgeIndex >= 0) {
      setSelectedRuleIndex(edgeIndex)
      setSelectedStateKey('')
    }
  }

  if (loading) {
    return <div className="rounded-xl border border-line bg-surface p-6 text-sm text-muted">Loading workflows…</div>
  }

  return (
    <div className="space-y-6">
      <section className="rounded-2xl border border-line bg-surface p-6 shadow-panel">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h2 className="text-xl font-bold text-body">Workflow Designer</h2>
            <p className="mt-1 text-sm text-muted">Edit workflow states and transitions on a drag-and-drop canvas, then validate, simulate, and publish through the existing workflow APIs.</p>
          </div>
          {message ? <div className="rounded-lg border border-line bg-accent-soft px-4 py-2 text-sm text-body">{message}</div> : null}
        </div>

        <div className="mt-6 grid grid-cols-1 gap-4 xl:grid-cols-[1.2fr_1fr_auto]">
          <Field label="Workflow">
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
                  {item.key}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Version">
            <select className="admin-input" value={selectedVersion || ''} onChange={(event) => setSelectedVersion(Number(event.target.value) || null)}>
              {versions.map((item) => (
                <option key={item.version} value={item.version}>
                  v{item.version} · {item.status}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Actions">
            <div className="flex flex-wrap gap-2">
              <button type="button" className="admin-button admin-button-secondary" onClick={() => navigate('/workflows')}>
                Back to List
              </button>
              <button type="button" className="admin-button admin-button-secondary" disabled={busy} onClick={() => void createDraft()}>
                Create Draft
              </button>
              <button type="button" className="admin-button admin-button-secondary" disabled={busy || !draft} onClick={() => addState()}>
                Add State
              </button>
              <button type="button" className="admin-button" disabled={busy || !draft} onClick={() => void saveDraft()}>
                Save Draft
              </button>
              <button type="button" className="admin-button admin-button-secondary" disabled={busy || !draft} onClick={() => void validateDraft()}>
                Validate
              </button>
              <button type="button" className="admin-button admin-button-secondary" disabled={busy || !draft} onClick={() => void simulateRouting()}>
                Simulate
              </button>
              <button type="button" className="admin-button" disabled={busy || !draft} onClick={() => void publishDraft()}>
                Publish
              </button>
            </div>
          </Field>
        </div>

        <div className="mt-6 grid grid-cols-1 gap-4 md:grid-cols-4">
          <MetricCard label="Current Draft" value={draft ? `v${draft.version} · ${draft.status}` : '-'} />
          <MetricCard label="States" value={String(draft?.states?.length || 0)} />
          <MetricCard label="Transitions" value={String(draft?.actions?.length || 0)} />
          <MetricCard label="Published" value={formatDateTime(draft?.published_at)} />
        </div>
      </section>

      <div className="grid grid-cols-1 gap-6 2xl:grid-cols-[320px_minmax(0,1fr)_360px]">
        <section className="rounded-2xl border border-line bg-surface p-5 shadow-panel">
          <h3 className="text-sm font-semibold uppercase tracking-[0.14em] text-body">Graph Outline</h3>
          <p className="mt-1 text-sm text-muted">Select a state or transition here, or work directly on the canvas.</p>
          <div className="mt-4 space-y-4">
            <div>
              <div className="mb-2 text-xs font-semibold uppercase tracking-[0.14em] text-muted">States</div>
              <div className="space-y-2">
                {(draft?.states || []).map((state) => (
                  <button
                    key={state}
                    type="button"
                    className={`block w-full rounded-xl border p-3 text-left ${selectedState === state ? 'border-accent bg-accent-soft text-accent-dark' : 'border-line bg-surface text-body'}`}
                    onClick={() => {
                      setSelectedStateKey(state)
                      setSelectedRuleIndex(-1)
                    }}
                  >
                    <div className="text-sm font-semibold">{state}</div>
                    <div className="mt-1 text-xs uppercase tracking-[0.14em] text-muted">
                      {(draft?.actions || []).filter((rule) => rule.from_state === state).length} outgoing · {(draft?.actions || []).filter((rule) => rule.to_state === state).length} incoming
                    </div>
                  </button>
                ))}
              </div>
            </div>
            <div>
              <div className="mb-2 text-xs font-semibold uppercase tracking-[0.14em] text-muted">Transitions</div>
              <div className="space-y-2">
                {(draft?.actions || []).map((rule, index) => (
                  <button
                    key={edgeIDForRule(rule, index)}
                    type="button"
                    className={`block w-full rounded-xl border p-3 text-left ${selectedRuleIndex === index ? 'border-accent bg-accent-soft text-accent-dark' : 'border-line bg-surface text-body'}`}
                    onClick={() => {
                      setSelectedRuleIndex(index)
                      setSelectedStateKey('')
                      setSimulationInput((current) => ({
                        ...current,
                        current_state: rule.from_state || current.current_state,
                        action: rule.action || current.action,
                      }))
                    }}
                  >
                    <div className="text-sm font-semibold">{rule.action || 'Untitled action'}</div>
                    <div className="mt-1 text-xs uppercase tracking-[0.14em] text-muted">
                      {[rule.from_state || '*', rule.to_state || '*', rule.assignment_strategy || 'direct'].join(' -> ')}
                    </div>
                  </button>
                ))}
              </div>
            </div>
          </div>
        </section>

        <section className="rounded-2xl border border-line bg-surface p-5 shadow-panel">
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <div>
              <h3 className="text-lg font-bold text-body">Drag-and-Drop Canvas</h3>
              <p className="mt-1 text-sm text-muted">Drag states freely, connect handles to create transitions, and select edges to edit full action details.</p>
            </div>
            <button
              type="button"
              className="admin-button admin-button-secondary"
              disabled={!draft}
              onClick={() => setNodePositions(computeAutoLayout(draft?.states || [], draft?.actions || []))}
            >
              Auto Layout
            </button>
          </div>
          <div className="mb-4 grid grid-cols-1 gap-3 lg:grid-cols-3">
            <InfoListCard title="Start State" items={[startState || 'No state defined.']} />
            <InfoListCard title="Terminal States" items={terminalStates.length ? terminalStates : ['No terminal state detected.']} />
            <InfoListCard
              title="Loopbacks"
              items={
                (draft?.actions || [])
                  .filter((rule) => (draft?.states || []).indexOf(rule.to_state) <= (draft?.states || []).indexOf(rule.from_state))
                  .map((rule) => `${rule.action || 'Untitled'}: ${rule.from_state} -> ${rule.to_state}`) || ['No loopback transitions.']
              }
            />
          </div>
          <div className="workflow-canvas-shell">
            <ReactFlow
              nodes={nodes}
              edges={edges}
              nodeTypes={nodeTypes}
              fitView
              nodesDraggable
              elementsSelectable
              connectionMode={ConnectionMode.Loose}
              onNodesChange={handleNodesChange}
              onConnect={handleConnect}
              onNodeClick={handleNodeClick}
              onEdgeClick={handleEdgeClick}
              deleteKeyCode={null}
              selectionOnDrag
              elevateNodesOnSelect
            >
              <Background gap={20} size={1} />
              <MiniMap pannable zoomable nodeColor={() => 'var(--color-accent)'} maskColor="color-mix(in srgb, var(--color-shell) 82%, transparent)" />
              <Controls />
            </ReactFlow>
          </div>
        </section>

        <section className="space-y-6">
          <Panel
            title={selectedRule ? 'Transition Inspector' : selectedState ? 'State Inspector' : 'Inspector'}
            subtitle={selectedRule ? 'Edit the selected transition directly against the workflow action rule schema.' : selectedState ? 'Rename or remove the selected state. Renames update all connected transitions.' : 'Select a state or transition to edit it.'}
          >
            {selectedRule ? (
              <div className="space-y-3">
                <Field label="Action">
                  <input
                    className="admin-input"
                    value={selectedRule.action}
                    onChange={(event) =>
                      updateSelectedRule((rule) => {
                        rule.action = event.target.value
                      })
                    }
                  />
                </Field>
                <div className="grid grid-cols-2 gap-3">
                  <Field label="From State">
                    <select
                      className="admin-input"
                      value={selectedRule.from_state}
                      onChange={(event) =>
                        updateSelectedRule((rule) => {
                          rule.from_state = event.target.value
                        }, { relayout: true })
                      }
                    >
                      {(draft?.states || []).map((state) => (
                        <option key={state} value={state}>
                          {state}
                        </option>
                      ))}
                    </select>
                  </Field>
                  <Field label="To State">
                    <select
                      className="admin-input"
                      value={selectedRule.to_state}
                      onChange={(event) =>
                        updateSelectedRule((rule) => {
                          rule.to_state = event.target.value
                        }, { relayout: true })
                      }
                    >
                      {(draft?.states || []).map((state) => (
                        <option key={state} value={state}>
                          {state}
                        </option>
                      ))}
                    </select>
                  </Field>
                </div>
                <Field label="Assignment Strategy">
                  <select
                    className="admin-input"
                    value={selectedRule.assignment_strategy || ''}
                    onChange={(event) =>
                      updateSelectedRule((rule) => {
                        rule.assignment_strategy = event.target.value
                      })
                    }
                  >
                    {ASSIGNMENT_STRATEGIES.map((item) => (
                      <option key={item || 'default'} value={item}>
                        {item || 'default'}
                      </option>
                    ))}
                  </select>
                </Field>
                <Field label="Permission Key">
                  <input
                    className="admin-input"
                    value={selectedRule.permission_key || ''}
                    onChange={(event) =>
                      updateSelectedRule((rule) => {
                        rule.permission_key = event.target.value
                      })
                    }
                  />
                </Field>
                <Field label="Assignee Role">
                  <input
                    className="admin-input"
                    value={selectedRule.assignee_role_key || ''}
                    onChange={(event) =>
                      updateSelectedRule((rule) => {
                        rule.assignee_role_key = event.target.value
                      })
                    }
                  />
                </Field>
                <Field label="Fallback Role">
                  <input
                    className="admin-input"
                    value={selectedRule.fallback_role_key || ''}
                    onChange={(event) =>
                      updateSelectedRule((rule) => {
                        rule.fallback_role_key = event.target.value
                      })
                    }
                  />
                </Field>
                <Field label="Task Type">
                  <input
                    className="admin-input"
                    value={selectedRule.task_type || ''}
                    onChange={(event) =>
                      updateSelectedRule((rule) => {
                        rule.task_type = event.target.value
                      })
                    }
                  />
                </Field>
                <Field label="Approval Stage">
                  <input
                    className="admin-input"
                    value={selectedRule.approval_stage_key || ''}
                    onChange={(event) =>
                      updateSelectedRule((rule) => {
                        rule.approval_stage_key = event.target.value
                      })
                    }
                  />
                </Field>
                <div className="grid grid-cols-2 gap-3">
                  <Field label="Due After Seconds">
                    <input
                      className="admin-input"
                      type="number"
                      value={selectedRule.due_after_seconds || ''}
                      onChange={(event) =>
                        updateSelectedRule((rule) => {
                          rule.due_after_seconds = parseOptionalNumber(event.target.value)
                        })
                      }
                    />
                  </Field>
                  <Field label="Escalate After Seconds">
                    <input
                      className="admin-input"
                      type="number"
                      value={selectedRule.escalate_after_seconds || ''}
                      onChange={(event) =>
                        updateSelectedRule((rule) => {
                          rule.escalate_after_seconds = parseOptionalNumber(event.target.value)
                        })
                      }
                    />
                  </Field>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <CheckboxField
                    label="Create Approval"
                    checked={Boolean(selectedRule.create_approval)}
                    onChange={(checked) =>
                      updateSelectedRule((rule) => {
                        rule.create_approval = checked
                      })
                    }
                  />
                  <CheckboxField
                    label="Step-Up Required"
                    checked={Boolean(selectedRule.step_up_required)}
                    onChange={(checked) =>
                      updateSelectedRule((rule) => {
                        rule.step_up_required = checked
                      })
                    }
                  />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <CheckboxField
                    label="Different Actor Required"
                    checked={Boolean(selectedRule.requires_different_actor)}
                    onChange={(checked) =>
                      updateSelectedRule((rule) => {
                        rule.requires_different_actor = checked
                      })
                    }
                  />
                  <CheckboxField
                    label="Link Review Only"
                    checked={Boolean(selectedRule.link_review_only)}
                    onChange={(checked) =>
                      updateSelectedRule((rule) => {
                        rule.link_review_only = checked
                      })
                    }
                  />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <Field label="Link Mode">
                    <input
                      className="admin-input"
                      value={selectedRule.link_mode || ''}
                      onChange={(event) =>
                        updateSelectedRule((rule) => {
                          rule.link_mode = event.target.value
                        })
                      }
                    />
                  </Field>
                  <Field label="Link TTL Seconds">
                    <input
                      className="admin-input"
                      type="number"
                      value={selectedRule.link_ttl_seconds || ''}
                      onChange={(event) =>
                        updateSelectedRule((rule) => {
                          rule.link_ttl_seconds = parseOptionalNumber(event.target.value)
                        })
                      }
                    />
                  </Field>
                </div>
                <Field label="Link Allowed Actions">
                  <input
                    className="admin-input"
                    value={(selectedRule.link_allowed_actions || []).join(', ')}
                    onChange={(event) =>
                      updateSelectedRule((rule) => {
                        rule.link_allowed_actions = splitCSV(event.target.value)
                      })
                    }
                  />
                </Field>
                <Field label="Candidate Roles">
                  <input
                    className="admin-input"
                    value={(selectedRule.candidate_role_keys || []).join(', ')}
                    onChange={(event) =>
                      updateSelectedRule((rule) => {
                        rule.candidate_role_keys = splitCSV(event.target.value)
                      })
                    }
                  />
                </Field>
                <div className="flex gap-2">
                  <button type="button" className="admin-button admin-button-secondary" onClick={() => duplicateSelectedTransition()}>
                    Duplicate
                  </button>
                  <button type="button" className="admin-button admin-button-secondary" onClick={() => deleteSelectedTransition()}>
                    Delete
                  </button>
                </div>
              </div>
            ) : selectedState ? (
              <div className="space-y-3">
                <Field label="State Key">
                  <input
                    className="admin-input"
                    value={selectedState}
                    onChange={(event) => {
                      const nextState = sanitizeStateKey(event.target.value)
                      if (!nextState) {
                        setSelectedStateKey('')
                        return
                      }
                      updateDraft((next) => {
                        if ((next.states || []).includes(nextState) && nextState !== selectedState) return
                        next.states = (next.states || []).map((state) => (state === selectedState ? nextState : state))
                        next.actions = (next.actions || []).map((rule) => ({
                          ...rule,
                          from_state: rule.from_state === selectedState ? nextState : rule.from_state,
                          to_state: rule.to_state === selectedState ? nextState : rule.to_state,
                        }))
                        setSelectedStateKey(nextState)
                        setSimulationInput((current) => ({
                          ...current,
                          current_state: current.current_state === selectedState ? nextState : current.current_state,
                        }))
                      }, { relayout: true })
                    }}
                  />
                </Field>
                <InfoListCard
                  title="Connected Transitions"
                  items={(draft?.actions || [])
                    .filter((rule) => rule.from_state === selectedState || rule.to_state === selectedState)
                    .map((rule) => `${rule.action || 'Untitled'}: ${rule.from_state} -> ${rule.to_state}`)}
                />
                <button type="button" className="admin-button admin-button-secondary" onClick={() => deleteSelectedState()}>
                  Delete State
                </button>
              </div>
            ) : (
              <div className="rounded-xl border border-dashed border-line p-4 text-sm text-muted">Select a state or transition to edit it.</div>
            )}
          </Panel>

          <Panel title="Routing Inspector" subtitle="Run a live simulation against the selected draft to inspect assignment and fallback behavior.">
            <div className="space-y-3">
              <Field label="Current State">
                <select className="admin-input" value={simulationInput.current_state} onChange={(event) => setSimulationInput((current) => ({ ...current, current_state: event.target.value }))}>
                  {(draft?.states || []).map((state) => (
                    <option key={state} value={state}>
                      {state}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="Action">
                <input className="admin-input" value={simulationInput.action} onChange={(event) => setSimulationInput((current) => ({ ...current, action: event.target.value }))} />
              </Field>
              <Field label="Requester / Actor">
                <input className="admin-input" value={simulationInput.requester_user_id} onChange={(event) => setSimulationInput((current) => ({ ...current, requester_user_id: event.target.value, actor_id: event.target.value }))} />
              </Field>
              <Field label="Previous Approver">
                <input className="admin-input" value={simulationInput.previous_approver_id} onChange={(event) => setSimulationInput((current) => ({ ...current, previous_approver_id: event.target.value }))} />
              </Field>
              <Field label="Location">
                <input className="admin-input" value={simulationInput.location_id} onChange={(event) => setSimulationInput((current) => ({ ...current, location_id: event.target.value }))} />
              </Field>
              <button type="button" className="admin-button admin-button-secondary" disabled={busy || !draft} onClick={() => void simulateRouting()}>
                Run Simulation
              </button>
            </div>
            <div className="mt-4 space-y-3">
              <InfoListCard title="Validation" items={validationIssues.length ? validationIssues : ['Run Validate to inspect workflow and policy issues.']} />
              <InfoListCard
                title="Simulation"
                items={[
                  simulation?.simulation?.transition
                    ? `${simulation.simulation.transition.from_state || '*'} -> ${simulation.simulation.transition.to_state || '*'} via ${simulation.simulation.transition.action || '-'}`
                    : 'No simulation loaded.',
                  simulation?.routing_preview?.resolved_via ? `Resolution: ${simulation.routing_preview.resolved_via}` : '',
                  simulation?.routing_preview?.resolved_assignee_username
                    ? `Assignee: ${simulation.routing_preview.resolved_assignee_username} (${simulation.routing_preview.resolved_assignee_user_id || ''})`
                    : '',
                  simulation?.routing_preview?.fallback_role_key ? `Fallback role: ${simulation.routing_preview.fallback_role_key}` : '',
                  simulation?.routing_preview?.error ? `Error: ${simulation.routing_preview.error}` : '',
                ].filter(Boolean)}
              />
            </div>
          </Panel>
        </section>
      </div>
    </div>
  )
}

function WorkflowStateNode({ data, selected }: NodeProps<Node<WorkflowNodeData>>) {
  return (
    <div className={`workflow-state-card${selected ? ' workflow-state-card-selected' : ''}`}>
      <Handle type="target" position={FlowPosition.Left} />
      <div className="workflow-state-card__eyebrow">
        <span>State</span>
        {data.isStart ? <span className="workflow-state-card__badge workflow-state-card__badge-start">Start</span> : null}
        {data.isTerminal ? <span className="workflow-state-card__badge workflow-state-card__badge-terminal">End</span> : null}
      </div>
      <div className="workflow-state-card__title">{data.label}</div>
      <div className="workflow-state-card__meta">{data.detail}</div>
      <Handle type="source" position={FlowPosition.Right} />
    </div>
  )
}

function computeAutoLayout(states: string[], actions: WorkflowActionRule[]): Record<string, Position> {
  if (!states.length) return {}

  const incoming = new Map<string, number>()
  const outgoing = new Map<string, string[]>()
  for (const state of states) {
    incoming.set(state, 0)
    outgoing.set(state, [])
  }
  for (const rule of actions) {
    if (rule.from_state && outgoing.has(rule.from_state)) {
      outgoing.get(rule.from_state)?.push(rule.to_state)
    }
    if (rule.to_state && incoming.has(rule.to_state)) {
      incoming.set(rule.to_state, (incoming.get(rule.to_state) || 0) + 1)
    }
  }

  const roots = states.filter((state) => (incoming.get(state) || 0) === 0)
  const queue = roots.length ? [...roots] : [states[0]]
  const levels = new Map<string, number>()
  const seen = new Set<string>()

  while (queue.length) {
    const state = queue.shift() || ''
    if (!state || seen.has(state)) continue
    seen.add(state)
    const parentLevel = levels.get(state) || 0
    for (const next of outgoing.get(state) || []) {
      const candidate = parentLevel + 1
      levels.set(next, Math.max(levels.get(next) || 0, candidate))
      if (!seen.has(next)) queue.push(next)
    }
  }

  for (const state of states) {
    if (!levels.has(state)) {
      levels.set(state, levels.size ? Math.max(...Array.from(levels.values())) + 1 : 0)
    }
  }

  const columns = new Map<number, string[]>()
  for (const state of states) {
    const level = levels.get(state) || 0
    const column = columns.get(level) || []
    column.push(state)
    columns.set(level, column)
  }

  const positioned: Record<string, Position> = {}
  for (const [level, columnStates] of Array.from(columns.entries()).sort((a, b) => a[0] - b[0])) {
    columnStates.sort()
    columnStates.forEach((state, index) => {
      positioned[state] = {
        x: level * 280,
        y: index * 150,
      }
    })
  }
  return positioned
}

function mergePositions(current: Record<string, Position>, computed: Record<string, Position>, states: string[]): Record<string, Position> {
  const next: Record<string, Position> = {}
  for (const state of states) {
    next[state] = current[state] || computed[state] || { x: 0, y: 0 }
  }
  return next
}

function nextStateKey(states: string[]): string {
  let counter = states.length + 1
  let candidate = `state_${counter}`
  while (states.includes(candidate)) {
    counter += 1
    candidate = `state_${counter}`
  }
  return candidate
}

function sanitizeStateKey(value: string): string {
  return value.trim().replace(/\s+/g, '_')
}

function edgeIDForRule(rule: WorkflowActionRule, index: number): string {
  return `workflow-edge-${index}-${rule.action || 'action'}-${rule.from_state || 'from'}-${rule.to_state || 'to'}`
}

function splitCSV(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function parseOptionalNumber(value: string): number | undefined {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : undefined
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
  const reactID = useId()
  const token = label.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
  const fieldID = `admin-field-${token || 'field'}-${reactID.replace(/[:]/g, '')}`
  let control = children
  if (isValidElement(children) && typeof children.type === 'string' && ['input', 'select', 'textarea'].includes(children.type)) {
    const element = children as ReactElement<{ id?: string; name?: string }>
    control = cloneElement(element, {
      id: element.props.id || fieldID,
      name: element.props.name || fieldID,
    })
  }
  return (
    <label className="block">
      <span className="mb-2 block text-xs font-semibold uppercase tracking-[0.14em] text-muted">{label}</span>
      {control}
    </label>
  )
}

function CheckboxField({ label, checked, onChange }: { label: string; checked: boolean; onChange: (checked: boolean) => void }) {
  return (
    <label className="flex items-center gap-3 rounded-xl border border-line bg-accent-soft/60 px-4 py-3 text-sm text-body">
      <input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} />
      <span>{label}</span>
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
