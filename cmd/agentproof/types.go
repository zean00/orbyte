package main

import "time"

type scenarioManifest struct {
	Version          string                    `json:"version"`
	Scenario         string                    `json:"scenario"`
	DomainBundle     string                    `json:"domain_bundle"`
	RunID            string                    `json:"run_id"`
	GeneratedAt      time.Time                 `json:"generated_at"`
	BaseURL          string                    `json:"base_url"`
	WorkingDir       string                    `json:"working_dir,omitempty"`
	AgentRoutePath   string                    `json:"agent_route_path"`
	SessionTitleHint string                    `json:"session_title_hint"`
	ServicePrincipal servicePrincipalOutput    `json:"service_principal"`
	OpencodeConfig   opencodeConfigOutput      `json:"opencode_config"`
	Entities         map[string]map[string]any `json:"entities,omitempty"`
	Documents        map[string]documentFacts  `json:"documents,omitempty"`
	GroundTruth      map[string]any            `json:"ground_truth,omitempty"`
	SessionInstructions string                   `json:"session_instructions,omitempty"`
	PromptPack           []promptExpectation     `json:"prompt_pack"`
}

type servicePrincipalOutput struct {
	ID    string `json:"id"`
	Key   string `json:"key"`
	Token string `json:"token,omitempty"`
}

type opencodeConfigOutput struct {
	ServerName string         `json:"server_name"`
	URL        string         `json:"url"`
	Bearer     string         `json:"bearer_token,omitempty"`
	Snippet    map[string]any `json:"snippet"`
	Provider   map[string]any `json:"provider"`
}

type documentFacts struct {
	ID      string         `json:"id"`
	Number  string         `json:"number"`
	Status  string         `json:"status"`
	Payload map[string]any `json:"payload"`
}

type promptExpectation struct {
	ID               string         `json:"id"`
	Prompt           string         `json:"prompt"`
	RequiredFacts    []requiredFact `json:"required_facts"`
	AllowedVariants  []string       `json:"allowed_variants,omitempty"`
	ForbiddenPhrases []string       `json:"forbidden_phrases,omitempty"`
}

type requiredFact struct {
	Key      string   `json:"key"`
	Severity string   `json:"severity"`
	Checks   []string `json:"checks"`
}

type sessionTranscript struct {
	ID       string              `json:"id"`
	Title    string              `json:"title"`
	Messages []sessionMessage    `json:"messages"`
	Trace    []sessionTraceEvent `json:"trace"`
}

type sessionMessage struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

type sessionTraceEvent struct {
	ID      string         `json:"id"`
	Kind    string         `json:"kind"`
	Payload map[string]any `json:"payload"`
}

type analysisReport struct {
	Version      string                 `json:"version"`
	Scenario     string                 `json:"scenario"`
	RunID        string                 `json:"run_id"`
	BaseURL      string                 `json:"base_url"`
	SessionID    string                 `json:"session_id"`
	SessionTitle string                 `json:"session_title"`
	GeneratedAt  time.Time              `json:"generated_at"`
	Results      []promptAnalysisResult `json:"results"`
	Summary      analysisSummary        `json:"summary"`
}

type promptAnalysisResult struct {
	PromptID           string   `json:"prompt_id"`
	Prompt             string   `json:"prompt"`
	UserMessageID      string   `json:"user_message_id,omitempty"`
	AssistantMessageID string   `json:"assistant_message_id,omitempty"`
	Answer             string   `json:"answer"`
	Classification     string   `json:"classification"`
	MatchedFacts       []string `json:"matched_facts"`
	MissingFacts       []string `json:"missing_facts"`
	Contradictions     []string `json:"contradictions"`
	Investigation      string   `json:"investigation"`
	ToolCallCount      int      `json:"tool_call_count"`
}

type analysisSummary struct {
	ExactCount        int `json:"exact_count"`
	ReasonableCount   int `json:"reasonable_count"`
	UnacceptableCount int `json:"unacceptable_count"`
}
