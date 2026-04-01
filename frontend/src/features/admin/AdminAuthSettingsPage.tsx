import { useEffect, useState } from "react";
import { mutateJson } from "./adminClient";
import {
  EditableFieldSection,
  formatDate,
  normalizeEditorPayload,
} from "./adminShared";

export function AdminAuthSettingsPage({
  payload,
  renderSummaryCard,
}: {
  payload: Record<string, unknown> | null;
  renderSummaryCard: (props: { label: string; value: string }) => JSX.Element;
}) {
  const definition = (payload?.definition || {}) as Record<string, unknown>;
  const entry = (payload?.entry || {}) as Record<string, unknown>;
  const fields = Array.isArray(definition.fields)
    ? (definition.fields as Array<Record<string, unknown>>)
    : [];
  const settings =
    entry.value && typeof entry.value === "object"
      ? (entry.value as Record<string, unknown>)
      : {};
  const [draft, setDraft] = useState<Record<string, unknown>>(settings);
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    setDraft(settings);
  }, [payload]);

  async function save() {
    setBusy(true);
    setMessage("");
    try {
      const response = await mutateJson<{
        entry: { value: Record<string, unknown> };
      }>("/admin/api/auth/settings", {
        method: "PUT",
        body: JSON.stringify({
          scope: String(entry.source_scope || entry.scope || "deployment"),
          scope_id: String(entry.source_scope_id || entry.scope_id || ""),
          value: normalizeEditorPayload(fields, draft),
        }),
      });
      setDraft((response.entry?.value as Record<string, unknown>) || draft);
      setMessage("Authentication settings updated.");
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : "Failed to save authentication settings.",
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        {renderSummaryCard({
          label: "Key",
          value: String(definition.key || "identity.auth"),
        })}
        {renderSummaryCard({
          label: "Scope",
          value: String(entry.source_scope || entry.scope || "deployment"),
        })}
        {renderSummaryCard({
          label: "Resolved At",
          value: formatDate(entry.resolved_at),
        })}
      </div>
      <EditableFieldSection
        label="Authentication Settings"
        fields={fields}
        values={draft}
        onChange={setDraft}
      />
      <div className="flex items-center gap-3">
        <button
          type="button"
          className="admin-button"
          disabled={busy}
          onClick={() => void save()}
        >
          Save Settings
        </button>
        {message ? <div className="text-sm text-body">{message}</div> : null}
      </div>
    </div>
  );
}
