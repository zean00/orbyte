import { useEffect, useState } from "react";
import { fetchJson, mutateJson } from "./adminClient";
import {
  DataGrid,
  EditableFieldSection,
  displayValue,
  normalizeEditorPayload,
  normalizeEditorScope,
  normalizeEditorScopeID,
  resolvePath,
} from "./adminShared";

export function AdminConfigManagementPage({
  definitions,
}: {
  definitions: Array<Record<string, unknown>>;
}) {
  const [effective, setEffective] = useState<Array<Record<string, unknown>>>([]);
  const [selectedKey, setSelectedKey] = useState("");
  const [draft, setDraft] = useState<Record<string, unknown>>({});
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let mounted = true;
    async function load() {
      try {
        const payload = await fetchJson<{
          items: Array<Record<string, unknown>>;
        }>("/admin/api/config/effective");
        if (!mounted) return;
        setEffective(payload.items || []);
      } catch {
        if (!mounted) return;
      }
    }
    void load();
    return () => {
      mounted = false;
    };
  }, []);

  const selectedDefinition =
    definitions.find((item) => String(item.key || "") === selectedKey) || null;
  const selectedEffective =
    effective.find((item) => String(item.key || "") === selectedKey) || null;
  const selectedFields = Array.isArray(selectedDefinition?.fields)
    ? (selectedDefinition.fields as Array<Record<string, unknown>>)
    : [];

  useEffect(() => {
    if (selectedKey || definitions.length === 0) {
      return;
    }
    const first = definitions[0];
    if (!first) {
      return;
    }
    const key = String(first.key || "");
    const current = effective.find((item) => String(item.key || "") === key);
    setSelectedKey(key);
    setDraft(
      ((current?.value as Record<string, unknown>) ||
        (first.default_value as Record<string, unknown>) ||
        {}) as Record<string, unknown>,
    );
  }, [definitions, effective, selectedKey]);

  function openEditor(row: Record<string, unknown>) {
    const key = String(resolvePath(row, "key") || "");
    const current = effective.find((item) => String(item.key || "") === key);
    setSelectedKey(key);
    setDraft(
      ((current?.value as Record<string, unknown>) ||
        (row.default_value as Record<string, unknown>) ||
        {}) as Record<string, unknown>,
    );
    setMessage("");
  }

  async function save() {
    if (!selectedDefinition || !selectedKey) return;
    setBusy(true);
    setMessage("");
    try {
      const response = await mutateJson<Record<string, unknown>>(
        `/admin/api/config/entries/${encodeURIComponent(selectedKey)}/value`,
        {
          method: "PUT",
          body: JSON.stringify({
            scope: normalizeEditorScope(selectedEffective?.source_scope),
            scope_id: normalizeEditorScopeID(
              selectedEffective?.source_scope,
              selectedEffective?.source_scope_id,
            ),
            value: normalizeEditorPayload(selectedFields, draft),
          }),
        },
      );
      setEffective((current) => {
        const next = current.filter(
          (item) => String(item.key || "") !== selectedKey,
        );
        next.push(response);
        return next.sort((left, right) =>
          String(left.key || "").localeCompare(String(right.key || "")),
        );
      });
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

  const rows = definitions.map((item) => {
    const current = effective.find(
      (value) => String(value.key || "") === String(item.key || ""),
    );
    return {
      ...item,
      current_scope: current?.source_scope || "default",
      current_value: displayValue(current?.value),
    };
  });

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-line bg-accent-soft/60 p-4 text-sm text-body">
        Configuration values are editable. Select a row to load its form, then
        use <span className="font-semibold">Save Configuration</span>.
      </div>
      <DataGrid
        columns={[
          { key: "key", label: "Key" },
          { key: "module_key", label: "Module" },
          { key: "current_scope", label: "Current Scope" },
          { key: "current_value", label: "Current Value" },
        ]}
        rows={rows}
        actionLabel="Edit"
        onAction={openEditor}
      />
      {selectedDefinition ? (
        <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
          <div className="mb-3 text-sm font-semibold text-body">
            Edit {String(selectedDefinition.key || "")}
          </div>
          <EditableFieldSection
            label="Value"
            fields={selectedFields}
            values={draft}
            onChange={setDraft}
          />
          <div className="mt-4 flex items-center gap-3">
            <button
              type="button"
              className="admin-button"
              disabled={busy}
              onClick={() => void save()}
            >
              Save Configuration
            </button>
            {message ? <div className="text-sm text-body">{message}</div> : null}
          </div>
        </section>
      ) : null}
    </div>
  );
}
