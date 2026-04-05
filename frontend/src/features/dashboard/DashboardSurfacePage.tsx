import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { fetchWorkspaceBootstrap, pickText, toShellRoutes } from "@/services/bootstrap";
import { useShellStore } from "@/stores/shellStore";

type DashboardWidgetDefinition = {
  key: string;
  title: string;
  title_i18n?: Record<string, string>;
  renderer_kind: string;
  refresh_policy?: string;
  data_path: string;
  default_width?: number;
  default_height?: number;
  metric?: {
    value_path: string;
    format?: string;
    unit?: string;
    delta_path?: string;
    target_path?: string;
  };
  table?: {
    rows_path: string;
    columns?: Array<{ key: string; label: string; label_i18n?: Record<string, string>; path: string }>;
  };
  chart?: {
    series_path: string;
    category: string;
    value: string;
    series?: string;
    stacked?: boolean;
    format?: string;
  };
  gauge?: {
    value_path: string;
    min_value?: number;
    max_value?: number;
    thresholds?: number[];
    format?: string;
  };
  map?: {
    points_path: string;
    latitude: string;
    longitude: string;
    label?: string;
    value?: string;
  };
};

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
  widgets: Array<{
    id: string;
    title: string;
    kind: string;
    width: number;
    height: number;
    refresh_override?: string;
    definition: DashboardWidgetDefinition;
  }>;
};

type WidgetDataState = {
  data: unknown;
  loading: boolean;
  error: string | null;
  lastUpdated: Date | null;
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
    defaultPath,
  } = useShellStore();
  const [payload, setPayload] = useState<DashboardBoardResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const surface = "dashboard";

  useEffect(() => {
    setCurrentRoute("/dashboard");
  }, [setCurrentRoute]);

  async function switchSurface(nextSurface: string) {
    const bootstrap = await fetchWorkspaceBootstrap(nextSurface);
    setWorkspaceBootstrap(bootstrap);
    setRoutes(toShellRoutes(bootstrap.menus, bootstrap.actions, bootstrap.locale, "workspace"));
    navigate(defaultPath || "/", { replace: true });
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
                    className="animate-[fade-in_0.45s_ease_forwards] opacity-0"
                    style={{
                      animationDelay: `${index * 60}ms`,
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

function DashboardWidgetCard({
  widget,
  locale,
  state,
}: {
  widget: DashboardBoardResponse["widgets"][number];
  locale: string;
  state: WidgetDataState;
}) {
  const definition = widget.definition;
  const cadence = widget.refresh_override || definition.refresh_policy || "manual";

  return (
    <article
      className="group flex h-full min-h-[160px] flex-col overflow-hidden rounded-[1.2rem] border border-line bg-surface p-5 shadow-panel transition duration-200 hover:border-accent/35"
      style={{ minHeight: `${Math.max(widget.height || definition.default_height || 1, 1) * 150}px` }}
    >
      <div className="flex items-start justify-between gap-4">
        <div>
          <div className="text-[11px] font-bold uppercase tracking-[0.22em] text-muted">
            {cadence.replace(/_/g, " ")}
          </div>
          <h3 className="mt-2 text-lg font-semibold tracking-tight text-body">
            {pickText(definition, "title", locale) || widget.title}
          </h3>
        </div>
        <div className="rounded-full border border-line bg-shell px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] text-muted">
          {definition.renderer_kind}
        </div>
      </div>

      <div className="mt-5 flex-1">
        {state.loading ? <WidgetSkeleton kind={definition.renderer_kind} /> : null}
        {!state.loading && state.error ? (
          <div className="rounded-[1rem] border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{state.error}</div>
        ) : null}
        {!state.loading && !state.error ? renderDashboardWidget(definition, state.data) : null}
      </div>

      <div className="mt-4 flex items-center justify-between text-xs text-slate-500">
        <span className="text-muted">{state.lastUpdated ? `Refreshed ${state.lastUpdated.toLocaleTimeString()}` : "Awaiting data"}</span>
        <span className="h-2 w-2 rounded-full bg-emerald-500 shadow-[0_0_0_6px_rgba(16,185,129,0.08)]" />
      </div>
    </article>
  );
}

function useSharedDashboardData(widgets: DashboardBoardResponse["widgets"]) {
  const configs = useMemo(() => {
    const map = new Map<string, number>();
    for (const widget of widgets) {
      const path = widget.definition.data_path?.trim();
      if (!path) continue;
      const refreshMs = refreshIntervalForPolicy(widget.refresh_override || widget.definition.refresh_policy || "manual");
      const current = map.get(path);
      if (current == null || (refreshMs > 0 && (current <= 0 || refreshMs < current))) {
        map.set(path, refreshMs);
      }
    }
    return Array.from(map.entries()).map(([path, refreshMs]) => ({ path, refreshMs }));
  }, [widgets]);
  const [states, setStates] = useState<Record<string, WidgetDataState>>({});

  useEffect(() => {
    let active = true;
    const timers = new Map<string, number>();
    if (!configs.length) {
      setStates({});
      return () => {
        active = false;
      };
    }

    setStates((previous) => {
      const next: Record<string, WidgetDataState> = {};
      for (const { path } of configs) {
        next[path] = previous[path] || defaultWidgetDataState();
      }
      return next;
    });

    async function load(path: string, refreshMs: number) {
      setStates((previous) => ({
        ...previous,
        [path]: {
          ...(previous[path] || defaultWidgetDataState()),
          loading: true,
          error: null,
        },
      }));
      try {
        const response = await fetch(path, { credentials: "include" });
        if (!response.ok) {
          throw new Error(`Widget load failed: ${response.status}`);
        }
        const next = (await response.json()) as unknown;
        if (!active) return;
        setStates((previous) => ({
          ...previous,
          [path]: {
            data: next,
            loading: false,
            error: null,
            lastUpdated: new Date(),
          },
        }));
      } catch (nextError) {
        if (!active) return;
        setStates((previous) => ({
          ...previous,
          [path]: {
            ...(previous[path] || defaultWidgetDataState()),
            loading: false,
            error: nextError instanceof Error ? nextError.message : "Widget load failed.",
          },
        }));
      } finally {
        if (!active || refreshMs <= 0) {
          return;
        }
        timers.set(
          path,
          window.setTimeout(() => {
            void load(path, refreshMs);
          }, refreshMs),
        );
      }
    }

    for (const { path, refreshMs } of configs) {
      void load(path, refreshMs);
    }

    return () => {
      active = false;
      for (const timer of timers.values()) {
        window.clearTimeout(timer);
      }
    };
  }, [configs]);

  return states;
}

function defaultWidgetDataState(): WidgetDataState {
  return {
    data: null,
    loading: true,
    error: null,
    lastUpdated: null,
  };
}

function WidgetSkeleton({ kind }: { kind: string }) {
  if (kind === "metric") {
    return (
      <div className="animate-pulse">
        <div className="h-10 w-32 rounded bg-slate-200" />
        <div className="mt-3 h-4 w-20 rounded bg-slate-100" />
      </div>
    );
  }
  return <div className="h-32 animate-pulse rounded-[1.2rem] bg-slate-100" />;
}

function renderDashboardWidget(definition: DashboardWidgetDefinition, data: unknown) {
  switch (definition.renderer_kind) {
    case "metric":
      return renderMetricWidget(definition, data);
    case "table":
      return renderTableWidget(definition, data);
    case "gauge":
      return renderGaugeWidget(definition, data);
    case "chart_bar":
    case "chart_line":
    case "chart_area":
    case "chart_pie":
      return renderChartWidget(definition, data);
    case "map":
      return renderMapWidget(definition, data);
    default:
      return <pre className="overflow-auto text-xs text-slate-600">{JSON.stringify(data, null, 2)}</pre>;
  }
}

function renderMetricWidget(definition: DashboardWidgetDefinition, data: unknown) {
  const spec = definition.metric;
  if (!spec) return null;
  const value = valueAtPath(data, spec.value_path);
  const delta = spec.delta_path ? valueAtPath(data, spec.delta_path) : undefined;
  return (
    <div>
      <div className="text-4xl font-semibold tracking-tight text-slate-950">{formatWidgetValue(value, spec.format, spec.unit)}</div>
      {delta !== undefined ? (
        <div className="mt-3 inline-flex rounded-full border border-line/70 bg-slate-50 px-3 py-1 text-xs font-semibold text-slate-600">
          Context {formatWidgetValue(delta, spec.format, spec.unit)}
        </div>
      ) : null}
    </div>
  );
}

function renderTableWidget(definition: DashboardWidgetDefinition, data: unknown) {
  const spec = definition.table;
  if (!spec) return null;
  const rows = asRecordList(valueAtPath(data, spec.rows_path)).slice(0, 6);
  const columns = spec.columns || [];
  if (!rows.length) {
    return <div className="rounded-[1rem] border border-dashed border-line px-4 py-8 text-sm text-slate-500">No rows available.</div>;
  }
  return (
    <div className="overflow-hidden rounded-[1rem] border border-line/70">
      <table className="min-w-full text-sm">
        <thead className="bg-slate-50/90 text-slate-500">
          <tr>
            {columns.map((column) => (
              <th key={column.key} className="px-3 py-2 text-left text-[11px] font-bold uppercase tracking-[0.18em]">
                {column.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-line/70 bg-white/70">
          {rows.map((row, index) => (
            <tr key={`${index}-${String(row.id || row.code || index)}`}>
              {columns.map((column) => (
                <td key={column.key} className="px-3 py-2 text-slate-700">
                  {formatWidgetValue(valueAtPath(row, column.path))}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function renderGaugeWidget(definition: DashboardWidgetDefinition, data: unknown) {
  const spec = definition.gauge;
  if (!spec) return null;
  const rawValue = Number(valueAtPath(data, spec.value_path) || 0);
  const min = spec.min_value ?? 0;
  const max = spec.max_value ?? Math.max(rawValue, 1);
  const ratio = Math.min(Math.max((rawValue - min) / Math.max(max - min, 1), 0), 1);
  const hue =
    ratio < 0.35 ? "#16a34a" : ratio < 0.7 ? "#d97706" : "#dc2626";
  return (
    <div>
      <div className="flex items-end justify-between gap-4">
        <div className="text-4xl font-semibold tracking-tight text-slate-950">
          {formatWidgetValue(rawValue, spec.format)}
        </div>
        <div className="text-xs font-semibold uppercase tracking-[0.18em] text-slate-500">
          {Math.round(ratio * 100)}%
        </div>
      </div>
      <div className="mt-4 h-3 overflow-hidden rounded-full bg-slate-100">
        <div className="h-full rounded-full transition-all duration-500" style={{ width: `${ratio * 100}%`, backgroundColor: hue }} />
      </div>
    </div>
  );
}

function renderChartWidget(definition: DashboardWidgetDefinition, data: unknown) {
  const spec = definition.chart;
  if (!spec) return null;
  const rows = asRecordList(valueAtPath(data, spec.series_path));
  if (!rows.length) {
    return <div className="rounded-[1rem] border border-dashed border-line px-4 py-8 text-sm text-slate-500">No chart data available.</div>;
  }
  if (definition.renderer_kind === "chart_pie") {
    const total = rows.reduce((sum, row) => sum + Number(valueAtPath(row, spec.value) || 0), 0);
    let start = 0;
    return (
      <div className="flex items-center gap-5">
        <svg viewBox="0 0 42 42" className="h-28 w-28 -rotate-90">
          {rows.map((row, index) => {
            const value = Number(valueAtPath(row, spec.value) || 0);
            const portion = total > 0 ? (value / total) * 100 : 0;
            const dash = `${portion} ${100 - portion}`;
            const element = (
              <circle
                key={index}
                cx="21"
                cy="21"
                r="15.915"
                fill="transparent"
                stroke={palette(index)}
                strokeWidth="6"
                strokeDasharray={dash}
                strokeDashoffset={-start}
              />
            );
            start += portion;
            return element;
          })}
        </svg>
        <div className="space-y-2">
          {rows.slice(0, 4).map((row, index) => (
            <div key={index} className="flex items-center gap-2 text-sm text-slate-600">
              <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: palette(index) }} />
              <span>{String(valueAtPath(row, spec.category) ?? "-")}</span>
            </div>
          ))}
        </div>
      </div>
    );
  }
  const values = rows.map((row) => Number(valueAtPath(row, spec.value) || 0));
  const maxValue = Math.max(...values, 1);
  if (definition.renderer_kind === "chart_bar") {
    return (
      <div className="space-y-3">
        {rows.slice(0, 6).map((row, index) => {
          const value = Number(valueAtPath(row, spec.value) || 0);
          return (
            <div key={index}>
              <div className="mb-1 flex items-center justify-between text-xs text-slate-500">
                <span>{String(valueAtPath(row, spec.category) ?? "-")}</span>
                <span>{formatWidgetValue(value, spec.format)}</span>
              </div>
              <div className="h-2.5 rounded-full bg-slate-100">
                <div
                  className="h-full rounded-full"
                  style={{ width: `${(value / maxValue) * 100}%`, backgroundColor: palette(index) }}
                />
              </div>
            </div>
          );
        })}
      </div>
    );
  }
  const width = 320;
  const height = 140;
  const points = rows.map((row, index) => {
    const value = Number(valueAtPath(row, spec.value) || 0);
    const x = rows.length === 1 ? width / 2 : (index / Math.max(rows.length - 1, 1)) * (width - 20) + 10;
    const y = height - (value / maxValue) * (height - 20) - 10;
    return `${x},${y}`;
  });
  const polyline = points.join(" ");
  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="h-36 w-full">
      <defs>
        <linearGradient id="dashboard-area-fill" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="rgba(37,99,235,0.32)" />
          <stop offset="100%" stopColor="rgba(37,99,235,0.02)" />
        </linearGradient>
      </defs>
      {definition.renderer_kind === "chart_area" ? (
        <polygon
          points={`${points.join(" ")} ${width - 10},${height - 10} 10,${height - 10}`}
          fill="url(#dashboard-area-fill)"
        />
      ) : null}
      <polyline fill="none" stroke="#2563eb" strokeWidth="3" points={polyline} />
      {points.map((point, index) => {
        const [cx, cy] = point.split(",").map(Number);
        return <circle key={index} cx={cx} cy={cy} r="3.5" fill="#2563eb" />;
      })}
    </svg>
  );
}

function renderMapWidget(definition: DashboardWidgetDefinition, data: unknown) {
  const spec = definition.map;
  if (!spec) return null;
  const points = asRecordList(valueAtPath(data, spec.points_path));
  if (!points.length) {
    return <div className="rounded-[1rem] border border-dashed border-line px-4 py-8 text-sm text-slate-500">No map points available.</div>;
  }
  const latitudes = points.map((point) => Number(valueAtPath(point, spec.latitude) || 0));
  const longitudes = points.map((point) => Number(valueAtPath(point, spec.longitude) || 0));
  const minLat = Math.min(...latitudes);
  const maxLat = Math.max(...latitudes);
  const minLng = Math.min(...longitudes);
  const maxLng = Math.max(...longitudes);
  return (
    <svg viewBox="0 0 320 180" className="h-40 w-full rounded-[1rem] bg-[linear-gradient(180deg,#f8fbff,#ecf4ff)]">
      {points.map((point, index) => {
        const lat = Number(valueAtPath(point, spec.latitude) || 0);
        const lng = Number(valueAtPath(point, spec.longitude) || 0);
        const x = ((lng - minLng) / Math.max(maxLng - minLng, 0.0001)) * 280 + 20;
        const y = 160 - ((lat - minLat) / Math.max(maxLat - minLat, 0.0001)) * 120;
        return <circle key={index} cx={x} cy={y} r="5" fill={palette(index)} />;
      })}
    </svg>
  );
}

function refreshIntervalForPolicy(policy: string): number {
  switch (policy) {
    case "realtime":
      return 15000;
    case "minutes":
      return 5 * 60 * 1000;
    case "hourly":
      return 60 * 60 * 1000;
    case "daily":
      return 24 * 60 * 60 * 1000;
    case "weekly":
      return 7 * 24 * 60 * 60 * 1000;
    case "monthly":
      return 30 * 24 * 60 * 60 * 1000;
    default:
      return 0;
  }
}

function formatWidgetValue(value: unknown, format?: string, unit?: string): string {
  if (typeof value === "number") {
    if (format === "currency") {
      return new Intl.NumberFormat(undefined, {
        maximumFractionDigits: 0,
      }).format(value);
    }
    if (format === "percent") {
      return `${value.toFixed(1)}%`;
    }
    return new Intl.NumberFormat().format(value) + (unit ? ` ${unit}` : "");
  }
  if (typeof value === "string" && value.trim()) {
    return value;
  }
  if (typeof value === "boolean") {
    return value ? "Yes" : "No";
  }
  return "-";
}

function valueAtPath(value: unknown, path: string): unknown {
  if (!path.trim()) return value;
  return path.split(".").reduce<unknown>((current, segment) => {
    if (current == null) return undefined;
    if (Array.isArray(current)) {
      const index = Number(segment);
      return Number.isInteger(index) ? current[index] : undefined;
    }
    if (typeof current === "object") {
      return (current as Record<string, unknown>)[segment];
    }
    return undefined;
  }, value);
}

function asRecordList(value: unknown): Array<Record<string, unknown>> {
  return Array.isArray(value) ? value.filter((item): item is Record<string, unknown> => Boolean(item) && typeof item === "object") : [];
}

function palette(index: number): string {
  const colors = ["#2563eb", "#0891b2", "#7c3aed", "#d97706", "#dc2626", "#059669"];
  return colors[index % colors.length] || colors[0] || "#2563eb";
}
