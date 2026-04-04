import {
  normalizeShellPath,
  pickText,
  type ActionDefinition,
  type DocumentFlowDefinition,
  type FieldDefinition,
  type SectionDefinition,
  type ViewDefinition,
} from "@/services/bootstrap";
import { humanize, resolvePath } from "./workspaceShared";
import type { FormState, ValidationErrors } from "./workspaceFormTypes";

export function deriveSectionsFromFields(
  fields: FieldDefinition[],
): SectionDefinition[] {
  return fields.length ? [{ key: "main", title: "Details", fields }] : [];
}

export function resolveSections(
  view: Pick<ViewDefinition, "sections" | "tabs" | "fields">,
): SectionDefinition[] {
  if (view.sections?.length) return view.sections;
  if (view.tabs?.length) {
    return view.tabs.flatMap((tab) =>
      (tab.sections || []).map((section) => ({
        ...section,
        key: `${tab.key}.${section.key}`,
        title: section.title || tab.title,
        title_i18n: section.title_i18n || tab.title_i18n,
      })),
    );
  }
  return deriveSectionsFromFields(view.fields || []);
}

export function normalizeActionPath(path: string): string {
  return normalizeShellPath(path, "workspace").replace(
    /\/details(?=\/|$)/,
    "/detail",
  );
}

export function normalizeWorkspaceRoute(path: string): string {
  const normalized = normalizeShellPath(path, "workspace");
  if (!normalized) return "";
  return normalized.startsWith("/ui/")
    ? normalized.slice(3)
    : normalized === "/ui"
      ? "/"
      : normalized;
}

export function routeForCreate(
  currentPath: string,
  actions: ActionDefinition[],
): string {
  const normalizedCurrent = normalizeActionPath(currentPath);
  const basePath = stripEditorSuffix(normalizedCurrent);
  const createAction = actions.find(
    (item) => normalizeActionPath(item.route_path) === `${basePath}/new`,
  );
  if (createAction?.route_path) return normalizeActionPath(createAction.route_path);

  const fallback = actions.find(
    (item) =>
      item.render_mode === "flow" &&
      normalizeActionPath(item.route_path).endsWith("/new"),
  );
  if (fallback?.route_path) return normalizeActionPath(fallback.route_path);
  return "";
}

export function routeForDocument(
  _documentType: string,
  kind: "detail" | "form",
  actions: ActionDefinition[],
  currentPath = "/documents",
): string {
  const normalizedCurrent = normalizeActionPath(currentPath);
  const basePath = stripEditorSuffix(normalizedCurrent);
  const suffix = kind === "detail" ? "/detail" : "/form";
  const action = actions.find(
    (item) => normalizeActionPath(item.route_path) === `${basePath}${suffix}`,
  );
  if (action?.route_path) return normalizeActionPath(action.route_path);

  const fallback = actions.find((item) => {
    const path = normalizeActionPath(item.route_path);
    return (
      item.render_mode === "generic" &&
      item.view_key &&
      path.includes("/documents") &&
      path.endsWith(suffix)
    );
  });
  if (fallback?.route_path) return normalizeActionPath(fallback.route_path);
  return kind === "detail" ? "/documents/detail" : "/documents/form";
}

export function routeForEdit(
  currentPath: string,
  documentType: string,
  actions: ActionDefinition[],
): string {
  return routeForDocument(documentType, "form", actions, currentPath);
}

export function routeForModel(
  modelKey: string,
  kind: "detail" | "form",
  actions: ActionDefinition[],
  currentPath = "",
): string {
  const normalizedCurrent = normalizeActionPath(currentPath);
  const basePath = stripEditorSuffix(normalizedCurrent);
  const suffix = `/${kind}`;

  if (basePath) {
    const exact = actions.find(
      (item) => normalizeActionPath(item.route_path) === `${basePath}${suffix}`,
    );
    if (exact?.route_path) return normalizeActionPath(exact.route_path);
  }

  const fallback = actions.find((item) => {
    const path = normalizeActionPath(item.route_path);
    return (
      item.render_mode === "generic" &&
      item.view_key &&
      path.endsWith(suffix) &&
      (item.view_key.includes(modelKey) || item.key.includes(modelKey))
    );
  });
  return fallback?.route_path ? normalizeActionPath(fallback.route_path) : "";
}

export function routeForWorkItem(
  row: Record<string, unknown>,
  actions: ActionDefinition[],
): string {
  const directPath = String(row.open_path || '');
  if (directPath) return normalizeWorkspaceRoute(directPath);
  const documentType = String(row.document_type || "");
  const targetID = String(row.target_id || "");
  const path = routeForDocument(documentType, "detail", actions, "/documents");
  return path && targetID ? `${path}?id=${encodeURIComponent(targetID)}` : "";
}

export function actionVisibleForStatus(
  actionKey: string,
  status: string,
  documentType: string,
): boolean {
  const normalizedAction = actionKey.toLowerCase();
  const normalizedStatus = status.toLowerCase();
  const normalizedType = documentType.toLowerCase();
  if (!normalizedStatus) return true;
  switch (normalizedAction) {
    case "submit":
      return normalizedStatus === "draft";
    case "approve":
    case "reject":
      return normalizedStatus === "submitted";
    case "cancel":
      if (normalizedStatus === "draft" || normalizedStatus === "submitted") {
        return true;
      }
      if (normalizedType === "invoice" && normalizedStatus === "issued") {
        return true;
      }
      if (
        normalizedType === "payment_receipt" &&
        normalizedStatus === "received"
      ) {
        return true;
      }
      if (
        normalizedType === "payment_refund" &&
        normalizedStatus === "refunded"
      ) {
        return true;
      }
      if (
        normalizedType === "goods_receipt" &&
        normalizedStatus === "received"
      ) {
        return true;
      }
      if (
        normalizedType === "vendor_bill" &&
        (normalizedStatus === "issued" || normalizedStatus === "partially_paid")
      ) {
        return true;
      }
      if (normalizedType === "payment_out" && normalizedStatus === "paid") {
        return true;
      }
      if (
        normalizedType === "vendor_credit_note" &&
        normalizedStatus === "issued"
      ) {
        return true;
      }
      if (
        normalizedType === "supplier_return" &&
        normalizedStatus === "approved"
      ) {
        return true;
      }
      return false;
    case "reopen":
      return normalizedStatus !== "draft" && normalizedStatus !== "submitted";
    case "generate_invoice":
    case "generate_fulfillment":
      return normalizedStatus === "confirmed";
    case "generate_production_order":
      return normalizedStatus === "confirmed" || normalizedStatus === "approved";
    case "register_delivery":
      return normalizedStatus === "issued";
    case "mark_delivered":
      return normalizedStatus === "dispatched";
    case "register_payment":
      return normalizedStatus === "issued" || normalizedStatus === "partially_paid";
    case "issue_credit_note":
      if (normalizedType === "sales_return") {
        return normalizedStatus === "approved" || normalizedStatus === "received";
      }
      return (
        normalizedStatus === "issued" ||
        normalizedStatus === "partially_paid" ||
        normalizedStatus === "paid"
      );
    case "register_refund":
      if (normalizedType === "sales_return") {
        return normalizedStatus === "approved" || normalizedStatus === "received";
      }
      return normalizedStatus === "issued";
    case "register_return":
      return normalizedStatus === "issued";
    case "register_return_receipt":
      return normalizedStatus === "approved";
    case "create_replacement_order":
      return (
        normalizedType === "sales_return" &&
        (normalizedStatus === "approved" || normalizedStatus === "received")
      );
    case "register_supplier_return":
      if (normalizedType === "goods_receipt") return normalizedStatus === "received";
      if (normalizedType === "vendor_bill") {
        return (
          normalizedStatus === "issued" ||
          normalizedStatus === "partially_paid" ||
          normalizedStatus === "paid"
        );
      }
      return false;
    case "generate_purchase_order":
      return normalizedStatus === "approved";
    case "register_receipt":
      return (
        normalizedStatus === "approved" ||
        normalizedStatus === "partially_received"
      );
    case "register_vendor_bill":
      if (normalizedType === "purchase_order") {
        return (
          normalizedStatus === "approved" ||
          normalizedStatus === "partially_received" ||
          normalizedStatus === "received"
        );
      }
      if (normalizedType === "goods_receipt") return normalizedStatus === "received";
      return false;
    case "register_payment_out":
      return normalizedStatus === "issued" || normalizedStatus === "partially_paid";
    case "issue_vendor_credit_note":
      if (normalizedType === "supplier_return") {
        return normalizedStatus === "approved";
      }
      return (
        normalizedStatus === "issued" ||
        normalizedStatus === "partially_paid" ||
        normalizedStatus === "paid"
      );
    case "register_production_issue":
    case "register_production_output":
      return normalizedStatus === "approved" || normalizedStatus === "in_progress";
    default:
      return true;
  }
}

function isLockedByStatus(
  documentType: string,
  status: string,
  supportedTypes: string[],
): boolean {
  const normalizedType = documentType.toLowerCase();
  const normalizedStatus = status.toLowerCase();
  if (!supportedTypes.includes(normalizedType)) return false;
  if (!normalizedStatus) return false;
  return normalizedStatus !== "draft" && normalizedStatus !== "rejected";
}

export function isCommercialDocumentLocked(
  documentType: string,
  status: string,
): boolean {
  return isLockedByStatus(documentType, status, [
    "sales_order",
    "invoice",
    "credit_note",
    "payment_receipt",
    "payment_refund",
    "ledger_posting",
  ]);
}

export function isProcurementDocumentLocked(
  documentType: string,
  status: string,
): boolean {
  return isLockedByStatus(documentType, status, [
    "purchase_request",
    "purchase_order",
    "goods_receipt",
    "vendor_bill",
    "payment_out",
    "vendor_credit_note",
  ]);
}

export function isFulfillmentDocumentLocked(
  documentType: string,
  status: string,
): boolean {
  return isLockedByStatus(documentType, status, [
    "sales_fulfillment",
    "delivery_order",
  ]);
}

export function isReturnsDocumentLocked(
  documentType: string,
  status: string,
): boolean {
  return isLockedByStatus(documentType, status, [
    "sales_return",
    "return_receipt",
  ]);
}

export function isSupplierReturnsDocumentLocked(
  documentType: string,
  status: string,
): boolean {
  return isLockedByStatus(documentType, status, ["supplier_return"]);
}

export function isProductionDocumentLocked(
  documentType: string,
  status: string,
): boolean {
  return isLockedByStatus(documentType, status, [
    "production_order",
    "production_issue",
    "production_output",
  ]);
}

export function isRecallDocumentLocked(
  documentType: string,
  status: string,
): boolean {
  return isLockedByStatus(documentType, status, ["recall_case", "recall_action"]);
}

export function readCookie(name: string): string {
  const cookie = document.cookie
    .split("; ")
    .find((part) => part.startsWith(`${name}=`));
  return cookie ? decodeURIComponent(cookie.split("=").slice(1).join("=")) : "";
}

export function collectFlowFields(doc: {
  fields?: FieldDefinition[];
  sections?: SectionDefinition[];
  tabs?: Array<{ sections?: SectionDefinition[] }>;
}): FieldDefinition[] {
  const fields: FieldDefinition[] = [...(doc.fields || [])];
  for (const section of doc.sections || []) {
    fields.push(...(section.fields || []));
  }
  for (const tab of doc.tabs || []) {
    for (const section of tab.sections || []) {
      fields.push(...(section.fields || []));
    }
  }
  return fields;
}

export function validateFieldCollection(
  fields: FieldDefinition[],
  values: FormState,
  model: boolean,
  locale: string,
  scope = "",
): ValidationErrors {
  const errors: ValidationErrors = {};
  for (const field of fields) {
    if (field.read_only) continue;
    const message = validateFieldInput(
      field,
      resolvePath(values, normalizeFieldPath(field, model)),
      locale,
    );
    if (message) {
      errors[validationFieldKey(scope, field.key)] = message;
    }
  }
  return errors;
}

export function validateFieldInput(
  field: FieldDefinition,
  value: unknown,
  locale: string,
): string {
  const label = pickText(field, "label", locale) || humanize(field.key);
  const asString =
    typeof value === "string" ? value : value == null ? "" : String(value);
  const trimmed = asString.trim();
  const isEmpty = field.type === "bool" ? value == null : trimmed === "";

  if (field.required && isEmpty) {
    return `${label} is required.`;
  }
  if (isEmpty) {
    return "";
  }
  if (field.options?.length && !field.options.includes(asString)) {
    return `${label} must be one of: ${field.options.join(", ")}.`;
  }
  if (field.min_length && trimmed.length < field.min_length) {
    return `${label} must be at least ${field.min_length} characters.`;
  }
  if (field.max_length && trimmed.length > field.max_length) {
    return `${label} must be at most ${field.max_length} characters.`;
  }
  if (field.pattern) {
    try {
      const expression = new RegExp(field.pattern);
      if (!expression.test(asString)) {
        return `${label} has an invalid format.`;
      }
    } catch {
      return `${label} has an invalid format.`;
    }
  }
  if (
    field.type === "int" ||
    field.type === "number" ||
    field.min_value != null ||
    field.max_value != null
  ) {
    const numericValue = typeof value === "number" ? value : Number(asString);
    if (Number.isNaN(numericValue)) {
      return `${label} must be a number.`;
    }
    if (field.min_value != null && numericValue < field.min_value) {
      return `${label} must be at least ${field.min_value}.`;
    }
    if (field.max_value != null && numericValue > field.max_value) {
      return `${label} must be at most ${field.max_value}.`;
    }
  }
  return "";
}

export function validationFieldKey(scope: string, fieldKey: string): string {
  return scope ? `${scope}:${fieldKey}` : fieldKey;
}

export function normalizeFieldPath(
  field: FieldDefinition,
  model: boolean,
): string {
  return model
    ? field.path.replace(/^values\./, "")
    : field.path.replace(/^body\.payload\./, "");
}

export function stripEditorSuffix(path: string): string {
  return path.replace(/\/(details|detail|form|new)$/, "");
}

export function normalizeLegacyWorkspacePath(path: string): string {
  return path.replace(/\/details(?=\/|$)/g, "/detail");
}

export function resolveFlowSequence(
  flow: DocumentFlowDefinition,
  draft: Record<string, { payload: FormState }>,
) {
  const steps = flow.steps || [];
  const map = new Map(steps.map((step) => [step.key, step]));
  const sequence: typeof steps = [];
  const seen = new Set<string>();
  let current = steps[0];
  while (current && !seen.has(current.key)) {
    seen.add(current.key);
    sequence.push(current);
    let nextKey = current.next_step_key || "";
    for (const rule of current.next_rules || []) {
      const value = resolveFlowRuleValue(draft, rule.path);
      if (rule.truthy && value) {
        nextKey = rule.next_step_key;
        break;
      }
      if (rule.equals !== undefined && String(value ?? "") === String(rule.equals)) {
        nextKey = rule.next_step_key;
        break;
      }
      if (rule.in?.length && rule.in.includes(String(value ?? ""))) {
        nextKey = rule.next_step_key;
        break;
      }
    }
    current = nextKey ? map.get(nextKey) : undefined;
  }
  return sequence;
}

export function resolveFlowRuleValue(
  draft: Record<string, { payload: FormState }>,
  path: string,
): unknown {
  const trimmed = path.trim();
  const documentMatch = trimmed.match(/^documents\.([^.]+)\.payload\.(.+)$/);
  if (documentMatch) {
    const docKey = documentMatch[1] || "";
    const payloadPath = documentMatch[2] || "";
    return resolvePath(draft[docKey]?.payload, payloadPath);
  }
  const rawPath = trimmed
    .replace(/^body\.payload\./, "")
    .replace(/^payload\./, "");
  const docMatch = Object.values(draft).find(
    (item) => resolvePath(item.payload, rawPath) !== undefined,
  );
  return docMatch ? resolvePath(docMatch.payload, rawPath) : undefined;
}
