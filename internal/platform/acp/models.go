package acp

import "time"

type Provider struct {
	Key         string            `json:"key"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Cwd         string            `json:"cwd,omitempty"`
	Transport   string            `json:"transport,omitempty"`
}

type ProviderInfo struct {
	Key               string         `json:"key"`
	Name              string         `json:"name"`
	Description       string         `json:"description,omitempty"`
	Available         bool           `json:"available"`
	ContractVersion   string         `json:"contract_version,omitempty"`
	Stability         string         `json:"stability,omitempty"`
	ProtocolVersion   int            `json:"protocol_version,omitempty"`
	AgentInfo         map[string]any `json:"agent_info,omitempty"`
	AgentCapabilities map[string]any `json:"agent_capabilities,omitempty"`
	SupportsApprovals bool           `json:"supports_approvals,omitempty"`
	SupportsStreaming bool           `json:"supports_streaming,omitempty"`
	SessionLifecycle  []string       `json:"session_lifecycle,omitempty"`
	Error             string         `json:"error,omitempty"`
}

type ContextBlock struct {
	Key      string         `json:"key"`
	Label    string         `json:"label"`
	Kind     string         `json:"kind"`
	Selected bool           `json:"selected"`
	Value    map[string]any `json:"value,omitempty"`
}

type Session struct {
	ID             string         `json:"id"`
	ProviderKey    string         `json:"provider_key"`
	ProviderName   string         `json:"provider_name"`
	UserID         string         `json:"user_id"`
	Shell          string         `json:"shell"`
	RoutePath      string         `json:"route_path,omitempty"`
	Title          string         `json:"title,omitempty"`
	Status         string         `json:"status"`
	WorkingDir     string         `json:"working_dir,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	Messages       []Message      `json:"messages,omitempty"`
	ContextBlocks  []ContextBlock `json:"context_blocks,omitempty"`
	Approvals      []Approval     `json:"approvals,omitempty"`
	Artifacts      []Artifact     `json:"artifacts,omitempty"`
	Trace          []Event        `json:"trace,omitempty"`
	CurrentPlan    []PlanEntry    `json:"current_plan,omitempty"`
	ProviderInfo   map[string]any `json:"provider_info,omitempty"`
	LastError      string         `json:"last_error,omitempty"`
	RemoteSession  string         `json:"remote_session_id,omitempty"`
	TurnInProgress bool           `json:"turn_in_progress"`
}

type Message struct {
	ID        string         `json:"id"`
	Role      string         `json:"role"`
	Format    string         `json:"format,omitempty"`
	Content   string         `json:"content"`
	CreatedAt time.Time      `json:"created_at"`
	Meta      map[string]any `json:"meta,omitempty"`
}

type Approval struct {
	ID          string         `json:"id"`
	Status      string         `json:"status"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	Method      string         `json:"method,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	ResolvedAt  time.Time      `json:"resolved_at,omitempty"`
}

type Artifact struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	Title       string         `json:"title"`
	ContentType string         `json:"content_type,omitempty"`
	Content     string         `json:"content,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type Event struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	SessionID string         `json:"session_id"`
	CreatedAt time.Time      `json:"created_at"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type PlanEntry struct {
	Content  string `json:"content"`
	Priority string `json:"priority,omitempty"`
	Status   string `json:"status,omitempty"`
}

type StartSessionRequest struct {
	ProviderKey   string         `json:"provider_key"`
	UserID        string         `json:"user_id"`
	Shell         string         `json:"shell"`
	RoutePath     string         `json:"route_path,omitempty"`
	Title         string         `json:"title,omitempty"`
	WorkingDir    string         `json:"working_dir,omitempty"`
	ContextBlocks []ContextBlock `json:"context_blocks,omitempty"`
}

type PromptRequest struct {
	Content       string         `json:"content"`
	ContextBlocks []ContextBlock `json:"context_blocks,omitempty"`
}
