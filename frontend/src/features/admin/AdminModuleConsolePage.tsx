import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { mutateJson } from "./adminClient";
import {
  EditableFieldSection,
  normalizeEditorPayload,
  normalizeEditorScope,
  normalizeEditorScopeID,
  resolvePath,
  ValueCard,
} from "./adminShared";

type ModuleConsoleSection = {
  key: string;
  title?: string;
  description?: string;
  kind: string;
  config_key?: string;
  definition?: Record<string, unknown>;
  entry?: Record<string, unknown>;
  editable?: boolean;
  links?: Array<Record<string, unknown>>;
};

type ModuleDependencyNode = {
  module_key: string;
  name?: string;
  version?: string;
  enabled?: boolean;
  lifecycle_state?: string;
  role?: string;
  domain_family?: string;
  category?: string;
  status?: string;
  console_path?: string;
};

type ModuleDependencyEdge = {
  source_module_key: string;
  target_module_key: string;
  kind?: string;
  version_range?: string;
  status?: string;
  reason?: string;
};

type ModuleDependencyGraph = {
  nodes?: ModuleDependencyNode[];
  edges?: ModuleDependencyEdge[];
  summary?: {
    total_modules?: number;
    enabled_modules?: number;
    unhealthy_modules?: number;
    total_edges?: number;
  };
};

export function buildClientModuleDependencyGraph(
  rows: Array<Record<string, unknown>>,
): ModuleDependencyGraph {
  const nodes = rows
    .map((row): ModuleDependencyNode | null => {
      const moduleKey = String(resolvePath(row, "manifest.key") || "");
      if (!moduleKey) return null;
      const lifecycleState = String(resolvePath(row, "lifecycle_state") || "");
      const enabled = Boolean(resolvePath(row, "installed.enabled"));
      return {
        module_key: moduleKey,
        name: String(resolvePath(row, "manifest.name") || moduleKey),
        version: String(resolvePath(row, "manifest.version") || ""),
        enabled,
        lifecycle_state: lifecycleState,
        role: String(resolvePath(row, "manifest.role") || ""),
        domain_family: String(resolvePath(row, "manifest.domain_family") || ""),
        category: String(resolvePath(row, "manifest.category") || ""),
        status:
          !enabled || lifecycleState === "disabled"
            ? "disabled"
            : lifecycleState === "healthy"
              ? "healthy"
              : "warning",
        console_path: `/admin/modules/${moduleKey}`,
      };
    })
    .filter((node): node is ModuleDependencyNode => node !== null);

  const detailByKey = new Map(
    rows.map((row) => [String(resolvePath(row, "manifest.key") || ""), row]),
  );
  const edges: ModuleDependencyEdge[] = [];
  for (const row of rows) {
    const sourceModuleKey = String(resolvePath(row, "manifest.key") || "");
    if (!sourceModuleKey) continue;
    const diagnostics = Array.isArray(
      resolvePath(row, "dependency_diagnostics"),
    )
      ? (resolvePath(row, "dependency_diagnostics") as Array<
          Record<string, unknown>
        >)
      : [];
    const diagnosticsByKey = new Map(
      diagnostics.map((item) => [String(item.module_key || ""), item]),
    );
    const requirements = clientManifestDependencies(row);
    for (const requirement of requirements) {
      const targetModuleKey = String(requirement.module_key || "");
      if (!targetModuleKey) continue;
      const diagnostic = diagnosticsByKey.get(targetModuleKey);
      const dependencyRow = detailByKey.get(targetModuleKey);
      const kind = String(requirement.kind || "required");
      const enabled = diagnostic
        ? Boolean(diagnostic.enabled)
        : Boolean(
            dependencyRow
              ? resolvePath(dependencyRow, "installed.enabled")
              : false,
          );
      const compatible = diagnostic ? Boolean(diagnostic.compatible) : true;
      edges.push({
        source_module_key: sourceModuleKey,
        target_module_key: targetModuleKey,
        kind,
        version_range: String(requirement.version_range || ""),
        status: !diagnostic
          ? "missing"
          : !enabled
            ? kind === "optional"
              ? "optional"
              : "disabled"
            : !compatible
              ? "incompatible"
              : "ok",
        reason: String((diagnostic && diagnostic.reason) || ""),
      });
    }
  }

  return {
    nodes,
    edges,
    summary: {
      total_modules: nodes.length,
      enabled_modules: nodes.filter((node) => node.enabled).length,
      unhealthy_modules: nodes.filter((node) => node.status !== "healthy")
        .length,
      total_edges: edges.length,
    },
  };
}

export function AdminModuleConsolePage({
  payload,
}: {
  payload: Record<string, unknown> | null;
}) {
  const navigate = useNavigate();
  const consolePayload = ((payload?.console as Record<
    string,
    unknown
  > | null) || {}) as Record<string, unknown>;
  const moduleDetail = ((payload?.module as Record<string, unknown> | null) ||
    {}) as Record<string, unknown>;
  const dependencyGraph =
    ((payload?.dependency_graph as ModuleDependencyGraph | null) ||
      {}) as ModuleDependencyGraph;
  const sections = Array.isArray(consolePayload.sections)
    ? (consolePayload.sections as ModuleConsoleSection[])
    : [];
  const title = String(
    consolePayload.title ||
      resolvePath(moduleDetail, "manifest.name") ||
      "Module Console",
  );
  const description = String(consolePayload.description || "");

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-line bg-accent-soft/60 p-4 text-sm text-body">
        <div className="font-semibold text-body">{title}</div>
        {description ? (
          <div className="mt-1 text-sm text-body">{description}</div>
        ) : null}
      </div>
      <ModuleDependencyGraphPanel
        title="Dependency Tree"
        description="This focused view shows the current module, its direct dependencies, and modules that depend on it."
        graph={dependencyGraph}
        compact
        onSelectModule={(moduleKey) =>
          navigate(`/modules/${encodeURIComponent(moduleKey)}`)
        }
      />
      {sections.length ? (
        sections.map((section) =>
          section.kind === "settings_form" ? (
            <ModuleConsoleSettingsSection key={section.key} section={section} />
          ) : (
            <ModuleConsoleLinkSection key={section.key} section={section} />
          ),
        )
      ) : (
        <ValueCard label="Console Payload" value={payload ?? {}} />
      )}
    </div>
  );
}

export function ModuleDependencyGraphPanel({
  title,
  description,
  graph,
  onSelectModule,
  compact = false,
}: {
  title: string;
  description?: string;
  graph: ModuleDependencyGraph;
  onSelectModule: (moduleKey: string) => void;
  compact?: boolean;
}) {
  const nodes = Array.isArray(graph.nodes) ? graph.nodes : [];
  const edges = Array.isArray(graph.edges) ? graph.edges : [];
  const summary = graph.summary || {};

  if (!nodes.length) {
    return (
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <div className="text-sm font-semibold text-body">{title}</div>
        {description ? (
          <div className="mt-1 text-sm text-muted">{description}</div>
        ) : null}
        <div className="mt-4 rounded-xl border border-dashed border-line p-4 text-sm text-muted">
          No dependency graph is available.
        </div>
      </section>
    );
  }

  const incoming = new Map<string, number>();
  const outgoing = new Map<string, number>();
  for (const edge of edges) {
    outgoing.set(
      edge.source_module_key,
      (outgoing.get(edge.source_module_key) || 0) + 1,
    );
    incoming.set(
      edge.target_module_key,
      (incoming.get(edge.target_module_key) || 0) + 1,
    );
  }

  const sortedNodes = [...nodes].sort((left, right) => {
    const leftWeight =
      (incoming.get(left.module_key) || 0) +
      (outgoing.get(left.module_key) || 0);
    const rightWeight =
      (incoming.get(right.module_key) || 0) +
      (outgoing.get(right.module_key) || 0);
    if (leftWeight === rightWeight) {
      return String(left.name || left.module_key).localeCompare(
        String(right.name || right.module_key),
      );
    }
    return rightWeight - leftWeight;
  });

  const maxRows = compact ? 12 : 24;
  const visibleNodes = sortedNodes.slice(0, maxRows);
  const visibleKeys = new Set(visibleNodes.map((node) => node.module_key));
  const visibleEdges = edges.filter(
    (edge) =>
      visibleKeys.has(edge.source_module_key) &&
      visibleKeys.has(edge.target_module_key),
  );
  const layout = buildModuleGraphLayout(visibleNodes, visibleEdges);

  return (
    <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="text-sm font-semibold text-body">{title}</div>
          {description ? (
            <div className="mt-1 text-sm text-muted">{description}</div>
          ) : null}
        </div>
        <div className="grid grid-cols-2 gap-2 text-xs text-body md:grid-cols-4">
          <MiniStat
            label="Modules"
            value={String(summary.total_modules || nodes.length)}
          />
          <MiniStat
            label="Enabled"
            value={String(
              summary.enabled_modules ||
                nodes.filter((node) => node.enabled).length,
            )}
          />
          <MiniStat
            label="Warnings"
            value={String(
              summary.unhealthy_modules ||
                nodes.filter((node) => node.status !== "healthy").length,
            )}
          />
          <MiniStat
            label="Edges"
            value={String(summary.total_edges || edges.length)}
          />
        </div>
      </div>
      <div className="overflow-auto rounded-xl border border-line bg-shell/40 p-3 dark:bg-ink/70">
        <svg width={layout.width} height={layout.height} className="min-w-full">
          <defs>
            <marker
              id="module-edge-arrow"
              markerWidth="10"
              markerHeight="10"
              refX="9"
              refY="3"
              orient="auto"
              markerUnits="strokeWidth"
            >
              <path d="M0,0 L0,6 L9,3 z" fill="#6b7280" />
            </marker>
          </defs>
          {visibleEdges.map((edge) => {
            const source = layout.positions.get(edge.source_module_key);
            const target = layout.positions.get(edge.target_module_key);
            if (!source || !target) return null;
            const startX = source.x + layout.nodeWidth;
            const startY = source.y + layout.nodeHeight / 2;
            const endX = target.x;
            const endY = target.y + layout.nodeHeight / 2;
            const delta = Math.max(40, (endX - startX) / 2);
            const path = `M ${startX} ${startY} C ${startX + delta} ${startY}, ${endX - delta} ${endY}, ${endX} ${endY}`;
            return (
              <path
                key={`${edge.source_module_key}:${edge.target_module_key}:${edge.kind}`}
                d={path}
                fill="none"
                stroke={moduleEdgeColor(edge.status)}
                strokeWidth={2}
                markerEnd="url(#module-edge-arrow)"
                opacity={0.9}
              />
            );
          })}
          {visibleNodes.map((node) => {
            const position = layout.positions.get(node.module_key);
            if (!position) return null;
            return (
              <g
                key={node.module_key}
                transform={`translate(${position.x}, ${position.y})`}
                className="cursor-pointer"
                onClick={() => onSelectModule(node.module_key)}
              >
                <rect
                  width={layout.nodeWidth}
                  height={layout.nodeHeight}
                  rx={14}
                  fill={moduleNodeFill(node.status)}
                  stroke={moduleNodeStroke(node.status)}
                  strokeWidth={2}
                />
                <text
                  x={14}
                  y={24}
                  fontSize="13"
                  fontWeight="700"
                  fill="#111827"
                >
                  {String(node.name || node.module_key).slice(0, 28)}
                </text>
                <text x={14} y={42} fontSize="11" fill="#374151">
                  {node.module_key}
                </text>
                <text x={14} y={60} fontSize="11" fill="#4b5563">
                  {node.version || "-"} • {node.role || "module"}
                </text>
                <text x={14} y={78} fontSize="11" fill="#4b5563">
                  {node.lifecycle_state || node.status || "-"}
                </text>
              </g>
            );
          })}
        </svg>
      </div>
      {visibleNodes.length < nodes.length ? (
        <div className="mt-3 text-xs text-muted">
          Showing {visibleNodes.length} of {nodes.length} modules. Open a module
          console for a focused local dependency view.
        </div>
      ) : null}
    </section>
  );
}

function MiniStat({ label, value }: { label: string; value: string }) {
  return (
    <article className="rounded-lg border border-line bg-surface px-3 py-2 dark:bg-ink/60">
      <div className="text-[10px] font-semibold uppercase tracking-[0.14em] text-muted">
        {label}
      </div>
      <div className="mt-1 text-sm font-semibold text-body">{value}</div>
    </article>
  );
}

function clientManifestDependencies(
  row: Record<string, unknown>,
): Array<Record<string, unknown>> {
  const dependencyRequirements = resolvePath(
    row,
    "manifest.dependency_requirements",
  );
  if (
    Array.isArray(dependencyRequirements) &&
    dependencyRequirements.length > 0
  ) {
    return dependencyRequirements.filter(
      (item): item is Record<string, unknown> =>
        Boolean(item && typeof item === "object"),
    );
  }
  const dependencies = resolvePath(row, "manifest.dependencies");
  if (!Array.isArray(dependencies)) return [];
  return dependencies.map((moduleKey) => ({
    module_key: String(moduleKey || ""),
    kind: "required",
    version_range: "",
  }));
}

function buildModuleGraphLayout(
  nodes: ModuleDependencyNode[],
  edges: ModuleDependencyEdge[],
) {
  const nodeWidth = 240;
  const nodeHeight = 92;
  const columnGap = 90;
  const rowGap = 26;
  const incoming = new Map<string, number>();
  const outgoing = new Map<string, number>();
  const dependents = new Map<string, string[]>();
  const indegree = new Map<string, number>();
  for (const node of nodes) {
    incoming.set(node.module_key, 0);
    outgoing.set(node.module_key, 0);
    dependents.set(node.module_key, []);
    indegree.set(node.module_key, 0);
  }
  for (const edge of edges) {
    if (
      !incoming.has(edge.target_module_key) ||
      !outgoing.has(edge.source_module_key)
    ) {
      continue;
    }
    incoming.set(
      edge.target_module_key,
      (incoming.get(edge.target_module_key) || 0) + 1,
    );
    outgoing.set(
      edge.source_module_key,
      (outgoing.get(edge.source_module_key) || 0) + 1,
    );
    dependents.set(edge.source_module_key, [
      ...(dependents.get(edge.source_module_key) || []),
      edge.target_module_key,
    ]);
    indegree.set(
      edge.target_module_key,
      (indegree.get(edge.target_module_key) || 0) + 1,
    );
  }

  const levels = new Map<string, number>();
  const queue: string[] = [];
  for (const node of nodes) {
    if ((indegree.get(node.module_key) || 0) === 0) {
      queue.push(node.module_key);
      levels.set(node.module_key, 0);
    }
  }
  queue.sort();
  while (queue.length) {
    const current = queue.shift() || "";
    const level = levels.get(current) || 0;
    for (const target of dependents.get(current) || []) {
      const nextLevel = Math.max(levels.get(target) || 0, level + 1);
      levels.set(target, nextLevel);
      indegree.set(target, (indegree.get(target) || 0) - 1);
      if ((indegree.get(target) || 0) === 0) queue.push(target);
    }
  }
  for (const node of nodes) {
    if (!levels.has(node.module_key)) levels.set(node.module_key, 0);
  }

  const columns = new Map<number, ModuleDependencyNode[]>();
  for (const node of nodes) {
    const level = levels.get(node.module_key) || 0;
    columns.set(level, [...(columns.get(level) || []), node]);
  }
  for (const [level, group] of columns.entries()) {
    group.sort((left, right) => {
      const leftWeight =
        (incoming.get(left.module_key) || 0) +
        (outgoing.get(left.module_key) || 0);
      const rightWeight =
        (incoming.get(right.module_key) || 0) +
        (outgoing.get(right.module_key) || 0);
      if (leftWeight === rightWeight) {
        return String(left.name || left.module_key).localeCompare(
          String(right.name || right.module_key),
        );
      }
      return rightWeight - leftWeight;
    });
    columns.set(level, group);
  }

  const positions = new Map<string, { x: number; y: number }>();
  const maxColumnSize = Math.max(
    ...Array.from(columns.values()).map((group) => group.length),
    1,
  );
  const width = Math.max(860, columns.size * (nodeWidth + columnGap) + 80);
  const height = Math.max(240, maxColumnSize * (nodeHeight + rowGap) + 60);
  for (const [level, group] of Array.from(columns.entries()).sort(
    (left, right) => left[0] - right[0],
  )) {
    const totalHeight =
      group.length * nodeHeight + Math.max(0, group.length - 1) * rowGap;
    const startY = Math.max(24, Math.round((height - totalHeight) / 2));
    group.forEach((node, index) => {
      positions.set(node.module_key, {
        x: 24 + level * (nodeWidth + columnGap),
        y: startY + index * (nodeHeight + rowGap),
      });
    });
  }
  return { positions, width, height, nodeWidth, nodeHeight };
}

function moduleNodeFill(status: string | undefined) {
  switch (status) {
    case "disabled":
      return "#f3f4f6";
    case "warning":
      return "#fff7ed";
    default:
      return "#ecfdf5";
  }
}

function moduleNodeStroke(status: string | undefined) {
  switch (status) {
    case "disabled":
      return "#9ca3af";
    case "warning":
      return "#f97316";
    default:
      return "#10b981";
  }
}

function moduleEdgeColor(status: string | undefined) {
  switch (status) {
    case "disabled":
      return "#9ca3af";
    case "missing":
    case "incompatible":
      return "#dc2626";
    case "optional":
      return "#f59e0b";
    default:
      return "#6b7280";
  }
}

function ModuleConsoleSettingsSection({
  section,
}: {
  section: ModuleConsoleSection;
}) {
  const definition = (section.definition || {}) as Record<string, unknown>;
  const entry = (section.entry || {}) as Record<string, unknown>;
  const fields = Array.isArray(definition.fields)
    ? (definition.fields as Array<Record<string, unknown>>)
    : [];
  const resolved =
    entry.value && typeof entry.value === "object"
      ? (entry.value as Record<string, unknown>)
      : {};
  const defaults =
    (definition.default_value && typeof definition.default_value === "object"
      ? (definition.default_value as Record<string, unknown>)
      : {}) || {};
  const [draft, setDraft] = useState<Record<string, unknown>>({
    ...defaults,
    ...resolved,
  });
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    setDraft({ ...defaults, ...resolved });
  }, [section.config_key, definition.default_value, entry.value]);

  async function save() {
    if (!section.config_key || !section.editable) return;
    setBusy(true);
    setMessage("");
    try {
      const response = await mutateJson<Record<string, unknown>>(
        `/admin/api/config/entries/${encodeURIComponent(section.config_key)}/value`,
        {
          method: "PUT",
          body: JSON.stringify({
            scope: normalizeEditorScope(entry.source_scope),
            scope_id: normalizeEditorScopeID(
              entry.source_scope,
              entry.source_scope_id,
            ),
            value: normalizeEditorPayload(fields, draft),
          }),
        },
      );
      setDraft((response.value as Record<string, unknown>) || draft);
      setMessage("Configuration updated.");
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : "Failed to update configuration.",
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
      <div className="mb-3">
        <div className="text-sm font-semibold text-body">
          {String(section.title || section.config_key || "Settings")}
        </div>
        {section.description ? (
          <div className="mt-1 text-sm text-muted">{section.description}</div>
        ) : null}
      </div>
      <EditableFieldSection
        label={String(section.title || section.config_key || "Settings")}
        fields={fields}
        values={draft}
        onChange={setDraft}
        disabled={!section.editable}
      />
      <div className="mt-4 flex items-center gap-3">
        {section.editable ? (
          <button
            type="button"
            className="admin-button"
            disabled={busy}
            onClick={() => void save()}
          >
            Save Settings
          </button>
        ) : (
          <div className="text-sm text-muted">
            Read-only for your current permissions.
          </div>
        )}
        {message ? <div className="text-sm text-body">{message}</div> : null}
      </div>
    </section>
  );
}

function ModuleConsoleLinkSection({
  section,
}: {
  section: ModuleConsoleSection;
}) {
  const links = Array.isArray(section.links) ? section.links : [];

  return (
    <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
      <div className="mb-4">
        <div className="text-sm font-semibold text-body">
          {String(section.title || "Links")}
        </div>
        {section.description ? (
          <div className="mt-1 text-sm text-muted">{section.description}</div>
        ) : null}
      </div>
      {links.length ? (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
          {links.map((link) => {
            const routePath = String(link.route_path || "");
            return (
              <article
                key={String(link.key || routePath)}
                className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60"
              >
                <div className="text-sm font-semibold text-body">
                  {String(link.label || routePath)}
                </div>
                {link.description ? (
                  <div className="mt-1 text-sm text-muted">
                    {String(link.description)}
                  </div>
                ) : null}
                <div className="mt-4">
                  <button
                    type="button"
                    className="admin-button admin-button-secondary"
                    onClick={() => {
                      if (routePath) {
                        window.location.assign(routePath);
                      }
                    }}
                  >
                    Open
                  </button>
                </div>
              </article>
            );
          })}
        </div>
      ) : (
        <div className="rounded-xl border border-dashed border-line p-4 text-sm text-muted">
          No links available for your current permissions.
        </div>
      )}
    </section>
  );
}
