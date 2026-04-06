import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { fetchWorkspaceBootstrap, toShellRoutes, workspaceSurfaceTarget } from "@/services/bootstrap";
import { preloadSurfaceModule } from "@/services/surfaceModules";
import { useShellStore } from "@/stores/shellStore";
import {
  DashboardWidgetCard,
  type DashboardResolvedWidget,
  defaultWidgetDataState,
  useSharedDashboardData,
} from "@/features/dashboard/runtime";

type DashboardBoardResponse = {
  surface: string;
  board: {
    id: string;
    name: string;
    description?: string;
    surface?: string;
    is_default?: boolean;
    updated_at?: string;
  } | null;
  widgets: DashboardResolvedWidget[];
};

export default function DashboardSurfacePage() {
  const navigate = useNavigate();
  const {
    locale,
    currentSurface,
    workspaceBootstrap,
    setCurrentRoute,
    setWorkspaceBootstrap,
    setRoutes,
    adminAccess,
  } = useShellStore();
  const [payload, setPayload] = useState<DashboardBoardResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const surface = "dashboard";

  useEffect(() => {
    setCurrentRoute("/dashboard");
  }, [setCurrentRoute]);

  async function switchSurface(nextSurface: string) {
    const [bootstrap] = await Promise.all([
      fetchWorkspaceBootstrap(nextSurface),
      preloadSurfaceModule(nextSurface),
    ]);
    setWorkspaceBootstrap(bootstrap);
    setRoutes(toShellRoutes(bootstrap.menus, bootstrap.actions, bootstrap.locale, "workspace"));
    navigate(workspaceSurfaceTarget(bootstrap, nextSurface) || "/", { replace: true });
  }

  useEffect(() => {
    let mounted = true;

    async function load() {
      setLoading(true);
      setError(null);
      try {
        const bootstrap =
          currentSurface === surface && workspaceBootstrap?.surface === surface
            ? workspaceBootstrap
            : await fetchWorkspaceBootstrap(surface);
        if (!mounted) return;
        setWorkspaceBootstrap(bootstrap);
        setRoutes(toShellRoutes(bootstrap.menus, bootstrap.actions, bootstrap.locale, "workspace"));
        const response = await fetch("/ui/data/dashboard/boards/effective?surface=dashboard", {
          credentials: "include",
        });
        if (!response.ok) {
          throw new Error(`Dashboard load failed: ${response.status}`);
        }
        const next = (await response.json()) as DashboardBoardResponse;
        if (!mounted) return;
        setPayload(next);
      } catch (nextError) {
        if (!mounted) return;
        setError(nextError instanceof Error ? nextError.message : "Dashboard load failed.");
      } finally {
        if (mounted) setLoading(false);
      }
    }

    void load();
    return () => {
      mounted = false;
    };
  }, [currentSurface, setRoutes, setWorkspaceBootstrap, workspaceBootstrap]);

  const updatedLabel = useMemo(() => {
    const value = payload?.board?.updated_at;
    if (!value) return "No curated board yet";
    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime())) return value;
    return `Updated ${parsed.toLocaleString()}`;
  }, [payload?.board?.updated_at]);
  const widgetData = useSharedDashboardData(payload?.widgets || []);

  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_top,_rgba(33,92,155,0.08),_transparent_28%),linear-gradient(180deg,_var(--color-shell),_color-mix(in_srgb,var(--color-shell)_88%,#0f172a_12%))] text-body">
      <header className="sticky top-0 z-20 border-b border-line/80 bg-surface/92 backdrop-blur">
        <div className="mx-auto flex max-w-[1700px] items-center justify-between gap-4 px-6 py-4">
          <div>
            <p className="text-xs font-bold uppercase tracking-[0.22em] text-accent-dark">Dashboard Surface</p>
            <h1 className="text-2xl font-black tracking-tight text-body">
              {payload?.board?.name || "Operational dashboard"}
            </h1>
          </div>
          <div className="flex flex-wrap items-center justify-end gap-3">
            <SurfaceStat label="Widgets" value={String(payload?.widgets?.length || 0)} detail={updatedLabel} />
            <SurfaceStat
              label="Refresh"
              value={loading ? "Syncing" : "Live"}
              detail="Widget cadence follows each module policy"
            />
            {adminAccess ? (
              <button
                type="button"
                onClick={() => {
                  window.location.href = "/admin/dashboards";
                }}
                className="rounded-xl border border-line bg-surface px-4 py-2 text-sm font-semibold text-body transition hover:border-accent hover:text-accent"
              >
                Manage boards
              </button>
            ) : null}
            <button
              type="button"
              onClick={() => void switchSurface("backoffice")}
              className="rounded-xl border border-line bg-surface px-4 py-2 text-sm font-semibold text-body transition hover:border-accent hover:text-accent"
            >
              Backoffice
            </button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-[1700px] px-4 py-4 md:px-6">
        <section className="overflow-hidden rounded-[1.5rem] border border-line bg-surface shadow-panel">
          <div className="border-b border-line px-6 py-5">
            <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
              <div className="max-w-3xl">
                <h2 className="text-lg font-semibold tracking-tight text-body">Board summary</h2>
                <p className="mt-2 max-w-2xl text-sm leading-6 text-muted">
                  {payload?.board?.description ||
                    "Curate cross-module operational widgets here. Each tile follows the module-defined renderer and refresh cadence instead of a freeform card mosaic."}
                </p>
              </div>
              <div className="flex flex-wrap items-center gap-2 text-xs text-muted">
                <span className="rounded-full border border-line bg-shell px-3 py-1.5 font-semibold uppercase tracking-[0.16em]">
                  {payload?.board?.is_default ? "Default board" : "Scoped board"}
                </span>
                <span className="rounded-full border border-line bg-shell px-3 py-1.5 font-semibold uppercase tracking-[0.16em]">
                  {updatedLabel}
                </span>
              </div>
            </div>
          </div>

          <div className="px-4 py-5 md:px-6">
            {loading ? (
              <div className="grid gap-4 lg:grid-cols-12">
                {Array.from({ length: 4 }).map((_, index) => (
                  <div
                    key={index}
                    className="animate-pulse rounded-[1.6rem] border border-line/70 bg-slate-50/90 p-5 lg:col-span-3"
                  >
                    <div className="h-3 w-24 rounded bg-slate-200" />
                    <div className="mt-4 h-8 w-32 rounded bg-slate-200" />
                    <div className="mt-8 h-24 rounded-[1rem] bg-slate-100" />
                  </div>
                ))}
              </div>
            ) : null}

            {!loading && error ? (
              <DashboardEmptyState
                title="Dashboard unavailable"
                description={error}
                actionLabel="Back to backoffice"
                onAction={() => navigate("/")}
              />
            ) : null}

            {!loading && !error && !payload?.board ? (
              <DashboardEmptyState
                title="No curated board yet"
                description="Create a default dashboard board in the admin workspace, add module widgets, and this surface will render it automatically."
                actionLabel={adminAccess ? "Open dashboard admin" : undefined}
                onAction={
                  adminAccess
                    ? () => {
                        window.location.href = "/admin/dashboards";
                      }
                    : undefined
                }
              />
            ) : null}

            {!loading && !error && payload?.board ? (
              <div className="grid gap-4 lg:grid-cols-12">
                {payload.widgets.map((widget, index) => (
                  <div
                    key={widget.id}
                    className="motion-safe:transition-transform motion-safe:duration-200"
                    style={{
                      transitionDelay: `${index * 40}ms`,
                      gridColumn: `span ${Math.min(Math.max(widget.width || 3, 1), 12)} / span ${Math.min(Math.max(widget.width || 3, 1), 12)}`,
                    }}
                  >
                    <DashboardWidgetCard
                      widget={widget}
                      locale={locale}
                      state={widgetData[widget.definition.data_path] || defaultWidgetDataState()}
                    />
                  </div>
                ))}
              </div>
            ) : null}
          </div>
        </section>
      </main>
    </div>
  );
}

function SurfaceStat({
  label,
  value,
  detail,
}: {
  label: string;
  value: string;
  detail: string;
}) {
  return (
    <div className="rounded-xl border border-line bg-shell px-4 py-4">
      <div className="text-[11px] font-bold uppercase tracking-[0.22em] text-muted">{label}</div>
      <div className="mt-2 text-2xl font-semibold tracking-tight text-body">{value}</div>
      <div className="mt-2 text-xs leading-5 text-muted">{detail}</div>
    </div>
  );
}

function DashboardEmptyState({
  title,
  description,
  actionLabel,
  onAction,
}: {
  title: string;
  description: string;
  actionLabel?: string;
  onAction?: () => void;
}) {
  return (
    <section className="rounded-[1.4rem] border border-dashed border-line bg-shell px-6 py-12 text-center">
      <div className="mx-auto max-w-xl">
        <div className="text-[11px] font-bold uppercase tracking-[0.28em] text-muted">Dashboard</div>
        <h2 className="mt-3 text-2xl font-semibold tracking-tight text-body">{title}</h2>
        <p className="mt-3 text-sm leading-6 text-muted">{description}</p>
        {actionLabel && onAction ? (
          <button
            type="button"
            onClick={onAction}
            className="mt-6 rounded-xl bg-accent px-5 py-3 text-sm font-semibold text-white transition hover:opacity-95"
          >
            {actionLabel}
          </button>
        ) : null}
      </div>
    </section>
  );
}
