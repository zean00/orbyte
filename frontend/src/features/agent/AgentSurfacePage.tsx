import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { fetchWorkspaceBootstrap, toShellRoutes } from "@/services/bootstrap";
import { useShellStore } from "@/stores/shellStore";

type ProviderInfo = {
  key: string;
  name: string;
  description?: string;
  available?: boolean;
  supports_streaming?: boolean;
  supports_approvals?: boolean;
  error?: string;
};

type ACPMessage = {
  id: string;
  role: string;
  content: string;
  created_at?: string;
};

type ACPApproval = {
  id: string;
  status: string;
  title: string;
  description?: string;
};

type ACPEvent = {
  id: string;
  kind: string;
  created_at?: string;
  payload?: Record<string, unknown>;
};

type ACPSession = {
  id: string;
  provider_key: string;
  provider_name: string;
  title?: string;
  status: string;
  route_path?: string;
  updated_at?: string;
  turn_in_progress?: boolean;
  messages?: ACPMessage[];
  approvals?: ACPApproval[];
  trace?: ACPEvent[];
};

type MCPTool = {
  name: string;
  title?: string;
  description?: string;
  moduleKey?: string;
  sourceType?: string;
  contract?: {
    actionClass?: string;
    riskClass?: string;
    draftOnly?: boolean;
    requiresConfirmation?: boolean;
    requiresApproval?: boolean;
    governanceTags?: string[];
    businessDomains?: string[];
  };
};

export default function AgentSurfacePage() {
  const navigate = useNavigate();
  const {
    locale,
    setCurrentRoute,
    setWorkspaceBootstrap,
    setRoutes,
    defaultPath,
  } = useShellStore();
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [sessions, setSessions] = useState<ACPSession[]>([]);
  const [selectedSessionID, setSelectedSessionID] = useState("");
  const [session, setSession] = useState<ACPSession | null>(null);
  const [providerKey, setProviderKey] = useState("");
  const [prompt, setPrompt] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [mcpTools, setMcpTools] = useState<MCPTool[]>([]);
  const streamRef = useRef<EventSource | null>(null);

  useEffect(() => {
    setCurrentRoute("/agent/workspace");
  }, [setCurrentRoute]);

  useEffect(() => {
    let mounted = true;
    async function loadSurface() {
      const bootstrap = await fetchWorkspaceBootstrap("agent");
      if (!mounted) return;
      setWorkspaceBootstrap(bootstrap);
      setRoutes(
        toShellRoutes(
          bootstrap.menus,
          bootstrap.actions,
          bootstrap.locale,
          "workspace",
        ),
      );
    }
    void loadSurface();
    return () => {
      mounted = false;
    };
  }, [setRoutes, setWorkspaceBootstrap]);

  async function switchSurface(nextSurface: string) {
    const bootstrap = await fetchWorkspaceBootstrap(nextSurface);
    setWorkspaceBootstrap(bootstrap);
    setRoutes(
      toShellRoutes(
        bootstrap.menus,
        bootstrap.actions,
        bootstrap.locale,
        "workspace",
      ),
    );
    navigate(useShellStore.getState().defaultPath || defaultPath || "/", {
      replace: true,
    });
  }

  useEffect(() => {
    let mounted = true;
    async function load() {
      try {
        const [providerPayload, sessionPayload] = await Promise.all([
          fetchJson<{ enabled?: boolean; items?: ProviderInfo[] }>(
            "/agent/api/providers",
          ),
          fetchJson<{ items?: ACPSession[] }>("/agent/api/sessions"),
        ]);
        if (!mounted) return;
        const nextProviders = providerPayload.items || [];
        const nextSessions = sessionPayload.items || [];
        setProviders(nextProviders);
        setSessions(nextSessions);
        const firstProvider = nextProviders[0];
        if (!providerKey && firstProvider) {
          setProviderKey(firstProvider.key);
        }
        const firstSession = nextSessions[0];
        if (!selectedSessionID && firstSession) {
          setSelectedSessionID(firstSession.id);
        }
      } catch (error) {
        if (!mounted) return;
        setMessage(
          error instanceof Error
            ? error.message
            : "Failed to load ACP runtime.",
        );
      }
    }
    void load();
    return () => {
      mounted = false;
    };
  }, [providerKey, selectedSessionID]);

  useEffect(() => {
    let mounted = true;
    async function loadMcpTools() {
      try {
        const payload = await callMcp<{ tools?: MCPTool[] }>("tools/list");
        if (!mounted) return;
        setMcpTools(sortMcpTools(payload.tools || []));
      } catch {
        if (!mounted) return;
        setMcpTools([]);
      }
    }
    void loadMcpTools();
    return () => {
      mounted = false;
    };
  }, []);

  useEffect(() => {
    if (!selectedSessionID) {
      setSession(null);
      if (streamRef.current) {
        streamRef.current.close();
        streamRef.current = null;
      }
      return;
    }
    void refreshSession(selectedSessionID, setSession, setSessions);
    if (streamRef.current) {
      streamRef.current.close();
      streamRef.current = null;
    }
    const stream = new EventSource(
      `/agent/api/sessions/${encodeURIComponent(selectedSessionID)}/events`,
      { withCredentials: true },
    );
    stream.onmessage = () => {
      void refreshSession(selectedSessionID, setSession, setSessions);
    };
    stream.onerror = () => {
      stream.close();
    };
    streamRef.current = stream;
    return () => {
      stream.close();
      if (streamRef.current === stream) {
        streamRef.current = null;
      }
    };
  }, [selectedSessionID]);

  const selectedProvider = useMemo(
    () =>
      providers.find((item) => item.key === providerKey) ||
      providers[0] ||
      null,
    [providerKey, providers],
  );
  const investigationTools = useMemo(
    () =>
      mcpTools.filter(
        (item) =>
          item.contract?.actionClass === "analyze" ||
          item.contract?.actionClass === "read",
      ),
    [mcpTools],
  );
  const governedTools = useMemo(
    () =>
      mcpTools.filter((item) =>
        ["draft", "submit", "controlled_mutation"].includes(
          String(item.contract?.actionClass || ""),
        ),
      ),
    [mcpTools],
  );

  async function createSession() {
    if (!selectedProvider) return;
    setBusy(true);
    setMessage("");
    try {
      const created = await mutateJson<ACPSession>("/agent/api/sessions", {
        method: "POST",
        body: JSON.stringify({
          provider_key: selectedProvider.key,
          shell: "agent_surface",
          route_path: "/agent/workspace",
          title: selectedProvider.name,
        }),
      });
      setSessions((current) => [
        created,
        ...current.filter((item) => item.id !== created.id),
      ]);
      setSelectedSessionID(created.id);
    } catch (error) {
      setMessage(
        error instanceof Error ? error.message : "Failed to start session.",
      );
    } finally {
      setBusy(false);
    }
  }

  async function sendPrompt() {
    if (!selectedSessionID || !prompt.trim()) return;
    setBusy(true);
    setMessage("");
    try {
      const updated = await mutateJson<ACPSession>(
        `/agent/api/sessions/${encodeURIComponent(selectedSessionID)}/prompt`,
        {
          method: "POST",
          body: JSON.stringify({ content: prompt.trim() }),
        },
      );
      setPrompt("");
      setSession(updated);
      setSessions((current) =>
        current.map((item) => (item.id === updated.id ? updated : item)),
      );
    } catch (error) {
      setMessage(
        error instanceof Error ? error.message : "Failed to send prompt.",
      );
    } finally {
      setBusy(false);
    }
  }

  async function resolveApproval(
    approvalID: string,
    action: "approve" | "reject",
  ) {
    if (!selectedSessionID) return;
    setBusy(true);
    setMessage("");
    try {
      await mutateJson(
        `/agent/api/sessions/${encodeURIComponent(selectedSessionID)}/approvals/${encodeURIComponent(approvalID)}/${action}`,
        {
          method: "POST",
        },
      );
      await refreshSession(selectedSessionID, setSession, setSessions);
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : `Failed to ${action} approval.`,
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_top,_rgba(20,126,113,0.18),_transparent_35%),linear-gradient(180deg,_var(--color-shell),_color-mix(in_srgb,var(--color-shell)_84%,#081514_16%))] text-body">
      <header className="sticky top-0 z-20 border-b border-line/80 bg-surface/90 backdrop-blur">
        <div className="mx-auto flex max-w-[1800px] items-center justify-between gap-4 px-6 py-4">
          <div>
            <p className="text-xs font-bold uppercase tracking-[0.22em] text-accent-dark">
              Agent Surface
            </p>
            <h1 className="text-2xl font-black tracking-tight text-body">
              Orbyte Operator Agent
            </h1>
          </div>
          <div className="flex items-center gap-3">
            <span className="rounded-full border border-line bg-shell px-3 py-1 text-xs font-bold uppercase tracking-[0.16em] text-muted">
              {locale.toUpperCase()}
            </span>
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
      <main className="mx-auto grid max-w-[1800px] gap-4 px-4 py-4 md:grid-cols-[300px_minmax(0,1fr)] md:px-6">
        <section className="space-y-4 rounded-[1.5rem] border border-line bg-surface p-5 shadow-panel">
          <div>
            <div className="text-sm font-semibold text-body">Provider</div>
            <select
              className="mt-2 w-full rounded-xl border border-line bg-shell px-3 py-2 text-sm text-body"
              value={selectedProvider?.key || ""}
              onChange={(event) => setProviderKey(event.target.value)}
              name="agent_provider"
            >
              {providers.map((item) => (
                <option key={item.key} value={item.key}>
                  {item.name}
                </option>
              ))}
            </select>
            {selectedProvider?.description ? (
              <p className="mt-2 text-xs text-muted">
                {selectedProvider.description}
              </p>
            ) : null}
            <button
              type="button"
              className="mt-3 w-full rounded-xl bg-accent px-4 py-2 text-sm font-semibold text-white"
              disabled={busy || !selectedProvider}
              onClick={() => void createSession()}
            >
              Start Session
            </button>
          </div>
          <div>
            <div className="text-sm font-semibold text-body">Sessions</div>
            <div className="mt-2 space-y-2">
              {sessions.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => setSelectedSessionID(item.id)}
                  className={`w-full rounded-xl border px-3 py-3 text-left transition ${
                    selectedSessionID === item.id
                      ? "border-accent bg-accent-soft/50"
                      : "border-line bg-shell hover:border-accent/40"
                  }`}
                >
                  <div className="text-sm font-semibold text-body">
                    {item.title || item.provider_name}
                  </div>
                  <div className="mt-1 text-xs uppercase tracking-[0.16em] text-muted">
                    {item.status}
                  </div>
                </button>
              ))}
              {sessions.length === 0 ? (
                <p className="text-sm text-muted">No ACP sessions yet.</p>
              ) : null}
            </div>
          </div>
        </section>

        <section className="space-y-4">
          {message ? (
            <div className="rounded-xl border border-line bg-accent-soft/60 p-4 text-sm text-body">
              {message}
            </div>
          ) : null}
          <section className="rounded-[1.5rem] border border-line bg-surface p-5 shadow-panel">
            <div className="flex items-center justify-between gap-4">
              <div>
                <h2 className="text-xl font-black tracking-tight text-body">
                  {session?.title || "Session Detail"}
                </h2>
                <p className="mt-1 text-sm text-muted">
                  {session
                    ? `${session.provider_name} · ${session.status}`
                    : "Select or start an ACP session."}
                </p>
              </div>
              {session?.turn_in_progress ? (
                <span className="rounded-full border border-line bg-shell px-3 py-1 text-xs font-bold uppercase tracking-[0.16em] text-muted">
                  Thinking
                </span>
              ) : null}
            </div>
            <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,2fr)_320px]">
              <div className="space-y-3">
                <div className="max-h-[34rem] space-y-3 overflow-auto rounded-2xl border border-line bg-shell p-4">
                  {(session?.messages || []).map((item) => (
                    <article
                      key={item.id}
                      className="rounded-xl border border-line bg-surface p-3"
                    >
                      <div className="text-[11px] font-bold uppercase tracking-[0.16em] text-muted">
                        {item.role}
                      </div>
                      <div className="mt-2 whitespace-pre-wrap text-sm text-body">
                        {item.content}
                      </div>
                    </article>
                  ))}
                  {!session || (session.messages || []).length === 0 ? (
                    <p className="text-sm text-muted">No messages yet.</p>
                  ) : null}
                </div>
                <div className="rounded-2xl border border-line bg-shell p-4">
                  <label
                    className="text-sm font-semibold text-body"
                    htmlFor="agent_prompt"
                  >
                    Prompt
                  </label>
                  <textarea
                    id="agent_prompt"
                    value={prompt}
                    onChange={(event) => setPrompt(event.target.value)}
                    className="mt-2 min-h-32 w-full rounded-xl border border-line bg-surface px-3 py-3 text-sm text-body"
                    placeholder="Ask the configured ACP agent to investigate, summarize, or execute workflow steps."
                    name="agent_prompt"
                  />
                  <div className="mt-3 flex justify-end">
                    <button
                      type="button"
                      className="rounded-xl bg-accent px-4 py-2 text-sm font-semibold text-white"
                      disabled={busy || !selectedSessionID || !prompt.trim()}
                      onClick={() => void sendPrompt()}
                    >
                      Send Prompt
                    </button>
                  </div>
                </div>
              </div>

              <div className="space-y-4">
                <aside className="rounded-2xl border border-line bg-shell p-4">
                  <div className="flex items-center justify-between gap-3">
                    <div className="text-sm font-semibold text-body">
                      MCP Capabilities
                    </div>
                    <div className="text-xs uppercase tracking-[0.16em] text-muted">
                      {mcpTools.length} tools
                    </div>
                  </div>
                  <div className="mt-3 grid gap-3">
                    <div className="rounded-xl border border-line bg-surface p-3">
                      <div className="text-[11px] font-bold uppercase tracking-[0.16em] text-muted">
                        Business Comprehension
                      </div>
                      <div className="mt-2 space-y-2">
                        {investigationTools.slice(0, 6).map((item) => (
                          <div
                            key={item.name}
                            className="rounded-lg border border-line bg-shell px-3 py-2"
                          >
                            <div className="text-sm font-semibold text-body">
                              {item.title || item.name}
                            </div>
                            <div className="mt-1 text-[11px] uppercase tracking-[0.14em] text-muted">
                              {[
                                item.sourceType,
                                item.contract?.actionClass,
                                item.contract?.riskClass,
                              ]
                                .filter(Boolean)
                                .join(" · ")}
                            </div>
                            {item.contract?.businessDomains?.length ? (
                              <div className="mt-1 text-xs text-muted">
                                {item.contract.businessDomains.join(", ")}
                              </div>
                            ) : null}
                          </div>
                        ))}
                      </div>
                    </div>
                    <div className="rounded-xl border border-line bg-surface p-3">
                      <div className="text-[11px] font-bold uppercase tracking-[0.16em] text-muted">
                        Governed Actions
                      </div>
                      <div className="mt-2 space-y-2">
                        {governedTools.slice(0, 6).map((item) => (
                          <div
                            key={item.name}
                            className="rounded-lg border border-line bg-shell px-3 py-2"
                          >
                            <div className="text-sm font-semibold text-body">
                              {item.title || item.name}
                            </div>
                            <div className="mt-1 text-[11px] uppercase tracking-[0.14em] text-muted">
                              {[
                                item.contract?.actionClass,
                                item.contract?.riskClass,
                                item.contract?.draftOnly ? "draft-only" : "",
                                item.contract?.requiresConfirmation
                                  ? "confirm"
                                  : "",
                                item.contract?.requiresApproval
                                  ? "approval"
                                  : "",
                              ]
                                .filter(Boolean)
                                .join(" · ")}
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                </aside>

                <aside className="rounded-2xl border border-line bg-shell p-4">
                  <div className="text-sm font-semibold text-body">
                    Approvals
                  </div>
                  <div className="mt-3 space-y-3">
                    {(session?.approvals || []).map((item) => (
                      <article
                        key={item.id}
                        className="rounded-xl border border-line bg-surface p-3"
                      >
                        <div className="text-sm font-semibold text-body">
                          {item.title}
                        </div>
                        {item.description ? (
                          <p className="mt-1 text-xs text-muted">
                            {item.description}
                          </p>
                        ) : null}
                        <div className="mt-3 flex gap-2">
                          <button
                            type="button"
                            className="rounded-lg border border-line px-3 py-2 text-xs font-semibold text-body"
                            disabled={busy || item.status !== "pending"}
                            onClick={() =>
                              void resolveApproval(item.id, "approve")
                            }
                          >
                            Approve
                          </button>
                          <button
                            type="button"
                            className="rounded-lg border border-line px-3 py-2 text-xs font-semibold text-body"
                            disabled={busy || item.status !== "pending"}
                            onClick={() =>
                              void resolveApproval(item.id, "reject")
                            }
                          >
                            Reject
                          </button>
                        </div>
                      </article>
                    ))}
                    {(session?.approvals || []).length === 0 ? (
                      <p className="text-sm text-muted">
                        No pending approvals.
                      </p>
                    ) : null}
                  </div>
                </aside>

                <aside className="rounded-2xl border border-line bg-shell p-4">
                  <div className="text-sm font-semibold text-body">Trace</div>
                  <div className="mt-3 max-h-64 space-y-2 overflow-auto">
                    {(session?.trace || []).map((item) => (
                      <div
                        key={item.id}
                        className="rounded-lg border border-line bg-surface px-3 py-2"
                      >
                        <div className="text-[11px] font-bold uppercase tracking-[0.16em] text-muted">
                          {item.kind}
                        </div>
                        <div className="mt-1 text-xs text-body">
                          {formatDate(item.created_at)}
                        </div>
                      </div>
                    ))}
                    {(session?.trace || []).length === 0 ? (
                      <p className="text-sm text-muted">
                        Trace events will stream here.
                      </p>
                    ) : null}
                  </div>
                </aside>
              </div>
            </div>
          </section>
        </section>
      </main>
    </div>
  );
}

async function refreshSession(
  sessionID: string,
  setSession: (session: ACPSession | null) => void,
  setSessions: (updater: (current: ACPSession[]) => ACPSession[]) => void,
) {
  const current = await fetchJson<ACPSession>(
    `/agent/api/sessions/${encodeURIComponent(sessionID)}`,
  );
  setSession(current);
  setSessions((items) => {
    const next = items.filter((item) => item.id !== current.id);
    return [current, ...next];
  });
}

async function fetchJson<T>(url: string): Promise<T> {
  const response = await fetch(url, { credentials: "include" });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return (await response.json()) as T;
}

async function mutateJson<T>(url: string, init: RequestInit): Promise<T> {
  const response = await fetch(url, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      "X-CSRF-Token": readCookie("orbyte_csrf"),
      ...(init.headers || {}),
    },
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return (await response.json()) as T;
}

async function callMcp<T>(method: string, params?: Record<string, unknown>): Promise<T> {
  const response = await fetch("/mcp", {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      "X-CSRF-Token": readCookie("orbyte_csrf"),
    },
    body: JSON.stringify({
      jsonrpc: "2.0",
      id: 1,
      method,
      params,
    }),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  const payload = (await response.json()) as {
    error?: { message?: string };
    result?: T;
  };
  if (payload.error) {
    throw new Error(payload.error.message || "MCP request failed.");
  }
  return payload.result as T;
}

function sortMcpTools(items: MCPTool[]): MCPTool[] {
  return [...items].sort((left, right) =>
    String(left.title || left.name).localeCompare(String(right.title || right.name)),
  );
}

function readCookie(name: string): string {
  const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = document.cookie.match(new RegExp(`(?:^|; )${escaped}=([^;]*)`));
  return match?.[1] ? decodeURIComponent(match[1]) : "";
}

function formatDate(value?: string): string {
  if (!value) return "-";
  const current = new Date(value);
  if (Number.isNaN(current.getTime())) return value;
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(current);
}
