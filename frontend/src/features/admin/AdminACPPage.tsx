import { useEffect, useState } from "react";
import { mutateJson } from "./adminClient";
import {
  asItems,
  DataGrid,
  normalizeEditorScope,
  normalizeEditorScopeID,
  resolvePath,
  ValueCard,
} from "./adminShared";

export function AdminACPPage({
  payload,
  renderSummaryCard,
}: {
  payload: Record<string, unknown> | null;
  renderSummaryCard: (props: { label: string; value: string }) => JSX.Element;
}) {
  const definition = (payload?.definition || {}) as Record<string, unknown>;
  const entry = (payload?.entry || {}) as Record<string, unknown>;
  const runtime = (payload?.runtime || {}) as Record<string, unknown>;
  const [enabled, setEnabled] = useState(
    Boolean(resolvePath(entry, "value.enabled")),
  );
  const [providers, setProviders] = useState<Array<Record<string, unknown>>>([]);
  const [rawJSON, setRawJSON] = useState("[]");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const enabledFieldId = "acp-enabled";

  useEffect(() => {
    const text = String(resolvePath(entry, "value.providers_json") || "[]");
    setEnabled(Boolean(resolvePath(entry, "value.enabled")));
    setRawJSON(text);
    try {
      const parsed = JSON.parse(text);
      setProviders(
        Array.isArray(parsed)
          ? parsed.map((item) => ({
              ...item,
              args_json: JSON.stringify(
                Array.isArray(item?.args) ? item.args : [],
                null,
                2,
              ),
              env_json: JSON.stringify(
                item?.env && typeof item.env === "object" ? item.env : {},
                null,
                2,
              ),
            }))
          : [],
      );
    } catch {
      setProviders([]);
    }
  }, [entry]);

  function updateProvider(index: number, key: string, value: string) {
    setProviders((current) =>
      current.map((item, currentIndex) =>
        currentIndex === index ? { ...item, [key]: value } : item,
      ),
    );
  }

  function addProvider() {
    setProviders((current) => [
      ...current,
      { key: "", name: "", command: "", description: "" },
    ]);
  }

  function removeProvider(index: number) {
    setProviders((current) =>
      current.filter((_, currentIndex) => currentIndex !== index),
    );
  }

  async function saveStructured() {
    setBusy(true);
    setMessage("");
    try {
      const normalizedProviders = providers.map((item) => ({
        key: String(item.key || ""),
        name: String(item.name || ""),
        description: String(item.description || ""),
        command: String(item.command || ""),
        cwd: String(item.cwd || ""),
        args: JSON.parse(String(item.args_json || "[]")),
        env: JSON.parse(String(item.env_json || "{}")),
      }));
      const response = await mutateJson<Record<string, unknown>>(
        "/admin/api/config/entries/platform.acp/value",
        {
          method: "PUT",
          body: JSON.stringify({
            scope: normalizeEditorScope(entry.source_scope),
            scope_id: normalizeEditorScopeID(
              entry.source_scope,
              entry.source_scope_id,
            ),
            value: {
              enabled,
              providers_json: JSON.stringify(normalizedProviders, null, 2),
            },
          }),
        },
      );
      setRawJSON(String(resolvePath(response, "value.providers_json") || "[]"));
      setMessage("ACP configuration updated.");
    } catch (error) {
      setMessage(
        error instanceof Error ? error.message : "Failed to save ACP settings.",
      );
    } finally {
      setBusy(false);
    }
  }

  async function saveRaw() {
    setBusy(true);
    setMessage("");
    try {
      const parsed = JSON.parse(rawJSON);
      if (!Array.isArray(parsed)) {
        throw new Error("Providers JSON must be an array.");
      }
      setProviders(
        parsed.map((item) => ({
          ...item,
          args_json: JSON.stringify(
            Array.isArray(item?.args) ? item.args : [],
            null,
            2,
          ),
          env_json: JSON.stringify(
            item?.env && typeof item.env === "object" ? item.env : {},
            null,
            2,
          ),
        })),
      );
      const response = await mutateJson<Record<string, unknown>>(
        "/admin/api/config/entries/platform.acp/value",
        {
          method: "PUT",
          body: JSON.stringify({
            scope: normalizeEditorScope(entry.source_scope),
            scope_id: normalizeEditorScopeID(
              entry.source_scope,
              entry.source_scope_id,
            ),
            value: {
              enabled,
              providers_json: JSON.stringify(parsed, null, 2),
            },
          }),
        },
      );
      setRawJSON(String(resolvePath(response, "value.providers_json") || "[]"));
      setMessage("ACP raw JSON updated.");
    } catch (error) {
      setMessage(
        error instanceof Error ? error.message : "Failed to save ACP JSON.",
      );
    } finally {
      setBusy(false);
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
          label: "ACP Enabled",
          value: Boolean(runtime.enabled) ? "Yes" : "No",
        })}
        {renderSummaryCard({
          label: "Configured Providers",
          value: String(providers.length),
        })}
        {renderSummaryCard({
          label: "Available Providers",
          value: String(
            asItems({
              items: runtime.providers as
                | Array<Record<string, unknown>>
                | undefined,
            }).length,
          ),
        })}
        {renderSummaryCard({
          label: "Contract Version",
          value: String(resolvePath(runtime, "contract.version") || "-"),
        })}
      </div>
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <div className="mb-4 flex items-center justify-between">
          <div className="text-sm font-semibold text-body">
            Structured Provider Editor
          </div>
          <label
            className="flex items-center gap-2 text-sm text-body"
            htmlFor={enabledFieldId}
          >
            <input
              id={enabledFieldId}
              name={enabledFieldId}
              type="checkbox"
              checked={enabled}
              onChange={(event) => setEnabled(event.target.checked)}
            />
            Enabled
          </label>
        </div>
        <div className="space-y-4">
          {providers.map((item, index) => (
            <div
              key={`provider-${index}`}
              className="rounded-xl border border-line bg-shell p-4"
            >
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                <label className="text-sm text-body">
                  <div className="mb-1 font-medium">Key</div>
                  <input
                    className="admin-input"
                    value={String(item.key || "")}
                    onChange={(event) =>
                      updateProvider(index, "key", event.target.value)
                    }
                  />
                </label>
                <label className="text-sm text-body">
                  <div className="mb-1 font-medium">Name</div>
                  <input
                    className="admin-input"
                    value={String(item.name || "")}
                    onChange={(event) =>
                      updateProvider(index, "name", event.target.value)
                    }
                  />
                </label>
                <label className="text-sm text-body">
                  <div className="mb-1 font-medium">Command</div>
                  <input
                    className="admin-input"
                    value={String(item.command || "")}
                    onChange={(event) =>
                      updateProvider(index, "command", event.target.value)
                    }
                  />
                </label>
                <label className="text-sm text-body">
                  <div className="mb-1 font-medium">Working Directory</div>
                  <input
                    className="admin-input"
                    value={String(item.cwd || "")}
                    onChange={(event) =>
                      updateProvider(index, "cwd", event.target.value)
                    }
                  />
                </label>
              </div>
              <label className="mt-3 block text-sm text-body">
                <div className="mb-1 font-medium">Args JSON</div>
                <textarea
                  className="admin-input min-h-20"
                  value={String(item.args_json || "[]")}
                  onChange={(event) =>
                    updateProvider(index, "args_json", event.target.value)
                  }
                />
              </label>
              <label className="mt-3 block text-sm text-body">
                <div className="mb-1 font-medium">Env JSON</div>
                <textarea
                  className="admin-input min-h-20"
                  value={String(item.env_json || "{}")}
                  onChange={(event) =>
                    updateProvider(index, "env_json", event.target.value)
                  }
                />
              </label>
              <label className="mt-3 block text-sm text-body">
                <div className="mb-1 font-medium">Description</div>
                <textarea
                  className="admin-input min-h-20"
                  value={String(item.description || "")}
                  onChange={(event) =>
                    updateProvider(index, "description", event.target.value)
                  }
                />
              </label>
              <div className="mt-3 flex justify-end">
                <button
                  type="button"
                  className="admin-button admin-button-secondary"
                  disabled={busy}
                  onClick={() => removeProvider(index)}
                >
                  Remove Provider
                </button>
              </div>
            </div>
          ))}
        </div>
        <div className="mt-4 flex items-center gap-3">
          <button
            type="button"
            className="admin-button admin-button-secondary"
            disabled={busy}
            onClick={addProvider}
          >
            Add Provider
          </button>
          <button
            type="button"
            className="admin-button"
            disabled={busy}
            onClick={() => void saveStructured()}
          >
            Save Structured Settings
          </button>
        </div>
      </section>
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <div className="mb-3 text-sm font-semibold text-body">Raw JSON</div>
        <textarea
          id="acp-raw-json"
          name="acp-raw-json"
          className="admin-input min-h-64 w-full"
          value={rawJSON}
          onChange={(event) => setRawJSON(event.target.value)}
        />
        <div className="mt-4 flex items-center gap-3">
          <button
            type="button"
            className="admin-button"
            disabled={busy}
            onClick={() => void saveRaw()}
          >
            Save Raw JSON
          </button>
        </div>
      </section>
      <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
        <div className="mb-3 text-sm font-semibold text-body">
          Runtime Providers
        </div>
        <DataGrid
          columns={[
            { key: "key", label: "Provider" },
            { key: "name", label: "Name" },
            { key: "available", label: "Available" },
            { key: "supports_streaming", label: "Streaming" },
            { key: "supports_approvals", label: "Approvals" },
          ]}
          rows={asItems({
            items: runtime.providers as
              | Array<Record<string, unknown>>
              | undefined,
          })}
        />
      </section>
      <ValueCard
        label={String(definition.display_name || "ACP Definition")}
        value={definition}
      />
    </div>
  );
}
