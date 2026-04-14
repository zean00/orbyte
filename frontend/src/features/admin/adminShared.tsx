import { useMemo, useState } from "react";
import { PaginationBar } from "@/components/ui/PaginationBar";

export function DataGrid({
  columns,
  rows,
  actionLabel,
  onAction,
  actionLabelForRow,
  actionDisabledForRow,
  secondaryActionLabel,
  secondaryActionLabelForRow,
  onSecondaryAction,
  secondaryActionDisabledForRow,
  pagination,
  localPageSize,
}: {
  columns: Array<{ key: string; label: string }>;
  rows: Array<Record<string, unknown>>;
  actionLabel?: string;
  onAction?: (row: Record<string, unknown>) => void;
  actionLabelForRow?: (row: Record<string, unknown>) => string;
  actionDisabledForRow?: (row: Record<string, unknown>) => boolean;
  secondaryActionLabel?: string;
  secondaryActionLabelForRow?: (row: Record<string, unknown>) => string;
  onSecondaryAction?: (row: Record<string, unknown>) => void;
  secondaryActionDisabledForRow?: (row: Record<string, unknown>) => boolean;
  pagination?: {
    page: number;
    pageSize: number;
    total: number;
    onPageChange: (page: number) => void;
    onPageSizeChange?: (pageSize: number) => void;
  };
  localPageSize?: number;
}) {
  const [localPage, setLocalPage] = useState(1);
  const pageSize = localPageSize && localPageSize > 0 ? localPageSize : 0;
  const visibleRows = useMemo(() => {
    if (!pageSize) return rows;
    const totalPages = Math.max(1, Math.ceil(rows.length / pageSize));
    const currentPage = Math.min(localPage, totalPages);
    const start = (currentPage - 1) * pageSize;
    return rows.slice(start, start + pageSize);
  }, [localPage, pageSize, rows]);

  const rowKeyColumn = columns[0]?.key;

  return (
    <>
      <div className="overflow-hidden rounded-xl border border-line">
        <table className="min-w-full divide-y divide-line text-sm">
          <thead className="border-b border-line bg-accent-soft dark:bg-ink/60">
            <tr>
              {columns.map((column) => (
                <th
                  key={column.key}
                  className="px-4 py-3 text-left text-xs font-bold uppercase tracking-[0.14em] text-accent-dark dark:text-body"
                >
                  {column.label}
                </th>
              ))}
              {actionLabel || secondaryActionLabel ? (
                <th className="px-4 py-3" />
              ) : null}
            </tr>
          </thead>
          <tbody className="divide-y divide-line bg-surface">
            {visibleRows.length ? visibleRows.map((row, index) => (
              <tr
                key={`${index}-${String((rowKeyColumn && resolvePath(row, rowKeyColumn)) || index)}`}
              >
                {columns.map((column) => (
                  <td key={column.key} className="px-4 py-3 align-top text-body">
                    {displayValue(resolvePath(row, column.key))}
                  </td>
                ))}
                {actionLabel || secondaryActionLabel ? (
                  <td className="px-4 py-3 text-right">
                    <div className="flex justify-end gap-2">
                      {actionLabel ? (
                        <button
                          type="button"
                          className="admin-button admin-button-secondary"
                          disabled={actionDisabledForRow?.(row)}
                          onClick={() => onAction?.(row)}
                        >
                          {actionLabelForRow
                            ? actionLabelForRow(row)
                            : actionLabel}
                        </button>
                      ) : null}
                      {secondaryActionLabel ? (
                        <button
                          type="button"
                          className="admin-button admin-button-secondary"
                          disabled={secondaryActionDisabledForRow?.(row)}
                          onClick={() => onSecondaryAction?.(row)}
                        >
                          {secondaryActionLabelForRow
                            ? secondaryActionLabelForRow(row)
                            : secondaryActionLabel}
                        </button>
                      ) : null}
                    </div>
                  </td>
                ) : null}
              </tr>
            )) : (
              <tr>
                <td
                  colSpan={columns.length + (actionLabel || secondaryActionLabel ? 1 : 0)}
                  className="px-4 py-10 text-center text-sm text-muted"
                >
                  No data available.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      {(pagination || (pageSize && rows.length > 0)) ? (
        <PaginationBar
          page={pagination?.page || localPage}
          pageSize={pagination?.pageSize || pageSize}
          total={pagination?.total || rows.length}
          onPageChange={pagination?.onPageChange || setLocalPage}
          onPageSizeChange={pagination?.onPageSizeChange}
        />
      ) : null}
    </>
  );
}

export function EditableFieldSection({
  label,
  fields,
  values,
  onChange,
  disabled,
}: {
  label: string;
  fields: Array<Record<string, unknown>>;
  values: Record<string, unknown>;
  onChange: (next: Record<string, unknown>) => void;
  disabled?: boolean;
}) {
  const visibleFields = fields.filter((field) => typeof field.key === "string");
  return (
    <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
      <div className="mb-4 text-sm font-semibold text-body">{label}</div>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        {visibleFields.map((field) => {
          const key = String(field.key);
          const type = String(field.type || "string");
          const labelText = String(field.label || startCase(key));
          const value = values[key];
          const enumValues = Array.isArray(field.enum)
            ? (field.enum as string[])
            : [];
          const fieldId = `${label.toLowerCase().replace(/[^a-z0-9]+/g, "-")}-${key}`;
          if (type === "bool") {
            return (
              <label
                key={key}
                className="flex items-center gap-3 rounded-xl border border-line p-3 text-sm text-body"
                htmlFor={fieldId}
              >
                <input
                  id={fieldId}
                  name={fieldId}
                  type="checkbox"
                  disabled={disabled}
                  checked={Boolean(value)}
                  onChange={(event) =>
                    onChange({ ...values, [key]: event.target.checked })
                  }
                />
                <span>{labelText}</span>
              </label>
            );
          }
          if (enumValues.length > 0) {
            return (
              <label
                key={key}
                className="space-y-2 text-sm text-body"
                htmlFor={fieldId}
              >
                <span className="block text-xs font-semibold uppercase tracking-wide text-muted">
                  {labelText}
                </span>
                <select
                  id={fieldId}
                  name={fieldId}
                  className="admin-input"
                  disabled={disabled}
                  value={String(value ?? "")}
                  onChange={(event) =>
                    onChange({ ...values, [key]: event.target.value })
                  }
                >
                  <option value="">Select {labelText}</option>
                  {enumValues.map((item) => (
                    <option key={item} value={item}>
                      {item}
                    </option>
                  ))}
                </select>
              </label>
            );
          }
          if (type === "string_list") {
            return (
              <label
                key={key}
                className="space-y-2 text-sm text-body md:col-span-2"
                htmlFor={fieldId}
              >
                <span className="block text-xs font-semibold uppercase tracking-wide text-muted">
                  {labelText}
                </span>
                <textarea
                  id={fieldId}
                  name={fieldId}
                  className="admin-input min-h-24"
                  disabled={disabled}
                  value={
                    Array.isArray(value) ? value.map(String).join("\n") : ""
                  }
                  onChange={(event) =>
                    onChange({
                      ...values,
                      [key]: event.target.value
                        .split("\n")
                        .map((item) => item.trim())
                        .filter(Boolean),
                    })
                  }
                />
              </label>
            );
          }
          return (
            <label
              key={key}
              className="space-y-2 text-sm text-body"
              htmlFor={fieldId}
            >
              <span className="block text-xs font-semibold uppercase tracking-wide text-muted">
                {labelText}
              </span>
              <input
                id={fieldId}
                name={fieldId}
                className="admin-input"
                disabled={disabled}
                type={type === "int" ? "number" : "text"}
                value={
                  type === "int" ? String(value ?? 0) : String(value ?? "")
                }
                onChange={(event) =>
                  onChange({
                    ...values,
                    [key]:
                      type === "int" ? event.target.value : event.target.value,
                  })
                }
              />
            </label>
          );
        })}
      </div>
    </section>
  );
}

export function ValueCard({ label, value }: { label: string; value: unknown }) {
  return (
    <section className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
      <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-body">
        {label}
      </div>
      <pre className="overflow-auto text-xs text-body">
        {JSON.stringify(value ?? {}, null, 2)}
      </pre>
    </section>
  );
}

export function asItems(
  payload: Record<string, unknown> | null,
): Array<Record<string, unknown>> {
  const items = payload?.items;
  return Array.isArray(items) ? (items as Array<Record<string, unknown>>) : [];
}

export function resolvePath(
  payload: Record<string, unknown>,
  path: string,
): unknown {
  return path.split(".").reduce<unknown>((current, key) => {
    if (
      current &&
      typeof current === "object" &&
      key in (current as Record<string, unknown>)
    ) {
      return (current as Record<string, unknown>)[key];
    }
    return undefined;
  }, payload);
}

export function displayValue(value: unknown): string {
  if (value == null) return "";
  if (typeof value === "boolean") return value ? "Yes" : "No";
  if (Array.isArray(value)) {
    return value
      .map((item) => displayValue(item))
      .filter(Boolean)
      .join(", ");
  }
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

export function normalizeEditorPayload(
  fields: Array<Record<string, unknown>>,
  values: Record<string, unknown>,
): Record<string, unknown> {
  const fieldTypes = new Map(
    fields.map((field) => [
      String(field.key || ""),
      String(field.type || "string"),
    ]),
  );
  const payload: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(values)) {
    const type = fieldTypes.get(key) || "string";
    if (type === "int") {
      payload[key] =
        typeof value === "number"
          ? value
          : Number.parseInt(String(value || "0"), 10) || 0;
      continue;
    }
    if (type === "bool") {
      payload[key] = Boolean(value);
      continue;
    }
    if (type === "string_list") {
      payload[key] = Array.isArray(value)
        ? value.map((item) => String(item))
        : [];
      continue;
    }
    payload[key] = value;
  }
  return payload;
}

export function normalizeEditorScope(scope: unknown): string {
  const value = String(scope || "").trim();
  if (value === "" || value === "default") {
    return "deployment";
  }
  return value;
}

export function normalizeEditorScopeID(
  scope: unknown,
  scopeID: unknown,
): string {
  return normalizeEditorScope(scope) === "deployment"
    ? ""
    : String(scopeID || "").trim();
}

export function formatDate(value: unknown): string {
  if (typeof value !== "string" || !value) return "Unknown";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

export function startCase(value: string): string {
  return value
    .replace(/[_-]+/g, " ")
    .replace(/\b\w/g, (character) => character.toUpperCase());
}
