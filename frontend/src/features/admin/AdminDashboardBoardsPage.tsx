import { useEffect, useMemo, useState } from "react";
import { PaginationBar } from "@/components/ui/PaginationBar";
import { formatDateTime, mutateJson, startCase } from "./adminClient";

type DashboardPayload = {
  items?: DashboardBoard[];
  widgets?: WidgetDefinition[];
  roles?: Array<{ id: string; name?: string; key?: string }>;
  locations?: Array<{ id: string; name?: string; key?: string }>;
  root?: { id: string; name?: string };
  surface?: string;
};

type DashboardBoard = {
  id?: string;
  name: string;
  description?: string;
  visibility?: string;
  surface?: string;
  is_default?: boolean;
  scope_type?: string;
  scope_id?: string;
  status?: string;
  updated_at?: string;
  widgets?: DashboardPlacement[];
};

type DashboardPlacement = {
  id?: string;
  widget_key: string;
  title?: string;
  kind?: string;
  width?: number;
  height?: number;
  order?: number;
  refresh_override?: string;
};

type WidgetDefinition = {
  key: string;
  title: string;
  renderer_kind: string;
  refresh_policy?: string;
  required_permissions?: string[];
  default_width?: number;
  default_height?: number;
};

const EMPTY_BOARD: DashboardBoard = {
  name: "Operations board",
  description: "",
  visibility: "private",
  surface: "dashboard",
  is_default: true,
  scope_type: "deployment",
  scope_id: "",
  status: "active",
  widgets: [],
};

export function AdminDashboardBoardsPage({
  payload,
  pagination,
}: {
  payload: Record<string, unknown> | null;
  pagination?: {
    page: number;
    pageSize: number;
    total: number;
    onPageChange: (page: number) => void;
    onPageSizeChange: (pageSize: number) => void;
  };
}) {
  const typed = (payload || {}) as DashboardPayload;
  const [boards, setBoards] = useState<DashboardBoard[]>(typed.items || []);
  const [selectedID, setSelectedID] = useState<string>("new");
  const [editor, setEditor] = useState<DashboardBoard>(EMPTY_BOARD);
  const [message, setMessage] = useState("");
  const [saving, setSaving] = useState(false);

  const widgetDefs = typed.widgets || [];
  const roleOptions = typed.roles || [];
  const locationOptions = typed.locations || [];
  const root = typed.root;

  useEffect(() => {
    const nextBoards = typed.items || [];
    setBoards(nextBoards);
    if (selectedID === "new") {
      return;
    }
    const active = nextBoards.find((item) => item.id === selectedID);
    if (active) {
      setEditor(cloneBoard(active));
    }
  }, [payload, selectedID, typed.items]);

  const selectedBoard = useMemo(
    () => boards.find((item) => item.id === selectedID),
    [boards, selectedID],
  );

  function resetToNew() {
    setSelectedID("new");
    setEditor(cloneBoard(EMPTY_BOARD));
    setMessage("");
  }

  function selectBoard(id: string) {
    if (id === "new") {
      resetToNew();
      return;
    }
    const next = boards.find((item) => item.id === id);
    if (!next) return;
    setSelectedID(id);
    setEditor(cloneBoard(next));
    setMessage("");
  }

  function addWidget(def: WidgetDefinition) {
    setEditor((current) => ({
      ...current,
      widgets: [
        ...(current.widgets || []),
        {
          id: `draft-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
          widget_key: def.key,
          title: def.title,
          kind: def.renderer_kind,
          width: def.default_width || 3,
          height: def.default_height || 1,
          order: (current.widgets?.length || 0) + 1,
          refresh_override: "",
        },
      ],
    }));
  }

  function updateWidget(index: number, patch: Partial<DashboardPlacement>) {
    setEditor((current) => ({
      ...current,
      widgets: (current.widgets || []).map((item, itemIndex) =>
        itemIndex === index ? { ...item, ...patch } : item,
      ),
    }));
  }

  function moveWidget(index: number, direction: -1 | 1) {
    setEditor((current) => {
      const widgets = [...(current.widgets || [])];
      const nextIndex = index + direction;
      if (nextIndex < 0 || nextIndex >= widgets.length) return current;
      const currentItem = widgets[index];
      const nextItem = widgets[nextIndex];
      if (!currentItem || !nextItem) return current;
      widgets[index] = nextItem;
      widgets[nextIndex] = currentItem;
      return {
        ...current,
        widgets: widgets.map((item, itemIndex) => ({
          ...item,
          order: itemIndex + 1,
        })),
      };
    });
  }

  function removeWidget(index: number) {
    setEditor((current) => ({
      ...current,
      widgets: (current.widgets || [])
        .filter((_, itemIndex) => itemIndex !== index)
        .map((item, itemIndex) => ({ ...item, order: itemIndex + 1 })),
    }));
  }

  async function saveBoard() {
    setSaving(true);
    setMessage("");
    try {
      const saved = await mutateJson<DashboardBoard>("/admin/api/dashboards", {
        method: "POST",
        body: JSON.stringify({
          ...editor,
          surface: "dashboard",
          widgets: (editor.widgets || []).map((item, index) => ({
            ...item,
            order: index + 1,
          })),
        }),
      });
      const nextBoards = upsertBoard(boards, saved);
      setBoards(nextBoards);
      setSelectedID(String(saved.id || "new"));
      setEditor(cloneBoard(saved));
      setMessage("Dashboard board saved.");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Failed to save dashboard board.");
    } finally {
      setSaving(false);
    }
  }

  async function deleteBoard() {
    if (!editor.id) {
      resetToNew();
      return;
    }
    setSaving(true);
    setMessage("");
    try {
      await mutateJson(`/admin/api/dashboards/${encodeURIComponent(editor.id)}`, {
        method: "DELETE",
      });
      setBoards((current) => current.filter((item) => item.id !== editor.id));
      resetToNew();
      setMessage("Dashboard board deleted.");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Failed to delete dashboard board.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-5">
      <section className="grid gap-4 lg:grid-cols-[280px_minmax(0,1fr)]">
        <div className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-xs font-semibold uppercase tracking-[0.2em] text-muted">Boards</div>
              <div className="mt-1 text-lg font-semibold text-body">{pagination?.total || boards.length} configured</div>
            </div>
            <button type="button" className="admin-button" onClick={resetToNew}>
              New
            </button>
          </div>
          <div className="mt-4 space-y-2">
            {boards.length === 0 ? (
              <div className="rounded-xl border border-dashed border-line px-4 py-5 text-sm text-muted">
                No dashboard boards saved yet.
              </div>
            ) : null}
            {boards.map((item) => (
              <button
                key={item.id}
                type="button"
                onClick={() => selectBoard(String(item.id || ""))}
                className={`w-full rounded-xl border px-4 py-3 text-left transition ${
                  selectedID === item.id
                    ? "border-accent bg-accent-soft/60"
                    : "border-line bg-surface hover:border-accent/50"
                }`}
              >
                <div className="flex items-center justify-between gap-3">
                  <div className="min-w-0">
                    <div className="truncate font-semibold text-body">{item.name}</div>
                    <div className="mt-1 text-xs uppercase tracking-[0.18em] text-muted">
                      {item.scope_type || "deployment"}
                    </div>
                  </div>
                  {item.is_default ? (
                    <span className="rounded-full border border-line px-2 py-1 text-[10px] font-bold uppercase tracking-[0.18em] text-muted">
                      Default
                    </span>
                  ) : null}
                </div>
                <div className="mt-2 text-xs text-muted">{formatDateTime(item.updated_at)}</div>
              </button>
            ))}
          </div>
          {pagination ? (
            <PaginationBar
              page={pagination.page}
              pageSize={pagination.pageSize}
              total={pagination.total}
              onPageChange={pagination.onPageChange}
              onPageSizeChange={pagination.onPageSizeChange}
              className="mt-4"
            />
          ) : null}
        </div>

        <div className="space-y-4">
          {message ? (
            <div className="rounded-xl border border-line bg-accent-soft/50 px-4 py-3 text-sm text-body">
              {message}
            </div>
          ) : null}

          <section className="rounded-xl border border-line bg-surface p-5 dark:bg-ink/60">
            <div className="grid gap-4 md:grid-cols-2">
              <label className="space-y-2 text-sm text-body">
                <span className="block text-xs font-semibold uppercase tracking-wide text-muted">Name</span>
                <input
                  className="admin-input"
                  value={editor.name || ""}
                  onChange={(event) => setEditor((current) => ({ ...current, name: event.target.value }))}
                />
              </label>
              <label className="space-y-2 text-sm text-body">
                <span className="block text-xs font-semibold uppercase tracking-wide text-muted">Status</span>
                <select
                  className="admin-input"
                  value={editor.status || "active"}
                  onChange={(event) => setEditor((current) => ({ ...current, status: event.target.value }))}
                >
                  <option value="active">active</option>
                  <option value="draft">draft</option>
                  <option value="archived">archived</option>
                </select>
              </label>
              <label className="space-y-2 text-sm text-body md:col-span-2">
                <span className="block text-xs font-semibold uppercase tracking-wide text-muted">Description</span>
                <textarea
                  className="admin-input min-h-24"
                  value={editor.description || ""}
                  onChange={(event) => setEditor((current) => ({ ...current, description: event.target.value }))}
                />
              </label>
              <label className="space-y-2 text-sm text-body">
                <span className="block text-xs font-semibold uppercase tracking-wide text-muted">Scope</span>
                <select
                  className="admin-input"
                  value={editor.scope_type || "deployment"}
                  onChange={(event) =>
                    setEditor((current) => ({
                      ...current,
                      scope_type: event.target.value,
                      scope_id: event.target.value === "deployment" ? "" : current.scope_id || "",
                    }))
                  }
                >
                  <option value="deployment">deployment</option>
                  <option value="organization">organization</option>
                  <option value="location">location</option>
                  <option value="role">role</option>
                </select>
              </label>
              <label className="space-y-2 text-sm text-body">
                <span className="block text-xs font-semibold uppercase tracking-wide text-muted">Scope target</span>
                <select
                  className="admin-input"
                  value={editor.scope_id || ""}
                  onChange={(event) => setEditor((current) => ({ ...current, scope_id: event.target.value }))}
                  disabled={(editor.scope_type || "deployment") === "deployment"}
                >
                  <option value="">
                    {(editor.scope_type || "deployment") === "deployment" ? "Deployment default" : "Select target"}
                  </option>
                  {(editor.scope_type || "deployment") === "organization" && root ? (
                    <option value={root.id}>{root.name || root.id}</option>
                  ) : null}
                  {(editor.scope_type || "deployment") === "location"
                    ? locationOptions.map((item) => (
                        <option key={item.id} value={item.id}>
                          {item.name || item.key || item.id}
                        </option>
                      ))
                    : null}
                  {(editor.scope_type || "deployment") === "role"
                    ? roleOptions.map((item) => (
                        <option key={item.id} value={item.id}>
                          {item.name || item.key || item.id}
                        </option>
                      ))
                    : null}
                </select>
              </label>
              <label className="flex items-center gap-3 rounded-2xl border border-line px-4 py-3 text-sm text-body">
                <input
                  type="checkbox"
                  checked={Boolean(editor.is_default)}
                  onChange={(event) => setEditor((current) => ({ ...current, is_default: event.target.checked }))}
                />
                Mark as default board for this scope
              </label>
              <label className="space-y-2 text-sm text-body">
                <span className="block text-xs font-semibold uppercase tracking-wide text-muted">Visibility</span>
                <select
                  className="admin-input"
                  value={editor.visibility || "private"}
                  onChange={(event) => setEditor((current) => ({ ...current, visibility: event.target.value }))}
                >
                  <option value="private">private</option>
                  <option value="shared">shared</option>
                </select>
              </label>
            </div>

            <div className="mt-5 flex flex-wrap gap-3">
              <button type="button" className="admin-button" disabled={saving} onClick={() => void saveBoard()}>
                {saving ? "Saving…" : "Save board"}
              </button>
              <button type="button" className="admin-button admin-button-secondary" disabled={saving} onClick={() => void deleteBoard()}>
                {editor.id ? "Delete board" : "Reset draft"}
              </button>
              {selectedBoard?.updated_at ? (
                <span className="inline-flex items-center rounded-full border border-line px-3 py-2 text-xs text-muted">
                  Last updated {formatDateTime(selectedBoard.updated_at)}
                </span>
              ) : null}
            </div>
          </section>

          <section className="grid gap-4 xl:grid-cols-[320px_minmax(0,1fr)]">
            <div className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
              <div className="text-xs font-semibold uppercase tracking-[0.2em] text-muted">Registered widgets</div>
              <div className="mt-4 space-y-2">
                {widgetDefs.map((def) => (
                  <button
                    key={def.key}
                    type="button"
                    onClick={() => addWidget(def)}
                    className="w-full rounded-xl border border-line bg-surface px-4 py-3 text-left transition hover:border-accent/50 dark:bg-ink/50"
                  >
                    <div className="font-semibold text-body">{def.title}</div>
                    <div className="mt-1 text-xs uppercase tracking-[0.18em] text-muted">
                      {def.renderer_kind} · {def.refresh_policy || "manual"}
                    </div>
                    <div className="mt-2 text-xs text-muted">
                      {(def.required_permissions || []).join(", ") || "No extra permissions"}
                    </div>
                  </button>
                ))}
              </div>
            </div>

            <div className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
              <div className="flex items-center justify-between">
                <div>
                  <div className="text-xs font-semibold uppercase tracking-[0.2em] text-muted">Board layout</div>
                  <div className="mt-1 text-lg font-semibold text-body">{(editor.widgets || []).length} widgets placed</div>
                </div>
              </div>

              <div className="mt-4 space-y-3">
                {(editor.widgets || []).length === 0 ? (
                  <div className="rounded-xl border border-dashed border-line px-4 py-8 text-sm text-muted">
                    Add module widgets from the left column to compose this board.
                  </div>
                ) : null}
                {(editor.widgets || []).map((item, index) => {
                  const def = widgetDefs.find((definition) => definition.key === item.widget_key);
                  return (
                    <div key={item.id || `${item.widget_key}-${index}`} className="rounded-xl border border-line bg-shell p-4 dark:bg-ink/40">
                      <div className="flex flex-wrap items-start justify-between gap-3">
                        <div>
                          <div className="font-semibold text-body">{item.title || def?.title || item.widget_key}</div>
                          <div className="mt-1 text-xs uppercase tracking-[0.18em] text-muted">
                            {def?.renderer_kind || item.kind || "widget"} · {(item.refresh_override || def?.refresh_policy || "module default").replace(/_/g, " ")}
                          </div>
                        </div>
                        <div className="flex gap-2">
                          <button type="button" className="admin-button admin-button-secondary" onClick={() => moveWidget(index, -1)} disabled={index === 0}>
                            Up
                          </button>
                          <button
                            type="button"
                            className="admin-button admin-button-secondary"
                            onClick={() => moveWidget(index, 1)}
                            disabled={index === (editor.widgets || []).length - 1}
                          >
                            Down
                          </button>
                          <button type="button" className="admin-button admin-button-secondary" onClick={() => removeWidget(index)}>
                            Remove
                          </button>
                        </div>
                      </div>

                      <div className="mt-4 grid gap-3 md:grid-cols-4">
                        <label className="space-y-2 text-sm text-body md:col-span-2">
                          <span className="block text-xs font-semibold uppercase tracking-wide text-muted">Title override</span>
                          <input
                            className="admin-input"
                            value={item.title || ""}
                            onChange={(event) => updateWidget(index, { title: event.target.value })}
                          />
                        </label>
                        <label className="space-y-2 text-sm text-body">
                          <span className="block text-xs font-semibold uppercase tracking-wide text-muted">Width</span>
                          <input
                            className="admin-input"
                            type="number"
                            min={1}
                            max={12}
                            value={item.width || 3}
                            onChange={(event) => updateWidget(index, { width: Number(event.target.value || 3) })}
                          />
                        </label>
                        <label className="space-y-2 text-sm text-body">
                          <span className="block text-xs font-semibold uppercase tracking-wide text-muted">Height</span>
                          <input
                            className="admin-input"
                            type="number"
                            min={1}
                            max={6}
                            value={item.height || 1}
                            onChange={(event) => updateWidget(index, { height: Number(event.target.value || 1) })}
                          />
                        </label>
                        <label className="space-y-2 text-sm text-body md:col-span-4">
                          <span className="block text-xs font-semibold uppercase tracking-wide text-muted">Refresh override</span>
                          <select
                            className="admin-input"
                            value={item.refresh_override || ""}
                            onChange={(event) => updateWidget(index, { refresh_override: event.target.value })}
                          >
                            <option value="">Module default</option>
                            {["realtime", "minutes", "hourly", "daily", "weekly", "monthly"].map((value) => (
                              <option key={value} value={value}>
                                {startCase(value)}
                              </option>
                            ))}
                          </select>
                        </label>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          </section>
        </div>
      </section>
    </div>
  );
}

function cloneBoard(board: DashboardBoard): DashboardBoard {
  return {
    ...EMPTY_BOARD,
    ...board,
    widgets: (board.widgets || []).map((item) => ({ ...item })),
  };
}

function upsertBoard(items: DashboardBoard[], next: DashboardBoard): DashboardBoard[] {
  const nextID = String(next.id || "");
  if (!nextID) return items;
  const existing = items.find((item) => item.id === nextID);
  if (!existing) {
    return [next, ...items];
  }
  return items.map((item) => (item.id === nextID ? next : item));
}
