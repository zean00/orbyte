import { useEffect, useState } from "react";
import { fetchAllPagedItems, fetchJson, mutateJson } from "./adminClient";
import {
  EditableFieldSection,
  normalizeEditorPayload,
  normalizeEditorScope,
  normalizeEditorScopeID,
} from "./adminShared";

export function AdminFinanceSettingsPage({
  renderSummaryCard,
}: {
  renderSummaryCard: (props: { label: string; value: string }) => JSX.Element;
}) {
  const [definition, setDefinition] = useState<Record<string, unknown> | null>(
    null,
  );
  const [effective, setEffective] = useState<Record<string, unknown> | null>(
    null,
  );
  const [draft, setDraft] = useState<Record<string, unknown>>({});
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let mounted = true;
    async function load() {
      const [definitions, effectivePayload] = await Promise.all([
        fetchAllPagedItems<Record<string, unknown>>(
          "/admin/api/config/definitions",
        ),
        fetchJson<{ items: Array<Record<string, unknown>> }>(
          "/admin/api/config/effective",
        ),
      ]);
      if (!mounted) return;
      const financeDefinition =
        definitions.find(
          (item) => String(item.key || "") === "commercial.posting",
        ) || null;
      const financeEffective =
        (effectivePayload.items || []).find(
          (item) => String(item.key || "") === "commercial.posting",
        ) || null;
      setDefinition(financeDefinition);
      setEffective(financeEffective);
      setDraft(
        ((financeEffective?.value as Record<string, unknown>) ||
          (financeDefinition?.default_value as Record<string, unknown>) ||
          {}) as Record<string, unknown>,
      );
    }
    void load();
    return () => {
      mounted = false;
    };
  }, []);

  const fields = Array.isArray(definition?.fields)
    ? (definition.fields as Array<Record<string, unknown>>)
    : [];

  async function save() {
    if (!definition) return;
    setBusy(true);
    setMessage("");
    try {
      const response = await mutateJson<Record<string, unknown>>(
        "/admin/api/config/entries/commercial.posting/value",
        {
          method: "PUT",
          body: JSON.stringify({
            scope: normalizeEditorScope(effective?.source_scope),
            scope_id: normalizeEditorScopeID(
              effective?.source_scope,
              effective?.source_scope_id,
            ),
            value: normalizeEditorPayload(fields, draft),
          }),
        },
      );
      setEffective(response);
      setDraft((response.value as Record<string, unknown>) || draft);
      setMessage("Finance posting defaults updated.");
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : "Failed to update finance settings.",
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        {renderSummaryCard({ label: "Config Key", value: "commercial.posting" })}
        {renderSummaryCard({
          label: "Scope",
          value: String(
            effective?.source_scope || effective?.scope || "deployment",
          ),
        })}
        {renderSummaryCard({ label: "Purpose", value: "Posting Defaults" })}
      </div>
      <div className="rounded-xl border border-line bg-accent-soft/60 p-4 text-sm text-body">
        Set the default receivable, revenue, tax, and clearing accounts used by
        commercial invoice and payment postings when the document or catalog
        does not override them.
      </div>
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <EditableFieldSection
          label="Posting Defaults"
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
            Save Finance Settings
          </button>
          {message ? <div className="text-sm text-body">{message}</div> : null}
        </div>
      </section>
    </div>
  );
}
