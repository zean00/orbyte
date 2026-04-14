import { useEffect, useMemo, useState } from "react";
import { fetchJson, mutateJson } from "./adminClient";
import {
  asItems,
  DataGrid,
  displayValue,
  normalizeEditorScope,
  normalizeEditorScopeID,
  resolvePath,
} from "./adminShared";

function decorateMcpTools(
  items: Array<Record<string, unknown>>,
): Array<Record<string, unknown>> {
  return items.map((item) => ({
    ...item,
    action_class:
      String(item.action_class || resolvePath(item, "contract.actionClass") || "") ||
      "",
    risk_class:
      String(item.risk_class || resolvePath(item, "contract.riskClass") || "") ||
      "",
    source_type: String(item.source_type || "") || "",
    capability_keys: displayValue(
      item.capability_keys || resolvePath(item, "capabilityKeys"),
    ),
    capability_categories: displayValue(
      item.capability_categories || resolvePath(item, "capabilityCategories"),
    ),
    business_domains: displayValue(
      item.business_domains || resolvePath(item, "contract.businessDomains"),
    ),
    policy_state:
      String(item.policy_state || resolvePath(item, "policyState") || "") || "",
    policy_reason:
      String(item.policy_reason || resolvePath(item, "policyReason") || "") || "",
    governance_tags: displayValue(
      item.governance_tags || resolvePath(item, "contract.governanceTags"),
    ),
  }));
}

function buildMcpToolStatesJSON(
  items: Array<Record<string, unknown>>,
): string {
  const states: Record<string, boolean> = {};
  for (const item of items) {
    const key = String(item.key || item.name || "").trim();
    if (!key) continue;
    states[key] = Boolean(item.enabled);
  }
  return JSON.stringify(states);
}

export function AdminMCPPage({
  payload,
  renderSummaryCard,
}: {
  payload: Record<string, unknown> | null;
  renderSummaryCard: (props: { label: string; value: string }) => JSX.Element;
}) {
  const runtime = ((payload?.runtime || {}) as Record<string, unknown>) || {};
  const entry = (payload?.entry || {}) as Record<string, unknown>;
  const compactPreview =
    ((payload?.compact_preview || {}) as Record<string, unknown>) || {};
  const [runtimeState, setRuntimeState] = useState(runtime);
  const [capabilityRows, setCapabilityRows] = useState(
    asItems({
      items: payload?.capabilities as Array<Record<string, unknown>> | undefined,
    }),
  );
  const [compactPreviewState, setCompactPreviewState] = useState(compactPreview);
  const compactCatalogState =
    ((compactPreviewState.catalog || {}) as Record<string, unknown>) || {};
  const [policySummaryState, setPolicySummaryState] = useState(
    ((payload?.policy_summary || {}) as Record<string, unknown>) || {},
  );
  const [governanceActivityState, setGovernanceActivityState] = useState(
    asItems({
      items: payload?.governance_activity as
        | Array<Record<string, unknown>>
        | undefined,
    }),
  );
  const [tools, setTools] = useState(
    decorateMcpTools(
      asItems({
        items: payload?.tools as Array<Record<string, unknown>> | undefined,
      }),
    ),
  );
  const [busyKey, setBusyKey] = useState("");
  const [message, setMessage] = useState("");
  const [policyBusy, setPolicyBusy] = useState(false);
  const [governanceEnabled, setGovernanceEnabled] = useState(
    Boolean(resolvePath(entry, "value.governance_enabled")),
  );
  const [defaultActionMode, setDefaultActionMode] = useState(
    String(resolvePath(entry, "value.default_action_mode") || "draft_only"),
  );
  const [blockedActionClasses, setBlockedActionClasses] = useState("[]");
  const [blockedToolKeys, setBlockedToolKeys] = useState("[]");
  const [blockedDocumentTypes, setBlockedDocumentTypes] = useState("[]");
  const [allowedSubmitDocumentTypes, setAllowedSubmitDocumentTypes] =
    useState("[]");
  const [domainPolicyOverrides, setDomainPolicyOverrides] = useState("{}");

  useEffect(() => {
    setRuntimeState(runtime);
    setCapabilityRows(
      asItems({
        items: payload?.capabilities as
          | Array<Record<string, unknown>>
          | undefined,
      }),
    );
    setCompactPreviewState(compactPreview);
    setTools(
      decorateMcpTools(
        asItems({
          items: payload?.tools as Array<Record<string, unknown>> | undefined,
        }),
      ),
    );
    setPolicySummaryState(
      ((payload?.policy_summary || {}) as Record<string, unknown>) || {},
    );
    setGovernanceActivityState(
      asItems({
        items: payload?.governance_activity as
          | Array<Record<string, unknown>>
          | undefined,
      }),
    );
  }, [compactPreview, payload, runtime]);

  useEffect(() => {
    setGovernanceEnabled(Boolean(resolvePath(entry, "value.governance_enabled")));
    setDefaultActionMode(
      String(resolvePath(entry, "value.default_action_mode") || "draft_only"),
    );
    setBlockedActionClasses(
      String(resolvePath(entry, "value.blocked_action_classes_json") || "[]"),
    );
    setBlockedToolKeys(
      String(resolvePath(entry, "value.blocked_tool_keys_json") || "[]"),
    );
    setBlockedDocumentTypes(
      String(resolvePath(entry, "value.blocked_document_types_json") || "[]"),
    );
    setAllowedSubmitDocumentTypes(
      String(
        resolvePath(entry, "value.allowed_submit_document_types_json") || "[]",
      ),
    );
    setDomainPolicyOverrides(
      String(resolvePath(entry, "value.domain_policy_overrides_json") || "{}"),
    );
  }, [entry]);

  const groupedCounts = useMemo(() => {
    const actionClasses = new Set<string>();
    const advisorTools = new Set<string>();
    for (const tool of tools) {
      const actionClass = String(tool.action_class || "");
      if (actionClass) actionClasses.add(actionClass);
      const governance = String(tool.governance_tags || "");
      if (governance.includes("advisor-pack")) {
        advisorTools.add(String(tool.key || ""));
      }
    }
    return {
      actionClasses: actionClasses.size,
      advisorTools: advisorTools.size,
    };
  }, [tools]);

  async function toggleTool(row: Record<string, unknown>) {
    const key = String(row.key || "");
    const enabled = Boolean(row.enabled);
    if (!key) return;
    setBusyKey(key);
    setMessage("");
    try {
      const response = await mutateJson<{ runtime?: Record<string, unknown> }>(
        `/admin/api/mcp/tools/${encodeURIComponent(key)}/enabled`,
        {
          method: "PUT",
          body: JSON.stringify({ enabled: !enabled }),
        },
      );
      const nextRuntime = (response.runtime || {}) as Record<string, unknown>;
      setRuntimeState(nextRuntime);
      setCapabilityRows(
        asItems({
          items: nextRuntime.capabilities as
            | Array<Record<string, unknown>>
            | undefined,
        }),
      );
      setCompactPreviewState(
        ((nextRuntime.compact_preview || {}) as Record<string, unknown>) || {},
      );
      setTools(
        decorateMcpTools(
          asItems({
            items: nextRuntime.tools as
              | Array<Record<string, unknown>>
              | undefined,
          }),
        ),
      );
      setMessage(`Tool ${enabled ? "disabled" : "enabled"}.`);
      setPolicySummaryState(
        ((nextRuntime.policy_summary || {}) as Record<string, unknown>) || {},
      );
      setGovernanceActivityState(
        asItems({
          items: nextRuntime.governance_activity as
            | Array<Record<string, unknown>>
            | undefined,
        }),
      );
    } catch (error) {
      setMessage(
        error instanceof Error ? error.message : "Failed to update tool state.",
      );
    } finally {
      setBusyKey("");
    }
  }

  async function saveGovernance() {
    setPolicyBusy(true);
    setMessage("");
    try {
      JSON.parse(blockedActionClasses);
      JSON.parse(blockedToolKeys);
      JSON.parse(blockedDocumentTypes);
      JSON.parse(allowedSubmitDocumentTypes);
      JSON.parse(domainPolicyOverrides);
      await mutateJson<Record<string, unknown>>(
        "/admin/api/config/entries/platform.mcp/value",
        {
          method: "PUT",
          body: JSON.stringify({
            scope: normalizeEditorScope(entry.source_scope),
            scope_id: normalizeEditorScopeID(
              entry.source_scope,
              entry.source_scope_id,
            ),
            value: {
              enabled: Boolean(runtimeState.enabled),
              tool_states_json: buildMcpToolStatesJSON(tools),
              governance_enabled: governanceEnabled,
              default_action_mode: defaultActionMode,
              blocked_action_classes_json: blockedActionClasses,
              blocked_tool_keys_json: blockedToolKeys,
              blocked_document_types_json: blockedDocumentTypes,
              allowed_submit_document_types_json: allowedSubmitDocumentTypes,
              domain_policy_overrides_json: domainPolicyOverrides,
            },
          }),
        },
      );
      const nextRuntime = await fetchJson<Record<string, unknown>>("/admin/api/mcp");
      setRuntimeState(nextRuntime);
      setCapabilityRows(
        asItems({
          items: nextRuntime.capabilities as
            | Array<Record<string, unknown>>
            | undefined,
        }),
      );
      setCompactPreviewState(
        ((nextRuntime.compact_preview || {}) as Record<string, unknown>) || {},
      );
      setTools(
        decorateMcpTools(
          asItems({
            items: nextRuntime.tools as
              | Array<Record<string, unknown>>
              | undefined,
          }),
        ),
      );
      setPolicySummaryState(
        ((nextRuntime.policy_summary || {}) as Record<string, unknown>) || {},
      );
      setGovernanceActivityState(
        asItems({
          items: nextRuntime.governance_activity as
            | Array<Record<string, unknown>>
            | undefined,
        }),
      );
      setMessage("MCP governance policy updated.");
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : "Failed to update MCP governance policy.",
      );
    } finally {
      setPolicyBusy(false);
    }
  }

  return (
    <div className="space-y-4">
      {message ? (
        <div className="rounded-xl border border-line bg-accent-soft/60 p-4 text-sm text-body">
          {message}
        </div>
      ) : null}
      <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
        {renderSummaryCard({
          label: "MCP Enabled",
          value: Boolean(runtimeState.enabled) ? "Yes" : "No",
        })}
        {renderSummaryCard({
          label: "Bind Address",
          value: String(runtimeState.http_address || ":8080"),
        })}
        {renderSummaryCard({ label: "Port", value: String(runtimeState.port || "-") })}
        {renderSummaryCard({ label: "Tools", value: String(tools.length) })}
        {renderSummaryCard({
          label: "Action Classes",
          value: String(groupedCounts.actionClasses),
        })}
        {renderSummaryCard({
          label: "Governance",
          value: Boolean(policySummaryState.governance_enabled) ? "On" : "Off",
        })}
        {renderSummaryCard({
          label: "Advisor Tools",
          value: String(groupedCounts.advisorTools),
        })}
        {renderSummaryCard({
          label: "Compact Default",
          value: String(compactCatalogState.returned_tools || 0),
        })}
        {renderSummaryCard({
          label: "Hidden by Default",
          value: String(compactCatalogState.hidden_tools || 0),
        })}
        {renderSummaryCard({
          label: "Draft Enabled",
          value: String(policySummaryState.draft_enabled_tools || 0),
        })}
        {renderSummaryCard({
          label: "Submit Allowed",
          value: String(policySummaryState.submit_allowlisted_tools || 0),
        })}
        {renderSummaryCard({
          label: "Blocked Attempts",
          value: String(policySummaryState.blocked_attempts || 0),
        })}
      </div>
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <div className="mb-3 text-sm font-semibold text-body">
          Governance Policy
        </div>
        {/*
          Keep explicit ids/names on these fields so browser a11y audits can
          associate the controls even when labels wrap rich content.
        */}
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <label className="space-y-2 text-sm text-body">
            <span className="block text-xs font-semibold uppercase tracking-wide text-muted">
              Governance Enabled
            </span>
            <input
              id="mcp-governance-enabled"
              name="mcp-governance-enabled"
              type="checkbox"
              checked={governanceEnabled}
              onChange={(event) => setGovernanceEnabled(event.target.checked)}
            />
          </label>
          <label className="space-y-2 text-sm text-body">
            <span className="block text-xs font-semibold uppercase tracking-wide text-muted">
              Default Action Mode
            </span>
            <select
              id="mcp-default-action-mode"
              name="mcp-default-action-mode"
              className="admin-input"
              value={defaultActionMode}
              onChange={(event) => setDefaultActionMode(event.target.value)}
            >
              <option value="draft_only">Draft Only</option>
              <option value="governed">Governed</option>
            </select>
          </label>
          <label className="space-y-2 text-sm text-body">
            <span className="block text-xs font-semibold uppercase tracking-wide text-muted">
              Blocked Action Classes JSON
            </span>
            <textarea
              id="mcp-blocked-action-classes"
              name="mcp-blocked-action-classes"
              className="admin-input min-h-24"
              value={blockedActionClasses}
              onChange={(event) => setBlockedActionClasses(event.target.value)}
            />
          </label>
          <label className="space-y-2 text-sm text-body">
            <span className="block text-xs font-semibold uppercase tracking-wide text-muted">
              Blocked Tool Keys JSON
            </span>
            <textarea
              id="mcp-blocked-tool-keys"
              name="mcp-blocked-tool-keys"
              className="admin-input min-h-24"
              value={blockedToolKeys}
              onChange={(event) => setBlockedToolKeys(event.target.value)}
            />
          </label>
          <label className="space-y-2 text-sm text-body">
            <span className="block text-xs font-semibold uppercase tracking-wide text-muted">
              Blocked Document Types JSON
            </span>
            <textarea
              id="mcp-blocked-document-types"
              name="mcp-blocked-document-types"
              className="admin-input min-h-24"
              value={blockedDocumentTypes}
              onChange={(event) => setBlockedDocumentTypes(event.target.value)}
            />
          </label>
          <label className="space-y-2 text-sm text-body">
            <span className="block text-xs font-semibold uppercase tracking-wide text-muted">
              Allowed Submit Document Types JSON
            </span>
            <textarea
              id="mcp-allowed-submit-document-types"
              name="mcp-allowed-submit-document-types"
              className="admin-input min-h-24"
              value={allowedSubmitDocumentTypes}
              onChange={(event) =>
                setAllowedSubmitDocumentTypes(event.target.value)
              }
            />
          </label>
          <label className="space-y-2 text-sm text-body lg:col-span-2">
            <span className="block text-xs font-semibold uppercase tracking-wide text-muted">
              Domain Policy Overrides JSON
            </span>
            <textarea
              id="mcp-domain-policy-overrides"
              name="mcp-domain-policy-overrides"
              className="admin-input min-h-32"
              value={domainPolicyOverrides}
              onChange={(event) => setDomainPolicyOverrides(event.target.value)}
            />
          </label>
        </div>
        <div className="mt-4 flex items-center gap-3">
          <button
            type="button"
            className="admin-button"
            disabled={policyBusy}
            onClick={() => void saveGovernance()}
          >
            Save Governance
          </button>
          <div className="text-xs text-muted">
            Draft-only mode blocks submit and controlled mutation unless allowlisted.
          </div>
        </div>
      </section>
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <div className="mb-3 text-sm font-semibold text-body">Endpoints</div>
        <DataGrid
          columns={[
            { key: "label", label: "Endpoint" },
            { key: "path", label: "Path" },
            { key: "url", label: "URL" },
          ]}
          rows={asItems({
            items: runtime.paths as Array<Record<string, unknown>> | undefined,
          })}
        />
      </section>
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <div className="mb-3 text-sm font-semibold text-body">Capabilities</div>
        <DataGrid
          columns={[
            { key: "key", label: "Capability" },
            { key: "category", label: "Category" },
            { key: "default_for_agent", label: "Agent Default" },
            { key: "estimated_tool_count", label: "Tools" },
            { key: "description", label: "Description" },
          ]}
          rows={capabilityRows}
        />
      </section>
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <div className="mb-3 text-sm font-semibold text-body">
          Agent Default Compact Preview
        </div>
        <div className="mb-4 grid grid-cols-1 gap-4 md:grid-cols-4">
          {renderSummaryCard({
            label: "Mode",
            value: String(compactCatalogState.mode || "compact"),
          })}
          {renderSummaryCard({
            label: "Returned",
            value: String(compactCatalogState.returned_tools || 0),
          })}
          {renderSummaryCard({
            label: "Hidden",
            value: String(compactCatalogState.hidden_tools || 0),
          })}
          {renderSummaryCard({
            label: "Max Tools",
            value: String(compactCatalogState.max_tools || 0),
          })}
        </div>
        <DataGrid
          columns={[
            { key: "name", label: "Tool" },
            { key: "sourceType", label: "Source" },
            { key: "moduleKey", label: "Module" },
            { key: "capabilityKeys", label: "Capabilities" },
            { key: "contract.actionClass", label: "Action" },
          ]}
          rows={asItems({
            items: compactPreviewState.tools as
              | Array<Record<string, unknown>>
              | undefined,
          })}
        />
      </section>
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <div className="mb-3 text-sm font-semibold text-body">Tools</div>
        <DataGrid
          columns={[
            { key: "key", label: "Tool" },
            { key: "capability_keys", label: "Capabilities" },
            { key: "source_type", label: "Source" },
            { key: "module_key", label: "Module" },
            { key: "action_class", label: "Action" },
            { key: "risk_class", label: "Risk" },
            { key: "policy_state", label: "Policy" },
            { key: "business_domains", label: "Domains" },
            { key: "policy_reason", label: "Reason" },
            { key: "endpoint_scope", label: "Scope" },
            { key: "enabled", label: "Enabled" },
            { key: "operation", label: "Operation" },
          ]}
          rows={tools}
          secondaryActionLabel="Toggle"
          secondaryActionLabelForRow={(row) =>
            Boolean(row.enabled) ? "Disable" : "Enable"
          }
          onSecondaryAction={(row) => void toggleTool(row)}
          secondaryActionDisabledForRow={(row) =>
            busyKey === String(row.key || "")
          }
        />
      </section>
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <div className="mb-3 text-sm font-semibold text-body">Resources</div>
        <DataGrid
          columns={[
            { key: "key", label: "Resource" },
            { key: "module_key", label: "Module" },
            { key: "uri", label: "URI" },
            { key: "endpoint_scope", label: "Scope" },
          ]}
          rows={asItems({
            items: payload?.resources as
              | Array<Record<string, unknown>>
              | undefined,
          })}
        />
      </section>
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <div className="mb-3 text-sm font-semibold text-body">Apps</div>
        <DataGrid
          columns={[
            { key: "key", label: "App" },
            { key: "module_key", label: "Module" },
            { key: "resource_key", label: "Resource" },
            { key: "endpoint_scope", label: "Scope" },
          ]}
          rows={asItems({
            items: payload?.apps as Array<Record<string, unknown>> | undefined,
          })}
        />
      </section>
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <div className="mb-3 text-sm font-semibold text-body">
          Governance Activity
        </div>
        <DataGrid
          columns={[
            { key: "occurred_at", label: "Occurred" },
            { key: "tool_name", label: "Tool" },
            { key: "action_class", label: "Action" },
            { key: "risk_class", label: "Risk" },
            { key: "policy_state", label: "Policy" },
            { key: "status", label: "Status" },
            { key: "policy_reason", label: "Reason" },
            { key: "actor_id", label: "Actor" },
          ]}
          rows={governanceActivityState}
        />
      </section>
    </div>
  );
}
