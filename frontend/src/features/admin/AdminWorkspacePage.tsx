import { useEffect, useMemo, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { PageSection } from "@/components/layout/PageSection";
import { Shell } from "@/components/layout/Shell";
import { useShellStore } from "@/stores/shellStore";
import { normalizeShellPath } from "@/services/bootstrap";
import { AdminContentRouter } from "./AdminContentRouter";
import { AdminConfigManagementPage } from "./AdminConfigManagementPage";
import { AdminDefinitionsGrid } from "./AdminDefinitionsGrid";
import { AdminAuthSettingsPage } from "./AdminAuthSettingsPage";
import { AdminACPPage } from "./AdminACPPage";
import { AdminFinanceSettingsPage } from "./AdminFinanceSettingsPage";
import { AdminMCPPage } from "./AdminMCPPage";
import {
  AdminModuleConsolePage,
  buildClientModuleDependencyGraph,
  ModuleDependencyGraphPanel,
} from "./AdminModuleConsolePage";
import { AdminObservabilityContent } from "./AdminObservabilityContent";
import { AdminDashboardBoardsPage } from "./AdminDashboardBoardsPage";
import { AdminSecurityHooksPage } from "./AdminSecurityHooksPage";
import { AdminTemplateListPage } from "./AdminTemplateListPage";
import { AdminWorkflowListPage } from "./AdminWorkflowListPage";
import {
  asItems,
  DataGrid,
  resolvePath,
  ValueCard,
} from "./adminShared";
import { titleForAdminPath } from "./adminRouting";
import { useAdminPageData } from "./useAdminPageData";
import { mutateJson } from "./adminClient";
import { TemplateDesignerPage } from "./TemplateDesignerPage";
import { WorkflowDesignerPage } from "./WorkflowDesignerPage";

export default function AdminWorkspacePage() {
  const location = useLocation();
  const navigate = useNavigate();
  const bootstrap = useShellStore((state) => state.adminBootstrap);
  const defaultPath = useShellStore((state) => state.defaultPath);
  const routes = useShellStore((state) => state.routes);
  const setNavigationPending = useShellStore((state) => state.setNavigationPending);
  const path = normalizeShellPath(location.pathname || "/", "admin");
  const { payload, loading } = useAdminPageData(path, !!bootstrap);

  useEffect(() => {
    if (path === "/" && defaultPath && defaultPath !== "/") {
      navigate(defaultPath, { replace: true });
    }
  }, [defaultPath, navigate, path]);

  const title = useMemo(() => {
    return (
      routes.find((item) => item.path === path)?.label || titleForAdminPath(path)
    );
  }, [path, routes]);

  useEffect(() => {
    setNavigationPending(loading);
    return () => {
      setNavigationPending(false);
    };
  }, [loading, setNavigationPending]);

  return (
    <Shell
      loading={loading}
      loadingLabel="Loading admin data from PostgreSQL."
    >
      <PageSection
        title={title}
        status={
          loading
            ? "Loading admin data from the current server APIs."
            : "Admin data rendered from the existing server APIs."
        }
      >

        {path === "/org" && bootstrap ? (
          <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
            <SummaryCard
              label="Organization"
              value={String(
                (bootstrap.organization as Record<string, unknown> | undefined)
                  ?.name || "Root",
              )}
            />
            <SummaryCard
              label="Locations"
              value={String(bootstrap.locations?.length || 0)}
            />
            <SummaryCard
              label="Operating Units"
              value={String(bootstrap.operating_units?.length || 0)}
            />
          </div>
        ) : null}

        <div className="mt-4">
          <AdminContent path={path} payload={payload} bootstrap={bootstrap} />
        </div>
      </PageSection>
    </Shell>
  );
}

function AdminContent({
  path,
  payload,
  bootstrap,
}: {
  path: string;
  payload: Record<string, unknown> | null;
  bootstrap: ReturnType<typeof useShellStore.getState>["adminBootstrap"];
}) {
  return (
    <AdminContentRouter
      path={path}
      payload={payload}
      bootstrap={bootstrap}
      renderModules={(data) => <ModuleManagementPage payload={data} />}
      renderModuleConsole={(data) => <AdminModuleConsolePage payload={data} />}
      renderAuth={(data) => (
        <AdminAuthSettingsPage
          payload={data}
          renderSummaryCard={(props) => <SummaryCard {...props} />}
        />
      )}
      renderMcp={(data) => (
        <AdminMCPPage
          payload={data}
          renderSummaryCard={(props) => <SummaryCard {...props} />}
        />
      )}
      renderAcp={(data) => (
        <AdminACPPage
          payload={data}
          renderSummaryCard={(props) => <SummaryCard {...props} />}
        />
      )}
      renderConfig={(data) => (
        <AdminConfigManagementPage definitions={asItems(data)} />
      )}
      renderFinance={() => (
        <AdminFinanceSettingsPage
          renderSummaryCard={(props) => <SummaryCard {...props} />}
        />
      )}
      renderDefinitions={(data) => (
        <AdminDefinitionsGrid
          rows={asItems(data)}
          renderDataGrid={({ columns, rows }) => (
            <DataGrid columns={columns} rows={rows} />
          )}
        />
      )}
      renderTemplates={(data) => (
        <AdminTemplateListPage
          rows={asItems(data)}
          renderDataGrid={(props) => <DataGrid {...props} />}
        />
      )}
      renderTemplateDesigner={() => <TemplateDesignerPage />}
      renderWorkflows={(data) => (
        <AdminWorkflowListPage
          rows={asItems(data)}
          renderDataGrid={(props) => <DataGrid {...props} />}
        />
      )}
      renderWorkflowDesigner={() => <WorkflowDesignerPage />}
      renderSecurity={(data) => (
        <AdminSecurityHooksPage rows={asItems(data)} />
      )}
      renderObservability={(data) => {
        return (
          <AdminObservabilityContent
            payload={data}
            asItems={(value) => asItems(value ?? null)}
            renderSummaryCard={({ label, value }) => (
              <SummaryCard label={label} value={value} />
            )}
            renderDataGrid={({ columns, rows }) => (
              <DataGrid columns={columns} rows={rows} />
            )}
          />
        );
      }}
      renderDashboards={(data) => <AdminDashboardBoardsPage payload={data} />}
      renderFallback={(targetPath, data, adminBootstrap) => (
        <ValueCard
          label="Raw payload"
          value={data ?? bootstrapSummary(targetPath, adminBootstrap as ReturnType<typeof useShellStore.getState>["adminBootstrap"])}
        />
      )}
    />
  );
}

function bootstrapSummary(
  path: string,
  bootstrap: ReturnType<typeof useShellStore.getState>["adminBootstrap"],
) {
  const normalizedPath = normalizeShellPath(path, "admin");
  if (!bootstrap) return {};
  if (normalizedPath === "/" || normalizedPath === "") {
    return {
      menus: bootstrap.menus,
      actions: bootstrap.actions,
      default_path: bootstrap.default_path,
    };
  }
  if (normalizedPath === "/org") {
    return {
      organization: bootstrap.organization,
      locations: bootstrap.locations,
      operating_units: bootstrap.operating_units,
      roles: bootstrap.roles,
    };
  }
  return bootstrap;
}

function SummaryCard({ label, value }: { label: string; value: string }) {
  return (
    <article className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
      <div className="text-xs font-semibold uppercase tracking-wide text-body">
        {label}
      </div>
      <div className="mt-2 text-2xl font-bold text-body">{value}</div>
    </article>
  );
}

function ModuleManagementPage({
  payload,
}: {
  payload: Record<string, unknown> | null;
}) {
  const navigate = useNavigate();
  const rows = asItems(payload);
  const [items, setItems] = useState(rows);
  const [busyKey, setBusyKey] = useState("");
  const [message, setMessage] = useState("");

  useEffect(() => {
    setItems(rows);
  }, [rows]);

  const graph = buildClientModuleDependencyGraph(items);

  async function toggleModule(row: Record<string, unknown>) {
    const key = String(resolvePath(row, "manifest.key") || "");
    const enabled = Boolean(resolvePath(row, "installed.enabled"));
    if (!key) return;
    setBusyKey(key);
    setMessage("");
    try {
      const updated = await mutateJson<Record<string, unknown>>(
        `/admin/api/modules/${encodeURIComponent(key)}/actions/${enabled ? "disable" : "enable"}`,
        {
          method: "POST",
        },
      );
      setItems((current) =>
        current.map((item) =>
          String(resolvePath(item, "manifest.key") || "") === key
            ? {
                ...item,
                installed: {
                  ...(((item.installed as Record<string, unknown>) ||
                    {}) as Record<string, unknown>),
                  enabled: Boolean(updated.enabled),
                  updated_at:
                    updated.updated_at ||
                    resolvePath(item, "installed.updated_at"),
                  updated_by:
                    updated.updated_by ||
                    resolvePath(item, "installed.updated_by"),
                },
                lifecycle_state:
                  updated.lifecycle_state || item.lifecycle_state,
              }
            : item,
        ),
      );
      setMessage(`Module ${enabled ? "disabled" : "enabled"}.`);
    } catch (error) {
      setMessage(
        error instanceof Error ? error.message : "Failed to update module.",
      );
    } finally {
      setBusyKey("");
    }
  }

  return (
    <div className="space-y-4">
      {message ? (
        <div className="rounded-xl border border-line bg-accent-soft/60 p-4 text-sm text-body">
          {message}
        </div>
      ) : null}
      <ModuleDependencyGraphPanel
        title="Dependency Tree"
        description="Visualize module dependencies, dependency health, and direct navigation to each module console."
        graph={graph}
        onSelectModule={(moduleKey) =>
          navigate(`/modules/${encodeURIComponent(moduleKey)}`)
        }
      />
      <DataGrid
        columns={[
          { key: "manifest.key", label: "Module" },
          { key: "manifest.name", label: "Name" },
          { key: "manifest.version", label: "Version" },
          { key: "installed.enabled", label: "Enabled" },
          { key: "lifecycle_state", label: "Lifecycle" },
        ]}
        rows={items}
        actionLabel="Open Console"
        onAction={(row) =>
          navigate(
            `/modules/${encodeURIComponent(String(resolvePath(row, "manifest.key") || ""))}`,
          )
        }
        actionDisabledForRow={(row) =>
          !Array.isArray(
            resolvePath(row, "manifest.admin_console.sections") as
              | unknown[]
              | undefined,
          ) ||
          !(
            resolvePath(row, "manifest.admin_console.sections") as
              | unknown[]
              | undefined
          )?.length
        }
        secondaryActionLabel="Toggle"
        secondaryActionLabelForRow={(row) =>
          Boolean(resolvePath(row, "installed.enabled")) ? "Disable" : "Enable"
        }
        onSecondaryAction={(row) => void toggleModule(row)}
        secondaryActionDisabledForRow={(row) =>
          busyKey === String(resolvePath(row, "manifest.key") || "")
        }
      />
    </div>
  );
}
