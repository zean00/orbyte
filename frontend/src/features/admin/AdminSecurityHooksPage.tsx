import { useEffect, useState } from "react";
import { mutateJson } from "./adminClient";
import {
  DataGrid,
  EditableFieldSection,
  normalizeEditorPayload,
  normalizeEditorScope,
  normalizeEditorScopeID,
  resolvePath,
} from "./adminShared";

export function AdminSecurityHooksPage({
  rows,
}: {
  rows: Array<Record<string, unknown>>;
}) {
  const [items, setItems] = useState(rows);
  const [selectedKey, setSelectedKey] = useState("");
  const [draft, setDraft] = useState<Record<string, unknown>>({});
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  const selected =
    items.find(
      (row) => String(resolvePath(row, "definition.key") || "") === selectedKey,
    ) || null;
  const fields = Array.isArray(selected?.rule_fields)
    ? (selected.rule_fields as Array<Record<string, unknown>>)
    : [];

  useEffect(() => {
    setItems(rows);
  }, [rows]);

  function openEditor(row: Record<string, unknown>) {
    setSelectedKey(String(resolvePath(row, "definition.key") || ""));
    setDraft(
      ((resolvePath(row, "rule.value") as Record<string, unknown>) ||
        {}) as Record<string, unknown>,
    );
    setMessage("");
  }

  async function save() {
    if (!selected || !selectedKey) return;
    setBusy(true);
    setMessage("");
    try {
      const response = await mutateJson<Record<string, unknown>>(
        `/admin/api/security/policy-hooks/${encodeURIComponent(selectedKey)}`,
        {
          method: "PUT",
          body: JSON.stringify({
            scope: normalizeEditorScope(
              resolvePath(selected, "rule.source_scope"),
            ),
            scope_id: normalizeEditorScopeID(
              resolvePath(selected, "rule.source_scope"),
              resolvePath(selected, "rule.source_scope_id"),
            ),
            value: normalizeEditorPayload(fields, draft),
          }),
        },
      );
      setItems((current) =>
        current.map((row) =>
          String(resolvePath(row, "definition.key") || "") === selectedKey
            ? response
            : row,
        ),
      );
      setDraft(
        ((response.rule as Record<string, unknown>)?.value as Record<
          string,
          unknown
        >) || draft,
      );
      setMessage("Security policy updated.");
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : "Failed to update security policy.",
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-4">
      <DataGrid
        columns={[
          { key: "definition.key", label: "Hook" },
          { key: "definition.kind", label: "Kind" },
          { key: "definition.target", label: "Target" },
          { key: "rule.source_scope", label: "Scope" },
          { key: "engine", label: "Engine" },
          { key: "eval_valid", label: "Valid" },
        ]}
        rows={items}
        actionLabel="Edit"
        onAction={openEditor}
      />
      {selected ? (
        <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
          <div className="mb-3 text-sm font-semibold text-body">
            Edit {String(resolvePath(selected, "definition.key") || "")}
          </div>
          <EditableFieldSection
            label="Rule"
            fields={fields}
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
              Save Policy
            </button>
            {message ? <div className="text-sm text-body">{message}</div> : null}
          </div>
        </section>
      ) : null}
    </div>
  );
}
