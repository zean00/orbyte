import { useEffect, useMemo, useRef, useState } from "react";
import type { ChangeEvent, KeyboardEvent } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { AnimatePresence, motion } from "framer-motion";
import { useNavigate } from "react-router-dom";
import {
  DashboardWidgetCard,
  type DashboardResolvedWidget,
  type DashboardWidgetDefinition,
  type WidgetDataState,
  defaultWidgetDataState,
  useSharedDashboardData,
} from "@/features/dashboard/runtime";
import {
  fetchWorkspaceBootstrap,
  toShellRoutes,
  workspaceSurfaceTarget,
} from "@/services/bootstrap";
import { preloadSurfaceModule } from "@/services/surfaceModules";
import { useShellStore } from "@/stores/shellStore";

type ProviderInfo = {
  key: string;
  name: string;
  description?: string;
  available?: boolean;
  supports_streaming?: boolean;
  supports_approvals?: boolean;
  supports_model_listing?: boolean;
  supports_model_selection?: boolean;
  supports_plan_updates?: boolean;
  default_model?: string;
  error?: string;
};

type ACPModelInfo = {
  id: string;
  label: string;
  provider_key: string;
  raw_model_id?: string;
  description?: string;
  selectable?: boolean;
  default?: boolean;
};

type ACPMessage = {
  id: string;
  role: string;
  content: string;
  format?: string;
  created_at?: string;
  meta?: Record<string, unknown>;
};

type ACPApproval = {
  id: string;
  status: string;
  title: string;
  description?: string;
  payload?: Record<string, unknown>;
};

type ACPEvent = {
  id: string;
  kind: string;
  created_at?: string;
  payload?: Record<string, unknown>;
};

type ACPPlanEntry = {
  content: string;
  priority?: string;
  status?: string;
};

type ACPClarificationQuestion = {
  id: string;
  content: string;
  source_message_id?: string;
};

type ACPArtifact = {
  id: string;
  kind: string;
  title: string;
  content_type?: string;
  content?: string;
  created_at?: string;
  metadata?: Record<string, unknown>;
};

type ACPSession = {
  id: string;
  provider_key: string;
  provider_name: string;
  requested_model?: string;
  current_model?: string;
  title?: string;
  status: string;
  route_path?: string;
  created_at?: string;
  updated_at?: string;
  current_turn_id?: string;
  turn_in_progress?: boolean;
  messages?: ACPMessage[];
  approvals?: ACPApproval[];
  trace?: ACPEvent[];
  current_plan?: ACPPlanEntry[];
  artifacts?: ACPArtifact[];
  pending_questions?: ACPClarificationQuestion[];
  pending_question_set_id?: string;
  awaiting_input_kind?: string;
};

type DashboardWidgetArtifact = {
  id: string;
  kind: "dashboard_widget";
  title: string;
  widget: DashboardResolvedWidget;
};

type DashboardBoardArtifact = {
  id: string;
  kind: "dashboard_board";
  title: string;
  openPath?: string;
  boardID?: string;
  widgets: DashboardResolvedWidget[];
};

type DashboardArtifact = DashboardWidgetArtifact | DashboardBoardArtifact;

type LiveToolCall = {
  id: string;
  name: string;
  status: string;
  summary?: string;
  state: "active" | "completed";
};

type LiveTurnState = {
  turnID: string;
  phase: "thinking" | "tooling" | "streaming" | "approval";
  activeTools: LiveToolCall[];
  recentTools: LiveToolCall[];
};

type LocalPendingTurn = {
  sessionID?: string;
  phase: LiveTurnState["phase"];
};

type ComposerMode = "ask" | "plan" | "execute";

function acpModeForComposerMode(mode: ComposerMode): "plan" | undefined {
  return mode === "plan" ? "plan" : undefined;
}
type AgentContentTab = "conversation" | "artifacts";

const NEW_SESSION_PENDING_ID = "__new_session__";

type MCPTool = {
  name: string;
  title?: string;
  description?: string;
  moduleKey?: string;
  sourceType?: string;
  policyState?: string;
  policyReason?: string;
  contract?: {
    actionClass?: string;
    riskClass?: string;
    businessDomains?: string[];
  };
};

type MCPToolSummary = {
  tool_id: string;
  name: string;
  title?: string;
  description?: string;
  module_key?: string;
  source_type?: string;
  domains?: string[];
  labels?: string[];
};

type MCPPlaybookSummary = {
  id: string;
  name: string;
  description?: string;
  domains?: string[];
  labels?: string[];
};

type MCPCapability = {
  key: string;
  title: string;
  description?: string;
};

type MCPToolCatalog = {
  mode?: string;
  returned_tools?: number;
  hidden_tools?: number;
  total_matching_tools?: number;
};

type StreamEventName =
  | "turn_started"
  | "session_started"
  | "session_update"
  | "user_message"
  | "turn_completed"
  | "turn_failed"
  | "tool_call_started"
  | "tool_call_updated"
  | "tool_call_completed"
  | "approval_requested"
  | "approval_approved"
  | "approval_rejected"
  | "clarification_requested"
  | "clarification_resolved"
  | "notification";

type TranscriptItem = ACPMessage & {
  streaming?: boolean;
};

type DraftLink = {
  key: string;
  openPath: string;
  title?: string;
  documentID?: string;
};

const DEFAULT_AGENT_CAPABILITIES = [
  "discovery",
  "business_overview",
  "cross_domain_analytics",
  "relationships_timeline",
  "governed_drafts",
];

const OPTIONAL_AGENT_CAPABILITIES: MCPCapability[] = [
  { key: "pricing_promotion", title: "Pricing" },
  { key: "tax_structure", title: "Tax" },
  { key: "treasury_reconciliation", title: "Treasury" },
  { key: "inventory_health", title: "Inventory" },
  { key: "party_master", title: "Party Master" },
];

const STREAM_EVENT_NAMES: StreamEventName[] = [
  "turn_started",
  "session_started",
  "session_update",
  "user_message",
  "turn_completed",
  "turn_failed",
  "tool_call_started",
  "tool_call_updated",
  "tool_call_completed",
  "approval_requested",
  "approval_approved",
  "approval_rejected",
  "clarification_requested",
  "clarification_resolved",
  "notification",
];

const MCP_ONLY_STORAGE_KEY = "orbyte.agent.mcp_only";
const MCP_EXPOSURE_MODE_STORAGE_KEY = "orbyte.agent.mcp_exposure_mode";
type MCPExposureMode = "minimal" | "compact" | "full";
const MCP_ONLY_PREFIX =
  "Use Orbyte MCP tools as the source of truth for this answer. In minimal mode, the required first step for any workflow-like business task is skills.search or skills.list. If one or more skills match, call one bulk skills.describe before any business tool call. Only if no skill matches should you fall back to tool discovery with tools.search or tools.list, then use one bulk tools.describe call for the relevant tool ids before tools.call. Do not invoke business tools from memory before discovery. Use exact discovered ids and do not guess tool names, prefixes, or schemas.";

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
  const [providerModels, setProviderModels] = useState<ACPModelInfo[]>([]);
  const [selectedModel, setSelectedModel] = useState("");
  const [modelsLoading, setModelsLoading] = useState(false);
  const [mcpOnlyEnabled, setMcpOnlyEnabled] = useState(true);
  const [mcpExposureMode, setMcpExposureMode] =
    useState<MCPExposureMode>("minimal");
  const [localPendingTurn, setLocalPendingTurn] =
    useState<LocalPendingTurn | null>(null);
  const [composerMode, setComposerMode] = useState<ComposerMode>("ask");
  const [contentTab, setContentTab] = useState<AgentContentTab>("conversation");
  const [prompt, setPrompt] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [mcpTools, setMcpTools] = useState<MCPTool[]>([]);
  const [discoverableTools, setDiscoverableTools] = useState<MCPToolSummary[]>(
    [],
  );
  const [playbooks, setPlaybooks] = useState<MCPPlaybookSummary[]>([]);
  const [activeCapabilities, setActiveCapabilities] = useState<string[]>(
    DEFAULT_AGENT_CAPABILITIES,
  );
  const [catalogSummary, setCatalogSummary] = useState<MCPToolCatalog | null>(
    null,
  );
  const [fullCatalogLoading, setFullCatalogLoading] = useState(false);
  const [showFullCatalog, setShowFullCatalog] = useState(false);
  const [fullCatalogQuery, setFullCatalogQuery] = useState("");
  const [suggestedExpansions, setSuggestedExpansions] = useState<
    MCPCapability[]
  >([]);
  const [showInspector, setShowInspector] = useState(false);
  const streamRef = useRef<EventSource | null>(null);
  const transcriptRef = useRef<HTMLDivElement | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const shouldStickToBottomRef = useRef(true);
  const seenEventIDsRef = useRef<Set<string>>(new Set());
  const sendInFlightRef = useRef(false);

  useEffect(() => {
    setCurrentRoute("/agent/workspace");
  }, [setCurrentRoute]);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const stored = window.localStorage.getItem(MCP_ONLY_STORAGE_KEY);
    if (stored === "false") {
      setMcpOnlyEnabled(false);
    }
    const storedExposure = window.localStorage.getItem(
      MCP_EXPOSURE_MODE_STORAGE_KEY,
    );
    if (
      storedExposure === "minimal" ||
      storedExposure === "compact" ||
      storedExposure === "full"
    ) {
      setMcpExposureMode(storedExposure);
    }
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") return;
    window.localStorage.setItem(
      MCP_ONLY_STORAGE_KEY,
      mcpOnlyEnabled ? "true" : "false",
    );
  }, [mcpOnlyEnabled]);

  useEffect(() => {
    if (typeof window === "undefined") return;
    window.localStorage.setItem(MCP_EXPOSURE_MODE_STORAGE_KEY, mcpExposureMode);
  }, [mcpExposureMode]);

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
    const [bootstrap] = await Promise.all([
      fetchWorkspaceBootstrap(nextSurface),
      preloadSurfaceModule(nextSurface),
    ]);
    navigate(
      workspaceSurfaceTarget(bootstrap, nextSurface) || defaultPath || "/",
      {
        replace: true,
      },
    );
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
        if (!providerKey && nextProviders[0]) {
          setProviderKey(nextProviders[0].key);
        }
        const orderedSessions = orderSessions(nextSessions);
        const preferredSession =
          orderedSessions.find((item) => item.turn_in_progress) ||
          orderedSessions[0];
        const selectedStillExists =
          selectedSessionID &&
          nextSessions.some((item) => item.id === selectedSessionID);
        if (selectedSessionID && !selectedStillExists) {
          setSelectedSessionID(preferredSession?.id || "");
          setSession(preferredSession || null);
          setLocalPendingTurn(null);
        } else if (!selectedSessionID && preferredSession) {
          setSelectedSessionID(preferredSession.id);
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
        const payload = await callMcp<{
          tools?: MCPTool[];
          catalog?: MCPToolCatalog;
        }>("tools/list", {
          exposure_mode: mcpExposureMode,
          capabilities: activeCapabilities,
          include_summary: true,
          include_hidden_counts: true,
        });
        if (!mounted) return;
        setMcpTools(sortMcpTools(payload.tools || []));
        setCatalogSummary(payload.catalog || null);
        setSuggestedExpansions([]);
      } catch {
        if (!mounted) return;
        setMcpTools([]);
        setCatalogSummary(null);
        setSuggestedExpansions([]);
      }
    }
    void loadMcpTools();
    return () => {
      mounted = false;
    };
  }, [activeCapabilities, mcpExposureMode]);

  useEffect(() => {
    let mounted = true;
    setFullCatalogLoading(true);
    Promise.all([
      callMcp<{ structuredContent?: { items?: MCPToolSummary[] } }>(
        "tools/search",
        {},
      ),
      callMcp<{ structuredContent?: { items?: MCPPlaybookSummary[] } }>(
        "skills/list",
        {},
      ),
    ])
      .then(([toolsPayload, playbooksPayload]) => {
        if (!mounted) return;
        setDiscoverableTools(
          sortMcpToolSummaries(toolsPayload.structuredContent?.items || []),
        );
        setPlaybooks(
          sortPlaybooks(playbooksPayload.structuredContent?.items || []),
        );
      })
      .catch(() => {
        if (!mounted) return;
        setDiscoverableTools([]);
        setPlaybooks([]);
      })
      .finally(() => {
        if (mounted) {
          setFullCatalogLoading(false);
        }
      });
    return () => {
      mounted = false;
    };
  }, []);

  useEffect(() => {
    const textarea = textareaRef.current;
    if (!textarea) return;
    textarea.style.height = "0px";
    const nextHeight = Math.min(Math.max(textarea.scrollHeight, 84), 220);
    textarea.style.height = `${nextHeight}px`;
  }, [prompt]);

  useEffect(() => {
    if (!selectedSessionID) {
      setSession(null);
      setLocalPendingTurn(null);
      setComposerMode("ask");
      seenEventIDsRef.current = new Set();
      closeStream(streamRef);
      return;
    }
    let disposed = false;
    closeStream(streamRef);
    void (async () => {
      let current: ACPSession;
      try {
        current = await refreshSession(
          selectedSessionID,
          setSession,
          setSessions,
        );
      } catch (error) {
        if (disposed) return;
        const messageText =
          error instanceof Error ? error.message : "Failed to refresh session.";
        if (messageText.toLowerCase().includes("session not found")) {
          setSelectedSessionID("");
          setSession(null);
          setLocalPendingTurn(null);
          return;
        }
        setMessage(messageText);
        return;
      }
      if (disposed) return;
      seenEventIDsRef.current = new Set(
        (current.trace || []).map((item) => item.id).filter(Boolean),
      );
      const stream = new EventSource(
        `/agent/api/sessions/${encodeURIComponent(selectedSessionID)}/events`,
        { withCredentials: true },
      );
      const onNamedEvent = (event: MessageEvent<string>) => {
        if (disposed) return;
        const parsed = parseACPStreamEvent(event.data);
        if (!parsed) return;
        if (parsed.id && seenEventIDsRef.current.has(parsed.id)) {
          return;
        }
        if (parsed.id) {
          seenEventIDsRef.current.add(parsed.id);
        }
        setSession((currentSession) =>
          applyACPStreamEvent(currentSession, parsed),
        );
        if (
          parsed.kind === "tool_call_started" ||
          parsed.kind === "tool_call_updated"
        ) {
          setLocalPendingTurn((currentTurn) =>
            currentTurn?.sessionID === selectedSessionID
              ? { ...currentTurn, phase: "tooling" }
              : currentTurn,
          );
        } else if (
          parsed.kind === "session_update" ||
          parsed.kind === "turn_started" ||
          parsed.kind === "user_message"
        ) {
          setLocalPendingTurn((currentTurn) =>
            currentTurn?.sessionID === selectedSessionID
              ? {
                  ...currentTurn,
                  phase:
                    parsed.kind === "session_update" &&
                    stringValue(parsed.payload?.update_kind) ===
                      "agent_message_chunk"
                      ? "streaming"
                      : currentTurn.phase === "tooling"
                        ? "tooling"
                        : "thinking",
                }
              : currentTurn,
          );
        }
        setSessions((currentSessions) =>
          currentSessions.map((item) =>
            item.id === selectedSessionID
              ? applyACPStreamEvent(item, parsed) || item
              : item,
          ),
        );
        if (
          parsed.kind === "turn_completed" ||
          parsed.kind === "turn_failed" ||
          parsed.kind.startsWith("approval_")
        ) {
          setLocalPendingTurn((currentTurn) =>
            currentTurn?.sessionID === selectedSessionID ? null : currentTurn,
          );
          void refreshSession(selectedSessionID, setSession, setSessions).then(
            (next) => {
              if (!next.turn_in_progress) {
                setLocalPendingTurn((currentTurn) =>
                  currentTurn?.sessionID === selectedSessionID
                    ? null
                    : currentTurn,
                );
              }
              seenEventIDsRef.current = new Set(
                (next.trace || []).map((item) => item.id).filter(Boolean),
              );
            },
          );
        }
      };
      for (const name of STREAM_EVENT_NAMES) {
        stream.addEventListener(name, onNamedEvent as EventListener);
      }
      stream.onerror = () => {
        stream.close();
        if (!disposed) {
          void refreshSession(selectedSessionID, setSession, setSessions).then(
            (next) => {
              if (!next.turn_in_progress) {
                setLocalPendingTurn((currentTurn) =>
                  currentTurn?.sessionID === selectedSessionID
                    ? null
                    : currentTurn,
                );
              }
              seenEventIDsRef.current = new Set(
                (next.trace || []).map((item) => item.id).filter(Boolean),
              );
            },
          );
        }
      };
      streamRef.current = stream;
    })().catch(() => {
      if (!disposed) {
        void refreshSession(selectedSessionID, setSession, setSessions).then(
          (next) => {
            if (!next.turn_in_progress) {
              setLocalPendingTurn((currentTurn) =>
                currentTurn?.sessionID === selectedSessionID
                  ? null
                  : currentTurn,
              );
            }
            seenEventIDsRef.current = new Set(
              (next.trace || []).map((item) => item.id).filter(Boolean),
            );
          },
        );
      }
    });
    return () => {
      disposed = true;
      closeStream(streamRef);
    };
  }, [selectedSessionID]);

  useEffect(() => {
    setComposerMode("ask");
  }, [selectedSessionID]);

  const selectedProvider = useMemo(
    () =>
      providers.find((item) => item.key === providerKey) ||
      providers[0] ||
      null,
    [providerKey, providers],
  );
  const selectedSessionProvider = useMemo(() => {
    const currentProviderKey = session?.provider_key || providerKey;
    return (
      providers.find((item) => item.key === currentProviderKey) ||
      selectedProvider
    );
  }, [providerKey, providers, selectedProvider, session?.provider_key]);

  const sortedSessions = useMemo(() => orderSessions(sessions), [sessions]);

  useEffect(() => {
    if (!selectedProvider) {
      setProviderModels([]);
      setSelectedModel("");
      setModelsLoading(false);
      return;
    }
    let mounted = true;
    if (!selectedProvider.supports_model_listing) {
      setProviderModels([]);
      setSelectedModel(selectedProvider.default_model || "");
      setModelsLoading(false);
      return;
    }
    setModelsLoading(true);
    void fetchJson<{ items?: ACPModelInfo[] }>(
      `/agent/api/providers/${encodeURIComponent(selectedProvider.key)}/models`,
    )
      .then((payload) => {
        if (!mounted) return;
        const items = payload.items || [];
        setProviderModels(items);
        const defaultModel =
          selectedProvider.default_model ||
          items.find((item) => item.default)?.id ||
          items.find((item) => item.selectable)?.id ||
          "";
        setSelectedModel(defaultModel);
      })
      .catch(() => {
        if (!mounted) return;
        setProviderModels([]);
        setSelectedModel(selectedProvider.default_model || "");
      })
      .finally(() => {
        if (mounted) {
          setModelsLoading(false);
        }
      });
    return () => {
      mounted = false;
    };
  }, [selectedProvider]);

  const transcriptMessages = useMemo<TranscriptItem[]>(() => {
    const items = (session?.messages || [])
      .filter((item) => item.role === "user" || item.role === "assistant")
      .map((item, index, source) => ({
        ...item,
        streaming:
          item.role === "assistant" &&
          !!session?.turn_in_progress &&
          (index === source.length - 1 ||
            stringValue(item.meta?.turn_id) === session?.current_turn_id),
      }));
    return items;
  }, [session?.messages, session?.turn_in_progress, session?.current_turn_id]);

  const activityMessages = useMemo(
    () => (session?.messages || []).filter((item) => item.role === "system"),
    [session?.messages],
  );

  const liveTurn = useMemo(() => deriveLiveTurnState(session), [session]);
  const effectiveLivePhase: LiveTurnState["phase"] | null =
    liveTurn?.phase ||
    (localPendingTurn &&
    (localPendingTurn.sessionID === selectedSessionID ||
      localPendingTurn.sessionID === NEW_SESSION_PENDING_ID ||
      (!localPendingTurn.sessionID && !selectedSessionID))
      ? localPendingTurn.phase
      : null);

  const renderedTranscriptMessages = useMemo<TranscriptItem[]>(() => {
    const items = [...transcriptMessages];
    if (!effectiveLivePhase) return items;
    const latestUserIndex = [...items]
      .reverse()
      .findIndex((item) => item.role === "user" && item.content.trim());
    if (latestUserIndex === -1) return items;
    const activeTurnID =
      liveTurn?.turnID ||
      session?.current_turn_id ||
      localPendingTurn?.sessionID ||
      NEW_SESSION_PENDING_ID;
    const hasActiveAssistantBubble = items.some(
      (item) =>
        item.role === "assistant" &&
        item.streaming &&
        stringValue(item.meta?.turn_id) === activeTurnID,
    );
    if (hasActiveAssistantBubble) return items;
    items.push({
      id: `pending-assistant-${activeTurnID}`,
      role: "assistant",
      content: "",
      streaming: true,
      meta: {
        turn_id: activeTurnID,
        live_phase: effectiveLivePhase,
      },
    });
    return items;
  }, [
    effectiveLivePhase,
    liveTurn?.turnID,
    localPendingTurn?.sessionID,
    session?.current_turn_id,
    transcriptMessages,
  ]);

  const visibleTools = useMemo(() => mcpTools.slice(0, 10), [mcpTools]);
  const filteredFullCatalogTools = useMemo(() => {
    const query = fullCatalogQuery.trim().toLowerCase();
    if (!query) {
      return discoverableTools;
    }
    return discoverableTools.filter((item) => {
      const haystack = [
        item.tool_id,
        item.name,
        item.title,
        item.description,
        item.module_key,
        item.source_type,
        ...(item.domains || []),
        ...(item.labels || []),
      ]
        .filter(Boolean)
        .join(" ")
        .toLowerCase();
      return haystack.includes(query);
    });
  }, [discoverableTools, fullCatalogQuery]);
  const currentPlan = useMemo(() => deriveCurrentPlan(session), [session]);
  const hasCurrentPlan = currentPlan.length > 0;
  const pendingInputQuestions = session?.pending_questions || [];
  const pendingInputKind = session?.awaiting_input_kind || "clarification";
  const hasPendingInput =
    session?.status === "awaiting_input" && pendingInputQuestions.length > 0;
  const hasPendingConfirmation =
    hasPendingInput && pendingInputKind === "confirmation";
  const dashboardArtifacts = useMemo(
    () => deriveDashboardArtifacts(session),
    [session],
  );
  const dashboardArtifactWidgets = useMemo(
    () => flattenArtifactWidgets(dashboardArtifacts),
    [dashboardArtifacts],
  );
  const dashboardArtifactData = useSharedDashboardData(
    dashboardArtifactWidgets,
  );
  const hasDashboardArtifacts = dashboardArtifacts.length > 0;
  const providerSupportsPlanUpdates =
    selectedSessionProvider?.supports_plan_updates ??
    selectedProvider?.supports_plan_updates ??
    false;
  const inspectorVisible =
    showInspector ||
    typeof window === "undefined" ||
    hasCurrentPlan ||
    composerMode !== "ask";

  useEffect(() => {
    if (contentTab === "artifacts" && !hasDashboardArtifacts) {
      setContentTab("conversation");
    }
  }, [contentTab, hasDashboardArtifacts]);

  useEffect(() => {
    const panel = transcriptRef.current;
    if (!panel || !shouldStickToBottomRef.current) return;
    panel.scrollTop = panel.scrollHeight;
  }, [renderedTranscriptMessages, session?.turn_in_progress]);

  function handleTranscriptScroll() {
    const panel = transcriptRef.current;
    if (!panel) return;
    shouldStickToBottomRef.current =
      panel.scrollHeight - panel.scrollTop - panel.clientHeight < 120;
  }

  function toggleCapability(key: string) {
    setActiveCapabilities((current) =>
      current.includes(key)
        ? current.filter((item) => item !== key)
        : [...current, key],
    );
  }

  async function createSession(): Promise<ACPSession | null> {
    if (!selectedProvider) return null;
    setBusy(true);
    setMessage("");
    try {
      const created = await mutateJson<ACPSession>("/agent/api/sessions", {
        method: "POST",
        body: JSON.stringify({
          provider_key: selectedProvider.key,
          model:
            selectedProvider.supports_model_selection && selectedModel
              ? selectedModel
              : undefined,
          shell: "agent_surface",
          route_path: "/agent/workspace",
          title: selectedProvider.name,
        }),
      });
      setSessions((current) => mergeSessionIntoList(current, created));
      setSelectedSessionID(created.id);
      setSession(created);
      setShowInspector(false);
      return created;
    } catch (error) {
      setMessage(
        error instanceof Error ? error.message : "Failed to start session.",
      );
      return null;
    } finally {
      setBusy(false);
    }
  }

  async function sendPrompt() {
    if (
      sendInFlightRef.current ||
      (composerMode !== "execute" && !prompt.trim())
    ) {
      return;
    }
    if (composerMode === "execute" && !hasCurrentPlan) {
      setMessage("Create a plan first before using Execute mode.");
      return;
    }
    sendInFlightRef.current = true;
    setMessage("");
    const displayPrompt =
      composerMode === "execute" && !prompt.trim()
        ? "Execute the current plan."
        : prompt.trim();
    let targetSessionID = selectedSessionID;
    let targetSession: ACPSession | null =
      selectedSessionID && session?.id === selectedSessionID ? session : null;
    if (
      targetSessionID &&
      !sessions.some((item) => item.id === targetSessionID)
    ) {
      targetSessionID = "";
      targetSession = null;
      setSelectedSessionID("");
      setSession(null);
      setLocalPendingTurn(null);
    }
    setLocalPendingTurn({
      sessionID: targetSessionID || NEW_SESSION_PENDING_ID,
      phase: "thinking",
    });
    if (!targetSessionID) {
      const created = await createSession();
      if (!created) {
        setLocalPendingTurn(null);
        sendInFlightRef.current = false;
        return;
      }
      targetSessionID = created.id;
      targetSession = created;
      setLocalPendingTurn({ sessionID: created.id, phase: "thinking" });
    }
    const effectiveComposerMode = hasPendingConfirmation ? "ask" : composerMode;
    const nextPrompt = buildPromptPayload(
      displayPrompt,
      mcpOnlyEnabled,
      effectiveComposerMode,
      currentPlan,
    );
    const dispatchedMode = effectiveComposerMode;
    const planSnapshot =
      currentPlan.length > 0 ? currentPlan : session?.current_plan || [];
    const optimisticTurnID = `pending-turn-${Date.now()}`;
    const optimisticUpdatedAt = new Date().toISOString();
    setPrompt("");
    shouldStickToBottomRef.current = true;
    setSession((current) =>
      optimisticPromptSession(
        current?.id === targetSessionID ? current : targetSession,
        displayPrompt,
        optimisticTurnID,
        optimisticUpdatedAt,
        dispatchedMode === "plan",
        planSnapshot,
      ),
    );
    setSessions((current) =>
      current.map((item) =>
        item.id === targetSessionID
          ? optimisticPromptSession(
              item,
              displayPrompt,
              optimisticTurnID,
              optimisticUpdatedAt,
              dispatchedMode === "plan",
              planSnapshot,
            ) || item
          : item,
      ),
    );
    if (dispatchedMode === "execute") {
      setComposerMode("ask");
    }
    try {
      const clientRequestID = optimisticTurnID;
      void mutateJson<ACPSession>(
        `/agent/api/sessions/${encodeURIComponent(targetSessionID)}/prompt`,
        {
          method: "POST",
          body: JSON.stringify({
            content: nextPrompt,
            display_content: displayPrompt,
            client_request_id: clientRequestID,
            mode: acpModeForComposerMode(dispatchedMode),
          }),
        },
      )
        .then((updated) => {
          const finalized =
            dispatchedMode === "execute" &&
            (!updated.current_plan || updated.current_plan.length === 0)
              ? { ...updated, current_plan: planSnapshot }
              : updated;
          setSessions((current) => mergeSessionIntoList(current, finalized));
          if (!updated.turn_in_progress) {
            setSession((current) =>
              current?.id === finalized.id ? finalized : current,
            );
            setLocalPendingTurn((currentTurn) =>
              currentTurn?.sessionID === finalized.id ? null : currentTurn,
            );
          }
          sendInFlightRef.current = false;
        })
        .catch(async (error) => {
          setMessage(
            error instanceof Error ? error.message : "Failed to send prompt.",
          );
          setSession((current) =>
            markPromptFailure(current, optimisticTurnID, error),
          );
          setSessions((current) =>
            current.map((item) =>
              item.id === targetSessionID
                ? markPromptFailure(item, optimisticTurnID, error) || item
                : item,
            ),
          );
          await refreshSession(targetSessionID, setSession, setSessions).catch(
            () => undefined,
          );
          setLocalPendingTurn((currentTurn) =>
            currentTurn?.sessionID === targetSessionID ? null : currentTurn,
          );
          sendInFlightRef.current = false;
        });
    } catch (error) {
      setMessage(
        error instanceof Error ? error.message : "Failed to send prompt.",
      );
      setLocalPendingTurn((currentTurn) =>
        currentTurn?.sessionID === targetSessionID ? null : currentTurn,
      );
      sendInFlightRef.current = false;
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

  async function deleteSession(sessionID: string) {
    const target = sessions.find((item) => item.id === sessionID);
    const label = target?.title || target?.provider_name || "this session";
    if (!window.confirm(`Delete ${label}? This cannot be undone.`)) {
      return;
    }
    setBusy(true);
    setMessage("");
    try {
      await mutateJson<void>(
        `/agent/api/sessions/${encodeURIComponent(sessionID)}`,
        {
          method: "DELETE",
        },
      );
      const remaining = orderSessions(
        sessions.filter((item) => item.id !== sessionID),
      );
      setSessions(remaining);
      if (selectedSessionID === sessionID) {
        closeStream(streamRef);
        seenEventIDsRef.current = new Set();
        setLocalPendingTurn(null);
        setSelectedSessionID(remaining[0]?.id || "");
        setSession(remaining[0] || null);
      }
    } catch (error) {
      setMessage(
        error instanceof Error ? error.message : "Failed to delete session.",
      );
    } finally {
      setBusy(false);
    }
  }

  function handlePromptKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key !== "Enter" || event.shiftKey) return;
    event.preventDefault();
    if (
      busy ||
      !!session?.turn_in_progress ||
      (composerMode !== "execute" && !prompt.trim()) ||
      (composerMode === "execute" && !hasCurrentPlan)
    ) {
      return;
    }
    void sendPrompt();
  }

  function handlePromptInput(event: ChangeEvent<HTMLTextAreaElement>) {
    setPrompt(event.target.value);
  }

  return (
    <div className="min-h-screen bg-[linear-gradient(180deg,#edf3fb_0%,#e9eff8_34%,#f8fbff_100%)] text-body dark:bg-[linear-gradient(180deg,#09111f_0%,#0b1526_32%,#0f172a_100%)]">
      <header className="sticky top-0 z-30 border-b border-line/70 bg-surface/85 backdrop-blur-xl">
        <div className="mx-auto flex max-w-[1700px] items-center justify-between gap-4 px-4 py-4 md:px-6">
          <div>
            <p className="text-[11px] font-black uppercase tracking-[0.24em] text-accent-dark">
              Agent Surface
            </p>
            <div className="mt-1 flex items-center gap-3">
              <h1 className="text-2xl font-black tracking-tight text-body">
                Orbyte Operator Agent
              </h1>
              {selectedProvider?.supports_streaming ? (
                <span className="rounded-full border border-accent/20 bg-accent-soft/70 px-3 py-1 text-[11px] font-bold uppercase tracking-[0.18em] text-accent">
                  Live stream
                </span>
              ) : null}
            </div>
          </div>
          <div className="flex items-center gap-3">
            <span className="hidden rounded-full border border-line bg-shell px-3 py-1 text-xs font-bold uppercase tracking-[0.16em] text-muted md:inline-flex">
              {locale.toUpperCase()}
            </span>
            <button
              type="button"
              onClick={() => setShowInspector((current) => !current)}
              className="rounded-xl border border-line bg-surface px-4 py-2 text-sm font-semibold text-body transition hover:border-accent hover:text-accent"
            >
              {showInspector ? "Hide" : "Show"} details
            </button>
            <button
              type="button"
              onClick={() => void switchSurface("backoffice")}
              className="rounded-xl bg-accent px-4 py-2 text-sm font-semibold text-white shadow-[0_12px_28px_rgba(29,78,216,0.22)] transition hover:opacity-95"
            >
              Backoffice
            </button>
          </div>
        </div>
      </header>

      <main className="mx-auto grid max-w-[1700px] gap-4 px-4 pb-40 pt-4 md:px-6 lg:grid-cols-[300px_minmax(0,1fr)]">
        <aside className="rounded-[1.75rem] border border-line/80 bg-surface/85 p-5 shadow-panel backdrop-blur lg:sticky lg:top-24 lg:flex lg:h-[calc(100svh-8rem)] lg:flex-col lg:overflow-hidden">
          <div className="shrink-0">
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
              <p className="mt-2 text-xs leading-5 text-muted">
                {selectedProvider.description}
              </p>
            ) : null}
            {selectedProvider?.supports_model_listing ? (
              <div className="mt-4">
                <div className="flex items-center justify-between gap-3">
                  <label
                    htmlFor="agent_model"
                    className="text-sm font-semibold text-body"
                  >
                    Model
                  </label>
                  <span className="text-[11px] font-bold uppercase tracking-[0.16em] text-muted">
                    {modelsLoading
                      ? "Loading"
                      : selectedProvider.supports_model_selection
                        ? "Selectable"
                        : "Catalog only"}
                  </span>
                </div>
                <select
                  id="agent_model"
                  name="agent_model"
                  className="mt-2 w-full rounded-xl border border-line bg-shell px-3 py-2 text-sm text-body disabled:cursor-not-allowed disabled:opacity-60"
                  value={selectedModel}
                  disabled={
                    modelsLoading || !selectedProvider.supports_model_selection
                  }
                  onChange={(event) => setSelectedModel(event.target.value)}
                >
                  {providerModels.length > 0 ? (
                    providerModels.map((item) => (
                      <option key={item.id} value={item.id}>
                        {item.label}
                        {item.default ? " (Default)" : ""}
                      </option>
                    ))
                  ) : selectedProvider.default_model ? (
                    <option value={selectedProvider.default_model}>
                      {selectedProvider.default_model}
                    </option>
                  ) : (
                    <option value="">Provider default</option>
                  )}
                </select>
                <p className="mt-2 text-xs leading-5 text-muted">
                  {selectedProvider.supports_model_selection
                    ? "Selected model is applied when a new session starts."
                    : "This provider exposes its model catalog, but ACP session-level model selection is not available yet."}
                </p>
              </div>
            ) : null}
            <button
              type="button"
              className="mt-4 w-full rounded-xl bg-accent px-4 py-3 text-sm font-semibold text-white shadow-[0_14px_30px_rgba(29,78,216,0.26)] transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-50"
              disabled={busy || !selectedProvider}
              onClick={() => void createSession()}
            >
              Start Session
            </button>
          </div>

          <div className="mt-6 flex min-h-0 flex-1 flex-col border-t border-line/80 pt-5">
            <div className="shrink-0 text-sm font-semibold text-body">
              Sessions
            </div>
            <div className="mt-3 min-h-0 flex-1 space-y-2 overflow-y-auto pr-1">
              {sortedSessions.map((item) => {
                const selected = selectedSessionID === item.id;
                const preview = sessionPreview(item);
                return (
                  <article
                    key={item.id}
                    className={`rounded-2xl border px-4 py-3 transition ${
                      selected
                        ? "border-accent bg-accent-soft/70 shadow-[0_10px_24px_rgba(29,78,216,0.14)] ring-1 ring-accent/20"
                        : "border-line bg-shell hover:border-accent/30"
                    }`}
                  >
                    <div className="flex items-start gap-3">
                      <button
                        type="button"
                        aria-pressed={selected}
                        onClick={() => setSelectedSessionID(item.id)}
                        className="min-w-0 flex-1 text-left"
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <div className="truncate text-sm font-semibold text-body">
                              {item.title || item.provider_name}
                            </div>
                            <div className="mt-1 text-[11px] uppercase tracking-[0.16em] text-muted">
                              {shortSessionID(item.id)}
                            </div>
                          </div>
                          {selected ? (
                            <span className="rounded-full bg-accent px-2 py-1 text-[10px] font-bold uppercase tracking-[0.16em] text-white">
                              Active
                            </span>
                          ) : null}
                        </div>
                        <div className="mt-3 line-clamp-2 min-h-[2.5rem] text-xs leading-5 text-muted">
                          {preview}
                        </div>
                        <div className="mt-3 flex items-center justify-between gap-2 text-[11px] uppercase tracking-[0.16em] text-muted">
                          <span>{sessionModelSummary(item)}</span>
                          <span>{formatDate(item.updated_at)}</span>
                        </div>
                      </button>
                      <button
                        type="button"
                        aria-label={`Delete ${item.title || item.provider_name}`}
                        onClick={(event) => {
                          event.stopPropagation();
                          void deleteSession(item.id);
                        }}
                        className="rounded-xl border border-line bg-surface px-3 py-2 text-[11px] font-bold uppercase tracking-[0.16em] text-muted transition hover:border-red-300 hover:text-red-700"
                      >
                        Delete
                      </button>
                    </div>
                  </article>
                );
              })}
              {sessions.length === 0 ? (
                <p className="text-sm text-muted">No ACP sessions yet.</p>
              ) : null}
            </div>
          </div>
        </aside>

        <section className="min-h-[calc(100svh-8rem)]">
          {message ? (
            <div className="mb-4 rounded-2xl border border-line bg-accent-soft/60 px-4 py-3 text-sm text-body shadow-panel">
              {message}
            </div>
          ) : null}

          <div className="relative overflow-hidden rounded-[2rem] border border-line/80 bg-surface/88 shadow-panel backdrop-blur">
            <div className="border-b border-line/70 px-5 py-4 md:px-7">
              <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
                <div>
                  <div className="text-[11px] font-bold uppercase tracking-[0.2em] text-muted">
                    Conversation
                  </div>
                  <h2 className="mt-1 text-2xl font-black tracking-tight text-body">
                    {session?.title || "Session Detail"}
                  </h2>
                  <p className="mt-1 text-sm text-muted">
                    {session
                      ? `${session.provider_name} · ${humanizeSessionStatus(session.status)}`
                      : "Send a message to start a new ACP session automatically."}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  {session?.requested_model &&
                  session?.requested_model !== session?.current_model ? (
                    <div className="rounded-full border border-line bg-shell px-3 py-1 text-[11px] font-bold uppercase tracking-[0.16em] text-muted">
                      Requested {session.requested_model}
                    </div>
                  ) : null}
                  {session?.current_model ? (
                    <div className="rounded-full border border-line bg-shell px-3 py-1 text-[11px] font-bold uppercase tracking-[0.16em] text-muted">
                      Current {session.current_model}
                    </div>
                  ) : null}
                  <div className="rounded-full border border-line bg-shell px-3 py-1 text-[11px] font-bold uppercase tracking-[0.16em] text-muted">
                    {catalogSummary?.returned_tools || mcpTools.length} minimal
                    tools
                  </div>
                  {effectiveLivePhase ? (
                    <div className="rounded-full border border-accent/20 bg-accent-soft/70 px-3 py-1 text-[11px] font-bold uppercase tracking-[0.18em] text-accent">
                      {effectiveLivePhase === "tooling"
                        ? "Using tools"
                        : effectiveLivePhase === "streaming"
                          ? "Streaming"
                          : "Thinking"}
                    </div>
                  ) : null}
                </div>
              </div>
            </div>

            <div className="grid min-h-[calc(100svh-14rem)] lg:grid-cols-[minmax(0,1fr)_340px]">
              <div className="relative bg-[linear-gradient(180deg,rgba(248,251,255,0.98)_0%,rgba(242,247,253,0.92)_100%)] dark:bg-[linear-gradient(180deg,rgba(8,15,28,0.98)_0%,rgba(11,20,36,0.94)_100%)]">
                <div className="border-b border-line/60 px-4 py-3 md:px-8">
                  <div className="mx-auto flex w-full max-w-4xl items-center justify-between gap-3">
                    <div className="inline-flex rounded-full border border-line bg-shell p-1">
                      {(
                        [
                          ["conversation", "Conversation"],
                          [
                            "artifacts",
                            `Artifacts${hasDashboardArtifacts ? ` (${dashboardArtifacts.length})` : ""}`,
                          ],
                        ] as const
                      ).map(([key, label]) => {
                        const selected = contentTab === key;
                        const disabled =
                          key === "artifacts" && !hasDashboardArtifacts;
                        return (
                          <button
                            key={key}
                            type="button"
                            disabled={disabled}
                            onClick={() => setContentTab(key)}
                            className={`rounded-full px-3 py-1 text-[11px] font-bold uppercase tracking-[0.14em] transition ${
                              selected
                                ? "bg-accent text-white shadow-[0_8px_18px_rgba(29,78,216,0.22)]"
                                : "text-muted hover:text-body"
                            } disabled:cursor-not-allowed disabled:opacity-50`}
                          >
                            {label}
                          </button>
                        );
                      })}
                    </div>
                    <div className="text-xs text-muted">
                      {contentTab === "conversation"
                        ? "Transcript with inline dashboard rendering."
                        : "Wide dashboard gallery for generated artifacts."}
                    </div>
                  </div>
                </div>
                <div
                  ref={transcriptRef}
                  onScroll={handleTranscriptScroll}
                  className="h-[calc(100svh-14rem)] overflow-auto px-4 py-6 md:px-8 md:py-8"
                >
                  <div className="mx-auto flex w-full max-w-4xl flex-col gap-4 pb-52">
                    {contentTab === "conversation" && hasPendingInput ? (
                      <div className="rounded-[1.5rem] border border-amber-200/80 bg-[linear-gradient(180deg,rgba(255,250,235,0.95)_0%,rgba(255,244,214,0.92)_100%)] p-5 shadow-[0_16px_36px_rgba(180,83,9,0.08)]">
                        <div className="text-[11px] font-black uppercase tracking-[0.2em] text-amber-700">
                          {hasPendingConfirmation
                            ? "Confirmation needed"
                            : "Clarification needed"}
                        </div>
                        <h3 className="mt-2 text-lg font-black tracking-tight text-body">
                          {hasPendingConfirmation
                            ? "Confirm how the agent should proceed in this session."
                            : "Reply with the missing details to continue this session."}
                        </h3>
                        <p className="mt-2 text-sm leading-6 text-muted">
                          You can answer in one message or point by point. The
                          agent will continue from the same ACP session.
                        </p>
                        <div className="mt-4 space-y-3">
                          {pendingInputQuestions.map((item, index) => (
                            <article
                              key={item.id}
                              className="rounded-2xl border border-amber-300/50 bg-white/70 px-4 py-3"
                            >
                              <div className="text-[11px] font-bold uppercase tracking-[0.16em] text-amber-700">
                                {hasPendingConfirmation
                                  ? "Confirmation"
                                  : `Question ${index + 1}`}
                              </div>
                              <p className="mt-2 text-sm leading-6 text-body">
                                {item.content}
                              </p>
                            </article>
                          ))}
                        </div>
                      </div>
                    ) : null}
                    {contentTab === "conversation" ? (
                      renderedTranscriptMessages.length === 0 ? (
                        <EmptyTranscript />
                      ) : (
                        renderedTranscriptMessages.map((item) => (
                          <MessageBubble
                            key={item.id}
                            item={item}
                            liveTurn={liveTurn}
                            locale={locale}
                            draftLinks={
                              item.role === "assistant"
                                ? draftLinksForTurn(
                                    session?.trace,
                                    stringValue(item.meta?.turn_id),
                                  )
                                : []
                            }
                            dashboardArtifactData={dashboardArtifactData}
                          />
                        ))
                      )
                    ) : (
                      <ArtifactGallery
                        artifacts={dashboardArtifacts}
                        locale={locale}
                        dashboardArtifactData={dashboardArtifactData}
                      />
                    )}
                  </div>
                </div>
              </div>

              <AnimatePresence initial={false}>
                {inspectorVisible && (
                  <motion.aside
                    initial={{ opacity: 0, x: 16 }}
                    animate={{ opacity: 1, x: 0 }}
                    exit={{ opacity: 0, x: 16 }}
                    transition={{ duration: 0.18 }}
                    className="border-l border-line/70 bg-shell/75 p-4 backdrop-blur lg:block lg:h-[calc(100svh-14rem)] lg:overflow-hidden"
                  >
                    <div className="flex h-full min-h-0 flex-col">
                      <section className="shrink-0 rounded-[1.5rem] border border-line/80 bg-surface/88 p-4 shadow-[0_10px_28px_rgba(15,23,42,0.06)]">
                        <div className="text-[11px] font-bold uppercase tracking-[0.18em] text-muted">
                          Suggested focus
                        </div>
                        <div className="mt-3 flex flex-wrap gap-2">
                          {OPTIONAL_AGENT_CAPABILITIES.map((item) => {
                            const active = activeCapabilities.includes(
                              item.key,
                            );
                            return (
                              <button
                                key={item.key}
                                type="button"
                                onClick={() => toggleCapability(item.key)}
                                className={`rounded-full border px-3 py-1.5 text-xs font-semibold transition ${
                                  active
                                    ? "border-accent bg-accent-soft/70 text-accent"
                                    : "border-line bg-shell text-muted hover:border-accent/30 hover:text-body"
                                }`}
                              >
                                {item.title}
                              </button>
                            );
                          })}
                        </div>
                        {suggestedExpansions.length > 0 ? (
                          <p className="mt-3 text-xs leading-5 text-muted">
                            Suggested:{" "}
                            {suggestedExpansions
                              .map((item) => item.title)
                              .join(", ")}
                          </p>
                        ) : null}
                      </section>
                      <div className="mt-4 min-h-0 flex-1 space-y-4 overflow-y-auto pr-1">
                        <InspectorSection
                          title="Current Plan"
                          kicker={
                            composerMode === "execute"
                              ? "execution target"
                              : "plan state"
                          }
                          summary={
                            hasCurrentPlan
                              ? `${currentPlan.length} step${currentPlan.length === 1 ? "" : "s"}`
                              : "No active plan"
                          }
                        >
                          <div className="space-y-3">
                            {!providerSupportsPlanUpdates ? (
                              <p className="rounded-2xl border border-line/70 bg-shell px-3 py-2 text-xs leading-5 text-muted">
                                This provider does not emit structured ACP plan
                                updates. The visible plan is derived from the
                                latest Plan-mode turn.
                              </p>
                            ) : null}
                            {hasCurrentPlan ? (
                              currentPlan.map((item, index) => (
                                <article
                                  key={`${index}-${item.content}`}
                                  className="rounded-2xl border border-line/70 bg-surface p-3"
                                >
                                  <div className="flex items-center justify-between gap-3">
                                    <div className="text-[11px] font-bold uppercase tracking-[0.16em] text-muted">
                                      Step {index + 1}
                                    </div>
                                    <div className="flex items-center gap-2">
                                      {item.priority ? (
                                        <span className="rounded-full border border-line bg-shell px-2 py-1 text-[10px] font-bold uppercase tracking-[0.14em] text-muted">
                                          {item.priority}
                                        </span>
                                      ) : null}
                                      {item.status ? (
                                        <span className="rounded-full border border-accent/20 bg-accent-soft/70 px-2 py-1 text-[10px] font-bold uppercase tracking-[0.14em] text-accent">
                                          {item.status}
                                        </span>
                                      ) : null}
                                    </div>
                                  </div>
                                  <p className="mt-2 text-sm leading-6 text-body">
                                    {item.content}
                                  </p>
                                </article>
                              ))
                            ) : (
                              <p className="text-sm text-muted">
                                Use Plan mode to generate a stepwise plan before
                                executing.
                              </p>
                            )}
                          </div>
                        </InspectorSection>
                        <InspectorSection
                          title="Capabilities"
                          kicker={catalogSummary?.mode || "compact catalog"}
                          summary={[
                            `${catalogSummary?.returned_tools || mcpTools.length} starter tools`,
                            typeof catalogSummary?.hidden_tools === "number"
                              ? `${catalogSummary.hidden_tools} hidden`
                              : "",
                          ]
                            .filter(Boolean)
                            .join(" · ")}
                        >
                          <div className="space-y-3">
                            <p className="rounded-2xl border border-line/70 bg-shell px-3 py-2 text-xs leading-5 text-muted">
                              This surface is running in minimal MCP mode. The
                              agent should search skills first, then use
                              <code>tools.search</code>,{" "}
                              <code>tools.describe</code>, and{" "}
                              <code>tools.call</code> to reach the relevant
                              underlying MCP tools.
                            </p>
                            {visibleTools.map((item) => (
                              <div
                                key={item.name}
                                className="rounded-2xl border border-line/70 bg-surface px-3 py-3"
                              >
                                <div className="text-sm font-semibold text-body">
                                  {item.title || item.name}
                                </div>
                                <div className="mt-1 text-[11px] uppercase tracking-[0.16em] text-muted">
                                  {[
                                    item.sourceType,
                                    item.contract?.actionClass,
                                    item.contract?.riskClass,
                                    item.policyState,
                                  ]
                                    .filter(Boolean)
                                    .join(" · ")}
                                </div>
                                {item.description ? (
                                  <p className="mt-2 text-xs leading-5 text-muted">
                                    {item.description}
                                  </p>
                                ) : null}
                              </div>
                            ))}
                            <div className="rounded-2xl border border-line/70 bg-surface p-3">
                              <div className="flex items-center justify-between gap-3">
                                <div>
                                  <div className="text-sm font-semibold text-body">
                                    Playbooks
                                  </div>
                                  <div className="mt-1 text-[11px] uppercase tracking-[0.16em] text-muted">
                                    {fullCatalogLoading
                                      ? "Loading skills"
                                      : `${playbooks.length} workflow skills`}
                                  </div>
                                </div>
                                <button
                                  type="button"
                                  onClick={() =>
                                    setShowFullCatalog((current) => !current)
                                  }
                                  className="rounded-xl border border-line bg-shell px-3 py-2 text-[11px] font-bold uppercase tracking-[0.16em] text-muted transition hover:border-accent hover:text-accent"
                                >
                                  {showFullCatalog ? "Hide details" : "Browse"}
                                </button>
                              </div>
                              {showFullCatalog ? (
                                <div className="mt-3 space-y-3">
                                  <input
                                    type="search"
                                    value={fullCatalogQuery}
                                    onChange={(event) =>
                                      setFullCatalogQuery(event.target.value)
                                    }
                                    placeholder="Search skills, tools, domains, or descriptions"
                                    className="w-full rounded-xl border border-line bg-shell px-3 py-2 text-sm text-body outline-none transition focus:border-accent"
                                  />
                                  <div className="space-y-2">
                                    {playbooks.map((item) => (
                                      <div
                                        key={item.id}
                                        className="rounded-2xl border border-line/70 bg-shell px-3 py-3"
                                      >
                                        <div className="text-sm font-semibold text-body">
                                          {item.name}
                                        </div>
                                        <div className="mt-1 break-all font-mono text-[11px] text-muted">
                                          {item.id}
                                        </div>
                                        <div className="mt-1 text-[11px] uppercase tracking-[0.16em] text-muted">
                                          {[
                                            ...(item.domains || []),
                                            ...(item.labels || []),
                                          ]
                                            .filter(Boolean)
                                            .join(" · ")}
                                        </div>
                                        {item.description ? (
                                          <p className="mt-2 text-xs leading-5 text-muted">
                                            {item.description}
                                          </p>
                                        ) : null}
                                      </div>
                                    ))}
                                    {!fullCatalogLoading &&
                                    playbooks.length === 0 ? (
                                      <p className="text-sm text-muted">
                                        No skills are configured.
                                      </p>
                                    ) : null}
                                  </div>
                                  <div className="max-h-80 space-y-2 overflow-y-auto pr-1">
                                    {filteredFullCatalogTools
                                      .slice(0, 40)
                                      .map((item) => (
                                        <div
                                          key={item.tool_id}
                                          className="rounded-2xl border border-line/70 bg-shell px-3 py-3"
                                        >
                                          <div className="text-sm font-semibold text-body">
                                            {item.title ||
                                              item.name ||
                                              item.tool_id}
                                          </div>
                                          <div className="mt-1 break-all font-mono text-[11px] text-muted">
                                            {item.tool_id}
                                          </div>
                                          <div className="mt-1 text-[11px] uppercase tracking-[0.16em] text-muted">
                                            {[
                                              item.module_key,
                                              item.source_type,
                                              ...(item.domains || []),
                                              ...(item.labels || []),
                                            ]
                                              .filter(Boolean)
                                              .join(" · ")}
                                          </div>
                                          {item.description ? (
                                            <p className="mt-2 text-xs leading-5 text-muted">
                                              {item.description}
                                            </p>
                                          ) : null}
                                        </div>
                                      ))}
                                    {!fullCatalogLoading &&
                                    filteredFullCatalogTools.length === 0 ? (
                                      <p className="text-sm text-muted">
                                        No tools matched this search.
                                      </p>
                                    ) : null}
                                  </div>
                                  {!fullCatalogLoading &&
                                  filteredFullCatalogTools.length > 40 ? (
                                    <p className="text-xs leading-5 text-muted">
                                      Showing the first 40 matching tools.
                                      Narrow the search to inspect a smaller
                                      slice.
                                    </p>
                                  ) : null}
                                </div>
                              ) : null}
                            </div>
                          </div>
                        </InspectorSection>

                        <InspectorSection
                          title="Approvals"
                          kicker="governed actions"
                          summary={String((session?.approvals || []).length)}
                        >
                          <div className="space-y-3">
                            {(session?.approvals || []).map((item) => (
                              <article
                                key={item.id}
                                className="rounded-2xl border border-line/70 bg-surface p-3"
                              >
                                <div className="text-sm font-semibold text-body">
                                  {item.title}
                                </div>
                                {item.description ? (
                                  <p className="mt-1 text-xs leading-5 text-muted">
                                    {item.description}
                                  </p>
                                ) : null}
                                <div className="mt-3 flex gap-2">
                                  <button
                                    type="button"
                                    className="rounded-xl border border-line px-3 py-2 text-xs font-semibold text-body"
                                    disabled={busy || item.status !== "pending"}
                                    onClick={() =>
                                      void resolveApproval(item.id, "approve")
                                    }
                                  >
                                    Approve
                                  </button>
                                  <button
                                    type="button"
                                    className="rounded-xl border border-line px-3 py-2 text-xs font-semibold text-body"
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
                        </InspectorSection>

                        <InspectorSection
                          title="Activity"
                          kicker="system + trace"
                          summary={`${activityMessages.length} notes · ${(session?.trace || []).length} events`}
                        >
                          <div className="space-y-3">
                            {activityMessages.slice(-4).map((item) => (
                              <div
                                key={item.id}
                                className="rounded-2xl border border-line/70 bg-surface px-3 py-3"
                              >
                                <div className="text-[11px] font-bold uppercase tracking-[0.16em] text-muted">
                                  System
                                </div>
                                <p className="mt-2 text-xs leading-5 text-body">
                                  {item.content}
                                </p>
                              </div>
                            ))}
                            {(session?.trace || []).slice(-6).map((item) => (
                              <div
                                key={item.id}
                                className="rounded-2xl border border-line/70 bg-surface px-3 py-3"
                              >
                                <div className="text-[11px] font-bold uppercase tracking-[0.16em] text-muted">
                                  {item.kind}
                                </div>
                                <div className="mt-1 text-xs text-muted">
                                  {formatDate(item.created_at)}
                                </div>
                              </div>
                            ))}
                            {activityMessages.length === 0 &&
                            (session?.trace || []).length === 0 ? (
                              <p className="text-sm text-muted">
                                Trace events will stream here.
                              </p>
                            ) : null}
                          </div>
                        </InspectorSection>
                      </div>
                    </div>
                  </motion.aside>
                )}
              </AnimatePresence>
            </div>
          </div>
        </section>
      </main>

      <div className="pointer-events-none fixed bottom-4 left-1/2 z-40 w-[min(920px,calc(100vw-1.5rem))] -translate-x-1/2">
        <div className="pointer-events-auto mx-auto overflow-hidden rounded-[1.6rem] border border-line/80 bg-surface/92 shadow-[0_20px_60px_rgba(15,23,42,0.18)] backdrop-blur-2xl dark:bg-surface/94">
          {liveTurn ? <LiveToolStrip liveTurn={liveTurn} /> : null}
          <div className="flex items-center justify-between gap-3 border-b border-line/60 px-4 py-3">
            <div>
              <div className="text-[11px] font-bold uppercase tracking-[0.18em] text-muted">
                Composer
              </div>
              <div className="mt-1 text-sm text-body">
                {hasPendingInput
                  ? hasPendingConfirmation
                    ? "Answer the confirmation question to continue this session."
                    : "Answer the clarification questions to continue this session."
                  : effectiveLivePhase === "tooling"
                    ? "Assistant is using tools right now."
                    : effectiveLivePhase === "streaming"
                      ? "Assistant is streaming the response."
                      : effectiveLivePhase
                        ? "Assistant is thinking."
                        : selectedSessionID
                          ? "Press Enter to send, Shift+Enter for a new line."
                          : "Press Enter to start a new session and send."}
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <div className="inline-flex rounded-full border border-line bg-shell p-1">
                {(["ask", "plan", "execute"] as ComposerMode[]).map((mode) => {
                  const selected = composerMode === mode;
                  const disabled =
                    !!session?.turn_in_progress ||
                    (mode === "execute" && !hasCurrentPlan);
                  return (
                    <button
                      key={mode}
                      type="button"
                      disabled={disabled}
                      onClick={() => {
                        setComposerMode(mode);
                        if (mode === "execute" && !hasCurrentPlan) {
                          setMessage(
                            "Create a plan first before using Execute mode.",
                          );
                        } else {
                          setMessage("");
                        }
                      }}
                      className={`rounded-full px-3 py-1 text-[11px] font-bold uppercase tracking-[0.14em] transition ${
                        selected
                          ? "bg-accent text-white shadow-[0_8px_18px_rgba(29,78,216,0.22)]"
                          : "text-muted hover:text-body"
                      } disabled:cursor-not-allowed disabled:opacity-50`}
                    >
                      {mode}
                    </button>
                  );
                })}
              </div>
              <label className="flex items-center gap-2 rounded-full border border-line bg-shell px-3 py-1 text-[11px] font-bold uppercase tracking-[0.14em] text-body">
                <input
                  type="checkbox"
                  checked={mcpOnlyEnabled}
                  onChange={(event) => setMcpOnlyEnabled(event.target.checked)}
                  className="h-3.5 w-3.5 rounded border-line text-accent focus:ring-accent"
                />
                Use Orbyte MCP only
              </label>
              <label className="flex items-center gap-2 rounded-full border border-line bg-shell px-3 py-1 text-[11px] font-bold uppercase tracking-[0.14em] text-body">
                <span>MCP mode</span>
                <select
                  value={mcpExposureMode}
                  onChange={(event) =>
                    setMcpExposureMode(event.target.value as MCPExposureMode)
                  }
                  className="rounded-full border border-line bg-shell px-2 py-0.5 text-[11px] font-bold uppercase tracking-[0.14em] text-body outline-none"
                >
                  <option value="minimal">Minimal</option>
                  <option value="compact">Compact</option>
                  <option value="full">Full</option>
                </select>
              </label>
              {effectiveLivePhase ? (
                <div className="flex items-center gap-2 rounded-full border border-accent/30 bg-accent px-3 py-1 text-[11px] font-bold uppercase tracking-[0.18em] text-white shadow-[0_8px_24px_rgba(29,78,216,0.25)]">
                  <span className="h-2 w-2 animate-pulse rounded-full bg-white" />
                  {effectiveLivePhase === "tooling"
                    ? "Using tools"
                    : effectiveLivePhase === "streaming"
                      ? "Streaming"
                      : "Thinking"}
                </div>
              ) : null}
            </div>
          </div>
          <div className="px-4 py-4">
            <textarea
              ref={textareaRef}
              id="agent_prompt"
              value={prompt}
              onChange={handlePromptInput}
              onInput={handlePromptInput}
              onKeyDown={handlePromptKeyDown}
              className="min-h-[84px] w-full resize-none rounded-2xl border border-line bg-shell px-4 py-3 text-sm text-body outline-none transition focus:border-accent"
              placeholder={
                hasPendingInput
                  ? hasPendingConfirmation
                    ? "Reply yes/no or describe how to proceed."
                    : "Reply with the missing details to continue."
                  : composerMode === "plan"
                    ? "Describe the goal or decision you want the agent to plan."
                    : composerMode === "execute"
                      ? "Describe any execution constraints or keep this aligned to the current plan."
                      : "Ask the agent to investigate, summarize, compare, or operate across Orbyte data."
              }
              name="agent_prompt"
            />
            <div className="mt-3 flex items-center justify-between gap-3">
              <p className="text-xs leading-5 text-muted">
                {hasPendingInput
                  ? hasPendingConfirmation
                    ? "Confirmation is pending. Your next reply will continue the same ACP session without re-entering Plan mode."
                    : "Clarification is pending. Your next reply will resolve it and continue the same ACP session."
                  : composerMode === "plan"
                    ? providerSupportsPlanUpdates
                      ? "Plan mode asks the agent to gather evidence, build a stepwise plan, and stop short of execution."
                      : "Plan mode asks the agent to gather evidence and build a stepwise plan. This provider does not emit structured ACP plan events, so the plan panel is derived from the planning answer."
                    : composerMode === "execute"
                      ? hasCurrentPlan
                        ? "Execute mode tells the agent to act on the current visible plan and report created artifacts."
                        : "Execute mode requires a current plan."
                      : mcpOnlyEnabled
                        ? "Prompts will automatically tell the agent to search skills first, then use MCP tool discovery and exact tool ids before any tools.call execution."
                        : "Markdown responses stream into the transcript as the ACP provider emits chunk updates."}
              </p>
              <button
                type="button"
                className="rounded-2xl bg-accent px-5 py-3 text-sm font-semibold text-white shadow-[0_14px_30px_rgba(29,78,216,0.26)] transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-50"
                disabled={
                  busy ||
                  !!session?.turn_in_progress ||
                  (composerMode !== "execute" && !prompt.trim()) ||
                  (composerMode === "execute" && !hasCurrentPlan)
                }
                onClick={() => void sendPrompt()}
              >
                {selectedSessionID ? "Send" : "Start & Send"}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function sessionPreview(session: ACPSession): string {
  const recentUserMessage = [...(session.messages || [])]
    .reverse()
    .find((item) => item.role === "user" && item.content.trim());
  if (recentUserMessage) {
    return recentUserMessage.content.trim();
  }
  return session.route_path || "No prompts sent yet.";
}

function sessionModelSummary(session: ACPSession): string {
  if (session.current_model) {
    return `${session.current_model} · ${humanizeSessionStatus(session.status)}`;
  }
  if (session.requested_model) {
    return `Requested ${session.requested_model} · ${humanizeSessionStatus(session.status)}`;
  }
  return humanizeSessionStatus(session.status);
}

function humanizeSessionStatus(status: string): string {
  const value = status.trim().toLowerCase();
  if (!value) return "unknown";
  if (value === "awaiting_input") return "awaiting input";
  return value.replace(/_/g, " ");
}

function shortSessionID(value: string): string {
  const trimmed = value.trim();
  if (trimmed.length <= 18) return trimmed;
  return trimmed.slice(-12);
}

function EmptyTranscript() {
  return (
    <div className="grid min-h-[50vh] place-items-center">
      <div className="max-w-xl text-center">
        <div className="text-[11px] font-black uppercase tracking-[0.22em] text-accent-dark">
          Live operator chat
        </div>
        <h3 className="mt-3 text-4xl font-black tracking-tight text-body">
          Ask the agent like a conversation, not a control panel.
        </h3>
        <p className="mt-4 text-sm leading-7 text-muted">
          Send a prompt to start a session automatically. The transcript will
          stream assistant output while approvals and tool activity stay in the
          side rail.
        </p>
      </div>
    </div>
  );
}

function ArtifactGallery({
  artifacts,
  locale,
  dashboardArtifactData,
  compact = false,
}: {
  artifacts: DashboardArtifact[];
  locale: string;
  dashboardArtifactData: Record<string, WidgetDataState>;
  compact?: boolean;
}) {
  if (!artifacts.length) {
    return (
      <div className="grid min-h-[32vh] place-items-center rounded-[1.5rem] border border-dashed border-line/70 bg-surface/70 p-6 text-center">
        <div>
          <div className="text-[11px] font-bold uppercase tracking-[0.2em] text-muted">
            Artifacts
          </div>
          <p className="mt-2 text-sm text-muted">
            Ask the agent for focused dashboard widgets or an explicit board
            preview to render live evidence here.
          </p>
        </div>
      </div>
    );
  }
  return (
    <div className={`space-y-4 ${compact ? "" : "pb-24"}`}>
      {artifacts.map((artifact) => (
        <article
          key={artifact.id}
          className="rounded-[1.5rem] border border-line/70 bg-surface/88 p-4 shadow-[0_10px_28px_rgba(15,23,42,0.06)]"
        >
          {artifact.kind === "dashboard_widget" ? (
            <>
              <div className="mb-3 text-[11px] font-bold uppercase tracking-[0.16em] text-muted">
                Dashboard widget
              </div>
              <DashboardWidgetCard
                widget={artifact.widget}
                locale={locale}
                state={
                  dashboardArtifactData[artifact.widget.definition.data_path] ||
                  defaultWidgetDataState()
                }
              />
            </>
          ) : (
            <>
              <div className="mb-3 flex items-center justify-between gap-3">
                <div className="text-[11px] font-bold uppercase tracking-[0.16em] text-muted">
                  Dashboard board
                </div>
                {artifact.openPath ? (
                  <a
                    href={artifact.openPath}
                    className="rounded-full border border-line bg-shell px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-body transition hover:border-accent/40 hover:text-accent"
                  >
                    Open dashboard
                  </a>
                ) : null}
              </div>
              <div className="grid gap-4 md:grid-cols-2">
                {artifact.widgets.map((widget) => (
                  <DashboardWidgetCard
                    key={`${artifact.id}-${widget.id}`}
                    widget={widget}
                    locale={locale}
                    state={
                      dashboardArtifactData[widget.definition.data_path] ||
                      defaultWidgetDataState()
                    }
                  />
                ))}
              </div>
            </>
          )}
        </article>
      ))}
    </div>
  );
}

function MessageBubble({
  item,
  liveTurn,
  locale,
  draftLinks,
  dashboardArtifactData,
}: {
  item: TranscriptItem;
  liveTurn: LiveTurnState | null;
  locale: string;
  draftLinks?: DraftLink[];
  dashboardArtifactData: Record<string, WidgetDataState>;
}) {
  const isUser = item.role === "user";
  const livePhase =
    stringValue(item.meta?.live_phase) ||
    (stringValue(item.meta?.turn_id) === liveTurn?.turnID
      ? liveTurn?.phase
      : "");
  const isWaitingAssistant = !isUser && item.streaming && !item.content.trim();
  const inlineArtifacts = !isUser
    ? dashboardArtifactsFromMessage(item.content)
    : [];
  const visibleContent = !isUser
    ? stripDashboardArtifactBlocks(item.content).trim()
    : item.content;
  return (
    <div className={`flex ${isUser ? "justify-end" : "justify-start"}`}>
      <article
        className={`max-w-[min(78ch,100%)] rounded-[1.6rem] px-5 py-4 shadow-[0_10px_30px_rgba(15,23,42,0.08)] ${
          isUser
            ? "border border-[#10233d]/30 bg-[#11284a] text-white shadow-[0_16px_34px_rgba(17,40,74,0.26)]"
            : "border border-line/70 bg-white/92 text-body dark:bg-surface"
        }`}
      >
        <div
          className={`mb-2 flex items-center gap-2 text-[11px] font-bold uppercase tracking-[0.18em] ${isUser ? "text-white/72" : "opacity-70"}`}
        >
          <span>{isUser ? "You" : "Assistant"}</span>
          {item.streaming ? (
            <span>
              {isWaitingAssistant
                ? livePhase === "tooling"
                  ? "Using tools"
                  : "Thinking"
                : "Streaming"}
            </span>
          ) : null}
        </div>
        {isUser ? (
          <div className="whitespace-pre-wrap text-sm leading-7">
            {item.content}
          </div>
        ) : isWaitingAssistant ? (
          <div className="flex items-center gap-3 py-2 text-sm text-muted">
            <span className="flex gap-1">
              <span className="h-2 w-2 animate-bounce rounded-full bg-accent [animation-delay:-0.2s]" />
              <span className="h-2 w-2 animate-bounce rounded-full bg-accent [animation-delay:-0.1s]" />
              <span className="h-2 w-2 animate-bounce rounded-full bg-accent" />
            </span>
            <span>
              {livePhase === "tooling"
                ? "Working through tool calls before writing the answer."
                : "Thinking through the request."}
            </span>
          </div>
        ) : (
          <>
            {visibleContent ? (
              <div className="agent-markdown text-sm leading-7">
                <ReactMarkdown
                  remarkPlugins={[remarkGfm]}
                  components={markdownComponents}
                >
                  {visibleContent}
                </ReactMarkdown>
              </div>
            ) : null}
            {inlineArtifacts.length > 0 ? (
              <div className="mt-4 border-t border-line/60 pt-4">
                <ArtifactGallery
                  artifacts={inlineArtifacts}
                  locale={locale}
                  dashboardArtifactData={dashboardArtifactData}
                  compact
                />
              </div>
            ) : null}
            {draftLinks && draftLinks.length > 0 ? (
              <div className="mt-4 flex flex-wrap gap-2 border-t border-line/60 pt-3">
                {draftLinks.map((link) => (
                  <a
                    key={link.key}
                    href={link.openPath}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-2 rounded-full border border-accent/30 bg-accent-soft/70 px-3 py-1.5 text-xs font-bold text-accent-dark transition hover:border-accent hover:bg-accent-soft"
                  >
                    <span>Open draft</span>
                    <span className="opacity-70">
                      {link.title?.trim() || link.documentID || "document"}
                    </span>
                  </a>
                ))}
              </div>
            ) : null}
          </>
        )}
      </article>
    </div>
  );
}

function LiveToolStrip({ liveTurn }: { liveTurn: LiveTurnState }) {
  const activeTool = liveTurn.activeTools[0];
  const activeToolLabel = activeTool
    ? formatLiveToolLabel(activeTool.name)
    : "";
  const activeSummary = activeTool ? formatLiveToolSummary(activeTool) : "";
  const completedCount = liveTurn.recentTools.filter(
    (item) => item.state === "completed",
  ).length;
  return (
    <div className="border-b border-line/60 bg-[linear-gradient(90deg,rgba(29,78,216,0.10)_0%,rgba(14,165,233,0.08)_100%)] px-4 py-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <div className="text-[11px] font-bold uppercase tracking-[0.18em] text-accent-dark">
            Live activity
          </div>
          <div className="mt-1 text-sm text-body">
            {activeTool
              ? `Using ${activeToolLabel}${activeSummary ? ` · ${activeSummary}` : ""}`
              : liveTurn.phase === "streaming"
                ? "Writing the final answer."
                : "Thinking through the request."}
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {activeTool ? (
            <span className="rounded-full border border-accent/30 bg-accent px-3 py-1 text-[11px] font-bold uppercase tracking-[0.16em] text-white shadow-[0_8px_20px_rgba(29,78,216,0.20)]">
              {activeTool.status || "running"}
            </span>
          ) : null}
          {completedCount > 0 ? (
            <span className="rounded-full border border-slate-300 bg-slate-800 px-3 py-1 text-[11px] font-bold uppercase tracking-[0.16em] text-slate-100">
              {completedCount} completed
            </span>
          ) : null}
        </div>
      </div>
    </div>
  );
}

function InspectorSection({
  title,
  kicker,
  summary,
  children,
}: {
  title: string;
  kicker: string;
  summary?: string;
  children: React.ReactNode;
}) {
  return (
    <section className="rounded-[1.5rem] border border-line/80 bg-shell/80 p-4">
      <div className="text-[11px] font-bold uppercase tracking-[0.18em] text-muted">
        {kicker}
      </div>
      <div className="mt-2 flex items-start justify-between gap-3">
        <h3 className="text-base font-black tracking-tight text-body">
          {title}
        </h3>
        {summary ? <div className="text-xs text-muted">{summary}</div> : null}
      </div>
      <div className="mt-4">{children}</div>
    </section>
  );
}

const markdownComponents = {
  h1: (props: React.HTMLAttributes<HTMLHeadingElement>) => (
    <h1
      className="mt-6 text-2xl font-black tracking-tight text-body first:mt-0"
      {...props}
    />
  ),
  h2: (props: React.HTMLAttributes<HTMLHeadingElement>) => (
    <h2
      className="mt-6 text-xl font-black tracking-tight text-body first:mt-0"
      {...props}
    />
  ),
  h3: (props: React.HTMLAttributes<HTMLHeadingElement>) => (
    <h3
      className="mt-5 text-lg font-bold tracking-tight text-body first:mt-0"
      {...props}
    />
  ),
  p: (props: React.HTMLAttributes<HTMLParagraphElement>) => (
    <p
      className="my-3 text-sm leading-7 text-body first:mt-0 last:mb-0"
      {...props}
    />
  ),
  ul: (props: React.HTMLAttributes<HTMLUListElement>) => (
    <ul
      className="my-3 list-disc space-y-2 pl-6 text-sm text-body"
      {...props}
    />
  ),
  ol: (props: React.HTMLAttributes<HTMLOListElement>) => (
    <ol
      className="my-3 list-decimal space-y-2 pl-6 text-sm text-body"
      {...props}
    />
  ),
  li: (props: React.HTMLAttributes<HTMLLIElement>) => (
    <li className="pl-1 leading-7" {...props} />
  ),
  a: (props: React.AnchorHTMLAttributes<HTMLAnchorElement>) => (
    <a
      className="font-semibold text-accent underline decoration-accent/40 underline-offset-4"
      target="_blank"
      rel="noreferrer"
      {...props}
    />
  ),
  blockquote: (props: React.HTMLAttributes<HTMLQuoteElement>) => (
    <blockquote
      className="my-4 border-l-4 border-accent/35 pl-4 text-sm italic text-muted"
      {...props}
    />
  ),
  code: ({
    inline,
    className,
    children,
    ...props
  }: React.HTMLAttributes<HTMLElement> & { inline?: boolean }) =>
    inline ? (
      <code
        className="rounded-md bg-shell px-1.5 py-0.5 font-mono text-[0.92em] text-accent-dark"
        {...props}
      >
        {children}
      </code>
    ) : (
      <code
        className={`block overflow-x-auto rounded-2xl bg-[#0d1727] px-4 py-4 font-mono text-[13px] leading-6 text-[#e5f0ff] ${className || ""}`}
        {...props}
      >
        {children}
      </code>
    ),
  pre: (props: React.HTMLAttributes<HTMLPreElement>) => (
    <pre className="my-4 overflow-hidden rounded-2xl" {...props} />
  ),
  table: (props: React.TableHTMLAttributes<HTMLTableElement>) => (
    <div className="my-4 overflow-x-auto rounded-2xl border border-line/70 bg-shell">
      <table
        className="min-w-full border-collapse text-left text-sm text-body"
        {...props}
      />
    </div>
  ),
  thead: (props: React.HTMLAttributes<HTMLTableSectionElement>) => (
    <thead className="bg-accent-soft/70" {...props} />
  ),
  th: (props: React.ThHTMLAttributes<HTMLTableCellElement>) => (
    <th
      className="px-3 py-2 text-[11px] font-bold uppercase tracking-[0.16em] text-accent-dark"
      {...props}
    />
  ),
  td: (props: React.TdHTMLAttributes<HTMLTableCellElement>) => (
    <td className="border-t border-line/70 px-3 py-2 align-top" {...props} />
  ),
};

function deriveLiveTurnState(session: ACPSession | null): LiveTurnState | null {
  if (!session?.turn_in_progress || !session.current_turn_id) return null;
  const turnID = session.current_turn_id;
  const toolByID = new Map<string, LiveToolCall>();
  for (const item of session.trace || []) {
    if (stringValue(item.payload?.turn_id) !== turnID) continue;
    if (
      item.kind !== "tool_call_started" &&
      item.kind !== "tool_call_updated" &&
      item.kind !== "tool_call_completed"
    ) {
      continue;
    }
    const toolCallID = stringValue(item.payload?.tool_call_id) || item.id;
    const previous = toolByID.get(toolCallID);
    const rawToolName = stringValue(item.payload?.tool_name);
    const normalizedToolName =
      rawToolName && rawToolName.toLowerCase() !== "other"
        ? rawToolName
        : previous?.name || "";
    const next: LiveToolCall = {
      id: toolCallID,
      name: normalizedToolName || "tool",
      status:
        stringValue(item.payload?.status) ||
        (item.kind === "tool_call_completed" ? "completed" : "running"),
      summary: stringValue(item.payload?.summary) || previous?.summary,
      state: item.kind === "tool_call_completed" ? "completed" : "active",
    };
    toolByID.set(toolCallID, next);
  }
  const toolCalls = [...toolByID.values()];
  const activeTools = toolCalls.filter((item) => item.state === "active");
  const recentTools = toolCalls.slice(-3).reverse();
  const assistantTurnMessage = [...(session.messages || [])]
    .reverse()
    .find(
      (item) =>
        item.role === "assistant" &&
        stringValue(item.meta?.turn_id) === turnID &&
        item.content.trim() !== "",
    );
  const hasPendingApproval = (session.approvals || []).some(
    (item) => item.status === "pending",
  );
  let phase: LiveTurnState["phase"] = "thinking";
  if (hasPendingApproval) {
    phase = "approval";
  } else if (activeTools.length > 0) {
    phase = "tooling";
  } else if (assistantTurnMessage) {
    phase = "streaming";
  }
  return { turnID, phase, activeTools, recentTools };
}

function formatLiveToolLabel(raw: string): string {
  let value = raw.trim();
  if (!value) return "tool";
  value = value.replace(/^orbyte-agentproof-\d+_/, "");
  value = value.replace(/^orbyte-agentproof-[^_]+_/, "");
  value = value.replace(/^orbyte[_-]/, "");

  const knownSuffixes: Array<[string, string]> = [
    ["_business_records_search", "Business records search"],
    ["_business_record_search", "Business records search"],
    ["_business_documents_search", "Business documents search"],
    ["_business_document_search", "Business documents search"],
    [".business.record.search", "Business records search"],
    [".business.document.search", "Business documents search"],
  ];
  for (const [suffix, label] of knownSuffixes) {
    if (value.endsWith(suffix)) {
      const prefix = value.slice(0, -suffix.length);
      const domain = humanizeToolToken(prefix);
      return domain ? `${domain} · ${label}` : label;
    }
  }
  return humanizeToolToken(value);
}

function formatLiveToolSummary(tool: LiveToolCall): string {
  const raw = tool.summary?.trim() || "";
  if (!raw) return "";
  if (raw === tool.name) return "";
  if (raw === formatLiveToolLabel(tool.name)) return "";
  if (raw.startsWith("orbyte-agentproof-")) return "";
  return raw;
}

function humanizeToolToken(raw: string): string {
  const value = raw.trim();
  if (!value) return "";
  return value
    .replace(/[._-]+/g, " ")
    .replace(/\s+/g, " ")
    .trim()
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

function optimisticPromptSession(
  current: ACPSession | null,
  prompt: string,
  turnID: string,
  updatedAt: string,
  resetPlan: boolean,
  planSeed?: ACPPlanEntry[],
): ACPSession | null {
  if (!current) return current;
  return {
    ...current,
    status: "running",
    turn_in_progress: true,
    current_turn_id: turnID,
    updated_at: updatedAt,
    current_plan: resetPlan
      ? []
      : planSeed && planSeed.length > 0
        ? planSeed
        : current.current_plan,
    messages: [
      ...(current.messages || []),
      {
        id: `optimistic-user-${turnID}`,
        role: "user",
        content: prompt,
        format: "markdown",
        created_at: updatedAt,
        meta: { turn_id: turnID, optimistic: true },
      },
      {
        id: `optimistic-assistant-${turnID}`,
        role: "assistant",
        content: "",
        format: "markdown",
        created_at: updatedAt,
        meta: { turn_id: turnID, optimistic: true, placeholder: true },
      },
    ],
  };
}

function markPromptFailure(
  current: ACPSession | null,
  turnID: string,
  error: unknown,
): ACPSession | null {
  if (!current) return current;
  return {
    ...current,
    status: "error",
    turn_in_progress: false,
    current_turn_id: "",
    messages: (current.messages || []).filter(
      (item) =>
        stringValue(item.meta?.turn_id) !== turnID || item.role === "user",
    ),
    trace: [
      ...(current.trace || []),
      {
        id: `client-failed-${turnID}`,
        kind: "turn_failed",
        created_at: new Date().toISOString(),
        payload: {
          turn_id: turnID,
          error:
            error instanceof Error ? error.message : "Failed to send prompt.",
        },
      },
    ],
  };
}

async function refreshSession(
  sessionID: string,
  setSession: (session: ACPSession | null) => void,
  setSessions: (updater: (current: ACPSession[]) => ACPSession[]) => void,
): Promise<ACPSession> {
  const current = await fetchJson<ACPSession>(
    `/agent/api/sessions/${encodeURIComponent(sessionID)}`,
  );
  setSession(current);
  setSessions((items) => mergeSessionIntoList(items, current));
  return current;
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
  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

async function callMcp<T>(
  method: string,
  params?: Record<string, unknown>,
): Promise<T> {
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

function parseACPStreamEvent(raw: string): ACPEvent | null {
  try {
    return JSON.parse(raw) as ACPEvent;
  } catch {
    return null;
  }
}

function applyACPStreamEvent(
  current: ACPSession | null,
  event: ACPEvent,
): ACPSession | null {
  if (!current) return current;
  const turnID = stringValue(event.payload?.turn_id);
  const next: ACPSession = {
    ...current,
    trace: dedupeTrace([...(current.trace || []), event]),
  };
  switch (event.kind) {
    case "turn_started":
      next.turn_in_progress = true;
      next.status = "running";
      next.current_turn_id = turnID || current.current_turn_id;
      next.messages = bindTurnToOptimisticMessages(
        next.messages || [],
        next.current_turn_id || "",
      );
      break;
    case "user_message": {
      next.turn_in_progress = true;
      next.current_turn_id = turnID || current.current_turn_id;
      next.status = "running";
      break;
    }
    case "session_update": {
      const updateKind = stringValue(event.payload?.update_kind);
      const content = mapValue(event.payload?.content);
      const text = stringValue(content.text);
      next.turn_in_progress = true;
      next.current_turn_id = turnID || current.current_turn_id;
      next.status = "running";
      if (updateKind === "agent_message_chunk") {
        next.messages = appendChunkMessage(next.messages || [], {
          id: `stream-${event.id}`,
          role: "assistant",
          content: text,
          format: "markdown",
          created_at: event.created_at,
          meta: {
            ...content,
            ...(next.current_turn_id ? { turn_id: next.current_turn_id } : {}),
          },
        });
      } else if (updateKind === "plan" && text) {
        next.current_plan = [...(next.current_plan || []), { content: text }];
      } else if (updateKind === "artifact") {
        const artifact = artifactFromEventContent(content);
        if (artifact) {
          next.artifacts = mergeArtifacts(next.artifacts || [], artifact);
        }
      } else if (text) {
        next.messages = appendChunkMessage(next.messages || [], {
          id: `stream-${event.id}`,
          role: "system",
          content: text,
          format: "markdown",
          created_at: event.created_at,
          meta: {
            ...content,
            ...(next.current_turn_id ? { turn_id: next.current_turn_id } : {}),
          },
        });
      }
      break;
    }
    case "turn_completed":
      next.turn_in_progress = false;
      next.current_turn_id = "";
      next.status = next.status === "awaiting_input" ? next.status : "ready";
      if (
        turnID &&
        isExecuteTurn(next.trace || [], turnID) &&
        next.current_plan?.length
      ) {
        next.current_plan = markPlanEntriesExecuted(next.current_plan);
      }
      break;
    case "turn_failed":
      next.turn_in_progress = false;
      next.current_turn_id = "";
      next.status = "error";
      break;
    case "clarification_requested": {
      next.turn_in_progress = false;
      next.current_turn_id = "";
      next.status = "awaiting_input";
      next.awaiting_input_kind =
        stringValue(event.payload?.awaiting_input_kind) || "clarification";
      next.pending_question_set_id =
        stringValue(event.payload?.question_set_id) ||
        next.pending_question_set_id;
      next.pending_questions = clarificationQuestionsFromPayload(
        event.payload?.questions,
      );
      break;
    }
    case "clarification_resolved":
      next.pending_questions = [];
      next.pending_question_set_id = "";
      next.awaiting_input_kind = "";
      if (!next.turn_in_progress) {
        next.status = "running";
      }
      break;
    default:
      break;
  }
  return next;
}

function clarificationQuestionsFromPayload(
  value: unknown,
): ACPClarificationQuestion[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .map((item) => mapValue(item))
    .filter((item): item is Record<string, unknown> => item !== null)
    .map((item, index) => ({
      id: stringValue(item.id) || `clarification-${index}`,
      content: stringValue(item.content),
      source_message_id: stringValue(item.source_message_id) || undefined,
    }))
    .filter((item) => item.content);
}

function appendChunkMessage(
  messages: ACPMessage[],
  incoming: ACPMessage,
): ACPMessage[] {
  const next = [...messages];
  const last = next[next.length - 1];
  const incomingTurnID = stringValue(incoming.meta?.turn_id);
  if (incomingTurnID) {
    for (let index = next.length - 1; index >= 0; index -= 1) {
      const item = next[index];
      if (!item || item.role !== incoming.role) continue;
      if (stringValue(item.meta?.turn_id) !== incomingTurnID) continue;
      if (incoming.role === "user") {
        if (item.content.trim() === incoming.content.trim()) {
          return next;
        }
        next[index] = {
          ...item,
          content: `${item.content}${incoming.content}`,
          created_at: incoming.created_at || item.created_at,
          meta: incoming.meta || item.meta,
        };
        return next;
      }
      if (incoming.role === "assistant") {
        next[index] = {
          ...item,
          id: item.id.startsWith("optimistic-assistant-")
            ? incoming.id
            : item.id,
          content: `${item.content}${incoming.content}`,
          created_at: incoming.created_at || item.created_at,
          meta: {
            ...(item.meta || {}),
            ...(incoming.meta || {}),
            placeholder: false,
          },
        };
        return next;
      }
    }
  }
  if (
    incoming.role === "assistant" &&
    last?.role === "assistant" &&
    stringValue(last.meta?.turn_id) === incomingTurnID &&
    last.meta?.placeholder
  ) {
    next[next.length - 1] = {
      ...last,
      id: incoming.id,
      content: `${last.content}${incoming.content}`,
      created_at: incoming.created_at || last.created_at,
      meta: {
        ...(last.meta || {}),
        ...(incoming.meta || {}),
        placeholder: false,
      },
    };
    return next;
  }
  if (!incoming.content) return messages;
  if (
    last &&
    last.role === incoming.role &&
    last.id.startsWith("stream-") &&
    incoming.id.startsWith("stream-") &&
    stringValue(last.meta?.turn_id) === incomingTurnID
  ) {
    next[next.length - 1] = {
      ...last,
      content: `${last.content}${incoming.content}`,
      created_at: incoming.created_at || last.created_at,
      meta: incoming.meta || last.meta,
    };
    return next;
  }
  if (
    incoming.role === "user" &&
    next.some(
      (item) =>
        item.role === "user" &&
        item.content.trim() === incoming.content.trim() &&
        stringValue(item.meta?.turn_id) === incomingTurnID,
    )
  ) {
    return next;
  }
  return [...next, incoming];
}

function bindTurnToOptimisticMessages(
  messages: ACPMessage[],
  turnID: string,
): ACPMessage[] {
  if (!turnID || messages.length === 0) return messages;
  const next = [...messages];
  for (let index = next.length - 1; index >= 0; index -= 1) {
    const item = next[index];
    if (!item) continue;
    if (!item.meta?.optimistic) break;
    next[index] = {
      ...item,
      meta: { ...(item.meta || {}), turn_id: turnID },
    };
    if (item.role === "user") {
      continue;
    }
  }
  return next;
}

function dedupeTrace(trace: ACPEvent[]): ACPEvent[] {
  const seen = new Set<string>();
  return trace.filter((item) => {
    if (!item.id || seen.has(item.id)) return false;
    seen.add(item.id);
    return true;
  });
}

function artifactFromEventContent(
  content: Record<string, unknown>,
): ACPArtifact | null {
  const kind =
    stringValue(content.kind) || stringValue(mapValue(content.metadata).kind);
  if (!kind) return null;
  return {
    id: stringValue(content.id) || `artifact-${kind}`,
    kind,
    title: stringValue(content.title) || "Artifact",
    content_type: stringValue(content.content_type) || undefined,
    content: stringValue(content.content) || undefined,
    created_at: new Date().toISOString(),
    metadata: mapValue(content.metadata),
  };
}

function mergeArtifacts(
  items: ACPArtifact[],
  artifact: ACPArtifact,
): ACPArtifact[] {
  if (!artifact.id) {
    return [...items, artifact];
  }
  let replaced = false;
  const next = items.map((item) => {
    if (item.id !== artifact.id) return item;
    replaced = true;
    return artifact;
  });
  return replaced ? next : [...next, artifact];
}

function draftLinksForTurn(
  trace: ACPEvent[] | undefined,
  turnID: string,
): DraftLink[] {
  if (!trace || !turnID) return [];
  const links = new Map<string, DraftLink>();
  for (const item of trace) {
    if (stringValue(item.payload?.turn_id) !== turnID) continue;
    for (const link of extractDraftLinks(item.payload)) {
      if (!links.has(link.key)) {
        links.set(link.key, link);
      }
    }
  }
  return [...links.values()];
}

function extractDraftLinks(value: unknown, depth = 0): DraftLink[] {
  if (depth > 6 || value == null) return [];
  if (Array.isArray(value)) {
    return value.flatMap((item) => extractDraftLinks(item, depth + 1));
  }
  if (typeof value !== "object") return [];
  const record = value as Record<string, unknown>;
  const directPath = stringValue(record.open_path);
  const links: DraftLink[] = [];
  if (directPath) {
    const documentID =
      stringValue(record.document_id) ||
      stringValue(
        (record.record as { header?: { id?: string } } | undefined)?.header?.id,
      );
    const title =
      stringValue(record.title) ||
      stringValue(
        (
          record.record as
            | {
                body?: { payload?: Record<string, unknown> };
              }
            | undefined
        )?.body?.payload?.title,
      );
    links.push({
      key: `${directPath}|${documentID}|${title}`,
      openPath: directPath,
      title,
      documentID,
    });
  }
  for (const nested of Object.values(record)) {
    links.push(...extractDraftLinks(nested, depth + 1));
  }
  return links;
}

function closeStream(ref: React.MutableRefObject<EventSource | null>) {
  if (ref.current) {
    ref.current.close();
    ref.current = null;
  }
}

function mergeSessionIntoList(
  items: ACPSession[],
  session: ACPSession,
): ACPSession[] {
  let replaced = false;
  const next = items.map((item) => {
    if (item.id !== session.id) return item;
    replaced = true;
    return session;
  });
  return replaced ? next : [session, ...next];
}

function orderSessions(items: ACPSession[]): ACPSession[] {
  return [...items].sort((left, right) => {
    const updatedDelta =
      parseDateValue(right.updated_at) - parseDateValue(left.updated_at);
    if (updatedDelta !== 0) return updatedDelta;
    const createdDelta =
      parseDateValue(right.created_at) - parseDateValue(left.created_at);
    if (createdDelta !== 0) return createdDelta;
    return left.id.localeCompare(right.id);
  });
}

function parseDateValue(value?: string): number {
  if (!value) return 0;
  const parsed = new Date(value).getTime();
  return Number.isNaN(parsed) ? 0 : parsed;
}

function buildPromptPayload(
  prompt: string,
  mcpOnlyEnabled: boolean,
  mode: ComposerMode,
  currentPlan: ACPPlanEntry[],
): string {
  const sections: string[] = [];
  if (mcpOnlyEnabled) {
    sections.push(MCP_ONLY_PREFIX);
    sections.push(
      "Treat the visible MCP tool list in the UI as the minimal MCP surface only. Search skills first when the request matches a known workflow. If multiple skills look relevant, load them together with one bulk skills.describe call. If no skill fits, use tool discovery and one bulk tools.describe call before any tools.call execution.",
    );
  }
  if (mode === "plan") {
    sections.push(
      [
        "Planning mode is active.",
        "Gather evidence from Orbyte MCP tools before proposing actions.",
        "Produce a concise stepwise plan and emit ACP plan updates for the current turn.",
        "Do not execute the plan or create records unless the user explicitly asks to execute later.",
        "When the request is about warehouse replenishment or inventory planning, call planning_core.replenishment.insight.summary first and then planning_core.replenishment.plan.summary before answering.",
        "If those tools disagree, report the discrepancy explicitly instead of flattening all items into a healthy/no-action conclusion.",
      ].join(" "),
    );
  }
  if (mode === "execute") {
    sections.push(
      [
        "Execute mode is active.",
        "Treat the current ACP plan in this session as the execution target.",
        "Execute the plan stepwise using Orbyte MCP tools, report created artifacts and deep links, and avoid re-planning unless a blocker forces it.",
      ].join(" "),
    );
    if (currentPlan.length > 0) {
      sections.push(
        `Current ACP plan:\n${currentPlan
          .map((item, index) => `${index + 1}. ${item.content}`)
          .join("\n")}`,
      );
    }
  }
  if (looksLikeCRMPrompt(prompt)) {
    sections.push(
      [
        "CRM requests are workflow-like business tasks, so the required first step in minimal mode is skills.search or skills.list.",
        "If more than one CRM skill looks relevant, load them together with one bulk skills.describe call before choosing the workflow.",
        "Do not call CRM business tools from memory before discovery.",
        "After you load the chosen CRM skill, follow its recommended workflow and use exact discovered tool ids only.",
      ].join(" "),
    );
  }
  if (looksLikeDashboardPrompt(prompt)) {
    const explicitFullBoard = looksLikeFullDashboardPrompt(prompt);
    sections.push(
      [
        "Dashboard requests are workflow-like business tasks, so search skills first if a dashboard insight workflow exists for the current request.",
        "If multiple dashboard skills look relevant, load them together with one bulk skills.describe call before choosing the workflow.",
        "If the dashboard tool family is still not obvious, use tools.search and one bulk tools.describe call before calling tools.call.",
        explicitFullBoard
          ? "For explicit full-dashboard or board-preview requests, prefer the discovered workflow that returns a dashboard_board artifact."
          : "For focused insight responses, prefer the discovered workflow that returns a small set of relevant dashboard_widget artifacts rather than a full board.",
        explicitFullBoard
          ? "Only save a dashboard when the user explicitly asks to save one, and then report the saved board link."
          : "Only request explicit widget keys when the user asks for specific widgets or renderers; otherwise let the discovered workflow infer them.",
        explicitFullBoard
          ? "Do not replace an explicit board request with standalone widget artifacts."
          : "For an insight answer, prefer a balanced mix of KPI, comparison, and trend evidence unless the user asks for a different shape.",
        "When dashboard tools return structured artifact metadata, rely on that artifact output instead of rewriting compatibility blocks manually.",
        explicitFullBoard
          ? "Mention the dashboard surface or open link for deeper exploration."
          : "If the user wants the full dashboard, direct them to the dashboard surface instead of embedding a full board inline in chat.",
      ].join(" "),
    );
  }
  sections.push(prompt);
  return sections.join("\n\n");
}

function deriveCurrentPlan(session: ACPSession | null): ACPPlanEntry[] {
  if (!session) return [];
  const trace = session.trace || [];
  let latestPlanTurnID = "";
  for (const item of trace) {
    if (
      item.kind === "session_update" &&
      stringValue(item.payload?.update_kind) === "plan"
    ) {
      const turnID = stringValue(item.payload?.turn_id);
      if (turnID) {
        latestPlanTurnID = turnID;
      }
    }
  }
  if (latestPlanTurnID) {
    const planEntries = trace
      .filter(
        (item) =>
          item.kind === "session_update" &&
          stringValue(item.payload?.update_kind) === "plan" &&
          stringValue(item.payload?.turn_id) === latestPlanTurnID,
      )
      .map((item) => {
        const content = mapValue(item.payload?.content);
        return {
          content: stringValue(content.text),
          priority: stringValue(content.priority) || undefined,
          status: stringValue(content.status) || undefined,
        };
      })
      .filter((item) => item.content);
    if (planEntries.length > 0) {
      return planEntries;
    }
  }
  const fallbackPlan = deriveFallbackPlanFromPlanTurn(session);
  if (fallbackPlan.entries.length > 0) {
    const mergedPlan = mergeDerivedPlanStatus(
      fallbackPlan.entries,
      session.current_plan || [],
    );
    const planTurnID = latestPlanTurnID || fallbackPlan.turnID;
    return hasCompletedExecuteTurnSincePlan(trace, planTurnID)
      ? markPlanEntriesExecuted(mergedPlan)
      : mergedPlan;
  }
  return session.current_plan || [];
}

function looksLikeCRMPrompt(prompt: string): boolean {
  const lower = prompt.trim().toLowerCase();
  if (!lower) return false;
  return [
    "crm",
    "customer 360",
    "customer health",
    "ticket backlog",
    "service backlog",
    "pipeline",
    "opportunity",
  ].some((token) => lower.includes(token));
}

function looksLikeDashboardPrompt(prompt: string): boolean {
  const normalized = prompt.trim().toLowerCase();
  if (!normalized) return false;
  if (normalized.includes("dashboard")) {
    return true;
  }
  const explicitPhrases = [
    "show me dashboard widgets",
    "show me a dashboard widget",
    "render dashboard widgets",
    "render a dashboard widget",
    "preview dashboard",
    "create dashboard",
    "save dashboard",
    "open dashboard",
    "live dashboard",
    "dashboard artifact",
  ];
  return explicitPhrases.some((phrase) => normalized.includes(phrase));
}

function looksLikeFullDashboardPrompt(prompt: string): boolean {
  const normalized = prompt.trim().toLowerCase();
  if (!normalized) return false;
  const explicitBoardPhrases = [
    "full dashboard",
    "full board",
    "board preview",
    "preview board",
    "dashboard board",
    "save board",
    "save dashboard",
    "create board",
    "create dashboard board",
  ];
  return explicitBoardPhrases.some((phrase) => normalized.includes(phrase));
}

function deriveDashboardArtifacts(
  session: ACPSession | null,
): DashboardArtifact[] {
  const items = session?.artifacts || [];
  const artifacts: Array<DashboardArtifact> = [];
  for (const item of items) {
    const metadata = item.metadata || {};
    if (item.kind === "dashboard_widget") {
      const widget = asResolvedWidget(metadata.widget);
      if (!widget) {
        continue;
      }
      artifacts.push({
        id: item.id,
        kind: "dashboard_widget",
        title: item.title || widget.title,
        widget,
      });
      continue;
    }
    if (item.kind === "dashboard_board") {
      const widgets = asResolvedWidgets(metadata.widgets);
      if (!widgets.length) {
        continue;
      }
      artifacts.push({
        id: item.id,
        kind: "dashboard_board",
        title: item.title || stringValue(metadata.title) || "Dashboard board",
        openPath: stringValue(metadata.open_path) || undefined,
        boardID: stringValue(metadata.board_id) || undefined,
        widgets,
      });
    }
  }
  return artifacts;
}

const DASHBOARD_ARTIFACT_BLOCK_PATTERN =
  /<orbyte-dashboard-artifact>([\s\S]*?)<\/orbyte-dashboard-artifact>/gi;

function stripDashboardArtifactBlocks(content: string): string {
  return content
    .replace(DASHBOARD_ARTIFACT_BLOCK_PATTERN, "")
    .replace(/\n{3,}/g, "\n\n");
}

function dashboardArtifactsFromMessage(content: string): DashboardArtifact[] {
  const artifacts: Array<DashboardArtifact> = [];
  for (const match of content.matchAll(DASHBOARD_ARTIFACT_BLOCK_PATTERN)) {
    const payload = match[1]?.trim();
    if (!payload) continue;
    try {
      const parsed = JSON.parse(payload) as Record<string, unknown>;
      const kind = stringValue(parsed.kind);
      const metadata =
        parsed.metadata && typeof parsed.metadata === "object"
          ? (parsed.metadata as Record<string, unknown>)
          : {};
      if (kind === "dashboard_widget") {
        const widget = asResolvedWidget(metadata.widget);
        if (!widget) continue;
        artifacts.push({
          id: stringValue(parsed.id) || `dashboard-widget-${artifacts.length}`,
          kind: "dashboard_widget",
          title: stringValue(parsed.title) || widget.title,
          widget,
        });
        continue;
      }
      if (kind === "dashboard_board") {
        const widgets = asResolvedWidgets(metadata.widgets);
        if (!widgets.length) continue;
        artifacts.push({
          id: stringValue(parsed.id) || `dashboard-board-${artifacts.length}`,
          kind: "dashboard_board",
          title:
            stringValue(parsed.title) ||
            stringValue(metadata.title) ||
            "Dashboard board",
          openPath: stringValue(metadata.open_path) || undefined,
          boardID: stringValue(metadata.board_id) || undefined,
          widgets,
        });
      }
    } catch {
      continue;
    }
  }
  return artifacts;
}

function flattenArtifactWidgets(
  artifacts: DashboardArtifact[],
): DashboardResolvedWidget[] {
  const widgets: DashboardResolvedWidget[] = [];
  for (const artifact of artifacts) {
    if (artifact.kind === "dashboard_widget") {
      widgets.push(artifact.widget);
      continue;
    }
    widgets.push(...artifact.widgets);
  }
  return widgets;
}

function asResolvedWidget(value: unknown): DashboardResolvedWidget | null {
  if (!value || typeof value !== "object") {
    return null;
  }
  const record = value as Record<string, unknown>;
  const definitionValue = record.definition;
  if (!definitionValue || typeof definitionValue !== "object") {
    return null;
  }
  return {
    id:
      stringValue(record.id) ||
      `artifact-widget-${Math.random().toString(36).slice(2)}`,
    title: stringValue(record.title) || "Dashboard widget",
    kind:
      stringValue(record.kind) ||
      stringValue((definitionValue as Record<string, unknown>).renderer_kind) ||
      "metric",
    width: numberValue(record.width, 4),
    height: numberValue(record.height, 1),
    refresh_override: stringValue(record.refresh_override) || undefined,
    definition: definitionValue as DashboardWidgetDefinition,
  };
}

function asResolvedWidgets(value: unknown): DashboardResolvedWidget[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .map((item) => asResolvedWidget(item))
    .filter((item): item is DashboardResolvedWidget => item !== null);
}

function isExecuteTurn(trace: ACPEvent[], turnID: string): boolean {
  return trace.some((item) => {
    if (item.kind !== "user_message") return false;
    if (stringValue(item.payload?.turn_id) !== turnID) return false;
    const content = stringValue(item.payload?.content);
    return content.includes("Execute mode is active.");
  });
}

function markPlanEntriesExecuted(entries: ACPPlanEntry[]): ACPPlanEntry[] {
  return entries.map((item) => ({
    ...item,
    status: item.status || "executed",
  }));
}

function hasCompletedExecuteTurnSincePlan(
  trace: ACPEvent[],
  latestPlanTurnID: string,
): boolean {
  if (!latestPlanTurnID) return false;
  let planCompletedAt = "";
  for (const item of trace) {
    if (
      item.kind === "turn_completed" &&
      stringValue(item.payload?.turn_id) === latestPlanTurnID
    ) {
      planCompletedAt = item.created_at || "";
    }
  }
  if (!planCompletedAt) return false;
  const executeTurnIDs = new Set<string>();
  for (const item of trace) {
    if (item.kind !== "user_message") continue;
    const content = stringValue(item.payload?.content);
    if (!content.includes("Execute mode is active.")) continue;
    const createdAt = item.created_at || "";
    if (createdAt < planCompletedAt) continue;
    const turnID = stringValue(item.payload?.turn_id);
    if (turnID) {
      executeTurnIDs.add(turnID);
    }
  }
  if (executeTurnIDs.size === 0) return false;
  return trace.some((item) => {
    if (item.kind !== "turn_completed") return false;
    const turnID = stringValue(item.payload?.turn_id);
    if (!executeTurnIDs.has(turnID)) return false;
    const createdAt = item.created_at || "";
    return createdAt >= planCompletedAt;
  });
}

function mergeDerivedPlanStatus(
  derivedEntries: ACPPlanEntry[],
  storedEntries: ACPPlanEntry[],
): ACPPlanEntry[] {
  if (storedEntries.length === 0) return derivedEntries;
  return derivedEntries.map((item, index) => {
    const matchingStored =
      storedEntries.find(
        (stored) =>
          stored.content.trim().toLowerCase() ===
          item.content.trim().toLowerCase(),
      ) || storedEntries[index];
    if (!matchingStored) return item;
    return {
      ...item,
      priority: item.priority || matchingStored.priority,
      status: item.status || matchingStored.status,
    };
  });
}

function deriveFallbackPlanFromPlanTurn(session: ACPSession): {
  turnID: string;
  entries: ACPPlanEntry[];
} {
  const messages = session.messages || [];
  const trace = session.trace || [];
  let latestPlanTurnID = "";
  for (const item of trace) {
    if (item.kind !== "user_message") continue;
    const content = stringValue(item.payload?.content);
    if (!content.includes("Planning mode is active.")) continue;
    const turnID = stringValue(item.payload?.turn_id);
    if (turnID) {
      latestPlanTurnID = turnID;
    }
  }
  if (!latestPlanTurnID) {
    return { turnID: "", entries: [] };
  }
  const assistantMessage = [...messages]
    .reverse()
    .find(
      (item) =>
        item.role === "assistant" &&
        stringValue(item.meta?.turn_id) === latestPlanTurnID &&
        item.content.trim() !== "",
    );
  if (!assistantMessage) {
    return { turnID: latestPlanTurnID, entries: [] };
  }
  return {
    turnID: latestPlanTurnID,
    entries: synthesizePlanEntries(assistantMessage.content),
  };
}

function synthesizePlanEntries(markdown: string): ACPPlanEntry[] {
  const entries: ACPPlanEntry[] = [];
  const seen = new Set<string>();
  const pushEntry = (value: string) => {
    const normalized = normalizePlanLine(value);
    if (!normalized) return;
    const key = normalized.toLowerCase();
    if (seen.has(key)) return;
    seen.add(key);
    entries.push({ content: normalized });
  };

  const lines = markdown.split(/\r?\n/);
  for (const rawLine of lines) {
    const line = rawLine.trim();
    if (!line) continue;
    if (/^\|/.test(line)) continue;
    if (/^[-|:\s]+$/.test(line)) continue;
    const bulletMatch = line.match(/^(\d+\.\s+|[-*]\s+)(.+)$/);
    if (bulletMatch) {
      pushEntry(bulletMatch[2] || "");
      continue;
    }
    const planMatch = line.match(/^\*\*?Plan:?\*?\*?\s*(.+)$/i);
    if (planMatch) {
      pushEntry(planMatch[1] || "");
      continue;
    }
    if (/^(Step|Action|Next)\b/i.test(line)) {
      pushEntry(line.replace(/^\*\*|\*\*$/g, ""));
      continue;
    }
  }

  if (entries.length > 0) {
    return entries;
  }

  const paragraphs = markdown
    .split(/\n\s*\n/)
    .map((item) => normalizePlanLine(item))
    .filter(Boolean);
  if (paragraphs.length > 0) {
    return paragraphs.slice(0, 3).map((content) => ({ content }));
  }
  return [];
}

function normalizePlanLine(value: string): string {
  return value
    .replace(/[*_`#>]/g, "")
    .replace(/\[(.*?)\]\((.*?)\)/g, "$1")
    .replace(/\s+/g, " ")
    .trim();
}

function sortMcpTools(items: MCPTool[]): MCPTool[] {
  return [...items].sort((left, right) =>
    String(left.title || left.name).localeCompare(
      String(right.title || right.name),
    ),
  );
}

function sortMcpToolSummaries(items: MCPToolSummary[]): MCPToolSummary[] {
  return [...items].sort((left, right) =>
    String(left.title || left.name || left.tool_id).localeCompare(
      String(right.title || right.name || right.tool_id),
    ),
  );
}

function sortPlaybooks(items: MCPPlaybookSummary[]): MCPPlaybookSummary[] {
  return [...items].sort((left, right) =>
    String(left.name || left.id).localeCompare(String(right.name || right.id)),
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

function stringValue(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function numberValue(value: unknown, fallback = 0): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function mapValue(value: unknown): Record<string, unknown> {
  return value && typeof value === "object"
    ? (value as Record<string, unknown>)
    : {};
}
