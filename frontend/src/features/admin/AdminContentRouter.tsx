export function AdminContentRouter({
  path,
  payload,
  bootstrap,
  renderModules,
  renderModuleConsole,
  renderAuth,
  renderMcp,
  renderAcp,
  renderConfig,
  renderFinance,
  renderDefinitions,
  renderTemplates,
  renderTemplateDesigner,
  renderWorkflows,
  renderWorkflowDesigner,
  renderSecurity,
  renderObservability,
  renderFallback,
}: {
  path: string
  payload: Record<string, unknown> | null
  bootstrap: unknown
  renderModules: (payload: Record<string, unknown> | null) => JSX.Element
  renderModuleConsole: (payload: Record<string, unknown> | null) => JSX.Element
  renderAuth: (payload: Record<string, unknown> | null) => JSX.Element
  renderMcp: (payload: Record<string, unknown> | null) => JSX.Element
  renderAcp: (payload: Record<string, unknown> | null) => JSX.Element
  renderConfig: (payload: Record<string, unknown> | null) => JSX.Element
  renderFinance: () => JSX.Element
  renderDefinitions: (payload: Record<string, unknown> | null) => JSX.Element
  renderTemplates: (payload: Record<string, unknown> | null) => JSX.Element
  renderTemplateDesigner: () => JSX.Element
  renderWorkflows: (payload: Record<string, unknown> | null) => JSX.Element
  renderWorkflowDesigner: () => JSX.Element
  renderSecurity: (payload: Record<string, unknown> | null) => JSX.Element
  renderObservability: (payload: Record<string, unknown> | null) => JSX.Element
  renderFallback: (path: string, payload: Record<string, unknown> | null, bootstrap: unknown) => JSX.Element
}) {
  if (path === '/modules') return renderModules(payload)
  if (path.startsWith('/modules/')) return renderModuleConsole(payload)
  if (path === '/auth') return renderAuth(payload)
  if (path === '/mcp') return renderMcp(payload)
  if (path === '/acp') return renderAcp(payload)
  if (path === '/config') return renderConfig(payload)
  if (path === '/finance') return renderFinance()
  if (path === '/definitions') return renderDefinitions(payload)
  if (path === '/templates') return renderTemplates(payload)
  if (path === '/templates/designer') return renderTemplateDesigner()
  if (path === '/workflows') return renderWorkflows(payload)
  if (path === '/workflows/designer') return renderWorkflowDesigner()
  if (path === '/security') return renderSecurity(payload)
  if (path === '/observability') return renderObservability(payload)
  return renderFallback(path, payload, bootstrap)
}
